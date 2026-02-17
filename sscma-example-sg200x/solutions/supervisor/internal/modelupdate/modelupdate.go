// Package modelupdate provides ML model update checking and downloading.
package modelupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"supervisor/internal/config"
	"supervisor/internal/device"
	"supervisor/internal/tls"
	"supervisor/pkg/logger"
)

const (
	InitialDelay       = 30 * time.Second  // Wait for registration to complete
	CheckInterval      = 15 * time.Minute  // Check for updates every 15 minutes
	RetryInterval      = 2 * time.Minute   // Retry on failure
	DownloadTimeout    = 10 * time.Minute  // Timeout for downloading model
	APITimeout         = 30 * time.Second  // Timeout for API calls
)

// ModelUpdateResponse represents the API response for model update check.
type ModelUpdateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		UpdateAvailable bool     `json:"update_available"`
		Message         string   `json:"message"`
		LatestVersion   float64  `json:"latest_version"`
		ModelURL        string   `json:"model_url"`
		PresignedURL    string   `json:"presigned_url"`
		Classes         []string `json:"classes"`
	} `json:"data"`
}

// Status represents the current state of the model update manager.
type Status struct {
	LastCheck       time.Time `json:"last_check"`
	LastUpdate      time.Time `json:"last_update"`
	CurrentVersion  float64   `json:"current_version"`
	LatestVersion   float64   `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	IsDownloading   bool      `json:"is_downloading"`
	DownloadProgress int      `json:"download_progress"`
	LastError       string    `json:"last_error"`
	ModelFile       string    `json:"model_file"`
}

// Manager handles ML model update checking and downloading.
type Manager struct {
	mu               sync.RWMutex
	lastCheck        time.Time
	lastUpdate       time.Time
	currentVersion   float64
	latestVersion    float64
	updateAvailable  bool
	isDownloading    bool
	downloadProgress int
	lastError        string
	modelFile        string
	cancel           context.CancelFunc
	done             chan struct{}
}

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// GetManager returns the singleton model update manager.
func GetManager() *Manager {
	managerOnce.Do(func() {
		globalManager = NewManager()
	})
	return globalManager
}

// NewManager creates a new model update manager.
func NewManager() *Manager {
	m := &Manager{
		done: make(chan struct{}),
	}
	// Load current version from model.json if it exists
	m.loadCurrentVersion()
	return m
}

// loadCurrentVersion reads the current model version from model.json.
func (m *Manager) loadCurrentVersion() {
	data, err := os.ReadFile(device.ModelInfoFile)
	if err != nil {
		logger.Debug("Model update: Could not read model.json: %v", err)
		return
	}

	var info struct {
		Version interface{} `json:"version"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		logger.Warning("Model update: Could not parse model.json: %v", err)
		return
	}

	// Handle version as string or number
	switch v := info.Version.(type) {
	case string:
		// Parse version string like "1.0.0" - extract major.minor as float
		var major, minor int
		if _, err := fmt.Sscanf(v, "%d.%d", &major, &minor); err == nil {
			m.currentVersion = float64(major) + float64(minor)/10.0
		} else {
			// Try parsing as float directly
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
				m.currentVersion = f
			}
		}
	case float64:
		m.currentVersion = v
	case int:
		m.currentVersion = float64(v)
	}

	if m.currentVersion > 0 {
		logger.Info("Model update: Loaded current version %.1f from model.json", m.currentVersion)
	}
}

// Start begins the model update checking service.
func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	go m.checkLoop(ctx)
}

// Stop halts the model update checking service.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	<-m.done
}

// GetStatus returns the current status of the model update manager.
func (m *Manager) GetStatus() *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &Status{
		LastCheck:        m.lastCheck,
		LastUpdate:       m.lastUpdate,
		CurrentVersion:   m.currentVersion,
		LatestVersion:    m.latestVersion,
		UpdateAvailable:  m.updateAvailable,
		IsDownloading:    m.isDownloading,
		DownloadProgress: m.downloadProgress,
		LastError:        m.lastError,
		ModelFile:        m.modelFile,
	}
}

// CheckNow triggers an immediate model update check.
func (m *Manager) CheckNow(ctx context.Context) error {
	return m.checkForUpdates(ctx)
}

func (m *Manager) checkLoop(ctx context.Context) {
	defer close(m.done)
	logger.Info("Model update: Starting model update service")

	// Wait for initial delay (give time for network and registration)
	select {
	case <-ctx.Done():
		return
	case <-time.After(InitialDelay):
	}

	for {
		// Check for updates
		if err := m.checkForUpdates(ctx); err != nil {
			logger.Warning("Model update: Check failed: %v", err)
			m.mu.Lock()
			m.lastError = err.Error()
			m.mu.Unlock()

			// Retry after shorter interval on failure
			select {
			case <-ctx.Done():
				return
			case <-time.After(RetryInterval):
			}
			continue
		}

		// Clear error on success
		m.mu.Lock()
		m.lastError = ""
		m.mu.Unlock()

		// Wait for next check interval
		select {
		case <-ctx.Done():
			return
		case <-time.After(CheckInterval):
		}
	}
}

// getPlatformCredentials retrieves the secret key from platform info.
func getPlatformCredentials() (secretKey string, err error) {
	platformInfo := device.GetPlatformInfo()
	if platformInfo == "" {
		return "", fmt.Errorf("camera not registered")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(platformInfo), &data); err != nil {
		return "", fmt.Errorf("invalid platform info: %w", err)
	}

	// Get secret key (prefer secret_key, fall back to api_key)
	if key, ok := data["secret_key"].(string); ok && key != "" {
		return key, nil
	}
	if key, ok := data["api_key"].(string); ok && key != "" {
		return key, nil
	}

	return "", fmt.Errorf("no API key found in platform info")
}

func platformURL(path string) string {
	base := strings.TrimSpace(config.Get().TPRPlatformURL)
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "https://dev.thepolicerecord.com"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func (m *Manager) checkForUpdates(ctx context.Context) error {
	secretKey, err := getPlatformCredentials()
	if err != nil {
		logger.Debug("Model update: Skipping check - %v", err)
		return nil // Not an error, just not registered yet
	}

	logger.Info("Model update: Checking for model updates...")

	// Create request
	url := platformURL("/api/v1/cameras/self-register/")
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("TPR-API-KEY", secretKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "recamera-supervisor")

	// Make request
	client := tls.PlatformHTTPClient(APITimeout)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var updateResp ModelUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Compute update_available locally based on version comparison
	// Download if no model installed (currentVersion == 0) or if newer version available
	needsDownload := m.currentVersion == 0 || m.currentVersion < updateResp.Data.LatestVersion

	m.mu.Lock()
	m.lastCheck = time.Now()
	m.latestVersion = updateResp.Data.LatestVersion
	m.updateAvailable = needsDownload && updateResp.Data.PresignedURL != ""
	m.mu.Unlock()

	logger.Info("Model update: current=%.1f, latest=%.1f, needs_download=%v, has_url=%v",
		m.currentVersion, updateResp.Data.LatestVersion, needsDownload, updateResp.Data.PresignedURL != "")

	// Download if we need it and have a URL
	if needsDownload && updateResp.Data.PresignedURL != "" {
		logger.Info("Model update: New model available, downloading...")
		if err := m.downloadModel(ctx, updateResp.Data.PresignedURL, updateResp.Data.ModelURL, updateResp.Data.LatestVersion, updateResp.Data.Classes); err != nil {
			return fmt.Errorf("failed to download model: %w", err)
		}

		m.mu.Lock()
		m.currentVersion = updateResp.Data.LatestVersion
		m.mu.Unlock()

		logger.Info("Model update: Model updated successfully to version %.1f", updateResp.Data.LatestVersion)
	}

	return nil
}

func (m *Manager) downloadModel(ctx context.Context, presignedURL, modelPath string, version float64, classes []string) error {
	m.mu.Lock()
	if m.isDownloading {
		m.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}
	m.isDownloading = true
	m.downloadProgress = 0
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.isDownloading = false
		m.mu.Unlock()
	}()

	// Create download request with timeout
	ctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", presignedURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	client := tls.PlatformHTTPClient(DownloadTimeout)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Determine filename from model path or Content-Disposition
	filename := filepath.Base(modelPath)
	if filename == "" || filename == "." {
		filename = "model.cvimodel"
	}

	// Ensure model directory exists
	if err := os.MkdirAll(device.ModelDir, 0755); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	// Download to temp file first
	tmpFile := filepath.Join(device.ModelDir, "."+filename+".tmp")
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Track download progress
	contentLength := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			nw, writeErr := f.Write(buf[:n])
			if writeErr != nil {
				f.Close()
				os.Remove(tmpFile)
				return fmt.Errorf("failed to write model file: %w", writeErr)
			}
			written += int64(nw)

			// Update progress
			if contentLength > 0 {
				m.mu.Lock()
				m.downloadProgress = int(float64(written) / float64(contentLength) * 100)
				m.mu.Unlock()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmpFile)
			return fmt.Errorf("failed to read response: %w", readErr)
		}
	}

	f.Close()
	logger.Info("Model update: Downloaded %d bytes", written)

	// Atomic rename to final location
	finalPath := filepath.Join(device.ModelDir, filename)
	if err := os.Rename(tmpFile, finalPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename model file: %w", err)
	}

	// Also update the default model.cvimodel symlink/copy if this is the active model
	defaultModel := device.ModelFile
	if finalPath != defaultModel {
		// Copy to default location so camera-streamer uses it
		if err := copyFile(finalPath, defaultModel); err != nil {
			logger.Warning("Model update: Failed to copy to default location: %v", err)
		}
	}

	// Update model.json with new version, filename, and classes
	if err := m.updateModelJSON(version, filename, classes); err != nil {
		logger.Warning("Model update: Failed to update model.json: %v", err)
	}

	m.mu.Lock()
	m.lastUpdate = time.Now()
	m.updateAvailable = false
	m.modelFile = finalPath
	m.downloadProgress = 100
	m.mu.Unlock()

	return nil
}

// updateModelJSON updates the version, filename, and classes in model.json, preserving other fields.
func (m *Manager) updateModelJSON(version float64, filename string, classes []string) error {
	// Read existing model.json if it exists
	var info map[string]interface{}
	data, err := os.ReadFile(device.ModelInfoFile)
	if err == nil {
		if err := json.Unmarshal(data, &info); err != nil {
			info = make(map[string]interface{})
		}
	} else {
		info = make(map[string]interface{})
	}

	// Update version and filename
	info["version"] = version
	info["filename"] = filename

	// Update classes if provided, otherwise remove old classes
	if len(classes) > 0 {
		info["classes"] = classes
	} else {
		delete(info, "classes")
	}

	// Write back
	newData, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal model.json: %w", err)
	}

	if err := os.WriteFile(device.ModelInfoFile, newData, 0644); err != nil {
		return fmt.Errorf("failed to write model.json: %w", err)
	}

	if len(classes) > 0 {
		logger.Info("Model update: Updated model.json with version %.1f, filename %s, %d classes", version, filename, len(classes))
	} else {
		logger.Info("Model update: Updated model.json with version %.1f, filename %s", version, filename)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
