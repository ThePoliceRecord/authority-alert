// Package handler provides HTTP handlers for the supervisor API.
package handler

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"supervisor/internal/cameradb"
	"supervisor/internal/detectionqueue"
	"supervisor/internal/security"
	"supervisor/pkg/logger"
)

// DetectionWsPort is the port for the detection WebSocket server
const DetectionWsPort = 8089

// WebSocket upgrader with no origin check (localhost only)
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  256 * 1024, // 256KB for large JPEG payloads
	WriteBufferSize: 4 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Accept all origins (localhost only anyway)
	},
}

// DetectionMessage is the JSON message format from camera-detector
type DetectionMessage struct {
	Type        string             `json:"type"`
	TimestampMs uint64             `json:"timestamp_ms"`
	FrameHash   string             `json:"frame_hash,omitempty"`
	ImageBase64 string             `json:"image_base64"`
	Detections  []DetectionObject  `json:"detections"`
}

// DetectionObject represents a single detection in the message
type DetectionObject struct {
	ClassID    int       `json:"class_id"`
	ClassLabel string    `json:"class_label"`
	Confidence float64   `json:"confidence"`
	BBox       []float64 `json:"bbox"` // [x, y, w, h] normalized 0-1
}

// HandleDetectionWebSocket handles WebSocket connections from camera-detector
func HandleDetectionWebSocket(w http.ResponseWriter, r *http.Request) {
	logger.Info("Detection WS: ========================================")
	logger.Info("Detection WS: Incoming connection from %s", r.RemoteAddr)

	// Verify request is from localhost
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.Warning("Detection WS: invalid remote addr: %s", r.RemoteAddr)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	ip := net.ParseIP(host)
	if ip == nil || (!ip.IsLoopback() && !ip.Equal(net.IPv4(127, 0, 0, 1))) {
		logger.Warning("Detection WS: REJECTED non-localhost connection: %s", host)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	logger.Info("Detection WS: Localhost verified, upgrading to WebSocket...")

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Detection WS: UPGRADE FAILED: %v", err)
		return
	}
	defer conn.Close()

	logger.Info("Detection WS: *** CLIENT CONNECTED ***")
	logger.Info("Detection WS: Ready to receive detections from camera-detector")

	// Get the uploader instance
	uploader := detectionqueue.GetUploader()
	msgCount := 0

	// Read messages in a loop
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("Detection WS: READ ERROR: %v", err)
			} else {
				logger.Info("Detection WS: Client disconnected (received %d messages)", msgCount)
			}
			return
		}

		msgCount++
		logger.Info("Detection WS: Received message #%d (type=%d, size=%d bytes)",
			msgCount, messageType, len(message))

		if messageType != websocket.TextMessage {
			logger.Warning("Detection WS: Ignoring non-text message (type=%d)", messageType)
			continue
		}

		// Parse the detection message
		if err := processDetectionMessage(message, uploader); err != nil {
			logger.Error("Detection WS: PROCESS ERROR: %v", err)
			// Send error response
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+err.Error()+`"}`))
		} else {
			// Send success acknowledgement
			conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"ok"}`))
		}
	}
}

// processDetectionMessage parses and queues a detection message
func processDetectionMessage(data []byte, uploader *detectionqueue.Uploader) error {
	logger.Info("Detection WS: Parsing message (%d bytes)...", len(data))

	var msg DetectionMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error("Detection WS: JSON parse failed: %v", err)
		// Log first 200 chars of message for debugging
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		logger.Error("Detection WS: Message preview: %s", preview)
		return err
	}

	logger.Info("Detection WS: Message type=%s, timestamp=%d, detections=%d, image_base64_len=%d",
		msg.Type, msg.TimestampMs, len(msg.Detections), len(msg.ImageBase64))

	if msg.Type != "detection" {
		logger.Warning("Detection WS: Ignoring non-detection message type: %s", msg.Type)
		return nil
	}

	if len(msg.Detections) == 0 {
		logger.Warning("Detection WS: Message has no detections, skipping")
		return nil
	}

	// Log each detection
	for i, det := range msg.Detections {
		logger.Info("Detection WS:   [%d] class=%s (id=%d) conf=%.4f bbox=%v",
			i, det.ClassLabel, det.ClassID, det.Confidence, det.BBox)
	}

	// Decode base64 JPEG image
	logger.Info("Detection WS: Decoding base64 image (%d chars)...", len(msg.ImageBase64))
	imageData, err := base64.StdEncoding.DecodeString(msg.ImageBase64)
	if err != nil {
		logger.Error("Detection WS: Base64 decode failed: %v", err)
		return err
	}
	logger.Info("Detection WS: Decoded JPEG: %d bytes", len(imageData))

	// Validate JPEG magic bytes
	if len(imageData) < 2 {
		logger.Error("Detection WS: Image data too small: %d bytes", len(imageData))
	} else if imageData[0] != 0xFF || imageData[1] != 0xD8 {
		logger.Warning("Detection WS: Invalid JPEG magic bytes: %02x %02x (expected FF D8)",
			imageData[0], imageData[1])
	} else {
		logger.Info("Detection WS: JPEG magic bytes OK (FF D8)")
	}

	// Generate unique ID
	id := generateDetectionID()

	// Convert detections for API (confidence already 0-1, convert to percentage)
	detectionsForAPI := make([]map[string]interface{}, len(msg.Detections))
	for i, det := range msg.Detections {
		detectionsForAPI[i] = map[string]interface{}{
			"class_id":    det.ClassID,
			"class_label": det.ClassLabel,
			"confidence":  det.Confidence * 100, // Convert to percentage
			"bbox":        det.BBox,
		}
	}
	detectionsJSON, _ := json.Marshal(detectionsForAPI)

	// Get primary detection (first one)
	var primaryClass string
	var primaryConfidence float64
	var primaryBBox string
	if len(msg.Detections) > 0 {
		det := msg.Detections[0]
		primaryClass = det.ClassLabel
		primaryConfidence = det.Confidence * 100 // Convert to percentage
		bboxJSON, _ := json.Marshal(det.BBox)
		primaryBBox = string(bboxJSON)
	}

	// Determine capture time - validate timestamp is reasonable
	const (
		minValidMs = 946684800000  // 2000-01-01
		maxValidMs = 4102444800000 // 2100-01-01
	)
	var captureTime time.Time
	if msg.TimestampMs >= minValidMs && msg.TimestampMs <= maxValidMs {
		captureTime = time.UnixMilli(int64(msg.TimestampMs))
	} else {
		captureTime = time.Now()
		if msg.TimestampMs != 0 {
			logger.Debug("Detection WS: invalid timestamp %d, using current time", msg.TimestampMs)
		}
	}

	// Determine TSA status
	tsaStatus := "skipped"
	frameHash := ""
	if security.EvidenceChainEnabled() && msg.FrameHash != "" {
		// Validate frame hash is valid hex
		if _, err := hex.DecodeString(msg.FrameHash); err == nil {
			frameHash = msg.FrameHash
			tsaStatus = "pending"
		}
	}

	// Create queue item with in-memory image data
	item := detectionqueue.DetectionItem{
		ID:              id,
		ImageData:       imageData, // In-memory JPEG
		ImagePath:       "",        // No disk path initially
		BoundingBox:     primaryBBox,
		ClassLabel:      primaryClass,
		ConfidenceScore: primaryConfidence,
		Detections:      string(detectionsJSON),
		Timestamp:       captureTime,
		RetryCount:      0,
		FrameHash:       frameHash,
		TSAStatus:       tsaStatus,
	}

	// Enqueue for upload
	if err := uploader.Enqueue(item); err != nil {
		return err
	}

	// Write detections to cameradb for correlation with recordings
	frameTsMs := int64(msg.TimestampMs)
	if frameTsMs == 0 {
		frameTsMs = captureTime.UnixMilli()
	}

	// Find recording that contains this timestamp
	var recordingID int64
	if rec, err := cameradb.FindRecordingForTimestamp(frameTsMs); err == nil && rec != nil {
		recordingID = rec.ID
	}

	// Insert each detection into the database
	for _, det := range msg.Detections {
		bbox := det.BBox
		if len(bbox) < 4 {
			bbox = []float64{0, 0, 0, 0}
		}
		dbDet := &cameradb.Detection{
			FrameTsMs:   frameTsMs,
			ClassID:     det.ClassID,
			ClassLabel:  det.ClassLabel,
			Confidence:  det.Confidence,
			BboxX:       bbox[0],
			BboxY:       bbox[1],
			BboxW:       bbox[2],
			BboxH:       bbox[3],
			ImagePath:   "", // No disk path for WS detections
			RecordingID: recordingID,
		}
		if _, err := cameradb.InsertDetection(dbDet); err != nil {
			logger.Warning("Detection WS: failed to insert to DB: %v", err)
		}
	}

	logger.Info("Detection WS: queued %s class=%s conf=%.1f%% size=%d",
		id, primaryClass, primaryConfidence, len(imageData))

	return nil
}

