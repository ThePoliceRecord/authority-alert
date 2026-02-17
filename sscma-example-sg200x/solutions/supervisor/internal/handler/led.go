package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"supervisor/internal/api"
	"supervisor/pkg/logger"
)

// LEDHandler handles LED control API requests.
type LEDHandler struct {
	ledBasePath string
}

// LEDInfo represents LED information.
type LEDInfo struct {
	Name          string `json:"name"`
	Brightness    int    `json:"brightness"`
	MaxBrightness int    `json:"max_brightness"`
	Trigger       string `json:"trigger"`
}

// NewLEDHandler creates a new LEDHandler.
func NewLEDHandler() *LEDHandler {
	return &LEDHandler{
		ledBasePath: "/sys/class/leds",
	}
}

// getSafeLEDPath returns a sanitized LED path that is guaranteed to be within /sys/class/leds.
// This function prevents path traversal by using filepath.Rel to verify the path boundary.
func (h *LEDHandler) getSafeLEDPath(name string) (string, error) {
	// Sanitize name to prevent path traversal
	// filepath.Base returns the last element, stripping any directory components
	safeName := filepath.Base(name)

	// Require non-empty name
	if safeName == "" || safeName == "." || safeName == ".." {
		return "", filepath.ErrBadPattern
	}

	// Construct the LED path
	ledPath := filepath.Join(h.ledBasePath, safeName)

	// Resolve to absolute paths for verification
	baseDirAbs, err := filepath.Abs(h.ledBasePath)
	if err != nil {
		return "", err
	}

	ledPathAbs, err := filepath.Abs(ledPath)
	if err != nil {
		return "", err
	}

	// Use filepath.Rel to verify the path is within base directory
	// This is the robust, CodeQL-recognized way to prevent path traversal
	rel, err := filepath.Rel(baseDirAbs, ledPathAbs)
	if err != nil {
		return "", err
	}

	// Reject any path that escapes the base directory
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", filepath.ErrBadPattern
	}

	return ledPathAbs, nil
}

// GetLEDs returns information about all LEDs.
func (h *LEDHandler) GetLEDs(w http.ResponseWriter, r *http.Request) {
	leds := []LEDInfo{}

	entries, err := os.ReadDir(h.ledBasePath)
	if err != nil {
		logger.Error("Failed to read LED directory %s: %v", h.ledBasePath, err)
		api.WriteError(w, -1, "Failed to read LED directory")
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		ledPath := filepath.Join(h.ledBasePath, name)

		// In sysfs, entries under /sys/class/leds are often symlinks.
		// DirEntry.IsDir() does NOT follow symlinks, so we must stat the path.
		fi, err := os.Stat(ledPath)
		if err != nil {
			continue
		}
		if !fi.IsDir() {
			continue
		}

		info := LEDInfo{Name: name}

		// Read brightness
		if data, err := os.ReadFile(filepath.Join(ledPath, "brightness")); err == nil {
			val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			info.Brightness = val
		}

		// Read max brightness
		if data, err := os.ReadFile(filepath.Join(ledPath, "max_brightness")); err == nil {
			val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			info.MaxBrightness = val
		}

		// Read trigger
		if data, err := os.ReadFile(filepath.Join(ledPath, "trigger")); err == nil {
			// Parse trigger - the current trigger is in brackets
			triggerStr := strings.TrimSpace(string(data))
			for _, t := range strings.Fields(triggerStr) {
				if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
					info.Trigger = strings.Trim(t, "[]")
					break
				}
			}
		}

		leds = append(leds, info)
	}

	api.WriteSuccess(w, map[string]interface{}{"leds": leds})
}

// GetLED returns information about a specific LED.
func (h *LEDHandler) GetLED(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		api.WriteError(w, -1, "LED name required")
		return
	}

	ledPath, err := h.getSafeLEDPath(name)
	if err != nil {
		api.WriteError(w, -1, "Invalid LED name")
		return
	}

	if _, err := os.Stat(ledPath); err != nil {
		api.WriteError(w, -1, "LED not found")
		return
	}

	info := LEDInfo{Name: filepath.Base(name)}

	if data, err := os.ReadFile(filepath.Join(ledPath, "brightness")); err == nil {
		val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		info.Brightness = val
	}

	if data, err := os.ReadFile(filepath.Join(ledPath, "max_brightness")); err == nil {
		val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		info.MaxBrightness = val
	}

	if data, err := os.ReadFile(filepath.Join(ledPath, "trigger")); err == nil {
		triggerStr := strings.TrimSpace(string(data))
		for _, t := range strings.Fields(triggerStr) {
			if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
				info.Trigger = strings.Trim(t, "[]")
				break
			}
		}
	}

	api.WriteSuccess(w, info)
}

// SetLEDRequest represents an LED set request.
type SetLEDRequest struct {
	Name       string `json:"name"`
	Brightness *int   `json:"brightness,omitempty"`
	Trigger    string `json:"trigger,omitempty"`
}

// SetLED sets LED brightness or trigger.
func (h *LEDHandler) SetLED(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req SetLEDRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	if req.Name == "" {
		api.WriteError(w, -1, "LED name required")
		return
	}

	ledPath, err := h.getSafeLEDPath(req.Name)
	if err != nil {
		api.WriteError(w, -1, "Invalid LED name")
		return
	}

	if _, err := os.Stat(ledPath); err != nil {
		api.WriteError(w, -1, "LED not found")
		return
	}

	// Set trigger if specified
	if req.Trigger != "" {
		// Validate trigger value - only allow safe trigger names
		if !isValidLEDTrigger(req.Trigger) {
			api.WriteError(w, -1, "Invalid trigger value")
			return
		}
		triggerPath := filepath.Join(ledPath, "trigger")
		if err := os.WriteFile(triggerPath, []byte(req.Trigger), 0644); err != nil {
			logger.Error("Failed to set LED trigger: %v", err)
			api.WriteError(w, -1, "Failed to set trigger")
			return
		}
	}

	// Set brightness if specified
	if req.Brightness != nil {
		// Validate brightness value
		if *req.Brightness < 0 {
			api.WriteError(w, -1, "Invalid brightness value")
			return
		}
		brightnessPath := filepath.Join(ledPath, "brightness")
		if err := os.WriteFile(brightnessPath, []byte(strconv.Itoa(*req.Brightness)), 0644); err != nil {
			logger.Error("Failed to set LED brightness: %v", err)
			api.WriteError(w, -1, "Failed to set brightness")
			return
		}
	}

	api.WriteSuccess(w, map[string]interface{}{"name": filepath.Base(req.Name), "status": "updated"})
}

// IsValidLEDTrigger validates LED trigger names
// Only allows alphanumeric, hyphen, underscore, and brackets
func isValidLEDTrigger(trigger string) bool {
	if trigger == "" || len(trigger) > 64 {
		return false
	}
	// Check for invalid characters
	for _, c := range trigger {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '[' || c == ']') {
			return false
		}
	}
	// Check for injection patterns
	if strings.Contains(trigger, "..") || strings.Contains(trigger, "/") ||
		strings.Contains(trigger, "\\") || strings.Contains(trigger, "\x00") {
		return false
	}
	return true
}

// GetLEDTriggers returns available LED triggers.
func (h *LEDHandler) GetLEDTriggers(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		api.WriteError(w, -1, "LED name required")
		return
	}

	ledPath, err := h.getSafeLEDPath(name)
	if err != nil {
		api.WriteError(w, -1, "Invalid LED name")
		return
	}

	triggerPath := filepath.Join(ledPath, "trigger")

	data, err := os.ReadFile(triggerPath)
	if err != nil {
		api.WriteError(w, -1, "Failed to read triggers")
		return
	}

	triggers := []string{}
	current := ""
	for _, t := range strings.Fields(string(data)) {
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			current = strings.Trim(t, "[]")
			triggers = append(triggers, current)
		} else {
			triggers = append(triggers, t)
		}
	}

	api.WriteSuccess(w, map[string]interface{}{
		"triggers": triggers,
		"current":  current,
	})
}
