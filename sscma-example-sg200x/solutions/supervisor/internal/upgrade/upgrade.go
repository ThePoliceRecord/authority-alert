// Package upgrade provides system upgrade management functionality.
package upgrade

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"supervisor/internal/system"
	tlsutil "supervisor/internal/tls"
	"supervisor/pkg/logger"
)

// URLs and file paths
const (
	OfficialURL      = "https://github.com/ThePoliceRecord/authority-alert-OS/releases/latest"
	ChecksumFileName = "sg2002_recamera_emmc_sha256sum.txt"
	URLFileName      = "url.txt"
	// UploadedPackageMetaFileName stores metadata about the last uploaded update package.
	// Stored in UpgradeFilesDir.
	UploadedPackageMetaFileName = "uploaded_package.json"

	DefaultUpgradeURL = "https://github.com/ThePoliceRecord/authority-alert-OS/releases/latest"
)

// Partitions
const (
	FIPPart     = "/dev/mmcblk0boot0"
	BootPart    = "/dev/mmcblk0p1"
	RecvPart    = "/dev/mmcblk0p5"
	RootFS      = "/dev/mmcblk0p3"
	RootFSB     = "/dev/mmcblk0p4"
	ForceROPath = "/sys/block/mmcblk0boot0/force_ro"
)

// Directories
const (
	ConfigDir       = "/etc/recamera.conf"
	UpgradeConfFile = "/etc/recamera.conf/upgrade"
	UpgradeTmpDir   = "/userdata/.upgrade"
	// Keep upgrade metadata (checksum/version/url files) on persistent storage; /tmp may be too small/volatile.
	UpgradeFilesDir = "/userdata/.upgrade-files"
	WorkDir         = "/tmp/supervisor"
)

// Work directory files
const (
	UpgradeCancelFile = WorkDir + "/upgrade.cancel"
	UpgradeDoneFile   = WorkDir + "/upgrade.done"
	UpgradeMutexFile  = WorkDir + "/upgrade.mutex"
	UpgradeProgFile   = WorkDir + "/upgrade.prog"
	UpgradeProgTmp    = WorkDir + "/upgrade.prog.tmp"
	// UpdateCheckProgFile stores progress/status for "check for updates" (distinct from OTA download/flash progress).
	UpdateCheckProgFile = WorkDir + "/update_check.prog"
)

// UpdateStatus represents the status of update availability.
type UpdateStatus int

const (
	UpdateStatusAvailable UpdateStatus = 1
	UpdateStatusUpgrading UpdateStatus = 2
	UpdateStatusQuerying  UpdateStatus = 3
)

// UpdateVersion represents available update information.
type UpdateVersion struct {
	OSName    string       `json:"osName"`
	OSVersion string       `json:"osVersion"`
	CheckedAt int64        `json:"checkedAt,omitempty"`
	Status    UpdateStatus `json:"status"`
	// Error captures the last update-check error (if any). When set, osName/osVersion may
	// reflect the last known cached value.
	Error string `json:"error,omitempty"`
}

// UpdateProgress represents upgrade progress.
type UpdateProgress struct {
	Progress int    `json:"progress"`
	Status   string `json:"status"` // download, upgrade, idle, cancelled
}

// VersionInfo from Checksum file
type VersionInfo struct {
	FileName string
	Checksum string
	OSName   string
	Version  string
}

// UploadedPackageType identifies the kind of locally uploaded update payload.
//
// These are intentionally stringly-typed because they cross the API boundary
// to the web UI.
type UploadedPackageType string

const (
	UploadedPackageTypeOTAZip     UploadedPackageType = "ota_zip"
	UploadedPackageTypeSWU        UploadedPackageType = "swu"
	UploadedPackageTypeUpgradeZip UploadedPackageType = "upgrade_zip"
)

// StagedPackageInfo describes an OTA package that has been uploaded to the device
// and staged for installation.
type StagedPackageInfo struct {
	Exists      bool                `json:"exists"`
	FileName    string              `json:"fileName"`
	Checksum    string              `json:"checksum"`
	OSName      string              `json:"osName"`
	Version     string              `json:"version"`
	Size        int64               `json:"size"`
	PackageType UploadedPackageType `json:"packageType,omitempty"`
}

// UpgradeManager handles system upgrades.
type UpgradeManager struct {
	mu          sync.RWMutex
	mountPath   string
	downloading bool
	upgrading   bool
	cancelled   bool
	querying    bool
}

// NewUpgradeManager creates a new upgrade manager.
func NewUpgradeManager() *UpgradeManager {
	// Ensure directories exist
	os.MkdirAll(WorkDir, 0700)
	os.MkdirAll(ConfigDir, 0755)
	os.MkdirAll(UpgradeTmpDir, 0755)
	os.MkdirAll(UpgradeFilesDir, 0755)
	return &UpgradeManager{}
}

func uploadedPackageMetaPath() string {
	return filepath.Join(UpgradeFilesDir, UploadedPackageMetaFileName)
}

// SaveUploadedPackageInfo persists the uploaded package metadata used by the UI.
func SaveUploadedPackageInfo(info *StagedPackageInfo) error {
	if info == nil {
		return fmt.Errorf("nil uploaded package info")
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(uploadedPackageMetaPath(), data, 0644)
}

func loadUploadedPackageInfo() (*StagedPackageInfo, error) {
	data, err := os.ReadFile(uploadedPackageMetaPath())
	if err != nil {
		return nil, err
	}
	var info StagedPackageInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetStagedLocalPackageInfo returns metadata about an uploaded OTA package, if present.
func (m *UpgradeManager) GetStagedLocalPackageInfo() (*StagedPackageInfo, error) {
	if meta, err := loadUploadedPackageInfo(); err == nil && meta != nil && meta.Exists && meta.FileName != "" {
		p := filepath.Join(UpgradeTmpDir, meta.FileName)
		st, statErr := os.Stat(p)
		if statErr != nil {
			return &StagedPackageInfo{Exists: false}, nil
		}
		meta.Size = st.Size()
		if meta.PackageType == "" {
			meta.PackageType = UploadedPackageTypeOTAZip
		}
		return meta, nil
	}

	checksumPath := filepath.Join(UpgradeFilesDir, ChecksumFileName)
	info, err := m.parseVersionInfo(checksumPath)
	if err != nil {
		return &StagedPackageInfo{Exists: false}, nil
	}

	zipPath := filepath.Join(UpgradeTmpDir, info.FileName)
	st, err := os.Stat(zipPath)
	if err != nil {
		return &StagedPackageInfo{Exists: false}, nil
	}

	return &StagedPackageInfo{
		Exists:      true,
		FileName:    info.FileName,
		Checksum:    info.Checksum,
		OSName:      info.OSName,
		Version:     info.Version,
		Size:        st.Size(),
		PackageType: UploadedPackageTypeOTAZip,
	}, nil
}

// UpdateSystemFromUploadedPackage starts an update using the currently staged uploaded package.
//
// This supports multiple package types (OTA zip, SWUpdate .swu, and upgrade/emmc zip).
func (m *UpgradeManager) UpdateSystemFromUploadedPackage() error {
	if m.IsUpgrading() {
		return fmt.Errorf("upgrade in progress")
	}
	info, err := m.GetStagedLocalPackageInfo()
	if err != nil {
		return err
	}
	if info == nil || !info.Exists || info.FileName == "" {
		return fmt.Errorf("no staged update package")
	}

	path := filepath.Join(UpgradeTmpDir, info.FileName)
	if _, statErr := os.Stat(path); statErr != nil {
		return fmt.Errorf("staged package not found")
	}

	os.Remove(UpgradeCancelFile)
	os.Remove(UpgradeDoneFile)
	os.Remove(UpgradeProgFile)

	m.cancelled = false

	switch info.PackageType {
	case "", UploadedPackageTypeOTAZip:
		return m.UpdateSystemFromLocal()
	case UploadedPackageTypeSWU:
		go m.performSWUUpgrade(path)
		return nil
	case UploadedPackageTypeUpgradeZip:
		go m.performUpgradeFromUpgradeZip(path)
		return nil
	default:
		return fmt.Errorf("unsupported package type: %s", info.PackageType)
	}
}

func (m *UpgradeManager) performSWUUpgrade(swuPath string) {
	m.mu.Lock()
	m.downloading = false
	m.upgrading = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.upgrading = false
		m.mu.Unlock()
	}()

	m.updateProgress(0, "swupdate: starting")

	// Select the correct image set based on the currently running slot.
	//
	// Our generated sw-description defines two sets:
	// - stable:a_write_b (write /dev/mmcblk0p4, set use_part_b=1)
	// - stable:b_write_a (write /dev/mmcblk0p3, set use_part_b=0)
	selection := "stable,a_write_b"
	if m.isRootFSB() {
		selection = "stable,b_write_a"
	}

	// Run swupdate directly (no daemon required). Use verbose mode to improve diagnosability.
	cmd := exec.Command("swupdate", "-v", "-i", swuPath, "-e", selection)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		logger.Error("SWUpdate failed: %v, output: %s", err, msg)
		m.updateProgress(0, "failed: "+truncateStatusError(fmt.Errorf("%v: %s", err, msg)))
		return
	}

	_ = os.Remove(swuPath)
	os.WriteFile(UpgradeDoneFile, []byte("1"), 0644)
	m.updateProgress(100, "upgrade: rebooting")
	m.scheduleReboot()
}

func (m *UpgradeManager) performUpgradeFromUpgradeZip(zipPath string) {
	m.mu.Lock()
	m.downloading = false
	m.upgrading = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.upgrading = false
		m.mu.Unlock()
	}()

	m.updateProgress(0, "upgrade_zip: preparing")

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		logger.Error("Failed to open upgrade zip: %v", err)
		m.updateProgress(0, "failed: "+truncateStatusError(err))
		return
	}
	defer zr.Close()

	find := func(name string) *zip.File {
		for _, f := range zr.File {
			// Some build pipelines may zip with a directory prefix; match by basename.
			if f.Name == name || filepath.Base(f.Name) == name {
				return f
			}
		}
		return nil
	}

	targetRootFS := RootFSB
	if m.isRootFSB() {
		targetRootFS = RootFS
	}

	// Rootfs is required
	rootfs := find("rootfs_ext4.emmc")
	if rootfs == nil {
		err := fmt.Errorf("rootfs_ext4.emmc not found in package")
		m.updateProgress(0, "failed: "+truncateStatusError(err))
		return
	}
	// Safety: only write the inactive rootfs slot.
	// We intentionally do NOT write boot.emmc / fip.bin here; those are recovery-style artifacts and
	// updating bootloader/boot partitions from a running system is high-risk.
	if err := m.updatePartitionFromZipNoVerify(rootfs, targetRootFS, false, 10, 90, "upgrade_zip: writing rootfs"); err != nil {
		logger.Error("upgrade_zip: rootfs update failed: %v", err)
		m.updateProgress(0, "failed: "+truncateStatusError(err))
		return
	}

	m.updateProgress(90, "upgrade_zip: switching slot")
	if err := m.switchPartition(); err != nil {
		m.updateProgress(0, "failed: "+truncateStatusError(err))
		return
	}

	_ = os.Remove(zipPath)
	os.WriteFile(UpgradeDoneFile, []byte("1"), 0644)
	m.updateProgress(100, "upgrade: rebooting")
	m.scheduleReboot()
}

// updatePartitionFromZipNoVerify streams a zip entry directly into a block device without requiring a checksum file.
func (m *UpgradeManager) updatePartitionFromZipNoVerify(zipFile *zip.File, partition string, isFIP bool, progressStart, progressEnd int, status string) error {
	if zipFile == nil {
		return fmt.Errorf("nil zip entry")
	}

	if isFIP {
		_ = os.WriteFile(ForceROPath, []byte("0"), 0644)
		defer os.WriteFile(ForceROPath, []byte("1"), 0644)
	}

	rc, err := zipFile.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	part, err := os.OpenFile(partition, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer part.Close()

	sha := sha256.New()
	writer := io.MultiWriter(part, sha)

	total := int64(zipFile.UncompressedSize64)
	var written int64
	buf := make([]byte, 256*1024)
	lastUpdate := time.Now()
	for {
		if m.cancelled {
			return fmt.Errorf("cancelled")
		}
		n, rerr := rc.Read(buf)
		if n > 0 {
			if _, werr := writer.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if total > 0 {
				pct := progressStart + int(float64(written)/float64(total)*float64(progressEnd-progressStart))
				if time.Since(lastUpdate) > 250*time.Millisecond {
					m.updateProgress(pct, fmt.Sprintf("%s %s/%s", status, formatMiB(written), formatMiB(total)))
					lastUpdate = time.Now()
				}
			} else if time.Since(lastUpdate) > 500*time.Millisecond {
				// Some zip entries may not report an uncompressed size; still show activity.
				m.updateProgress(progressStart, fmt.Sprintf("%s %s", status, formatMiB(written)))
				lastUpdate = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	m.updateProgress(progressEnd, fmt.Sprintf("%s done", status))
	logger.Info("Updated %s with %s (%d bytes, sha256=%s)", partition, zipFile.Name, written, hex.EncodeToString(sha.Sum(nil)))
	return nil
}

// UpdateSystemFromLocal starts an update using a locally-uploaded (staged) OTA zip.
// The OTA zip is expected to be present in UpgradeTmpDir and referenced by the checksum
// manifest at UpgradeFilesDir/ChecksumFileName.
func (m *UpgradeManager) UpdateSystemFromLocal() error {
	if m.IsUpgrading() {
		return fmt.Errorf("upgrade in progress")
	}

	// Ensure a staged package exists before we kick off the upgrade.
	info, err := m.GetStagedLocalPackageInfo()
	if err != nil {
		return err
	}
	if info == nil || !info.Exists {
		return fmt.Errorf("no staged OTA package")
	}
	if info.PackageType != "" && info.PackageType != UploadedPackageTypeOTAZip {
		return fmt.Errorf("staged package is not an OTA zip")
	}

	// Clear state files
	os.Remove(UpgradeCancelFile)
	os.Remove(UpgradeDoneFile)
	os.Remove(UpgradeProgFile)

	m.cancelled = false

	go m.performLocalUpgrade()
	return nil
}

// performLocalUpgrade performs an upgrade from a locally-uploaded OTA zip.
func (m *UpgradeManager) performLocalUpgrade() {
	m.mu.Lock()
	m.downloading = false
	m.upgrading = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.upgrading = false
		m.mu.Unlock()
	}()

	// Upgrade phase only (skip download)
	m.updateProgress(0, "upgrade: preparing")
	if err := m.performOTAUpgradeWithPlan(localOTAProgressPlan()); err != nil {
		logger.Error("Local upgrade failed: %v", err)
		if err.Error() == "cancelled" || m.cancelled {
			return
		}
		m.updateProgress(0, "failed: "+truncateStatusError(err))
		return
	}

	// Mark done, then reboot.
	os.WriteFile(UpgradeDoneFile, []byte("1"), 0644)
	m.updateProgress(100, "upgrade: rebooting")
	m.scheduleReboot()
}

// IsUpgrading checks if an upgrade is in progress.
func (m *UpgradeManager) IsUpgrading() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upgrading || m.downloading
}

// IsUpgradeDone checks if an upgrade has completed and requires restart.
func (m *UpgradeManager) IsUpgradeDone() bool {
	_, err := os.Stat(UpgradeDoneFile)
	return err == nil
}

// GetChannel returns the current upgrade channel and server URL.
func (m *UpgradeManager) GetChannel() (channel int, url string) {
	data, err := os.ReadFile(UpgradeConfFile)
	if err != nil {
		return 0, ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), ",", 2)
	if len(parts) >= 1 {
		channel, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		url = parts[1]
	}
	return
}

// GetChannelForDisplay returns the channel and base URL for display in the UI.
// This strips the checksum filename if present to show only the base URL.
func (m *UpgradeManager) GetChannelForDisplay() (channel int, url string) {
	channel, url = m.GetChannel()
	// Remove the checksum filename suffix for display
	if strings.HasSuffix(url, ChecksumFileName) {
		url = strings.TrimSuffix(url, "/"+ChecksumFileName)
	}
	return
}

// UpdateChannel sets the upgrade channel and server URL.
func (m *UpgradeManager) UpdateChannel(channel int, url string) error {
	logger.Info("UpdateChannel: channel=%d, url=%s", channel, url)
	if m.IsUpgrading() {
		return fmt.Errorf("upgrade in progress")
	}

	// Validate custom URL if provided
	if url != "" {
		// If URL doesn't end with the checksum file, append it
		if !strings.HasSuffix(url, ChecksumFileName) {
			// Remove trailing slash if present
			url = strings.TrimSuffix(url, "/")
			url = url + "/" + ChecksumFileName
		}
		logger.Info("Normalized URL to: %s", url)
	}

	content := strconv.Itoa(channel)
	if url != "" {
		content += "," + url
	}
	if err := os.WriteFile(UpgradeConfFile, []byte(content), 0644); err != nil {
		return err
	}

	// Clean up upgrade state
	m.Clean()

	// Trigger immediate query of latest version
	if url != "" {
		go m.QueryLatestVersion()
	}

	return nil
}

// Clean removes all upgrade files.
func (m *UpgradeManager) Clean() {
	os.RemoveAll(UpgradeFilesDir)
	os.MkdirAll(UpgradeFilesDir, 0755)
	files, _ := filepath.Glob(UpgradeTmpDir + "/*")
	for _, f := range files {
		os.RemoveAll(f)
	}
}

// GetSystemUpdateVersion checks for available updates.
func (m *UpgradeManager) GetSystemUpdateVersion() (*UpdateVersion, error) {
	return m.GetSystemUpdateVersionWithOptions(false)
}

// GetSystemUpdateVersionWithOptions checks for updates, optionally forcing a refresh.
//
// When force is true, a background query is triggered even if cached version.json exists.
// The returned payload may still include the cached version, but status will be "querying"
// while the refresh is in progress.
func (m *UpgradeManager) GetSystemUpdateVersionWithOptions(force bool) (*UpdateVersion, error) {
	result := &UpdateVersion{Status: UpdateStatusQuerying}

	if m.IsUpgrading() {
		result.Status = UpdateStatusUpgrading
		return result, nil
	}

	// Check for cached version info
	versionFile := filepath.Join(UpgradeFilesDir, "version.json")
	if data, err := os.ReadFile(versionFile); err == nil {
		var cached UpdateVersion
		if json.Unmarshal(data, &cached) == nil {
			if cached.CheckedAt == 0 {
				if st, statErr := os.Stat(versionFile); statErr == nil {
					cached.CheckedAt = st.ModTime().Unix()
				}
			}

			// If we have *any* meaningful cached state (version OR prior error), return it.
			if cached.OSName != "" || cached.OSVersion != "" || cached.Error != "" || cached.CheckedAt != 0 {
				if force {
					m.startQueryLatestVersion()
					cached.Status = UpdateStatusQuerying
					return &cached, nil
				}
				// Normal cached response.
				if cached.Status == 0 {
					cached.Status = UpdateStatusAvailable
				}
				return &cached, nil
			}
		}
	}

	// Start background query if not running
	_ = m.startQueryLatestVersion()

	return result, nil
}

// startQueryLatestVersion triggers a background update-check if one is not already running.
func (m *UpgradeManager) startQueryLatestVersion() bool {
	m.mu.RLock()
	already := m.querying
	m.mu.RUnlock()
	if already {
		return false
	}

	// Reset check progress for the new run.
	_ = os.Remove(UpdateCheckProgFile)
	m.updateCheckProgress(0, "checking: starting")

	go func() {
		_ = m.QueryLatestVersion()
	}()
	return true
}

// QueryLatestVersion queries for the latest version in background.
func (m *UpgradeManager) QueryLatestVersion() error {
	// Single-flight guard. (QueryLatestVersion may be called by cron and by the UI.)
	m.mu.Lock()
	if m.querying {
		m.mu.Unlock()
		return nil
	}
	m.querying = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.querying = false
		m.mu.Unlock()
	}()

	channel, customURL := m.GetChannel()
	logger.Info("QueryLatestVersion: channel=%d, customURL=%s", channel, customURL)
	m.updateCheckProgress(5, "checking: preparing")

	var checksumURL string
	if customURL != "" {
		checksumURL = customURL
		logger.Info("Using custom URL: %s", checksumURL)
	} else {
		// Parse GitHub releases URL
		m.updateCheckProgress(10, "checking: resolving latest release")
		url, err := m.parseGitHubReleasesURL(OfficialURL)
		if err != nil {
			logger.Error("Failed to parse GitHub URL: %v", err)
			m.saveVersionError(err)
			m.updateCheckProgress(100, "failed: "+truncateStatusError(err))
			return err
		}
		checksumURL = url
		logger.Info("Using official URL: %s", checksumURL)
	}

	// Download Checksum file
	checksumPath := filepath.Join(UpgradeFilesDir, ChecksumFileName)
	logger.Info("Downloading checksum from %s to %s", checksumURL, checksumPath)
	m.updateCheckProgress(15, "checking: downloading checksum")
	if err := m.downloadFileWithProgress(checksumURL, checksumPath, 15, 70); err != nil {
		logger.Error("Failed to download checksum file: %v", err)
		m.saveVersionError(err)
		m.updateCheckProgress(100, "failed: "+truncateStatusError(err))
		return err
	}
	logger.Info("Successfully downloaded checksum file")
	m.updateCheckProgress(75, "checking: parsing checksum")

	// Parse version info
	info, err := m.parseVersionInfo(checksumPath)
	if err != nil {
		logger.Error("Failed to parse version info: %v", err)
		m.saveVersionError(err)
		m.updateCheckProgress(100, "failed: "+truncateStatusError(err))
		return err
	}
	logger.Info("Parsed version info: OS=%s, Version=%s, File=%s", info.OSName, info.Version, info.FileName)

	// Save version info
	versionFile := filepath.Join(UpgradeFilesDir, "version.json")
	versionData := UpdateVersion{
		OSName:    info.OSName,
		OSVersion: info.Version,
		CheckedAt: time.Now().Unix(),
		Status:    UpdateStatusAvailable,
		Error:     "",
	}
	m.updateCheckProgress(90, "checking: saving version")
	data, _ := json.Marshal(versionData)
	os.WriteFile(versionFile, data, 0644)
	logger.Info("Saved version info to %s", versionFile)

	// Save URL for download
	baseURL := strings.TrimSuffix(checksumURL, "/"+ChecksumFileName)
	os.WriteFile(filepath.Join(UpgradeFilesDir, URLFileName), []byte(baseURL), 0644)
	logger.Info("Saved base URL: %s", baseURL)

	m.updateCheckProgress(100, "checking: done")

	return nil
}

// parseGitHubReleasesURL parses GitHub releases URL to get Checksum file URL.
func (m *UpgradeManager) parseGitHubReleasesURL(url string) (string, error) {
	resp, err := m.getWithTLSFallback(url, true)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Get the Location header for redirect
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no redirect location found")
	}

	// Convert tag URL to download URL
	// https://github.com/.../releases/tag/v1.0.0 -> https://github.com/.../releases/download/v1.0.0
	location = strings.Replace(location, "/tag/", "/download/", 1)
	return location + "/" + ChecksumFileName, nil
}

// parseVersionInfo parses the Checksum file to extract version information.
func (m *UpgradeManager) parseVersionInfo(checksumPath string) (*VersionInfo, error) {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return nil, err
	}

	// Parse line: <sha256sum>  <filename>_<os>_<version>_ota.zip
	re := regexp.MustCompile(`([a-f0-9]{64})\s+(\S+ota\.zip)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid Checksum file format")
	}

	info := &VersionInfo{
		Checksum: matches[1],
		FileName: matches[2],
	}

	// Parse filename: sg2002_reCamera_1.0.0_ota.zip
	parts := strings.Split(info.FileName, "_")
	if len(parts) >= 3 {
		info.OSName = parts[1]
		info.Version = parts[2]
	}

	return info, nil
}

// GetUpdateProgress returns the current upgrade progress.
func (m *UpgradeManager) GetUpdateProgress() (*UpdateProgress, error) {
	// Check if cancelled
	if _, err := os.Stat(UpgradeCancelFile); err == nil {
		return &UpdateProgress{Progress: 0, Status: "cancelled"}, nil
	}

	// Read progress file
	data, err := os.ReadFile(UpgradeProgFile)
	if err != nil || len(data) == 0 {
		return &UpdateProgress{Progress: 0, Status: "idle"}, nil
	}

	var progress UpdateProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return &UpdateProgress{Progress: 0, Status: "idle"}, nil
	}

	return &progress, nil
}

// GetUpdateCheckProgress returns the progress/status for "check for updates".
func (m *UpgradeManager) GetUpdateCheckProgress() (*UpdateProgress, error) {
	data, err := os.ReadFile(UpdateCheckProgFile)
	if err != nil || len(data) == 0 {
		return &UpdateProgress{Progress: 0, Status: "idle"}, nil
	}
	var progress UpdateProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return &UpdateProgress{Progress: 0, Status: "idle"}, nil
	}
	return &progress, nil
}

// UpdateSystem initiates a system update.
func (m *UpgradeManager) UpdateSystem() error {
	// Clear state files
	os.Remove(UpgradeCancelFile)
	os.Remove(UpgradeDoneFile)
	os.Remove(UpgradeProgFile)

	m.cancelled = false

	// Start download and upgrade in background
	go m.performUpgrade()

	return nil
}

// UpdateSystemSync performs a system update synchronously.
//
// This is intended for scheduled/cron execution paths where the process must
// remain alive for the upgrade to complete. (If we started the upgrade in a
// goroutine and then exited, the upgrade would be killed immediately.)
func (m *UpgradeManager) UpdateSystemSync() error {
	if m.IsUpgrading() {
		return fmt.Errorf("upgrade in progress")
	}

	// Clear state files
	os.Remove(UpgradeCancelFile)
	os.Remove(UpgradeDoneFile)
	os.Remove(UpgradeProgFile)

	m.cancelled = false

	// Run the upgrade in-process (blocks until finished or until the device reboots).
	m.performUpgrade()
	return nil
}

// performUpgrade performs the full upgrade process.
func (m *UpgradeManager) performUpgrade() {
	m.mu.Lock()
	m.downloading = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.upgrading = false
		m.mu.Unlock()
	}()

	// Download phase
	m.updateProgress(0, "download")
	if err := m.downloadOTA(); err != nil {
		logger.Error("Download failed: %v", err)
		if err.Error() == "cancelled" || m.cancelled {
			return
		}
		m.updateProgress(0, "failed: "+truncateStatusError(err))
		return
	}

	// Check for cancellation
	if m.cancelled {
		return
	}

	m.mu.Lock()
	m.downloading = false
	m.upgrading = true
	m.mu.Unlock()

	// Upgrade phase
	m.updateProgress(50, "upgrade: preparing")
	if err := m.performOTAUpgradeWithPlan(defaultOTAProgressPlan()); err != nil {
		logger.Error("Upgrade failed: %v", err)
		if err.Error() == "cancelled" || m.cancelled {
			return
		}
		m.updateProgress(50, "failed: "+truncateStatusError(err))
		return
	}

	// Mark done, then reboot.
	os.WriteFile(UpgradeDoneFile, []byte("1"), 0644)
	m.updateProgress(100, "upgrade: rebooting")
	m.scheduleReboot()
}

// downloadOTA downloads the OTA package.
func (m *UpgradeManager) downloadOTA() error {
	// Get version info
	checksumPath := filepath.Join(UpgradeFilesDir, ChecksumFileName)
	info, err := m.parseVersionInfo(checksumPath)
	if err != nil {
		// Try to query latest first
		if err := m.QueryLatestVersion(); err != nil {
			return err
		}
		info, err = m.parseVersionInfo(checksumPath)
		if err != nil {
			return err
		}
	}

	// Get download URL
	urlData, err := os.ReadFile(filepath.Join(UpgradeFilesDir, URLFileName))
	if err != nil {
		return fmt.Errorf("no URL file found")
	}
	baseURL := strings.TrimSpace(string(urlData))
	downloadURL := baseURL + "/" + info.FileName

	// Download to /userdata/.upgrade directory (not recovery partition)
	// This avoids "no space left" errors on small recovery partitions
	// Show real progress on the Updates page (download phase is 0-45%).
	tmpPath := filepath.Join(UpgradeTmpDir, info.FileName)
	logger.Info("Downloading OTA package to %s", tmpPath)
	if err := m.downloadFileWithUpgradeProgress(downloadURL, tmpPath, 0, 45); err != nil {
		return err
	}

	// Verify Checksum
	m.updateProgress(45, "download: verifying checksum")
	logger.Info("Verifying checksum of downloaded file")
	if err := m.verifyChecksum(tmpPath, info.Checksum); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Verify firmware signature (if enabled)
	m.updateProgress(47, "download: verifying signature")
	if err := m.VerifyDownloadedFirmware(downloadURL, tmpPath); err != nil {
		logger.Error("Firmware signature verification failed: %v", err)
		os.Remove(tmpPath)
		return err
	}

	logger.Info("OTA package downloaded and verified successfully")
	return nil
}

// downloadFileWithUpgradeProgress downloads a file while updating the OTA progress file
// (used by the Updates page to show download progress).
func (m *UpgradeManager) downloadFileWithUpgradeProgress(url, destPath string, progressStart, progressEnd int) error {
	resp, err := m.getWithTLSFallback(url, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	totalSize := resp.ContentLength
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	m.updateProgress(progressStart, "download: starting")

	var downloaded int64
	buf := make([]byte, 32*1024)
	lastUpdate := time.Now()
	for {
		if m.cancelled {
			return fmt.Errorf("cancelled")
		}

		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)

			// Rate-limit status updates to keep the progress file writes reasonable.
			if time.Since(lastUpdate) > 250*time.Millisecond {
				if totalSize > 0 {
					pct := progressStart + int(float64(downloaded)/float64(totalSize)*float64(progressEnd-progressStart))
					m.updateProgress(pct, fmt.Sprintf("download: %s/%s", formatMiB(downloaded), formatMiB(totalSize)))
				} else {
					m.updateProgress(progressStart, fmt.Sprintf("download: %s", formatMiB(downloaded)))
				}
				lastUpdate = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	m.updateProgress(progressEnd, "download: done")
	return nil
}

type otaProgressPlan struct {
	FIPStart     int
	FIPEnd       int
	BootStart    int
	BootEnd      int
	RootfsStart  int
	RootfsEnd    int
	SwitchProg   int
	FinalizeProg int
}

func defaultOTAProgressPlan() otaProgressPlan {
	return otaProgressPlan{
		FIPStart:     55,
		FIPEnd:       60,
		BootStart:    60,
		BootEnd:      65,
		RootfsStart:  65,
		RootfsEnd:    95,
		SwitchProg:   95,
		FinalizeProg: 98,
	}
}

// local (staged) OTA updates skip the download phase; start the visible progress at 0%.
func localOTAProgressPlan() otaProgressPlan {
	return otaProgressPlan{
		FIPStart:     5,
		FIPEnd:       10,
		BootStart:    10,
		BootEnd:      20,
		RootfsStart:  20,
		RootfsEnd:    90,
		SwitchProg:   90,
		FinalizeProg: 95,
	}
}

// performOTAUpgrade writes the OTA update to partitions.
func (m *UpgradeManager) performOTAUpgrade() error {
	return m.performOTAUpgradeWithPlan(defaultOTAProgressPlan())
}

// performOTAUpgradeWithPlan writes the OTA update to partitions using a caller-supplied progress plan.
func (m *UpgradeManager) performOTAUpgradeWithPlan(plan otaProgressPlan) error {
	// Get version info to find the OTA file
	checksumPath := filepath.Join(UpgradeFilesDir, ChecksumFileName)
	info, err := m.parseVersionInfo(checksumPath)
	if err != nil {
		return err
	}

	// Read OTA file from /userdata/.upgrade (not recovery partition)
	zipPath := filepath.Join(UpgradeTmpDir, info.FileName)
	if _, err := os.Stat(zipPath); err != nil {
		return fmt.Errorf("OTA file not found: %s", zipPath)
	}

	logger.Info("Opening OTA package from %s", zipPath)

	// Open zip file
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	// Read Checksum sums from zip
	checksumMap, err := m.readZipChecksum(zipReader)
	if err != nil {
		return err
	}

	// Update FIP partition
	m.updateProgress(plan.FIPStart, "upgrade: writing fip")
	if err := m.updatePartition(zipReader, FIPPart, "fip.bin", checksumMap, true, plan.FIPStart, plan.FIPEnd, "upgrade: writing fip"); err != nil {
		logger.Warning("FIP update skipped: %v", err)
	}

	// Update boot partition
	m.updateProgress(plan.BootStart, "upgrade: writing boot")
	if err := m.updatePartition(zipReader, BootPart, "boot.emmc", checksumMap, false, plan.BootStart, plan.BootEnd, "upgrade: writing boot"); err != nil {
		logger.Warning("Boot update skipped: %v", err)
	}

	// Determine target rootfs partition (opposite of current)
	targetRootFS := RootFSB
	if m.isRootFSB() {
		targetRootFS = RootFS
	}

	// Update rootfs
	m.updateProgress(plan.RootfsStart, "upgrade: writing rootfs")
	if err := m.updatePartition(zipReader, targetRootFS, "rootfs_ext4.emmc", checksumMap, false, plan.RootfsStart, plan.RootfsEnd, "upgrade: writing rootfs"); err != nil {
		return fmt.Errorf("rootfs update failed: %v", err)
	}

	// Switch boot partition
	m.updateProgress(plan.SwitchProg, "upgrade: switching slot")
	if err := m.switchPartition(); err != nil {
		return err
	}
	m.updateProgress(plan.FinalizeProg, "upgrade: finalizing")

	// Clean up OTA file after successful upgrade
	logger.Info("Cleaning up OTA file: %s", zipPath)
	os.Remove(zipPath)

	return nil
}

// mountRecovery mounts the recovery partition.
func (m *UpgradeManager) mountRecovery() error {
	// Check if already ext4
	out, _ := exec.Command("blkid", "-o", "value", "-s", "TYPE", RecvPart).Output()
	if strings.TrimSpace(string(out)) != "ext4" {
		// Format as ext4
		if err := exec.Command("mkfs.ext4", "-F", RecvPart).Run(); err != nil {
			return fmt.Errorf("format recovery partition failed: %v", err)
		}
	}

	// Create mount point
	m.mountPath = filepath.Join("/tmp", "recovery_mount")
	os.MkdirAll(m.mountPath, 0755)

	// Mount
	if err := exec.Command("mount", RecvPart, m.mountPath).Run(); err != nil {
		return fmt.Errorf("mount recovery partition failed: %v", err)
	}

	return nil
}

// unmountRecovery unmounts the recovery partition.
func (m *UpgradeManager) unmountRecovery() {
	if m.mountPath != "" {
		exec.Command("umount", m.mountPath).Run()
		os.RemoveAll(m.mountPath)
		m.mountPath = ""
	}
}

// isRootFSB checks if current root is on partition B.
func (m *UpgradeManager) isRootFSB() bool {
	out, err := exec.Command("mountpoint", "-n", "/").Output()
	if err != nil {
		return false
	}
	device := strings.Fields(string(out))[0]
	// Resolve symlinks
	resolved, _ := filepath.EvalSymlinks(device)
	resolvedB, _ := filepath.EvalSymlinks(RootFSB)
	return resolved == resolvedB
}

// switchPartition switches the boot partition.
func (m *UpgradeManager) switchPartition() error {
	partB := 0
	if m.isRootFSB() {
		partB = 1
	}
	partB = 1 - partB // Switch to opposite

	// Set U-Boot environment
	if err := exec.Command("fw_setenv", "use_part_b", strconv.Itoa(partB)).Run(); err != nil {
		return err
	}
	if err := exec.Command("fw_setenv", "boot_cnt", "0").Run(); err != nil {
		return err
	}
	if err := exec.Command("fw_setenv", "boot_failed_limits", "5").Run(); err != nil {
		return err
	}
	exec.Command("fw_setenv", "boot_rollback").Run() // Clear rollback flag

	return nil
}

// readZipChecksum reads SHA256 sums from sha256sum.txt inside the zip.
func (m *UpgradeManager) readZipChecksum(zipReader *zip.ReadCloser) (map[string]string, error) {
	checksumMap := make(map[string]string)

	for _, f := range zipReader.File {
		if f.Name == "sha256sum.txt" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			scanner := bufio.NewScanner(rc)
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) >= 2 {
					checksumMap[fields[1]] = fields[0]
				}
			}
			break
		}
	}

	return checksumMap, nil
}

// downloadFile downloads a file from a URL to a destination path.
func (m *UpgradeManager) downloadFile(url, destPath string) error {
	logger.Info("Downloading file from %s", url)
	resp, err := m.getWithTLSFallback(url, false)
	if err != nil {
		logger.Error("HTTP GET failed for %s: %v", url, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("HTTP error for %s: status=%d", url, resp.StatusCode)
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	out, err := os.Create(destPath)
	if err != nil {
		logger.Error("Failed to create file %s: %v", destPath, err)
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		logger.Error("Failed to write file %s: %v", destPath, err)
		return err
	}

	logger.Info("Successfully downloaded %d bytes to %s", written, destPath)
	return nil
}

// verifyChecksum verifies the SHA256 checksum of a file.
func (m *UpgradeManager) verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("Checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}
	return nil
}

// formatMiB formats bytes as mebibytes.
func formatMiB(b int64) string {
	return fmt.Sprintf("%.1fMiB", float64(b)/1024.0/1024.0)
}

// downloadFileWithProgress downloads a file with progress tracking.
func (m *UpgradeManager) downloadFileWithProgress(url, destPath string, progressStart, progressEnd int) error {
	resp, err := m.getWithTLSFallback(url, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	totalSize := resp.ContentLength
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	var downloaded int64
	buf := make([]byte, 32*1024)
	lastUpdate := time.Now()
	for {
		if m.cancelled {
			return fmt.Errorf("cancelled")
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)

			if totalSize > 0 {
				pct := progressStart + int(float64(downloaded)/float64(totalSize)*float64(progressEnd-progressStart))
				// Rate-limit status updates to keep the progress file writes reasonable.
				if time.Since(lastUpdate) > 250*time.Millisecond {
					m.updateCheckProgress(pct, fmt.Sprintf("checking: download %s/%s", formatMiB(downloaded), formatMiB(totalSize)))
					lastUpdate = time.Now()
				}
			} else if time.Since(lastUpdate) > 500*time.Millisecond {
				m.updateCheckProgress(progressStart, fmt.Sprintf("checking: download %s", formatMiB(downloaded)))
				lastUpdate = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Ensure we end at the top of the download range.
	m.updateCheckProgress(progressEnd, "checking: download done")
	return nil
}

// updatePartition updates a partition from the zip file with progress tracking.
func (m *UpgradeManager) updatePartition(zipReader *zip.ReadCloser, partition, filename string, checksumMap map[string]string, isFIP bool, progressStart, progressEnd int, status string) error {
	// Find file in zip
	var zipFile *zip.File
	for _, f := range zipReader.File {
		// Some build pipelines may zip with a directory prefix; match by basename.
		if f.Name == filename || filepath.Base(f.Name) == filename {
			zipFile = f
			break
		}
	}
	if zipFile == nil {
		return fmt.Errorf("file not found in zip: %s", filename)
	}

	expectedChecksum := checksumMap[filename]
	if expectedChecksum == "" {
		return fmt.Errorf("no Checksum for: %s", filename)
	}

	// For FIP partition, we need to disable write protection
	if isFIP {
		_ = os.WriteFile(ForceROPath, []byte("0"), 0644)
		defer os.WriteFile(ForceROPath, []byte("1"), 0644)
	}

	// Open zip file
	rc, err := zipFile.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Open partition for writing
	part, err := os.OpenFile(partition, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer part.Close()

	// Create SHA256 hash writer
	hash := sha256.New()
	writer := io.MultiWriter(part, hash)

	total := int64(zipFile.UncompressedSize64)
	var written int64
	buf := make([]byte, 256*1024)
	lastUpdate := time.Now()
	for {
		if m.cancelled {
			return fmt.Errorf("cancelled")
		}

		n, rerr := rc.Read(buf)
		if n > 0 {
			if _, werr := writer.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)

			if time.Since(lastUpdate) > 250*time.Millisecond {
				if total > 0 {
					pct := progressStart + int(float64(written)/float64(total)*float64(progressEnd-progressStart))
					m.updateProgress(pct, fmt.Sprintf("%s %s/%s", status, formatMiB(written), formatMiB(total)))
				} else {
					// Unknown size; still show activity.
					m.updateProgress(progressStart, fmt.Sprintf("%s %s", status, formatMiB(written)))
				}
				lastUpdate = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	// Verify Checksum
	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("Checksum mismatch for %s: expected %s, got %s", filename, expectedChecksum, actualChecksum)
	}

	// Ensure we end at the top of this step's range.
	m.updateProgress(progressEnd, fmt.Sprintf("%s done", status))

	logger.Info("Updated %s with %s (%d bytes)", partition, filename, written)
	return nil
}

// saveVersionError persists the last check error into version.json while preserving any
// previously cached osName/osVersion (if present).
func (m *UpgradeManager) saveVersionError(err error) {
	versionFile := filepath.Join(UpgradeFilesDir, "version.json")
	var cached UpdateVersion
	if data, rerr := os.ReadFile(versionFile); rerr == nil {
		_ = json.Unmarshal(data, &cached)
	}
	cached.CheckedAt = time.Now().Unix()
	if cached.Status == 0 {
		cached.Status = UpdateStatusAvailable
	}
	cached.Error = truncateStatusError(err)
	data, _ := json.Marshal(cached)
	_ = os.WriteFile(versionFile, data, 0644)
}

func truncateStatusError(err error) string {
	if err == nil {
		return "unknown error"
	}
	s := strings.TrimSpace(err.Error())
	// Avoid writing huge error strings to the progress file.
	const max = 160
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

func (m *UpgradeManager) scheduleReboot() {
	// NOTE: this function is intentionally synchronous.
	//
	// UpdateSystemSync() is used by scheduled/cron paths and exits the process after the update.
	// If reboot were triggered in a goroutine, the process could exit before reboot executes.
	// Keeping this synchronous ensures reboot is attempted before returning.
	//
	// Small delay to ensure the progress file write is flushed.
	time.Sleep(2 * time.Second)

	if err := system.Reboot(); err == nil {
		return
	} else {
		// If we get here, reboot failed; leave the done marker so the UI can show it needs restart.
		m.updateProgress(100, "upgrade: reboot failed (manual reboot required)")
		logger.Error("Failed to reboot after upgrade; manual reboot required: %v", err)
	}
}

// updateProgress updates the progress file.
func (m *UpgradeManager) updateProgress(progress int, status string) {
	data, _ := json.Marshal(UpdateProgress{Progress: progress, Status: status})
	os.WriteFile(UpgradeProgFile, data, 0644)
}

// updateCheckProgress updates the update-check progress file.
func (m *UpgradeManager) updateCheckProgress(progress int, status string) {
	data, _ := json.Marshal(UpdateProgress{Progress: progress, Status: status})
	_ = os.WriteFile(UpdateCheckProgFile, data, 0644)
}

// CancelUpdate cancels an ongoing update.
func (m *UpgradeManager) CancelUpdate() error {
	m.cancelled = true
	os.WriteFile(UpgradeCancelFile, []byte("1"), 0644)
	os.Remove(UpgradeProgFile)
	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// Recovery sets the factory reset flag.
func (m *UpgradeManager) Recovery() error {
	return exec.Command("fw_setenv", "factory_reset", "1").Run()
}

func isTLSUnknownAuthority(err error) bool {
	if err == nil {
		return false
	}
	// Usually wrapped in *url.Error.
	var ua x509.UnknownAuthorityError
	if errors.As(err, &ua) {
		return true
	}
	return strings.Contains(err.Error(), "x509: certificate signed by unknown authority")
}

func isTrustedFallbackHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	// Allow fallback only for GitHub hosts used by official releases.
	if host == "github.com" || strings.HasSuffix(host, ".github.com") {
		return true
	}
	if strings.HasSuffix(host, "githubusercontent.com") {
		return true
	}
	return false
}

func newHTTPClient(timeout time.Duration, noRedirect bool, insecureTLS bool) *http.Client {
	tr, ok := http.DefaultTransport.(*http.Transport)
	var transport http.RoundTripper
	if ok {
		cloned := tr.Clone()
		if insecureTLS {
			cloned.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			// Use the platform cert pool which includes both system certs
			// and our custom TPR CA certificate
			cloned.TLSClientConfig = &tls.Config{RootCAs: tlsutil.PlatformCertPool()}
		}
		transport = cloned
	} else {
		transport = http.DefaultTransport
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	if noRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

// getWithTLSFallback performs an HTTP GET with normal TLS verification first.
// If the device lacks a CA bundle (common on minimal images), and the host is a
// known GitHub host, retry once with TLS verification disabled.
func (m *UpgradeManager) getWithTLSFallback(rawURL string, noRedirect bool) (*http.Response, error) {
	client := newHTTPClient(60*time.Second, noRedirect, false)
	resp, err := client.Get(rawURL)
	if err == nil {
		return resp, nil
	}
	if isTrustedFallbackHost(rawURL) && isTLSUnknownAuthority(err) {
		logger.Warning("TLS verification failed for %s; retrying with InsecureSkipVerify (install ca-certificates to avoid this)", rawURL)
		insecureClient := newHTTPClient(60*time.Second, noRedirect, true)
		return insecureClient.Get(rawURL)
	}
	return nil, err
}
