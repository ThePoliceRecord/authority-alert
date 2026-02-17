package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"supervisor/internal/api"
	"supervisor/internal/config"
	"supervisor/internal/device"
	"supervisor/internal/modelupdate"
	"supervisor/internal/ntp"
	"supervisor/internal/system"
	"supervisor/internal/tls"
	"supervisor/internal/upgrade"
	"supervisor/pkg/logger"
)

// DeviceHandler handles device management API requests.
type DeviceHandler struct {
	modelDir    string
	modelSuffix string
	deviceInfo  *device.APIDeviceInfo
	upgradeMgr  *upgrade.UpgradeManager
	ntpManager  *ntp.Manager
}

const (
	// maxOTAUploadSize limits the maximum accepted OTA zip upload size.
	// OTA zips contain a full rootfs image and can be large.
	maxOTAUploadSize int64 = 2 << 30 // 2GiB
)

// CodeRegistrationState tracks the state of a code-based camera registration flow.
type CodeRegistrationState struct {
	Status            string                 `json:"status"`                      // idle | generating | active | claimed | expired | error
	Message           string                 `json:"message,omitempty"`           // Human-readable status message
	ClaimCode         string                 `json:"claim_code,omitempty"`        // The 6-character registration code
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`        // When the code expires
	StartedAt         *time.Time             `json:"started_at,omitempty"`        // When registration started
	Result            map[string]interface{} `json:"result,omitempty"`            // Registration result (api_key, etc.)
	InternetAvailable bool                   `json:"internet_available"`          // Whether internet is currently available
	LastError         string                 `json:"last_error,omitempty"`        // Last error message (for resilience)
	RetryCount        int                    `json:"retry_count,omitempty"`       // Number of consecutive retries
	cancel            chan struct{}                                               // Cancel signal (not serialized)
}

// CodeRegistrationRequest is the request body for starting code registration.
type CodeRegistrationRequest struct {
	LocationName string   `json:"location_name"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
}

// Package-level state for code registration (only one registration at a time)
var (
	codeRegState = &CodeRegistrationState{Status: "idle", InternetAvailable: true}
	codeRegMutex sync.Mutex
	codeRegReq   *CodeRegistrationRequest // Stored request for use in goroutine
)

const (
	// codeCharset contains safe characters for registration codes
	// Excludes: 0 (zero), O (oh), 1 (one), I (eye), L (ell)
	codeCharset    = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	codeLength     = 6
	codeExpiryMins = 10
	codePollIntervalSecs = 3
)

func platformBaseURL() string {
	base := strings.TrimSpace(config.Get().TPRPlatformURL)
	base = strings.TrimRight(base, "/")
	if base == "" {
		return "https://dev.thepolicerecord.com"
	}
	return base
}

func platformURL(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return platformBaseURL() + path
}

func applyPlatformCommonHeaders(req *http.Request) {
	// Some WAF/CDN setups treat empty User-Agent as suspicious.
	req.Header.Set("User-Agent", "recamera-supervisor")
	req.Header.Set("Accept", "application/json")
}

// NewDeviceHandler creates a new DeviceHandler.
func NewDeviceHandler(ntpManager *ntp.Manager) *DeviceHandler {
	h := &DeviceHandler{
		modelDir:    "/usr/share/supervisor/models",
		modelSuffix: ".cvimodel",
		upgradeMgr:  upgrade.NewUpgradeManager(),
		ntpManager:  ntpManager,
	}

	// Load device info
	h.deviceInfo = device.GetAPIDevice()
	if h.deviceInfo != nil {
		if h.deviceInfo.Model.Preset != "" {
			h.modelDir = h.deviceInfo.Model.Preset
		}
		if h.deviceInfo.Model.File != "" {
			ext := filepath.Ext(h.deviceInfo.Model.File)
			if ext != "" {
				h.modelSuffix = ext
			}
		}
	}

	return h
}

// QueryDeviceInfo returns device information.
func (h *DeviceHandler) QueryDeviceInfo(w http.ResponseWriter, r *http.Request) {
	info := device.QueryDeviceInfo()
	api.WriteSuccess(w, info)
}

// GetDeviceInfo returns detailed device information.
func (h *DeviceHandler) GetDeviceInfo(w http.ResponseWriter, r *http.Request) {
	info := device.QueryDeviceInfo()
	api.WriteSuccess(w, info)
}

// GetDeviceList returns list of devices on the network.
func (h *DeviceHandler) GetDeviceList(w http.ResponseWriter, r *http.Request) {
	devices, err := device.GetDeviceList()
	if err != nil {
		api.WriteSuccess(w, map[string]interface{}{"deviceList": []interface{}{}})
		return
	}
	api.WriteSuccess(w, map[string]interface{}{"deviceList": devices})
}

// UpdateDeviceNameRequest represents a device name update request.
type UpdateDeviceNameRequest struct {
	DeviceName string `json:"deviceName"`
}

// UpdateDeviceName updates the device name.
func (h *DeviceHandler) UpdateDeviceName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req UpdateDeviceNameRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	if req.DeviceName == "" {
		api.WriteError(w, -1, "Device name required")
		return
	}

	// Validate device name to prevent injection attacks
	if !isValidDeviceName(req.DeviceName) {
		api.WriteError(w, -1, "Invalid device name. Use only alphanumeric characters, hyphens, and underscores (max 63 characters)")
		return
	}

	if err := device.UpdateDeviceName(req.DeviceName); err != nil {
		api.WriteError(w, -1, "Failed to update device name")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{"deviceName": req.DeviceName})
}

// GetCameraWebsocketUrl returns the camera WebSocket URL (proxied through supervisor).
func (h *DeviceHandler) GetCameraWebsocketUrl(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	// Determine protocol based on request scheme
	// Return supervisor's WebSocket proxy endpoint (always uses same protocol as page)
	protocol := "ws://"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		protocol = "wss://"
	}

	// Return supervisor's camera proxy endpoint (not direct camera-streamer)
	// Supervisor will relay to ws://localhost:8765
	wsURL := protocol + host + "/ws/camera"
	api.WriteSuccess(w, map[string]interface{}{"websocketUrl": wsURL})
}

// QueryServiceStatus returns the status of services.
func (h *DeviceHandler) QueryServiceStatus(w http.ResponseWriter, r *http.Request) {
	// Since sscma-node has been removed, return success status directly
	// The frontend expects sscmaNode=0 and system=0 for "RUNNING" status
	api.WriteSuccess(w, map[string]interface{}{
		"sscmaNode": 0,
		"system":    0,
		"uptime":    system.GetUptime(),
	})
}

// GetSystemStatus returns system status information.
func (h *DeviceHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"uptime":     system.GetUptime(),
		"deviceName": system.GetDeviceName(),
		"osName":     system.GetOSName(),
		"osVersion":  system.GetOSVersion(),
	}

	// Add NTP status if manager is available
	if h.ntpManager != nil {
		response["ntpSynced"] = h.ntpManager.IsSynced()
		lastSync := h.ntpManager.LastSync()
		if !lastSync.IsZero() {
			response["lastNtpSync"] = lastSync.Unix()
		} else {
			response["lastNtpSync"] = nil
		}
	}

	api.WriteSuccess(w, response)
}

// GetInternetStatus checks and returns internet connectivity status.
func (h *DeviceHandler) GetInternetStatus(w http.ResponseWriter, r *http.Request) {
	status := system.CheckInternet()
	api.WriteSuccess(w, status)
}

// SetPowerRequest represents a power mode request.
type SetPowerRequest struct {
	Mode int `json:"mode"` // 0: shutdown, 1: reboot, 2: suspend
}

// SetPower sets the power mode.
func (h *DeviceHandler) SetPower(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req SetPowerRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	action := ""
	switch req.Mode {
	case 0:
		action = "poweroff"
	case 1:
		action = "reboot"
	case 2:
		action = "suspend"
	default:
		api.WriteError(w, -1, "Invalid power mode")
		return
	}

	// Send response before executing power command
	api.WriteSuccess(w, map[string]interface{}{"mode": req.Mode})

	// Execute power command in background
	go func(mode int, action string) {
		time.Sleep(1 * time.Second)
		var err error
		switch mode {
		case 0:
			err = system.Poweroff()
		case 1:
			err = system.Reboot()
		case 2:
			err = system.Suspend()
		}
		if err != nil {
			logger.Error("SetPower: %s failed: %v", action, err)
		}
	}(req.Mode, action)
}

// GetModelList returns the list of available models.
func (h *DeviceHandler) GetModelList(w http.ResponseWriter, r *http.Request) {
	models := []map[string]interface{}{}

	files, err := os.ReadDir(h.modelDir)
	if err != nil {
		api.WriteSuccess(w, map[string]interface{}{"models": models})
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(name, h.modelSuffix) {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		models = append(models, map[string]interface{}{
			"name":     name,
			"path":     filepath.Join(h.modelDir, name),
			"size":     info.Size(),
			"modified": info.ModTime().Unix(),
		})
	}

	api.WriteSuccess(w, map[string]interface{}{"models": models})
}

// GetModelInfo returns information about the current model.
func (h *DeviceHandler) GetModelInfo(w http.ResponseWriter, r *http.Request) {
	// Read model info from file
	infoFile := device.ModelDir + "/model.json"
	data, err := os.ReadFile(infoFile)
	if err != nil {
		api.WriteError(w, -1, "Failed to get model info")
		return
	}

	// The web UI expects the standard supervisor API envelope {code,msg,data}.
	// model.json contents vary by build, so decode to an interface{} and fall back to raw text.
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		api.WriteSuccess(w, map[string]interface{}{"raw": string(data)})
		return
	}

	api.WriteSuccess(w, parsed)
}

// GetModelFile serves a model file for download.
func (h *DeviceHandler) GetModelFile(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		api.WriteError(w, -1, "Model name required")
		return
	}

	// Sanitize path to prevent directory traversal
	// filepath.Base returns only the filename, stripping directory components
	name = filepath.Base(name)
	filePath := filepath.Join(h.modelDir, name)

	// Verify file exists and is under model directory using filepath.Rel
	// This is the CodeQL-recognized pattern for path traversal prevention
	absModelDir, err := filepath.Abs(h.modelDir)
	if err != nil {
		api.WriteError(w, -1, "Invalid model path")
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		api.WriteError(w, -1, "Invalid model path")
		return
	}

	// Use filepath.Rel to verify the path is within modelDir
	rel, err := filepath.Rel(absModelDir, absPath)
	if err != nil {
		api.WriteError(w, -1, "Invalid model path")
		return
	}

	// Reject any path that escapes the model directory
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		api.WriteError(w, -1, "Invalid model path")
		return
	}

	http.ServeFile(w, r, absPath)
}

// UploadModel handles model file uploads.
func (h *DeviceHandler) UploadModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		api.WriteError(w, -1, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		api.WriteError(w, -1, "File required")
		return
	}
	defer file.Close()

	// Validate filename
	filename := filepath.Base(header.Filename)
	if !strings.HasSuffix(filename, h.modelSuffix) {
		api.WriteError(w, -1, "Invalid model file type")
		return
	}

	// Create destination file
	dstPath := filepath.Join(h.modelDir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		logger.Error("Failed to create model file: %v", err)
		api.WriteError(w, -1, "Failed to save model")
		return
	}
	defer dst.Close()

	// Copy file
	if _, err := io.Copy(dst, file); err != nil {
		logger.Error("Failed to copy model file: %v", err)
		os.Remove(dstPath)
		api.WriteError(w, -1, "Failed to save model")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{
		"name": filename,
		"path": dstPath,
		"size": header.Size,
	})
}

// Supported upload types:
// - *_ota.zip (A/B OTA zip)
// - *.swu or *_swu.zip (SWUpdate bundle)
// - *_emmc.zip or upgrade.zip (upgrade zip containing fip.bin/boot.emmc/rootfs_ext4.emmc)
func (h *DeviceHandler) UploadUpdatePackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	if h.upgradeMgr.IsUpgrading() {
		api.WriteError(w, -1, "Update already in progress")
		return
	}

	parseOSAndVersion := func(name string) (osName, version string) {
		parts := strings.Split(name, "_")
		if len(parts) >= 3 {
			osName = parts[1]
			version = parts[2]
		}
		return
	}

	// Enforce an upper bound on upload size.
	r.Body = http.MaxBytesReader(w, r.Body, maxOTAUploadSize)

	// IMPORTANT: do not call r.ParseMultipartForm() for OTA packages.
	// ParseMultipartForm spills large uploads to os.TempDir() (typically /tmp), which is often too small
	// for multi-hundred-MB or multi-GB OTA zips and results in "Failed to parse form".
	// Instead, stream the multipart part directly into UpgradeTmpDir.
	reader, err := r.MultipartReader()
	if err != nil {
		logger.Error("UploadUpdatePackage: failed to create multipart reader: %v", err)
		api.WriteError(w, -1, "Invalid multipart upload")
		return
	}

	var part *multipart.Part
	for {
		p, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error("UploadUpdatePackage: failed to read multipart data: %v", err)
			api.WriteError(w, -1, "Failed to read upload")
			return
		}
		if p.FormName() == "file" {
			part = p
			break
		}
		_ = p.Close()
	}
	if part == nil {
		api.WriteError(w, -1, "File required")
		return
	}
	defer part.Close()

	filename := filepath.Base(part.FileName())
	var pkgType upgrade.UploadedPackageType
	isSWUZip := false
	switch {
	case strings.HasSuffix(filename, "ota.zip") && strings.HasSuffix(filename, ".zip"):
		pkgType = upgrade.UploadedPackageTypeOTAZip
	case strings.HasSuffix(filename, ".swu"):
		pkgType = upgrade.UploadedPackageTypeSWU
	case strings.HasSuffix(filename, "_swu.zip"):
		pkgType = upgrade.UploadedPackageTypeSWU
		isSWUZip = true
	case filename == "upgrade.zip" || strings.HasSuffix(filename, "_emmc.zip"):
		pkgType = upgrade.UploadedPackageTypeUpgradeZip
	default:
		api.WriteError(w, -1, "Invalid update package. Expected *_ota.zip, *.swu, *_swu.zip, *_emmc.zip, or upgrade.zip")
		return
	}

	// Ensure staging directories exist.
	if err := os.MkdirAll(upgrade.UpgradeTmpDir, 0755); err != nil {
		logger.Error("Failed to create upgrade tmp dir: %v", err)
		api.WriteError(w, -1, "Failed to stage update package")
		return
	}
	if err := os.MkdirAll(upgrade.UpgradeFilesDir, 0755); err != nil {
		logger.Error("Failed to create upgrade files dir: %v", err)
		api.WriteError(w, -1, "Failed to stage update package")
		return
	}

	// Write upload to a temp file first, then atomically rename.
	tmpDst, err := os.CreateTemp(upgrade.UpgradeTmpDir, "upload-*.partial")
	if err != nil {
		logger.Error("Failed to create temp OTA file: %v", err)
		api.WriteError(w, -1, "Failed to stage update package")
		return
	}
	tmpPath := tmpDst.Name()

	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmpDst, hash), part)
	closeErr := tmpDst.Close()
	if copyErr != nil || closeErr != nil {
		logger.Error("Failed to save OTA file: copyErr=%v closeErr=%v", copyErr, closeErr)
		os.Remove(tmpPath)
		api.WriteError(w, -1, "Failed to save update package")
		return
	}

	checksum := hex.EncodeToString(hash.Sum(nil))

	finalPath := filepath.Join(upgrade.UpgradeTmpDir, filename)
	// Remove any previously staged OTA with the same name.
	_ = os.Remove(finalPath)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		logger.Error("Failed to finalize OTA file: %v", err)
		os.Remove(tmpPath)
		api.WriteError(w, -1, "Failed to stage update package")
		return
	}

	storedFileName := filename
	storedChecksum := checksum
	storedSize := written

	// If the user uploaded *_swu.zip, extract the contained .swu and stage that instead.
	if isSWUZip {
		zr, err := zip.OpenReader(finalPath)
		if err != nil {
			api.WriteError(w, -1, "Invalid _swu.zip (failed to open)")
			return
		}
		var swuEntry *zip.File
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, ".swu") {
				swuEntry = f
				break
			}
		}
		if swuEntry == nil {
			_ = zr.Close()
			api.WriteError(w, -1, "Invalid _swu.zip (no .swu inside)")
			return
		}
		rc, err := swuEntry.Open()
		if err != nil {
			_ = zr.Close()
			api.WriteError(w, -1, "Invalid _swu.zip (failed to read .swu)")
			return
		}

		dstName := filepath.Base(swuEntry.Name)
		tmpOut, err := os.CreateTemp(upgrade.UpgradeTmpDir, "extract-*.partial")
		if err != nil {
			_ = rc.Close()
			_ = zr.Close()
			api.WriteError(w, -1, "Failed to stage .swu")
			return
		}
		outPath := tmpOut.Name()
		xh := sha256.New()
		xWritten, xCopyErr := io.Copy(io.MultiWriter(tmpOut, xh), rc)
		xCloseErr := tmpOut.Close()
		_ = rc.Close()
		_ = zr.Close()
		if xCopyErr != nil || xCloseErr != nil {
			_ = os.Remove(outPath)
			api.WriteError(w, -1, "Failed to extract .swu")
			return
		}
		finalSWUPath := filepath.Join(upgrade.UpgradeTmpDir, dstName)
		_ = os.Remove(finalSWUPath)
		if err := os.Rename(outPath, finalSWUPath); err != nil {
			_ = os.Remove(outPath)
			api.WriteError(w, -1, "Failed to stage .swu")
			return
		}
		// Remove the original zip to save space.
		_ = os.Remove(finalPath)

		storedFileName = dstName
		storedChecksum = hex.EncodeToString(xh.Sum(nil))
		storedSize = xWritten
	}

	osName, version := parseOSAndVersion(storedFileName)

	// OTA zip still uses the legacy manifest + version.json so existing OTA flows work.
	if pkgType == upgrade.UploadedPackageTypeOTAZip {
		manifestLine := fmt.Sprintf("%s  %s\n", storedChecksum, storedFileName)
		manifestPath := filepath.Join(upgrade.UpgradeFilesDir, upgrade.ChecksumFileName)
		if err := os.WriteFile(manifestPath, []byte(manifestLine), 0644); err != nil {
			logger.Error("Failed to write OTA manifest: %v", err)
			api.WriteError(w, -1, "Failed to stage update package")
			return
		}

		// Also write version.json so existing update flows can surface the staged update.
		versionFile := filepath.Join(upgrade.UpgradeFilesDir, "version.json")
		if data, err := json.Marshal(upgrade.UpdateVersion{OSName: osName, OSVersion: version, Status: upgrade.UpdateStatusAvailable}); err == nil {
			_ = os.WriteFile(versionFile, data, 0644)
		}
	}

	// Persist uploaded package metadata for the UI and apply path.
	_ = upgrade.SaveUploadedPackageInfo(&upgrade.StagedPackageInfo{
		Exists:      true,
		FileName:    storedFileName,
		Checksum:    storedChecksum,
		OSName:      osName,
		Version:     version,
		Size:        storedSize,
		PackageType: pkgType,
	})

	api.WriteSuccess(w, map[string]interface{}{
		"fileName":    storedFileName,
		"checksum":    storedChecksum,
		"size":        storedSize,
		"osName":      osName,
		"version":     version,
		"packageType": pkgType,
	})
}

// GetUploadedUpdatePackage returns information about a staged OTA package.
func (h *DeviceHandler) GetUploadedUpdatePackage(w http.ResponseWriter, r *http.Request) {
	info, err := h.upgradeMgr.GetStagedLocalPackageInfo()
	if err != nil {
		api.WriteError(w, -1, "Failed to read staged update package")
		return
	}
	api.WriteSuccess(w, info)
}

// ApplyUploadedUpdatePackage starts an upgrade using the currently staged OTA package.
func (h *DeviceHandler) ApplyUploadedUpdatePackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	if err := h.upgradeMgr.UpdateSystemFromUploadedPackage(); err != nil {
		api.WriteError(w, -1, err.Error())
		return
	}

	api.WriteSuccess(w, map[string]interface{}{"status": "updating"})
}

// Timestamp APIs

// SetTimestampRequest represents a timestamp set request.
type SetTimestampRequest struct {
	Timestamp int64 `json:"timestamp"`
}

// SetTimestamp sets the system timestamp.
func (h *DeviceHandler) SetTimestamp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req SetTimestampRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	// Set system time using date command
	t := time.Unix(req.Timestamp, 0)
	dateStr := t.Format("2006-01-02 15:04:05")
	if err := exec.Command("date", "-s", dateStr).Run(); err != nil {
		logger.Error("Failed to set timestamp: %v", err)
		api.WriteError(w, -1, "Failed to set timestamp")
		return
	}

	// Sync to hardware clock
	exec.Command("hwclock", "-w").Run()

	api.WriteSuccess(w, map[string]interface{}{"timestamp": req.Timestamp})
}

// GetTimestamp returns the current system timestamp.
func (h *DeviceHandler) GetTimestamp(w http.ResponseWriter, r *http.Request) {
	api.WriteSuccess(w, map[string]interface{}{"timestamp": time.Now().Unix()})
}

// SetTimezoneRequest represents a timezone set request.
type SetTimezoneRequest struct {
	Timezone string `json:"timezone"`
}

// SetTimezone sets the system timezone.
func (h *DeviceHandler) SetTimezone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req SetTimezoneRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	if req.Timezone == "" {
		api.WriteError(w, -1, "Timezone required")
		return
	}

	// Sanitize timezone input to prevent path traversal
	// Only allow alphanumeric characters, forward slashes, underscores, hyphens, and plus signs
	// This prevents directory traversal attacks like "../../../etc/passwd"
	if !isValidTimezone(req.Timezone) {
		api.WriteError(w, -1, "Invalid timezone format")
		return
	}

	// Verify timezone exists
	tzFile := "/usr/share/zoneinfo/" + req.Timezone

	// Resolve to absolute path and verify it's under /usr/share/zoneinfo
	absPath, err := filepath.Abs(tzFile)
	if err != nil {
		api.WriteError(w, -1, "Invalid timezone")
		return
	}

	// Ensure the resolved path is still under /usr/share/zoneinfo
	if !strings.HasPrefix(absPath, "/usr/share/zoneinfo/") {
		api.WriteError(w, -1, "Invalid timezone path")
		return
	}

	if _, err := os.Stat(absPath); err != nil {
		api.WriteError(w, -1, "Invalid timezone")
		return
	}

	// Create symlink
	localtime := "/etc/localtime"
	os.Remove(localtime)
	if err := os.Symlink(absPath, localtime); err != nil {
		logger.Error("Failed to set timezone: %v", err)
		api.WriteError(w, -1, "Failed to set timezone")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{"timezone": req.Timezone})
}

func isValidTimezone(tz string) bool {
	if tz == "" || len(tz) > 100 {
		return false
	}
	// Check for path traversal patterns
	if strings.Contains(tz, "..") || strings.Contains(tz, "\\") {
		return false
	}
	// Only allow safe characters for timezone paths
	for _, c := range tz {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '/' || c == '_' || c == '-' || c == '+') {
			return false
		}
	}
	// Ensure it doesn't start with slash or contain double slashes
	if strings.HasPrefix(tz, "/") || strings.Contains(tz, "//") {
		return false
	}
	return true
}

// GetTimezone returns the current timezone.
func (h *DeviceHandler) GetTimezone(w http.ResponseWriter, r *http.Request) {
	link, err := os.Readlink("/etc/localtime")
	if err != nil {
		api.WriteSuccess(w, map[string]interface{}{"timezone": "UTC"})
		return
	}

	tz := strings.TrimPrefix(link, "/usr/share/zoneinfo/")
	api.WriteSuccess(w, map[string]interface{}{"timezone": tz})
}

// GetTimezoneList returns a list of available timezones.
func (h *DeviceHandler) GetTimezoneList(w http.ResponseWriter, r *http.Request) {
	timezones := []string{}

	// Read common timezones
	commonTZ := []string{
		"UTC", "America/New_York", "America/Los_Angeles", "America/Chicago",
		"Europe/London", "Europe/Paris", "Europe/Berlin",
		"Asia/Tokyo", "Asia/Shanghai", "Asia/Singapore",
		"Australia/Sydney", "Pacific/Auckland",
	}

	for _, tz := range commonTZ {
		tzFile := "/usr/share/zoneinfo/" + tz
		if _, err := os.Stat(tzFile); err == nil {
			timezones = append(timezones, tz)
		}
	}

	api.WriteSuccess(w, map[string]interface{}{"timezones": timezones})
}

// System Update APIs

// UpdateChannel sets the update channel.
func (h *DeviceHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	logger.Info("UpdateChannel endpoint called: method=%s", r.Method)

	if r.Method != http.MethodPost {
		logger.Warning("UpdateChannel: Method not allowed: %s", r.Method)
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req struct {
		Channel int    `json:"channel"`
		URL     string `json:"url"`
		// Frontend sends serverUrl, so accept both for compatibility
		ServerURL string `json:"serverUrl"`
	}
	if err := api.ParseJSONBody(r, &req); err != nil {
		logger.Error("UpdateChannel: Failed to parse JSON body: %v", err)
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	// Use serverUrl if url is empty (frontend sends serverUrl)
	url := req.URL
	if url == "" {
		url = req.ServerURL
	}

	logger.Info("API UpdateChannel request: channel=%d, url=%s", req.Channel, url)

	if err := h.upgradeMgr.UpdateChannel(req.Channel, url); err != nil {
		logger.Error("UpdateChannel: Failed to update channel: %v", err)
		api.WriteError(w, -1, "Failed to update channel")
		return
	}

	logger.Info("UpdateChannel: Successfully updated channel=%d", req.Channel)
	api.WriteSuccess(w, map[string]interface{}{"channel": req.Channel})
}

// GetSystemUpdateVersion returns available update version.
func (h *DeviceHandler) GetSystemUpdateVersion(w http.ResponseWriter, r *http.Request) {
	// Allow the caller to force a refresh even if version.json is cached.
	force := false
	if r.Method == http.MethodPost {
		var req struct {
			Force bool `json:"force"`
		}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err == nil {
			force = req.Force
		} else if err != io.EOF {
			// Non-fatal: ignore malformed/empty body and fall back to cached behavior.
			logger.Warning("GetSystemUpdateVersion: failed to decode request body: %v", err)
		}
	}

	result, err := h.upgradeMgr.GetSystemUpdateVersionWithOptions(force)
	if err != nil {
		api.WriteError(w, -1, "Failed to get update version")
		return
	}
	api.WriteSuccess(w, result)
}

// GetUpdateCheckProgress returns progress/status for the "check for updates" operation.
func (h *DeviceHandler) GetUpdateCheckProgress(w http.ResponseWriter, r *http.Request) {
	result, err := h.upgradeMgr.GetUpdateCheckProgress()
	if err != nil {
		api.WriteSuccess(w, map[string]interface{}{"progress": 0, "status": "idle"})
		return
	}
	api.WriteSuccess(w, result)
}

// UpdateSystem initiates a system update.
func (h *DeviceHandler) UpdateSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	// Start update in background
	go func() {
		h.upgradeMgr.UpdateSystem()
	}()

	api.WriteSuccess(w, map[string]interface{}{"status": "updating"})
}

// GetUpdateProgress returns the update progress.
func (h *DeviceHandler) GetUpdateProgress(w http.ResponseWriter, r *http.Request) {
	result, err := h.upgradeMgr.GetUpdateProgress()
	if err != nil {
		api.WriteSuccess(w, map[string]interface{}{"progress": 0, "status": "idle"})
		return
	}
	api.WriteSuccess(w, result)
}

// CancelUpdate cancels an ongoing update.
func (h *DeviceHandler) CancelUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	if err := h.upgradeMgr.CancelUpdate(); err != nil {
		api.WriteError(w, -1, "Failed to cancel update")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{"status": "cancelled"})
}

// Platform Info APIs

// GetPlatformInfo returns platform configuration.
func (h *DeviceHandler) GetPlatformInfo(w http.ResponseWriter, r *http.Request) {
	info := device.GetPlatformInfo()
	api.WriteSuccess(w, map[string]interface{}{"platform_info": info})
}

// SavePlatformInfo saves platform configuration.
func (h *DeviceHandler) SavePlatformInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req struct {
		PlatformInfo string `json:"platform_info"`
	}
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	if err := device.SavePlatformInfo(req.PlatformInfo); err != nil {
		api.WriteError(w, -1, "Failed to save platform info")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{"message": "Platform info saved"})
}

// FactoryReset sets the factory reset flag for the next reboot.
// This will reset the device to factory defaults on next restart.
func (h *DeviceHandler) FactoryReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	// Set factory reset flag using fw_setenv
	if err := h.upgradeMgr.Recovery(); err != nil {
		logger.Error("Failed to set factory reset flag: %v", err)
		api.WriteError(w, -1, "Failed to initiate factory reset")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{
		"status":  "scheduled",
		"message": "Factory reset scheduled. Please reboot the device to apply.",
	})
}

// FormatSDCard formats the SD card with exfat filesystem.
func (h *DeviceHandler) FormatSDCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	logger.Info("Starting SD card format")

	// SD card root device
	sdDevice := "/dev/mmcblk1"

	// Check if device exists
	if _, err := os.Stat(sdDevice); err != nil {
		logger.Error("SD card device %s not found: %v", sdDevice, err)
		api.WriteError(w, -1, "SD card not detected")
		return
	}

	// Unmount any existing mounts first
	exec.Command("umount", "-f", sdDevice+"p1").Run()
	exec.Command("umount", "-f", sdDevice).Run()
	exec.Command("umount", "-f", "/mnt/sd").Run()
	time.Sleep(500 * time.Millisecond)

	// Try to find existing partition
	cmd := exec.Command("lsblk", "-ln", "-o", "NAME", sdDevice)
	output, err := cmd.Output()
	var targetDevice string

	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 1 {
			// Found partition
			partName := strings.TrimSpace(lines[1])
			targetDevice = "/dev/" + partName
			logger.Info("Found existing partition: %s", targetDevice)
		}
	}

	// If no partition found, create one
	if targetDevice == "" {
		logger.Info("No partition found, creating partition table on %s", sdDevice)

		// Create MBR partition table with single partition using fdisk
		// Commands: o (create DOS partition table), n (new partition), p (primary),
		// 1 (partition number), default start, default end, w (write)
		fdiskScript := "o\nn\np\n1\n\n\nw\n"
		fdiskCmd := exec.Command("sh", "-c", fmt.Sprintf("echo -e '%s' | fdisk %s", fdiskScript, sdDevice))
		fdiskOutput, fdiskErr := fdiskCmd.CombinedOutput()
		logger.Info("fdisk output: %s", string(fdiskOutput))

		if fdiskErr != nil {
			logger.Warning("fdisk command had errors: %v, trying partprobe", fdiskErr)
		}

		// Tell kernel to re-read partition table
		exec.Command("partprobe", sdDevice).Run()

		// Wait for kernel to recognize new partition
		time.Sleep(2 * time.Second)

		// Confirm partition was created
		targetDevice = sdDevice + "p1"
		if _, err := os.Stat(targetDevice); err != nil {
			logger.Error("Failed to create partition %s: %v", targetDevice, err)
			api.WriteError(w, -1, "Failed to create SD card partition")
			return
		}
		logger.Info("Created partition: %s", targetDevice)
	}

	// Format with exFAT - try without -f first
	logger.Info("Formatting %s with exFAT (without -f)", targetDevice)
	cmd = exec.Command("mkfs.exfat", targetDevice)
	output, err = cmd.CombinedOutput()
	if err != nil {
		logger.Warning("mkfs.exfat without -f failed: %v, output: %s", err, string(output))

		// Try with -f flag
		logger.Info("Retrying with -f flag")
		cmd = exec.Command("mkfs.exfat", "-f", targetDevice)
		output, err = cmd.CombinedOutput()
		if err != nil {
			logger.Error("mkfs.exfat with -f failed: %v, output: %s", err, string(output))
			api.WriteError(w, -1, fmt.Sprintf("Failed to format SD card: %s", string(output)))
			return
		}
	}

	logger.Info("SD card formatted successfully with exFAT, output: %s", string(output))

	// Wait before remounting
	time.Sleep(1 * time.Second)

	// Remount the SD card with explicit filesystem type
	os.MkdirAll("/mnt/sd", 0755)
	mountCmd := exec.Command("mount", "-t", "exfat", targetDevice, "/mnt/sd")
	if mountOutput, err := mountCmd.CombinedOutput(); err != nil {
		logger.Warning("SD card formatted but failed to remount: %v, output: %s", err, string(mountOutput))
		// Don't fail the request since format was successful
	} else {
		logger.Info("SD card remounted successfully at /mnt/sd")
	}

	api.WriteSuccess(w, map[string]interface{}{
		"status":  "success",
		"message": "SD card formatted successfully with exFAT",
	})
}

// isValidDeviceName validates device/hostname according to RFC 1123
// Allows alphanumeric, hyphens (not at start/end), max 63 chars
func isValidDeviceName(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	// Must start and end with alphanumeric
	if !isAlphaNumeric(name[0]) || !isAlphaNumeric(name[len(name)-1]) {
		return false
	}
	// Check all characters
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	// Check for injection patterns
	dangerous := []string{"\n", "\r", ";", "&", "|", "$", "`", "\\", "/", "<", ">", "'", "\""}
	for _, d := range dangerous {
		if strings.Contains(name, d) {
			return false
		}
	}
	return true
}

// isAlphaNumeric checks if a byte is alphanumeric
func isAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// Analytics Configuration APIs

// AnalyticsConfig represents the analytics configuration.
type AnalyticsConfig struct {
	Enabled bool `json:"enabled"`
}

const analyticsConfigPath = "/etc/supervisor/analytics.conf"

// GetAnalyticsConfig returns the current analytics configuration.
func (h *DeviceHandler) GetAnalyticsConfig(w http.ResponseWriter, r *http.Request) {
	config, err := loadAnalyticsConfig()
	if err != nil {
		// Return default config if file doesn't exist
		config = &AnalyticsConfig{Enabled: true}
	}

	api.WriteSuccess(w, config)
}

// SetAnalyticsConfig updates the analytics configuration.
func (h *DeviceHandler) SetAnalyticsConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var config AnalyticsConfig
	if err := api.ParseJSONBody(r, &config); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	// Save configuration
	if err := saveAnalyticsConfig(&config); err != nil {
		logger.Error("Failed to save analytics config: %v", err)
		api.WriteError(w, -1, "Failed to save configuration")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{
		"status":  "success",
		"message": "Analytics configuration saved",
		"enabled": config.Enabled,
	})
}

// loadAnalyticsConfig loads the analytics configuration from file.
func loadAnalyticsConfig() (*AnalyticsConfig, error) {
	data, err := os.ReadFile(analyticsConfigPath)
	if err != nil {
		return nil, err
	}

	var config AnalyticsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// saveAnalyticsConfig saves the analytics configuration to file.
func saveAnalyticsConfig(config *AnalyticsConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(analyticsConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(analyticsConfigPath, data, 0644)
}

// Camera Registration APIs

// CameraRegistrationResponse represents the response from the platform.
// Data can be either a map (on success) or a string (on error), so we use json.RawMessage.
type CameraRegistrationResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Key     string          `json:"key"`
	Data    json.RawMessage `json:"data"`
}

// AACamera represents the camera registration data for Authority Alert backend.
type AACamera struct {
	SerialNumber string `json:"serial_number"`
	DeviceName   string `json:"device_name"`
	OSVersion    string `json:"os_version"`
	ModelVersion string `json:"model_version,omitempty"`
}

// ReRegisterCamera re-registers the camera with the Authority Alert service.
func (h *DeviceHandler) ReRegisterCamera(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	logger.Info("Camera re-registration requested")

	// Read secret_key from platform info
	platformInfo := device.GetPlatformInfo()
	if platformInfo == "" {
		logger.Error("Platform info not found - camera may not be registered yet")
		api.WriteError(w, -1, "Camera not registered. Please complete OOBE first.")
		return
	}

	// Parse platform info to get secret_key
	var platformData map[string]interface{}
	if err := json.Unmarshal([]byte(platformInfo), &platformData); err != nil {
		logger.Error("Failed to parse platform info: %v", err)
		api.WriteError(w, -1, "Invalid platform configuration")
		return
	}

	// Look for secret_key (new) or fall back to api_key (old) for backwards compatibility
	secretKey, ok := platformData["secret_key"].(string)
	if !ok || secretKey == "" {
		// Try old api_key field for backwards compatibility
		secretKey, ok = platformData["api_key"].(string)
		if !ok || secretKey == "" {
			logger.Error("Secret key not found in platform info")
			api.WriteError(w, -1, "Secret key not found. Please complete OOBE registration again.")
			return
		}
		logger.Warning("Using legacy api_key field - should be secret_key")
	}

	// Build camera registration data
	cameraData := AACamera{
		SerialNumber: system.GetSerialNumber(),
		DeviceName:   system.GetDeviceName(),
		OSVersion:    system.GetOSVersion(),
	}

	// Call Authority Alert self-register endpoint
	if err := selfRegisterCamera(secretKey, &cameraData); err != nil {
		logger.Error("Failed to re-register camera: %v", err)
		api.WriteError(w, -1, "Failed to re-register camera: "+err.Error())
		return
	}

	logger.Info("Camera re-registered successfully")
	api.WriteSuccess(w, map[string]interface{}{
		"status":  "success",
		"message": "Camera re-registered successfully",
	})
}

// selfRegisterCamera calls the Authority Alert self-register API with PUT method.
func selfRegisterCamera(apiKey string, cameraData *AACamera) error {
	// Authority Alert backend URL
	backendURL := platformURL("/api/v1/cameras/self-register/")

	// Marshal camera data
	jsonData, err := json.Marshal(cameraData)
	if err != nil {
		return fmt.Errorf("failed to marshal camera data: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("PUT", backendURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	applyPlatformCommonHeaders(req)

	// Send request
	client := tls.PlatformHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		logger.Error("Authority Alert API returned status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Warning("Failed to parse response: %v", err)
		// Don't fail if we can't parse response, as long as status was 200
	}

	logger.Info("Camera self-registration successful: %s", string(body))
	return nil
}

// selfRegisterWithAPIKey performs camera self-registration using an API key.
// This is similar to RegisterCamera but skips the OAuth token exchange since we already have the API key.
func (h *DeviceHandler) selfRegisterWithAPIKey(apiKey string, userID interface{}, locationName string, lat, lon *float64) (map[string]interface{}, error) {
	// Collect device information (same as RegisterCamera)
	deviceInfo := device.QueryDeviceInfo()

	// Get MAC addresses
	wifiMac := system.GetMAC("wlan0")
	if wifiMac == "" {
		wifiMac = "unknown"
	}
	ethMac := system.GetMAC("eth0")
	if ethMac == "" {
		ethMac = "unknown"
	}

	// Use serial number as camera UID/ID
	cameraUID := deviceInfo.SN
	cameraID := deviceInfo.SN

	// Get current timestamp
	now := time.Now().Format(time.RFC3339)

	// Build registration payload
	platformPayload := map[string]interface{}{
		"uid":                     cameraUID,
		"user_id":                 userID,
		"camera_id":               cameraID,
		"location_name":           locationName,
		"serial_number":           deviceInfo.SN,
		"wifi_mac_address":        wifiMac,
		"mac_address":             ethMac,
		"device_model":            deviceInfo.Type,
		"firmware_version":        deviceInfo.OSVersion,
		"last_contacted":          now,
		"installation_date":       now,
		"creation_date":           now,
		"last_subscribed":         now,
		"notification_target_ids": []int{},
	}

	// Include lat/lon if provided
	if lat != nil {
		platformPayload["latitude"] = *lat
	}
	if lon != nil {
		platformPayload["longitude"] = *lon
	}

	// Marshal payload
	jsonData, err := json.Marshal(platformPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration payload: %w", err)
	}

	logger.Info("Sending self-registration to platform: %s", string(jsonData))

	// Call platform's self-register endpoint
	backendURL := platformURL("/api/v1/cameras/self-register/")
	httpReq, err := http.NewRequest("POST", backendURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("TPR-API-KEY", apiKey)
	applyPlatformCommonHeaders(httpReq)

	client := tls.PlatformHTTPClient(30 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to platform: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	logger.Info("Platform self-register response status: %d, body: %s", resp.StatusCode, string(body))

	// Parse response
	var platformResp CameraRegistrationResponse
	if err := json.Unmarshal(body, &platformResp); err != nil {
		return nil, fmt.Errorf("invalid platform response: %s", string(body))
	}

	// Check success (platform returns 200 or 201)
	if platformResp.Code != 200 && platformResp.Code != 201 {
		var errorDetail string
		if len(platformResp.Data) > 0 {
			json.Unmarshal(platformResp.Data, &errorDetail)
		}
		errMsg := platformResp.Message
		if errorDetail != "" {
			errMsg = errMsg + ": " + errorDetail
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Parse data as map
	var dataMap map[string]interface{}
	if len(platformResp.Data) > 0 {
		json.Unmarshal(platformResp.Data, &dataMap)
	}

	// Extract secret_key and UID
	var responseUID string
	var secretKey string
	if dataMap != nil {
		if uid, ok := dataMap["uid"].(string); ok {
			responseUID = uid
		}
		if key, ok := dataMap["secret_key"].(string); ok {
			secretKey = key
		}
	}

	finalUID := responseUID
	if finalUID == "" {
		finalUID = cameraUID
	}

	// Save platform info locally
	registrationData := map[string]interface{}{
		"platform_url":  platformBaseURL(),
		"secret_key":    secretKey,
		"camera_uid":    finalUID,
		"user_id":       userID,
		"registered_at": time.Now().Format(time.RFC3339),
		"camera_data":   dataMap,
	}

	registrationJSON, _ := json.MarshalIndent(registrationData, "", "  ")
	if err := device.SavePlatformInfo(string(registrationJSON)); err != nil {
		logger.Warning("Failed to save platform info locally: %v", err)
		// Don't fail - registration was successful
	}

	logger.Info("Camera registered successfully via API key, uid=%s", finalUID)

	return map[string]interface{}{
		"code":    platformResp.Code,
		"message": platformResp.Message,
		"uid":     finalUID,
		"user_id": userID,
		"data":    dataMap,
	}, nil
}

// ============================================================================
// Code-Based Camera Registration
// ============================================================================

// generateRegistrationCode generates a cryptographically random 6-character code.
func generateRegistrationCode() (string, error) {
	result := make([]byte, codeLength)
	randomBytes := make([]byte, codeLength)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	for i := 0; i < codeLength; i++ {
		result[i] = codeCharset[int(randomBytes[i])%len(codeCharset)]
	}

	return string(result), nil
}

// formatClaimCode formats the 6-char code for display (e.g., "ABC 123")
func formatClaimCode(code string) string {
	if len(code) != 6 {
		return code
	}
	return code[:3] + " " + code[3:]
}

// StartCodeRegistration initiates a code-based camera registration flow.
// The supervisor generates a 6-char code, registers it with the platform,
// and polls for claim status.
func (h *DeviceHandler) StartCodeRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req CodeRegistrationRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	if req.LocationName == "" {
		api.WriteError(w, -1, "Location name required")
		return
	}

	codeRegMutex.Lock()
	defer codeRegMutex.Unlock()

	// If registration is already in progress, return current status
	if codeRegState.Status == "generating" || codeRegState.Status == "active" {
		logger.Info("Code registration already in progress (status=%s), returning current state", codeRegState.Status)
		response := map[string]interface{}{
			"status":  codeRegState.Status,
			"message": codeRegState.Message,
		}
		if codeRegState.ClaimCode != "" {
			response["claim_code"] = codeRegState.ClaimCode
			response["claim_code_formatted"] = formatClaimCode(codeRegState.ClaimCode)
		}
		if codeRegState.ExpiresAt != nil {
			response["expires_at"] = codeRegState.ExpiresAt.Format(time.RFC3339)
		}
		if codeRegState.StartedAt != nil {
			response["started_at"] = codeRegState.StartedAt.Format(time.RFC3339)
		}
		api.WriteSuccess(w, response)
		return
	}

	// Store request for use in goroutine
	codeRegReq = &req

	// Generate registration code
	code, err := generateRegistrationCode()
	if err != nil {
		logger.Error("Failed to generate registration code: %v", err)
		api.WriteError(w, -1, "Failed to generate registration code")
		return
	}

	// Initialize state
	now := time.Now()
	expiresAt := now.Add(codeExpiryMins * time.Minute)
	codeRegState = &CodeRegistrationState{
		Status:            "generating",
		Message:           "Registering code with platform...",
		ClaimCode:         code,
		ExpiresAt:         &expiresAt,
		StartedAt:         &now,
		InternetAvailable: true,
		LastError:         "",
		RetryCount:        0,
		cancel:            make(chan struct{}),
	}

	// Start registration and polling in background
	go h.runCodeRegistration()

	logger.Info("Code registration started, code: %s, location: %s", formatClaimCode(code), req.LocationName)

	api.WriteSuccess(w, map[string]interface{}{
		"status":               codeRegState.Status,
		"message":              codeRegState.Message,
		"claim_code":           codeRegState.ClaimCode,
		"claim_code_formatted": formatClaimCode(codeRegState.ClaimCode),
		"expires_at":           codeRegState.ExpiresAt.Format(time.RFC3339),
		"started_at":           codeRegState.StartedAt.Format(time.RFC3339),
	})
}

// GetCodeRegistrationStatus returns the current state of code registration.
func (h *DeviceHandler) GetCodeRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	codeRegMutex.Lock()
	defer codeRegMutex.Unlock()

	response := map[string]interface{}{
		"status":             codeRegState.Status,
		"message":            codeRegState.Message,
		"internet_available": codeRegState.InternetAvailable,
	}

	if codeRegState.ClaimCode != "" {
		response["claim_code"] = codeRegState.ClaimCode
		response["claim_code_formatted"] = formatClaimCode(codeRegState.ClaimCode)
	}
	if codeRegState.ExpiresAt != nil {
		response["expires_at"] = codeRegState.ExpiresAt.Format(time.RFC3339)
	}
	if codeRegState.StartedAt != nil {
		response["started_at"] = codeRegState.StartedAt.Format(time.RFC3339)
	}
	if codeRegState.Result != nil {
		response["result"] = codeRegState.Result
	}
	if codeRegState.LastError != "" {
		response["last_error"] = codeRegState.LastError
	}
	if codeRegState.RetryCount > 0 {
		response["retry_count"] = codeRegState.RetryCount
	}

	api.WriteSuccess(w, response)
}

// CancelCodeRegistration cancels an active code registration.
func (h *DeviceHandler) CancelCodeRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	codeRegMutex.Lock()
	defer codeRegMutex.Unlock()

	if codeRegState.Status != "generating" && codeRegState.Status != "active" {
		api.WriteError(w, -1, "No active code registration to cancel")
		return
	}

	// Signal cancellation
	if codeRegState.cancel != nil {
		close(codeRegState.cancel)
	}

	codeRegState.Status = "idle"
	codeRegState.Message = "Cancelled"
	logger.Info("Code registration cancelled")

	api.WriteSuccess(w, map[string]interface{}{
		"status":  "cancelled",
		"message": "Code registration cancelled",
	})
}

// checkInternetAvailable performs a quick internet connectivity check.
func checkInternetAvailable() bool {
	status := system.CheckInternet()
	return status.Available
}

// runCodeRegistration runs the code registration and polling process in a goroutine.
func (h *DeviceHandler) runCodeRegistration() {
	codeRegMutex.Lock()
	code := codeRegState.ClaimCode
	expiresAt := codeRegState.ExpiresAt
	cancelChan := codeRegState.cancel
	req := codeRegReq
	codeRegState.InternetAvailable = true // Assume available initially
	codeRegMutex.Unlock()

	if req == nil {
		h.setCodeRegError("Registration request not found")
		return
	}

	serialNumber := system.GetSerialNumber()
	if serialNumber == "" {
		h.setCodeRegError("Failed to get device serial number")
		return
	}

	client := tls.PlatformHTTPClient(30 * time.Second)
	retryCount := 0
	const maxRetryBackoff = 30 // Max backoff seconds

	// Helper to calculate exponential backoff delay
	getBackoffDelay := func(retries int) time.Duration {
		delay := 1 << retries // 1, 2, 4, 8, 16, 32...
		if delay > maxRetryBackoff {
			delay = maxRetryBackoff
		}
		return time.Duration(delay) * time.Second
	}

	// Step 1: Register the code with the platform (with retry on network errors)
	logger.Info("Registering claim code %s with platform", formatClaimCode(code))

	registerPayload := map[string]interface{}{
		"serial_number": serialNumber,
		"claim_code":    code,
	}

	jsonData, err := json.Marshal(registerPayload)
	if err != nil {
		h.setCodeRegError("Failed to prepare registration request")
		return
	}

	registerURL := platformURL("/api/v2/camera/register/")
	registered := false

	for !registered {
		select {
		case <-cancelChan:
			logger.Info("Code registration was cancelled during initial registration")
			return
		default:
		}

		// Check if expired
		if time.Now().After(*expiresAt) {
			h.setCodeRegStatus("expired", "Registration code expired", nil)
			logger.Info("Code registration expired during initial registration")
			return
		}

		// Check internet connectivity
		internetOK := checkInternetAvailable()
		h.updateInternetStatus(internetOK, "")

		if !internetOK {
			logger.Warning("No internet connection, waiting before retry...")
			h.setCodeRegStatusWithRetry("generating", "No internet connection. Retrying...", retryCount)
			retryCount++
			time.Sleep(getBackoffDelay(retryCount))
			continue
		}

		httpReq, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(jsonData))
		if err != nil {
			h.setCodeRegError("Failed to create registration request")
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		applyPlatformCommonHeaders(httpReq)

		resp, err := client.Do(httpReq)
		if err != nil {
			logger.Warning("Failed to connect to platform: %v, will retry...", err)
			h.updateInternetStatus(false, err.Error())
			h.setCodeRegStatusWithRetry("generating", "Connection failed. Retrying...", retryCount)
			retryCount++
			time.Sleep(getBackoffDelay(retryCount))
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Warning("Failed to read platform response: %v, will retry...", err)
			retryCount++
			time.Sleep(getBackoffDelay(retryCount))
			continue
		}

		logger.Info("Platform register response status: %d, body: %s", resp.StatusCode, string(body))

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			// Parse initial registration response to get camera_id (for logging/validation)
			var regResp struct {
				Data struct {
					Camera struct {
						CameraID string `json:"camera_id"`
					} `json:"camera"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &regResp); err == nil && regResp.Data.Camera.CameraID != "" {
				logger.Info("Platform assigned camera_id: %s", regResp.Data.Camera.CameraID)
			}
			registered = true
			retryCount = 0 // Reset retry count on success
			h.updateInternetStatus(true, "")
		} else if resp.StatusCode >= 500 {
			// Server error, retry
			logger.Warning("Platform server error (status %d), will retry...", resp.StatusCode)
			retryCount++
			time.Sleep(getBackoffDelay(retryCount))
			continue
		} else {
			// Client error (4xx), don't retry
			h.setCodeRegError(fmt.Sprintf("Platform registration failed with status %d", resp.StatusCode))
			return
		}
	}

	// Update state to active (code registered, waiting for claim)
	h.setCodeRegStatus("active", "Code ready. Waiting for user to claim...", nil)

	// Step 2: Poll for claim status until claimed, expired, or cancelled
	ticker := time.NewTicker(codePollIntervalSecs * time.Second)
	defer ticker.Stop()

	statusURL := platformURL(fmt.Sprintf("/api/v1/claim_camera/%s/status", serialNumber))

	for {
		select {
		case <-cancelChan:
			logger.Info("Code registration was cancelled")
			return

		case <-ticker.C:
			// Check if expired
			if time.Now().After(*expiresAt) {
				h.setCodeRegStatus("expired", "Registration code expired", nil)
				logger.Info("Code registration expired")
				return
			}

			// Check internet connectivity
			internetOK := checkInternetAvailable()
			h.updateInternetStatus(internetOK, "")

			if !internetOK {
				logger.Warning("No internet during polling, will keep trying...")
				retryCount++
				// Update message but keep status as active
				codeRegMutex.Lock()
				codeRegState.Message = "No internet. We'll keep trying in the background."
				codeRegState.RetryCount = retryCount
				codeRegMutex.Unlock()
				continue
			}

			// Poll claim status
			statusReq, err := http.NewRequest("GET", statusURL, nil)
			if err != nil {
				logger.Warning("Failed to create status request: %v", err)
				continue
			}
			applyPlatformCommonHeaders(statusReq)

			statusResp, err := client.Do(statusReq)
			if err != nil {
				logger.Warning("Failed to poll claim status: %v", err)
				h.updateInternetStatus(false, err.Error())
				retryCount++
				codeRegMutex.Lock()
				codeRegState.Message = "Connection issue. Retrying..."
				codeRegState.RetryCount = retryCount
				codeRegMutex.Unlock()
				continue
			}

			statusBody, err := io.ReadAll(statusResp.Body)
			statusResp.Body.Close()
			if err != nil {
				logger.Warning("Failed to read status response: %v", err)
				continue
			}

			// Reset retry count on successful poll
			if retryCount > 0 {
				retryCount = 0
				h.updateInternetStatus(true, "")
				codeRegMutex.Lock()
				codeRegState.Message = "Code ready. Waiting for user to claim..."
				codeRegState.RetryCount = 0
				codeRegMutex.Unlock()
			}

			logger.Debug("Claim status response: %s", string(statusBody))

			// Parse response
			var statusData struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Data    struct {
					Claimed         bool   `json:"claimed"`
					APIKeyConfirmed bool   `json:"api_key_confirmed"`
					APIKey          string `json:"api_key"`
					SecretKey       string `json:"secret_key"`
					UserID          int    `json:"user_id"`
					Camera          struct {
						CameraID string `json:"camera_id"`
					} `json:"camera"`
				} `json:"data"`
			}

			if err := json.Unmarshal(statusBody, &statusData); err != nil {
				logger.Warning("Failed to parse status response: %v", err)
				continue
			}

			// Check if fully set up (claimed and confirmed)
			if statusData.Data.Claimed && statusData.Data.APIKeyConfirmed {
				logger.Info("Camera already claimed and confirmed, completing registration")

				// Get the API key (prefer secret_key, fall back to api_key)
				apiKey := statusData.Data.SecretKey
				if apiKey == "" {
					apiKey = statusData.Data.APIKey
				}

				// Validate that we have an API key before completing registration
				if apiKey == "" {
					logger.Warning("Camera claimed but no API key received yet, continuing to poll...")
					continue
				}

				// Save platform info locally and complete
				registrationData := map[string]interface{}{
					"platform_url":   platformBaseURL(),
					"secret_key":     apiKey,
					"camera_uid":     serialNumber,
					"tpr_camera_id":  statusData.Data.Camera.CameraID,
					"user_id":        statusData.Data.UserID,
					"registered_at":  time.Now().Format(time.RFC3339),
					"location_name":  req.LocationName,
				}
				if req.Latitude != nil {
					registrationData["latitude"] = *req.Latitude
				}
				if req.Longitude != nil {
					registrationData["longitude"] = *req.Longitude
				}

				registrationJSON, _ := json.MarshalIndent(registrationData, "", "  ")
				if err := device.SavePlatformInfo(string(registrationJSON)); err != nil {
					logger.Warning("Failed to save platform info locally: %v", err)
				}

				// Trigger immediate model download after registration
				triggerPostRegistrationModelDownload()

				h.setCodeRegStatus("claimed", "Camera registered successfully!", registrationData)
				logger.Info("Code registration completed successfully (already confirmed), tpr_camera_id=%s", statusData.Data.Camera.CameraID)
				return
			}

			// Check if claimed but needs confirmation
			if statusData.Data.Claimed && !statusData.Data.APIKeyConfirmed {
				logger.Info("Camera claimed, need to confirm API key...")

				// Get the API key (prefer secret_key, fall back to api_key)
				apiKey := statusData.Data.SecretKey
				if apiKey == "" {
					apiKey = statusData.Data.APIKey
				}

				// Validate that we have an API key before confirming
				if apiKey == "" {
					logger.Warning("Camera claimed but no API key received, continuing to poll...")
					continue
				}

				// Update status to show we're confirming
				codeRegMutex.Lock()
				codeRegState.Message = "Setting up..."
				codeRegMutex.Unlock()

				// Step 3: Confirm the code was successfully read/claimed (with retries)
				logger.Info("Confirming claim for serial %s with api_key", serialNumber)
				confirmURL := platformURL(fmt.Sprintf("/api/v1/claim_camera/%s/confirm", serialNumber))

				// Build confirm payload with api_key in JSON body
				confirmPayload := map[string]string{"api_key": apiKey}
				confirmJSON, err := json.Marshal(confirmPayload)
				if err != nil {
					h.setCodeRegError("Failed to prepare confirmation request")
					return
				}
				logger.Debug("Confirm payload: %s", string(confirmJSON))

				confirmed := false
				confirmRetries := 0
				const maxConfirmRetries = 5

				for !confirmed && confirmRetries < maxConfirmRetries {
					// Create fresh buffer for each attempt (buffer is consumed after request)
					confirmReq, err := http.NewRequest("POST", confirmURL, bytes.NewReader(confirmJSON))
					if err != nil {
						h.setCodeRegError("Failed to create confirmation request")
						return
					}
					confirmReq.Header.Set("Content-Type", "application/json")
					applyPlatformCommonHeaders(confirmReq)

					confirmResp, err := client.Do(confirmReq)
					if err != nil {
						logger.Warning("Failed to confirm claim (attempt %d): %v", confirmRetries+1, err)
						confirmRetries++
						time.Sleep(getBackoffDelay(confirmRetries))
						continue
					}

					confirmBody, err := io.ReadAll(confirmResp.Body)
					confirmResp.Body.Close()
					if err != nil {
						logger.Warning("Failed to read confirmation response: %v", err)
						confirmRetries++
						time.Sleep(getBackoffDelay(confirmRetries))
						continue
					}

					logger.Info("Platform confirm response status: %d, body: %s", confirmResp.StatusCode, string(confirmBody))

					if confirmResp.StatusCode == http.StatusOK || confirmResp.StatusCode == http.StatusCreated {
						confirmed = true
					} else if confirmResp.StatusCode >= 500 {
						// Server error, retry
						confirmRetries++
						time.Sleep(getBackoffDelay(confirmRetries))
						continue
					} else {
						// Client error, don't retry
						h.setCodeRegError(fmt.Sprintf("Platform confirmation failed with status %d", confirmResp.StatusCode))
						return
					}
				}

				if !confirmed {
					h.setCodeRegError("Failed to confirm claim after multiple attempts")
					return
				}

				// Save platform info locally
				// Note: For claim code registration, the platform already registered the camera
				// during the claim/confirm flow, so we just need to save the credentials locally.
				registrationData := map[string]interface{}{
					"platform_url":   platformBaseURL(),
					"secret_key":     apiKey,
					"camera_uid":     serialNumber,
					"tpr_camera_id":  statusData.Data.Camera.CameraID,
					"user_id":        statusData.Data.UserID,
					"registered_at":  time.Now().Format(time.RFC3339),
					"location_name":  req.LocationName,
				}
				if req.Latitude != nil {
					registrationData["latitude"] = *req.Latitude
				}
				if req.Longitude != nil {
					registrationData["longitude"] = *req.Longitude
				}

				registrationJSON, _ := json.MarshalIndent(registrationData, "", "  ")
				if err := device.SavePlatformInfo(string(registrationJSON)); err != nil {
					logger.Warning("Failed to save platform info locally: %v", err)
				}

				// Trigger immediate model download after registration
				triggerPostRegistrationModelDownload()

				// Success! Return the registration data as result
				h.setCodeRegStatus("claimed", "Camera registered successfully!", registrationData)
				logger.Info("Code registration completed successfully, tpr_camera_id=%s", statusData.Data.Camera.CameraID)
				return
			}
		}
	}
}

// updateInternetStatus updates the internet availability status in state.
func (h *DeviceHandler) updateInternetStatus(available bool, lastError string) {
	codeRegMutex.Lock()
	defer codeRegMutex.Unlock()
	codeRegState.InternetAvailable = available
	codeRegState.LastError = lastError
}

// setCodeRegStatusWithRetry updates status with retry count.
func (h *DeviceHandler) setCodeRegStatusWithRetry(status, message string, retryCount int) {
	codeRegMutex.Lock()
	defer codeRegMutex.Unlock()
	codeRegState.Status = status
	codeRegState.Message = message
	codeRegState.RetryCount = retryCount
}

// setCodeRegStatus updates the code registration state.
func (h *DeviceHandler) setCodeRegStatus(status, message string, result map[string]interface{}) {
	codeRegMutex.Lock()
	defer codeRegMutex.Unlock()

	codeRegState.Status = status
	codeRegState.Message = message
	if result != nil {
		codeRegState.Result = result
	}
}

// setCodeRegError sets the code registration state to error.
func (h *DeviceHandler) setCodeRegError(message string) {
	h.setCodeRegStatus("error", message, nil)
	logger.Error("Code registration error: %s", message)
}

// triggerPostRegistrationModelDownload initiates model download after registration.
func triggerPostRegistrationModelDownload() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		mgr := modelupdate.GetManager()
		if err := mgr.CheckNow(ctx); err != nil {
			logger.Warning("Post-registration model download failed: %v", err)
		}
	}()
}

// ============================================================================
// Model Update APIs
// ============================================================================

// GetModelUpdateStatus returns the current model update status.
func (h *DeviceHandler) GetModelUpdateStatus(w http.ResponseWriter, r *http.Request) {
	// Import modelupdate package dynamically to avoid circular imports
	// The modelupdate manager is accessed via a package-level function
	status := getModelUpdateStatus()
	api.WriteSuccess(w, status)
}

// CheckModelUpdates triggers an immediate model update check.
func (h *DeviceHandler) CheckModelUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	if err := triggerModelUpdateCheck(); err != nil {
		api.WriteError(w, -1, err.Error())
		return
	}

	api.WriteSuccess(w, map[string]interface{}{
		"status":  "checking",
		"message": "Model update check started",
	})
}
