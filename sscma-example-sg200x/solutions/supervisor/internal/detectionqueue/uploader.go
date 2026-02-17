// Package detectionqueue provides a persistent queue for detection uploads.
package detectionqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"supervisor/internal/device"
	"supervisor/internal/ntp"
	"supervisor/internal/timestamp"
	"supervisor/internal/tls"
	"supervisor/pkg/logger"
)

const (
	// Initial backoff duration
	initialBackoff = 1 * time.Second

	// Maximum backoff duration
	maxBackoff = 60 * time.Second

	// Idle poll interval when queue is empty
	idlePollInterval = 1 * time.Second

	// NTP sync check interval
	ntpCheckInterval = 5 * time.Second
)

// Uploader handles uploading detection items to the platform API
type Uploader struct {
	queue      *Queue
	httpClient *http.Client
	apiURL     string
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
	ntpManager *ntp.Manager
}

// Stats contains upload statistics
type Stats struct {
	QueueSize      int   `json:"queue_size"`
	TotalUploaded  int64 `json:"total_uploaded"`
	TotalFailed    int64 `json:"total_failed"`
	LastUploadTime int64 `json:"last_upload_time"`
}

// QueueItemInfo is a simplified view of a queued detection for the API
type QueueItemInfo struct {
	ID              string  `json:"id"`
	ClassLabel      string  `json:"class_label"`
	ConfidenceScore float64 `json:"confidence_score"`
	Timestamp       int64   `json:"timestamp"`
	RetryCount      int     `json:"retry_count"`
	TSAStatus       string  `json:"tsa_status"`
	ImagePath       string  `json:"image_path"`
}

var (
	uploaderInstance *Uploader
	uploaderOnce     sync.Once
	stats            = Stats{}
	statsMu          sync.Mutex
)

// GetUploader returns the singleton uploader instance
func GetUploader() *Uploader {
	uploaderOnce.Do(func() {
		uploaderInstance = &Uploader{
			queue:      NewQueue(),
			httpClient: tls.PlatformHTTPClient(30 * time.Second),
			stopCh:     make(chan struct{}),
		}
	})
	return uploaderInstance
}

// Start begins the upload worker
func (u *Uploader) Start(ctx context.Context) {
	u.mu.Lock()
	if u.running {
		u.mu.Unlock()
		return
	}
	u.running = true
	u.mu.Unlock()

	logger.Info("Detection uploader started")

	u.wg.Add(1)
	go u.run(ctx)
}

// Stop gracefully stops the uploader
func (u *Uploader) Stop() {
	u.mu.Lock()
	if !u.running {
		u.mu.Unlock()
		return
	}
	u.running = false
	u.mu.Unlock()

	close(u.stopCh)
	u.wg.Wait()
	logger.Info("Detection uploader stopped")
}

// SetNTPManager sets the NTP manager for time sync checking
func (u *Uploader) SetNTPManager(mgr *ntp.Manager) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ntpManager = mgr
}

// Enqueue adds a detection to the queue
func (u *Uploader) Enqueue(item DetectionItem) error {
	return u.queue.Enqueue(item)
}

// GetStats returns upload statistics
func (u *Uploader) GetStats() Stats {
	statsMu.Lock()
	defer statsMu.Unlock()

	s := stats
	s.QueueSize = u.queue.Size()
	return s
}

// GetQueueItems returns all items currently in the queue
func (u *Uploader) GetQueueItems() []QueueItemInfo {
	items := u.queue.GetItems()
	result := make([]QueueItemInfo, len(items))

	for i, item := range items {
		result[i] = QueueItemInfo{
			ID:              item.ID,
			ClassLabel:      item.ClassLabel,
			ConfidenceScore: item.ConfidenceScore,
			Timestamp:       item.Timestamp.Unix(),
			RetryCount:      item.RetryCount,
			TSAStatus:       item.TSAStatus,
			ImagePath:       item.ImagePath,
		}
	}

	return result
}

// run is the main upload loop
func (u *Uploader) run(ctx context.Context) {
	defer u.wg.Done()

	backoff := initialBackoff
	ntpWaitLogged := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-u.stopCh:
			return
		default:
		}

		// Wait for NTP sync before processing uploads
		// This prevents uploading detections with incorrect timestamps (e.g., year 2082)
		u.mu.Lock()
		ntpMgr := u.ntpManager
		u.mu.Unlock()

		if ntpMgr != nil && !ntpMgr.IsSynced() {
			if !ntpWaitLogged {
				logger.Info("Detection uploader waiting for NTP sync...")
				ntpWaitLogged = true
			}
			select {
			case <-time.After(ntpCheckInterval):
				continue
			case <-ctx.Done():
				return
			case <-u.stopCh:
				return
			}
		}

		// NTP is synced (or no manager configured), reset the log flag
		if ntpWaitLogged {
			logger.Info("NTP synced, detection uploader resuming")
			ntpWaitLogged = false
		}

		// Get next item
		item, ok := u.queue.Peek()
		if !ok {
			// Queue empty, wait before checking again
			select {
			case <-time.After(idlePollInterval):
			case <-ctx.Done():
				return
			case <-u.stopCh:
				return
			}
			continue
		}

		// Request TSA timestamp if pending (non-blocking with timeout)
		// This happens before upload so the token is included if available
		if item.TSAStatus == "pending" && timestamp.Enabled() {
			u.requestTSAToken(item)
		}

		// Try to upload
		err := u.upload(item)
		if err != nil {
			logger.Warning("Detection upload failed: %v (retry #%d)", err, item.RetryCount)

			// Update retry count (this also persists in-memory items to disk)
			u.queue.UpdateRetryCount()

			// Increment failed counter
			statsMu.Lock()
			stats.TotalFailed++
			statsMu.Unlock()

			// Exponential backoff
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			case <-u.stopCh:
				return
			}
			continue
		}

		// Success - remove from queue and clean up
		dequeued, _ := u.queue.Dequeue()
		u.queue.CleanupItem(dequeued)

		// Reset backoff
		backoff = initialBackoff

		// Update stats
		statsMu.Lock()
		stats.TotalUploaded++
		stats.LastUploadTime = time.Now().Unix()
		statsMu.Unlock()

		logger.Info("Detection uploaded successfully: %s", item.ID)
	}
}

// upload sends a detection item to the platform API
func (u *Uploader) upload(item *DetectionItem) error {
	// Get API key from platform info
	apiKey, err := getAPIKey()
	if err != nil {
		return fmt.Errorf("no API key configured: %w", err)
	}

	// Get platform URL
	apiURL := getPlatformURL()

	// Get image data - prefer in-memory, fall back to disk
	var imageData []byte
	if len(item.ImageData) > 0 {
		// Use in-memory data
		imageData = item.ImageData
		logger.Debug("Using in-memory image data (%d bytes)", len(imageData))
	} else if item.ImagePath != "" {
		// Fall back to reading from disk (for items loaded after restart)
		var err error
		imageData, err = os.ReadFile(item.ImagePath)
		if err != nil {
			return fmt.Errorf("failed to read image: %w", err)
		}
		logger.Debug("Loaded image from disk: %s (%d bytes)", item.ImagePath, len(imageData))
	} else {
		return fmt.Errorf("no image data or path available")
	}

	// Debug: log what we're uploading
	isJPEG := len(imageData) >= 2 && imageData[0] == 0xFF && imageData[1] == 0xD8
	logger.Info("DEBUG upload: size=%d isJPEG=%v class=%s conf=%.2f",
		len(imageData), isJPEG, item.ClassLabel, item.ConfidenceScore)
	if len(imageData) >= 4 {
		logger.Info("DEBUG upload: magic=0x%02x%02x%02x%02x",
			imageData[0], imageData[1], imageData[2], imageData[3])
	}

	// Build multipart request
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add image file
	part, err := writer.CreateFormFile("image", "detection.jpg")
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return fmt.Errorf("failed to write image data: %w", err)
	}

	// Add detection fields
	if item.BoundingBox != "" {
		writer.WriteField("bounding_box", item.BoundingBox)
	}
	if item.ClassLabel != "" {
		writer.WriteField("class_label", item.ClassLabel)
	}
	// ConfidenceScore is already 0-100 percentage (converted in processDetection)
	writer.WriteField("confidence_score", fmt.Sprintf("%.2f", item.ConfidenceScore))

	// Add full detections array if present
	if item.Detections != "" {
		writer.WriteField("detections", item.Detections)
	}

	// Add timestamp (capture time from camera)
	writer.WriteField("timestamp", item.Timestamp.Format(time.RFC3339))

	// Add trusted timestamp chain fields (for legal evidence)
	if item.FrameHash != "" {
		writer.WriteField("frame_hash", item.FrameHash)
	}
	if item.TimestampToken != "" {
		writer.WriteField("timestamp_token", item.TimestampToken)
	}
	// Indicate whether TSA verification succeeded
	if item.TSAStatus == "signed" {
		writer.WriteField("timestamp_verified", "true")
	} else {
		writer.WriteField("timestamp_verified", "false")
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL+"/api/v1/cameras/detection/", &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("TPR-API-KEY", apiKey)
	req.Header.Set("User-Agent", "recamera-supervisor")

	// Send request
	logger.Info("DEBUG upload: sending to %s", apiURL+"/api/v1/cameras/detection/")
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error messages
	body, _ := io.ReadAll(resp.Body)

	logger.Info("DEBUG upload: response status=%d body=%s", resp.StatusCode, string(body))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// requestTSAToken requests a timestamp token from FreeTSA for the detection.
// This is called before upload and has a timeout to avoid blocking.
// If TSA fails, the upload proceeds without a token.
func (u *Uploader) requestTSAToken(item *DetectionItem) {
	if item.FrameHash == "" || item.TSAStatus != "pending" {
		return
	}

	// Create TSA client
	client, err := timestamp.NewClient()
	if err != nil {
		logger.Warning("Failed to create TSA client: %v", err)
		item.TSAStatus = "failed"
		return
	}

	// Create combined hash: frame_hash + capture_timestamp + detections
	hash, err := timestamp.HashDetection(item.FrameHash, item.Timestamp, item.Detections)
	if err != nil {
		logger.Warning("Failed to hash detection for TSA: %v", err)
		item.TSAStatus = "failed"
		return
	}

	// Request timestamp from FreeTSA (client has 30s timeout)
	token, err := client.RequestTimestamp(hash)
	if err != nil {
		logger.Warning("TSA timestamp request failed for %s: %v", item.ID, err)
		item.TSAStatus = "failed"
		return
	}

	// Store the token (base64 encoded)
	item.TimestampToken = timestamp.TokenToBase64(token)
	item.TSAStatus = "signed"

	logger.Debug("TSA timestamp obtained for detection %s (%d bytes)", item.ID, len(token))
}

// getAPIKey retrieves the API key from platform info
func getAPIKey() (string, error) {
	platformInfo := device.GetPlatformInfo()
	if platformInfo == "" {
		return "", fmt.Errorf("platform info not found")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(platformInfo), &data); err != nil {
		return "", fmt.Errorf("failed to parse platform info: %w", err)
	}

	// Try secret_key first, then api_key for backwards compatibility
	if key, ok := data["secret_key"].(string); ok && key != "" {
		return key, nil
	}
	if key, ok := data["api_key"].(string); ok && key != "" {
		return key, nil
	}

	return "", fmt.Errorf("no API key in platform info")
}

// getPlatformURL gets the platform API base URL
func getPlatformURL() string {
	platformInfo := device.GetPlatformInfo()
	if platformInfo == "" {
		return "https://dev.thepolicerecord.com"
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(platformInfo), &data); err != nil {
		return "https://dev.thepolicerecord.com"
	}

	if url, ok := data["platform_url"].(string); ok && url != "" {
		return strings.TrimRight(url, "/")
	}

	return "https://dev.thepolicerecord.com"
}
