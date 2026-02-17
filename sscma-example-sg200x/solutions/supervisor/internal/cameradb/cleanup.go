// Package cameradb provides cleanup functionality for the camera database.
package cameradb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"supervisor/pkg/logger"
)

// CleanupStatus contains information about database cleanup needs
type CleanupStatus struct {
	MissingRecordingFiles int    `json:"missing_recording_files"`
	OrphanedDetections    int    `json:"orphaned_detections"`
	HashMismatches        int    `json:"hash_mismatches"`
	WastedBytes           int64  `json:"wasted_bytes"`
	CleanupRecommended    bool   `json:"cleanup_recommended"`
	Reasons               []string `json:"reasons,omitempty"`
}

// CleanupOptions specifies what cleanup operations to perform
type CleanupOptions struct {
	RemoveMissingRecordings  bool `json:"remove_missing_recordings"`
	RemoveOrphanedDetections bool `json:"remove_orphaned_detections"`
	RemoveOrphanedImages     bool `json:"remove_orphaned_images"`
	VerifyHashes             bool `json:"verify_hashes"`
}

// CleanupResult contains the results of a cleanup operation
type CleanupResult struct {
	RecordingsRemoved int64 `json:"recordings_removed"`
	DetectionsRemoved int64 `json:"detections_removed"`
	ImagesDeleted     int64 `json:"images_deleted"`
	BytesFreed        int64 `json:"bytes_freed"`
}

// GetCleanupStatus returns the current cleanup status
func GetCleanupStatus() (*CleanupStatus, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	status := &CleanupStatus{}

	// Count recordings with missing files
	db.QueryRow(`SELECT COUNT(*) FROM recordings WHERE file_missing = 1`).Scan(&status.MissingRecordingFiles)

	// Count orphaned detections (image file deleted)
	db.QueryRow(`SELECT COUNT(*) FROM detections WHERE orphaned = 1`).Scan(&status.OrphanedDetections)

	// Determine if cleanup is recommended
	if status.MissingRecordingFiles > 10 || status.OrphanedDetections > 50 || status.HashMismatches > 0 {
		status.CleanupRecommended = true
	}

	// Build reasons
	if status.MissingRecordingFiles > 0 {
		status.Reasons = append(status.Reasons,
			fmt.Sprintf("%d recording(s) reference files that no longer exist", status.MissingRecordingFiles))
	}
	if status.OrphanedDetections > 0 {
		status.Reasons = append(status.Reasons,
			fmt.Sprintf("%d detection image(s) are missing", status.OrphanedDetections))
	}
	if status.HashMismatches > 0 {
		status.Reasons = append(status.Reasons,
			fmt.Sprintf("%d file(s) have been modified or replaced since recording", status.HashMismatches))
	}

	return status, nil
}

// RunCleanup performs cleanup operations based on the provided options
func RunCleanup(options CleanupOptions) (*CleanupResult, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	result := &CleanupResult{}

	if options.RemoveMissingRecordings {
		res, err := db.Exec(`DELETE FROM recordings WHERE file_missing = 1`)
		if err != nil {
			logger.Error("Failed to remove missing recordings: %v", err)
		} else {
			affected, _ := res.RowsAffected()
			result.RecordingsRemoved = affected
		}
	}

	if options.RemoveOrphanedDetections {
		res, err := db.Exec(`DELETE FROM detections WHERE orphaned = 1`)
		if err != nil {
			logger.Error("Failed to remove orphaned detections: %v", err)
		} else {
			affected, _ := res.RowsAffected()
			result.DetectionsRemoved = affected
		}
	}

	logger.Info("Cleanup complete: %d recordings, %d detections removed",
		result.RecordingsRemoved, result.DetectionsRemoved)

	return result, nil
}

// VerifyRecordingHash verifies a recording file's hash against the stored value
// Returns: 0=OK, 1=hash mismatch, 2=file missing, -1=error
func VerifyRecordingHash(recordingID int64) (int, error) {
	rec, err := GetRecording(recordingID)
	if err != nil {
		return -1, err
	}
	if rec == nil {
		return -1, fmt.Errorf("recording not found")
	}

	// Check if file exists
	if _, err := os.Stat(rec.Path); os.IsNotExist(err) {
		return 2, nil // File missing
	}

	// If no hash stored, we can't verify
	if rec.FileHash == "" {
		return 0, nil // OK (no hash to verify)
	}

	// Compute file hash
	actualHash, err := ComputeFileHash(rec.Path)
	if err != nil {
		return -1, fmt.Errorf("failed to compute hash: %w", err)
	}

	if actualHash != rec.FileHash {
		return 1, nil // Hash mismatch
	}

	return 0, nil // OK
}

// ComputeFileHash computes the SHA-256 hash of a file
func ComputeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ScanForMissingFiles checks all recordings and detections for missing files
// and marks them appropriately in the database
func ScanForMissingFiles() (int, int, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return 0, 0, fmt.Errorf("database not open")
	}

	missingRecordings := 0
	missingDetections := 0

	// Check recordings
	rows, err := db.Query(`SELECT id, path FROM recordings WHERE file_missing = 0`)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query recordings: %w", err)
	}
	defer rows.Close()

	var recordingsToMark []int64
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			recordingsToMark = append(recordingsToMark, id)
		}
	}
	rows.Close()

	for _, id := range recordingsToMark {
		if _, err := db.Exec(`UPDATE recordings SET file_missing = 1 WHERE id = ?`, id); err == nil {
			missingRecordings++
		}
	}

	// Check detection images
	rows, err = db.Query(`SELECT id, image_path FROM detections WHERE orphaned = 0 AND image_path IS NOT NULL AND image_path != ''`)
	if err != nil {
		return missingRecordings, 0, fmt.Errorf("failed to query detections: %w", err)
	}
	defer rows.Close()

	var detectionsToMark []int64
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			detectionsToMark = append(detectionsToMark, id)
		}
	}
	rows.Close()

	for _, id := range detectionsToMark {
		if _, err := db.Exec(`UPDATE detections SET orphaned = 1 WHERE id = ?`, id); err == nil {
			missingDetections++
		}
	}

	if missingRecordings > 0 || missingDetections > 0 {
		logger.Info("Missing file scan: %d recordings, %d detections marked as missing",
			missingRecordings, missingDetections)
	}

	return missingRecordings, missingDetections, nil
}
