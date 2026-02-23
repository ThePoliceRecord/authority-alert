package ble

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"supervisor/internal/auth"
	"supervisor/internal/device"
	"supervisor/pkg/logger"
)

// handleSetup dispatches commands on the Setup characteristic (FC05).
func (s *Service) handleSetup(req Request) Response {
	// Commands that don't require auth
	switch req.Cmd {
	case "get_timezone_list":
		return s.handleGetTimezoneList()
	case "complete":
		return s.handleComplete()
	}

	// All other commands require a valid session token
	if err := s.validateSession(req.Token); err != nil {
		return Response{Code: 401, Msg: "unauthorized"}
	}

	switch req.Cmd {
	case "set_password":
		return s.handleSetPassword(req)
	case "set_device_name":
		return s.handleSetDeviceName(req)
	case "set_timezone":
		return s.handleSetTimezone(req)
	case "set_timestamp":
		return s.handleSetTimestamp(req)
	case "start_registration":
		return s.handleStartRegistration(req)
	case "registration_status":
		return s.handleRegistrationStatus()
	default:
		return Response{Code: 400, Msg: "unknown command"}
	}
}

// handleSetPassword sets the initial device password during OOBE.
func (s *Service) handleSetPassword(req Request) Response {
	var data struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(req.Data, &data); err != nil || data.Password == "" {
		return Response{Code: 400, Msg: "password required"}
	}

	if len(data.Password) < 8 {
		return Response{Code: 400, Msg: "password must be at least 8 characters"}
	}

	username := auth.GetUsername()

	// Change password using passwd command
	cmd := exec.Command("passwd", username)
	cmd.Stdin = strings.NewReader(data.Password + "\n" + data.Password + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("BLE set_password failed: %v, output: %s", err, string(output))
		return Response{Code: 500, Msg: "failed to set password"}
	}

	// Mark OOBE as started (password changed = point of no return)
	if _, err := os.Stat("/etc/oobe/flag"); err == nil {
		if err := os.WriteFile("/etc/oobe/started", []byte{}, 0644); err != nil {
			logger.Error("BLE: failed to create OOBE started marker: %v", err)
		}
	}

	logger.Info("BLE: password set for user %s", username)
	return Response{Code: 200, Msg: "password set", Data: map[string]string{"message": "password set"}}
}

// handleSetDeviceName sets the device name.
func (s *Service) handleSetDeviceName(req Request) Response {
	var data struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.Data, &data); err != nil || data.Name == "" {
		return Response{Code: 400, Msg: "name required"}
	}

	if err := device.UpdateDeviceName(data.Name); err != nil {
		logger.Error("BLE set_device_name failed: %v", err)
		return Response{Code: 500, Msg: "failed to set device name"}
	}

	logger.Info("BLE: device name set to %s", data.Name)
	return Response{Code: 200, Msg: "ok", Data: map[string]string{"name": data.Name}}
}

// handleSetTimezone sets the system timezone.
func (s *Service) handleSetTimezone(req Request) Response {
	var data struct {
		Timezone string `json:"timezone"`
	}
	if err := json.Unmarshal(req.Data, &data); err != nil || data.Timezone == "" {
		return Response{Code: 400, Msg: "timezone required"}
	}

	if !isValidTimezone(data.Timezone) {
		return Response{Code: 400, Msg: "invalid timezone format"}
	}

	tzFile := "/usr/share/zoneinfo/" + data.Timezone
	absPath, err := filepath.Abs(tzFile)
	if err != nil || !strings.HasPrefix(absPath, "/usr/share/zoneinfo/") {
		return Response{Code: 400, Msg: "invalid timezone"}
	}
	if _, err := os.Stat(absPath); err != nil {
		return Response{Code: 400, Msg: "invalid timezone"}
	}

	localtime := "/etc/localtime"
	os.Remove(localtime)
	if err := os.Symlink(absPath, localtime); err != nil {
		logger.Error("BLE set_timezone failed: %v", err)
		return Response{Code: 500, Msg: "failed to set timezone"}
	}

	logger.Info("BLE: timezone set to %s", data.Timezone)
	return Response{Code: 200, Msg: "ok", Data: map[string]string{"timezone": data.Timezone}}
}

// isValidTimezone validates timezone string (same logic as handler).
func isValidTimezone(tz string) bool {
	if tz == "" || len(tz) > 100 {
		return false
	}
	if strings.Contains(tz, "..") || strings.Contains(tz, "\\") {
		return false
	}
	for _, c := range tz {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '/' || c == '_' || c == '-' || c == '+') {
			return false
		}
	}
	if strings.HasPrefix(tz, "/") || strings.Contains(tz, "//") {
		return false
	}
	return true
}

// handleGetTimezoneList returns available timezones.
func (s *Service) handleGetTimezoneList() Response {
	commonTZ := []string{
		"UTC", "America/New_York", "America/Los_Angeles", "America/Chicago",
		"Europe/London", "Europe/Paris", "Europe/Berlin",
		"Asia/Tokyo", "Asia/Shanghai", "Asia/Singapore",
		"Australia/Sydney", "Pacific/Auckland",
	}

	var timezones []string
	for _, tz := range commonTZ {
		if _, err := os.Stat("/usr/share/zoneinfo/" + tz); err == nil {
			timezones = append(timezones, tz)
		}
	}

	return Response{Code: 200, Msg: "ok", Data: map[string]interface{}{"timezones": timezones}}
}

// handleSetTimestamp sets the system time.
func (s *Service) handleSetTimestamp(req Request) Response {
	var data struct {
		Timestamp int64 `json:"timestamp"`
	}
	if err := json.Unmarshal(req.Data, &data); err != nil || data.Timestamp == 0 {
		return Response{Code: 400, Msg: "timestamp required"}
	}

	t := time.Unix(data.Timestamp, 0)
	dateStr := t.Format("2006-01-02 15:04:05")
	if err := exec.Command("date", "-s", dateStr).Run(); err != nil {
		logger.Error("BLE set_timestamp failed: %v", err)
		return Response{Code: 500, Msg: "failed to set timestamp"}
	}

	exec.Command("hwclock", "-w").Run()

	logger.Info("BLE: timestamp set to %s", dateStr)
	return Response{Code: 200, Msg: "ok", Data: map[string]interface{}{"timestamp": data.Timestamp}}
}

// handleStartRegistration starts code-based camera registration.
func (s *Service) handleStartRegistration(req Request) Response {
	if s.regSvc == nil {
		return Response{Code: 500, Msg: "registration service not available"}
	}

	var data struct {
		LocationName string  `json:"location_name"`
		Lat          float64 `json:"lat"`
		Lon          float64 `json:"lon"`
	}
	if err := json.Unmarshal(req.Data, &data); err != nil || data.LocationName == "" {
		return Response{Code: 400, Msg: "location_name required"}
	}

	result, err := s.regSvc.StartRegistrationDirect(data.LocationName, data.Lat, data.Lon)
	if err != nil {
		logger.Error("BLE start_registration failed: %v", err)
		return Response{Code: 500, Msg: err.Error()}
	}

	return Response{Code: 200, Msg: "ok", Data: result}
}

// handleRegistrationStatus returns the current registration status.
func (s *Service) handleRegistrationStatus() Response {
	if s.regSvc == nil {
		return Response{Code: 500, Msg: "registration service not available"}
	}

	result := s.regSvc.GetRegistrationStatusDirect()
	return Response{Code: 200, Msg: "ok", Data: result}
}

// handleComplete marks OOBE as done by removing the flag file.
func (s *Service) handleComplete() Response {
	if err := os.Remove(oobeFlagFile); err != nil && !os.IsNotExist(err) {
		logger.Error("BLE complete: failed to remove OOBE flag: %v", err)
		return Response{Code: 500, Msg: "failed to complete OOBE"}
	}

	logger.Info("BLE: OOBE completed, flag removed")
	return Response{Code: 200, Msg: "ok", Data: map[string]string{"message": "setup complete"}}
}
