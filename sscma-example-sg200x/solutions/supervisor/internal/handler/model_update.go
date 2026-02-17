package handler

import (
	"context"
	"time"

	"supervisor/internal/modelupdate"
)

// getModelUpdateStatus returns the current model update status.
func getModelUpdateStatus() map[string]interface{} {
	mgr := modelupdate.GetManager()
	status := mgr.GetStatus()

	// Format times for JSON
	var lastCheckStr, lastUpdateStr string
	if !status.LastCheck.IsZero() {
		lastCheckStr = status.LastCheck.Format(time.RFC3339)
	}
	if !status.LastUpdate.IsZero() {
		lastUpdateStr = status.LastUpdate.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"last_check":        lastCheckStr,
		"last_update":       lastUpdateStr,
		"current_version":   status.CurrentVersion,
		"latest_version":    status.LatestVersion,
		"update_available":  status.UpdateAvailable,
		"is_downloading":    status.IsDownloading,
		"download_progress": status.DownloadProgress,
		"last_error":        status.LastError,
		"model_file":        status.ModelFile,
	}
}

// triggerModelUpdateCheck triggers an immediate model update check.
func triggerModelUpdateCheck() error {
	mgr := modelupdate.GetManager()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run check in background and return immediately
	go func() {
		mgr.CheckNow(ctx)
	}()

	return nil
}
