// Package handler provides HTTP handlers for the supervisor API.
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"supervisor/internal/api"
	"supervisor/internal/detectionqueue"
	"supervisor/internal/security"
	"supervisor/pkg/logger"
)

const (
	// Maximum detection image size (10MB)
	maxDetectionImageSize = 10 << 20

	// Detection queue directory
	detectionQueueDir = "/userdata/detection_queue"
)

// DetectionHandler handles detection uploads from camera-detector
type DetectionHandler struct {
	uploader *detectionqueue.Uploader
}

// NewDetectionHandler creates a new detection handler
func NewDetectionHandler() *DetectionHandler {
	return &DetectionHandler{
		uploader: detectionqueue.GetUploader(),
	}
}

// HandleDetection receives detection data from camera-detector and queues it for upload
func (h *DetectionHandler) HandleDetection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	// Parse multipart form with size limit
	if err := r.ParseMultipartForm(maxDetectionImageSize); err != nil {
		logger.Warning("Failed to parse detection form: %v", err)
		api.WriteError(w, -1, "Failed to parse form data")
		return
	}

	// Extract image file
	file, header, err := r.FormFile("image")
	if err != nil {
		logger.Warning("Detection missing image: %v", err)
		api.WriteError(w, -1, "Image file required")
		return
	}
	defer file.Close()

	// Read image data
	imageData, err := io.ReadAll(file)
	if err != nil {
		logger.Warning("Failed to read detection image: %v", err)
		api.WriteError(w, -1, "Failed to read image")
		return
	}

	// Generate unique ID
	id := generateDetectionID()

	// Ensure queue directory exists
	if err := os.MkdirAll(detectionQueueDir, 0755); err != nil {
		logger.Error("Failed to create detection queue dir: %v", err)
		api.WriteError(w, -1, "Internal error")
		return
	}

	// Save image to disk
	imagePath := filepath.Join(detectionQueueDir, id+".jpg")
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		logger.Error("Failed to save detection image: %v", err)
		api.WriteError(w, -1, "Failed to save image")
		return
	}

	// Extract detection metadata from form
	boundingBox := r.FormValue("bounding_box")
	classLabel := r.FormValue("class_label")
	confidenceStr := r.FormValue("confidence_score")
	detectionsJSON := r.FormValue("detections")

	// Extract frame evidence for trusted timestamps
	frameHash := r.FormValue("frame_hash")
	frameTimestampMs := r.FormValue("frame_timestamp_ms")

	// Parse confidence score
	confidence := 0.0
	if confidenceStr != "" {
		if parsed, err := strconv.ParseFloat(confidenceStr, 64); err == nil {
			confidence = parsed
		}
	}

	// Determine capture time: use camera timestamp if valid, otherwise fallback to now
	var captureTime time.Time
	if ts, err := strconv.ParseInt(frameTimestampMs, 10, 64); err == nil && ts > 0 {
		captureTime = time.UnixMilli(ts)
	} else {
		captureTime = time.Now()
	}

	// Determine TSA status based on frame evidence availability and security config
	tsaStatus := "skipped"
	includeFrameHash := ""

	// Only include evidence chain data if enabled in security config
	if security.EvidenceChainEnabled() {
		includeFrameHash = frameHash
		if frameHash != "" && len(frameHash) == 64 && frameTimestampMs != "" {
			tsaStatus = "pending"
		}
	}

	// Create queue item with trusted timestamp fields
	item := detectionqueue.DetectionItem{
		ID:              id,
		ImagePath:       imagePath,
		BoundingBox:     boundingBox,
		ClassLabel:      classLabel,
		ConfidenceScore: confidence,
		Detections:      detectionsJSON,
		Timestamp:       captureTime, // Use capture time from camera, not receive time
		RetryCount:      0,
		FrameHash:       includeFrameHash,
		TSAStatus:       tsaStatus,
	}

	// TSA timestamps are requested by the uploader (not here) to avoid blocking the HTTP response
	// The uploader will check TSAStatus=="pending" and request tokens before uploading

	// Enqueue for upload
	if err := h.uploader.Enqueue(item); err != nil {
		logger.Error("Failed to enqueue detection: %v", err)
		// Clean up saved image
		os.Remove(imagePath)
		api.WriteError(w, -1, "Failed to queue detection")
		return
	}

	logger.Debug("Detection queued: id=%s, class=%s, confidence=%.2f, image=%s (%d bytes), tsa=%s",
		id, classLabel, confidence, header.Filename, len(imageData), item.TSAStatus)

	// Return accepted response
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "queued",
		"id":      id,
		"message": "Detection queued for upload",
	})
}

// GetDetectionStats returns detection upload statistics
func (h *DetectionHandler) GetDetectionStats(w http.ResponseWriter, r *http.Request) {
	stats := h.uploader.GetStats()
	api.WriteSuccess(w, stats)
}

// GetDetectionQueue returns all items in the upload queue
func (h *DetectionHandler) GetDetectionQueue(w http.ResponseWriter, r *http.Request) {
	items := h.uploader.GetQueueItems()
	api.WriteSuccess(w, map[string]interface{}{
		"items": items,
		"count": len(items),
	})
}

// GetDetectionImage serves a detection image from the queue
func (h *DetectionHandler) GetDetectionImage(w http.ResponseWriter, r *http.Request) {
	// Get image ID from query parameter
	imageID := r.URL.Query().Get("id")
	if imageID == "" {
		api.WriteError(w, -1, "Image ID required")
		return
	}

	// Security: only allow alphanumeric, underscore, and hyphen in ID
	for _, c := range imageID {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			api.WriteError(w, -1, "Invalid image ID")
			return
		}
	}

	// Construct path (only allow files in the queue directory)
	imagePath := filepath.Join(detectionQueueDir, imageID+".jpg")
	cleanPath := filepath.Clean(imagePath)

	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		api.WriteError(w, -1, "Image not found")
		return
	}

	// Serve the file
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=60")
	http.ServeFile(w, r, cleanPath)
}

// generateDetectionID generates a unique detection ID
func generateDetectionID() string {
	return fmt.Sprintf("det_%d", time.Now().UnixNano())
}

