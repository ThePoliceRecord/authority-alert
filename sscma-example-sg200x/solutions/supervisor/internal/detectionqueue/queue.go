// Package detectionqueue provides a persistent queue for detection uploads.
package detectionqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"supervisor/pkg/logger"
)

const (
	// QueueDir is the directory where queued detections are stored
	QueueDir = "/userdata/detection_queue"

	// MaxQueueSize is the maximum number of items in the queue
	MaxQueueSize = 1000

	// MetadataFile stores the queue metadata
	MetadataFile = "queue.json"
)

// DetectionItem represents a queued detection with trusted timestamp support
type DetectionItem struct {
	ID              string    `json:"id"`
	ImageData       []byte    `json:"-"`                  // In-memory JPEG data (not serialized to disk)
	ImagePath       string    `json:"image_path"`         // Only set if persisted to disk
	BoundingBox     string    `json:"bounding_box"`       // JSON array: [x1,y1,x2,y2]
	ClassLabel      string    `json:"class_label"`
	ConfidenceScore float64   `json:"confidence_score"`
	Detections      string    `json:"detections"`         // Full JSON array of all detections
	Timestamp       time.Time `json:"timestamp"`          // Capture time from camera (not receive time)
	RetryCount      int       `json:"retry_count"`

	// Trusted timestamp chain fields (for legal evidence)
	FrameHash      string `json:"frame_hash,omitempty"`      // SHA256 of raw frame data (64 hex chars)
	TimestampToken string `json:"timestamp_token,omitempty"` // RFC 3161 TSA token (base64 DER)
	TSAStatus      string `json:"tsa_status,omitempty"`      // pending|signed|failed|skipped
}

// Queue is a persistent queue for detection items
type Queue struct {
	mu       sync.Mutex
	items    []DetectionItem
	queueDir string
}

// queueMetadata is the persisted queue state
type queueMetadata struct {
	Items []DetectionItem `json:"items"`
}

// NewQueue creates a new detection queue
func NewQueue() *Queue {
	q := &Queue{
		queueDir: QueueDir,
		items:    make([]DetectionItem, 0),
	}

	// Ensure directory exists
	if err := os.MkdirAll(QueueDir, 0755); err != nil {
		logger.Error("Failed to create queue directory: %v", err)
	}

	// Load existing queue from disk
	q.loadFromDisk()

	return q
}

// Enqueue adds a detection item to the queue
func (q *Queue) Enqueue(item DetectionItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Drop oldest if queue is full
	if len(q.items) >= MaxQueueSize {
		oldest := q.items[0]
		q.items = q.items[1:]
		// Clean up oldest image file
		if oldest.ImagePath != "" {
			os.Remove(oldest.ImagePath)
		}
		logger.Warning("Queue full, dropped oldest detection: %s", oldest.ID)
	}

	// Add new item
	q.items = append(q.items, item)

	// Persist to disk
	return q.saveToDisk()
}

// Peek returns the first item without removing it
func (q *Queue) Peek() (*DetectionItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil, false
	}

	item := q.items[0]
	return &item, true
}

// Dequeue removes and returns the first item
func (q *Queue) Dequeue() (*DetectionItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil, false
	}

	item := q.items[0]
	q.items = q.items[1:]

	// Persist to disk
	q.saveToDisk()

	return &item, true
}

// UpdateRetryCount increments the retry count for the first item
func (q *Queue) UpdateRetryCount() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) > 0 {
		q.items[0].RetryCount++
		q.saveToDisk()
	}
}

// Size returns the number of items in the queue
func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// loadFromDisk loads the queue from persistent storage
func (q *Queue) loadFromDisk() {
	metaPath := filepath.Join(q.queueDir, MetadataFile)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("Failed to read queue metadata: %v", err)
		}
		return
	}

	var meta queueMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		logger.Error("Failed to parse queue metadata: %v", err)
		return
	}

	// Validate that image files still exist
	validItems := make([]DetectionItem, 0, len(meta.Items))
	for _, item := range meta.Items {
		if item.ImagePath == "" {
			continue
		}
		if _, err := os.Stat(item.ImagePath); err == nil {
			validItems = append(validItems, item)
		} else {
			logger.Warning("Queue item %s has missing image, skipping", item.ID)
		}
	}

	q.items = validItems
	logger.Info("Loaded %d items from detection queue", len(q.items))
}

// saveToDisk persists the queue to disk
// For items with in-memory ImageData, writes the image to disk first
func (q *Queue) saveToDisk() error {
	metaPath := filepath.Join(q.queueDir, MetadataFile)

	// Before saving, ensure all items with ImageData have it written to disk
	for i := range q.items {
		if len(q.items[i].ImageData) > 0 && q.items[i].ImagePath == "" {
			// Write image to disk
			imagePath := filepath.Join(q.queueDir, fmt.Sprintf("det_%s.jpg", q.items[i].ID))
			if err := os.WriteFile(imagePath, q.items[i].ImageData, 0644); err != nil {
				logger.Warning("Failed to persist image for %s: %v", q.items[i].ID, err)
				continue
			}
			q.items[i].ImagePath = imagePath
			logger.Debug("Persisted image to %s (%d bytes)", imagePath, len(q.items[i].ImageData))
		}
	}

	meta := queueMetadata{Items: q.items}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}

	// Write atomically
	tmpPath := metaPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue: %w", err)
	}

	if err := os.Rename(tmpPath, metaPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename queue file: %w", err)
	}

	return nil
}

// CleanupItem removes the image file for an item
func (q *Queue) CleanupItem(item *DetectionItem) {
	if item != nil && item.ImagePath != "" {
		os.Remove(item.ImagePath)
	}
}

// Clear removes all items from the queue
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Clean up all image files
	for _, item := range q.items {
		if item.ImagePath != "" {
			os.Remove(item.ImagePath)
		}
	}

	q.items = make([]DetectionItem, 0)
	q.saveToDisk()
}

// GetItems returns a copy of all items in the queue
func (q *Queue) GetItems() []DetectionItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Return a copy to prevent race conditions
	items := make([]DetectionItem, len(q.items))
	copy(items, q.items)
	return items
}
