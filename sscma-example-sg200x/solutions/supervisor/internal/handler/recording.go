package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"supervisor/internal/api"
	"supervisor/pkg/logger"
)

// RecordingHandler handles recording configuration API requests.
type RecordingHandler struct {
	configPath string
}

// RecordingConfig represents the recording configuration.
type RecordingConfig struct {
	Location string   `json:"location"` // "sd_card" or "local_storage"
	Mode     string   `json:"mode"`     // "motion", "constant", "scheduled"
	Schedule Schedule `json:"schedule"`
}

// Schedule represents the recording schedule.
type Schedule struct {
	Days      map[string]bool `json:"days"`
	StartTime string          `json:"start_time"` // HH:MM format
	EndTime   string          `json:"end_time"`   // HH:MM format
}

// NewRecordingHandler creates a new RecordingHandler.
func NewRecordingHandler() *RecordingHandler {
	return &RecordingHandler{
		configPath: "/etc/supervisor/recording.conf",
	}
}

// GetConfig returns the current recording configuration.
func (h *RecordingHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.loadConfig()
	if err != nil {
		// Return default config if file doesn't exist
		config = h.getDefaultConfig()
	}

	api.WriteSuccess(w, config)
}

// SetConfig updates the recording configuration.
func (h *RecordingHandler) SetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var config RecordingConfig
	if err := api.ParseJSONBody(r, &config); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	// Validate configuration
	if err := h.validateConfig(&config); err != nil {
		api.WriteError(w, -1, err.Error())
		return
	}

	// Save configuration
	if err := h.saveConfig(&config); err != nil {
		logger.Error("Failed to save recording config: %v", err)
		api.WriteError(w, -1, "Failed to save configuration")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{
		"status":  "success",
		"message": "Recording configuration saved",
	})
}

// loadConfig loads the recording configuration from file.
func (h *RecordingHandler) loadConfig() (*RecordingConfig, error) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return nil, err
	}

	var config RecordingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// saveConfig saves the recording configuration to file with file locking.
func (h *RecordingHandler) saveConfig(config *RecordingConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(h.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal data first
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Open file with exclusive lock
	file, err := os.OpenFile(h.configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// Write data
	_, err = file.Write(data)
	return err
}

// validateConfig validates the recording configuration.
func (h *RecordingHandler) validateConfig(config *RecordingConfig) error {
	// Validate location
	if config.Location != "sd_card" && config.Location != "local_storage" {
		return api.NewError("Invalid location. Must be 'sd_card' or 'local_storage'")
	}

	// Validate mode
	if config.Mode != "motion" && config.Mode != "constant" && config.Mode != "scheduled" {
		return api.NewError("Invalid mode. Must be 'motion', 'constant', or 'scheduled'")
	}

	// Validate schedule if mode is scheduled
	if config.Mode == "scheduled" {
		if config.Schedule.Days == nil {
			return api.NewError("Schedule days are required for scheduled mode")
		}

		// Validate at least one day is selected
		hasSelectedDay := false
		for _, selected := range config.Schedule.Days {
			if selected {
				hasSelectedDay = true
				break
			}
		}
		if !hasSelectedDay {
			return api.NewError("At least one day must be selected for scheduled mode")
		}

		// Validate time format (HH:MM)
		if !isValidTimeFormat(config.Schedule.StartTime) {
			return api.NewError("Invalid start time format. Use HH:MM")
		}
		if !isValidTimeFormat(config.Schedule.EndTime) {
			return api.NewError("Invalid end time format. Use HH:MM")
		}
	}

	return nil
}

// isValidTimeFormat checks if a time string is in HH:MM format.
func isValidTimeFormat(timeStr string) bool {
	if len(timeStr) != 5 {
		return false
	}
	if timeStr[2] != ':' {
		return false
	}

	// Parse hours using integer comparison
	hourStr := timeStr[0:2]
	hourInt, err := strconv.Atoi(hourStr)
	if err != nil || hourInt < 0 || hourInt > 23 {
		return false
	}

	// Parse minutes using integer comparison
	minuteStr := timeStr[3:5]
	minuteInt, err := strconv.Atoi(minuteStr)
	if err != nil || minuteInt < 0 || minuteInt > 59 {
		return false
	}

	return true
}

// getDefaultConfig returns the default recording configuration.
func (h *RecordingHandler) getDefaultConfig() *RecordingConfig {
	return &RecordingConfig{
		Location: "local_storage",
		Mode:     "motion",
		Schedule: Schedule{
			Days: map[string]bool{
				"sunday":    false,
				"monday":    true,
				"tuesday":   true,
				"wednesday": true,
				"thursday":  true,
				"friday":    true,
				"saturday":  false,
			},
			StartTime: "00:00",
			EndTime:   "00:00",
		},
	}
}
