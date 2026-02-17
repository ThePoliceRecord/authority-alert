// Package handler provides HTTP handlers for storage management.
package handler

import (
	"encoding/json"
	"net/http"

	"supervisor/internal/api"
	"supervisor/internal/cameradb"
	"supervisor/pkg/logger"
)

// StorageHandler handles storage and database management endpoints
type StorageHandler struct{}

// NewStorageHandler creates a new storage handler
func NewStorageHandler() *StorageHandler {
	return &StorageHandler{}
}

// GetCleanupStatus returns the current cleanup status
func (h *StorageHandler) GetCleanupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	status, err := cameradb.GetCleanupStatus()
	if err != nil {
		logger.Error("Failed to get cleanup status: %v", err)
		api.WriteError(w, -1, "Failed to get cleanup status")
		return
	}

	api.WriteSuccess(w, status)
}

// RunCleanup performs cleanup operations
func (h *StorageHandler) RunCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var options cameradb.CleanupOptions
	if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	result, err := cameradb.RunCleanup(options)
	if err != nil {
		logger.Error("Failed to run cleanup: %v", err)
		api.WriteError(w, -1, "Failed to run cleanup")
		return
	}

	api.WriteSuccess(w, result)
}

// GetDatabaseStats returns database statistics
func (h *StorageHandler) GetDatabaseStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	stats, err := cameradb.GetStats()
	if err != nil {
		logger.Error("Failed to get database stats: %v", err)
		api.WriteError(w, -1, "Failed to get database stats")
		return
	}

	api.WriteSuccess(w, stats)
}

// ScanMissingFiles triggers a scan for missing files
func (h *StorageHandler) ScanMissingFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	missingRecordings, missingDetections, err := cameradb.ScanForMissingFiles()
	if err != nil {
		logger.Error("Failed to scan for missing files: %v", err)
		api.WriteError(w, -1, "Failed to scan for missing files")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{
		"missing_recordings":  missingRecordings,
		"missing_detections": missingDetections,
	})
}
