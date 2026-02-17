package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"supervisor/internal/api"
	"supervisor/internal/upgrade"
	"supervisor/pkg/logger"
)

// UpdateConfigHandler handles update configuration API requests.
type UpdateConfigHandler struct {
	configPath string
}

// UpdateConfig represents the update configuration.
type UpdateConfig struct {
	OSSource           string `json:"os_source"`    // "tpr_official" or "self_hosted"
	ModelSource        string `json:"model_source"` // "tpr_official" or "self_hosted"
	SelfHostedOSUrl    string `json:"self_hosted_os_url"`
	SelfHostedModelUrl string `json:"self_hosted_model_url"`
	CheckFrequency     string `json:"check_frequency"` // "30min", "daily", "weekly", "manual"
	WeeklyDay          string `json:"weekly_day"`      // "sunday", "monday", etc.
}

// NewUpdateConfigHandler creates a new UpdateConfigHandler.
func NewUpdateConfigHandler() *UpdateConfigHandler {
	return &UpdateConfigHandler{
		configPath: "/etc/supervisor/update.conf",
	}
}

// GetConfig returns the current update configuration.
func (h *UpdateConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.loadConfig()
	if err != nil {
		// Return default config if file doesn't exist
		config = h.getDefaultConfig()
	}

	api.WriteSuccess(w, config)
}

// SetConfig updates the update configuration.
func (h *UpdateConfigHandler) SetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var config UpdateConfig
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
		logger.Error("Failed to save update config: %v", err)
		api.WriteError(w, -1, "Failed to save configuration")
		return
	}

	// Apply OS-level schedule (cron) so the configured frequency actually runs.
	if err := applyUpdateCheckSchedule(&config); err != nil {
		logger.Error("Failed to apply update check schedule: %v", err)
		api.WriteError(w, -1, "Failed to apply update schedule")
		return
	}

	// Apply selected update source to the upgrade manager so checks/install use the correct URL.
	if err := applyUpdateSourceToUpgrade(&config); err != nil {
		logger.Error("Failed to apply update source: %v", err)
		api.WriteError(w, -1, "Failed to apply update source")
		return
	}

	api.WriteSuccess(w, map[string]interface{}{
		"status":  "success",
		"message": "Update configuration saved",
	})
}

func applyUpdateSourceToUpgrade(cfg *UpdateConfig) error {
	mgr := upgrade.NewUpgradeManager()
	url := ""
	if cfg != nil && cfg.OSSource == "self_hosted" {
		url = strings.TrimSpace(cfg.SelfHostedOSUrl)
	}
	if err := mgr.UpdateChannel(0, url); err != nil {
		return err
	}
	// Kick off a refresh so the UI can immediately show accurate "Latest" / "Last checked".
	go mgr.QueryLatestVersion()
	return nil
}

// loadConfig loads the update configuration from file.
func (h *UpdateConfigHandler) loadConfig() (*UpdateConfig, error) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return nil, err
	}

	var config UpdateConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// saveConfig saves the update configuration to file.
func (h *UpdateConfigHandler) saveConfig(config *UpdateConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(h.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.configPath, data, 0644)
}

// validateConfig validates the update configuration.
func (h *UpdateConfigHandler) validateConfig(config *UpdateConfig) error {
	// Validate OS source
	if config.OSSource != "tpr_official" && config.OSSource != "self_hosted" {
		return api.NewError("Invalid OS source. Must be 'tpr_official' or 'self_hosted'")
	}

	// Validate model source
	if config.ModelSource != "tpr_official" && config.ModelSource != "self_hosted" {
		return api.NewError("Invalid model source. Must be 'tpr_official' or 'self_hosted'")
	}

	// Validate self-hosted URLs if specified
	if config.OSSource == "self_hosted" && config.SelfHostedOSUrl != "" {
		if !isValidUpdateURL(config.SelfHostedOSUrl) {
			return api.NewError("Invalid OS update URL. Must be a valid http/https URL")
		}
	}
	if config.ModelSource == "self_hosted" && config.SelfHostedModelUrl != "" {
		if !isValidUpdateURL(config.SelfHostedModelUrl) {
			return api.NewError("Invalid model update URL. Must be a valid http/https URL")
		}
	}

	// Validate check frequency
	validFrequencies := map[string]bool{
		"1min":   true,
		"30min":  true,
		"daily":  true,
		"weekly": true,
		"manual": true,
	}
	if !validFrequencies[config.CheckFrequency] {
		return api.NewError("Invalid check frequency. Must be '1min', '30min', 'daily', 'weekly', or 'manual'")
	}

	// Validate weekly day if frequency is weekly
	if config.CheckFrequency == "weekly" {
		validDays := map[string]bool{
			"sunday":    true,
			"monday":    true,
			"tuesday":   true,
			"wednesday": true,
			"thursday":  true,
			"friday":    true,
			"saturday":  true,
		}
		if !validDays[config.WeeklyDay] {
			return api.NewError("Invalid weekly day")
		}
	}

	return nil
}

// isValidUpdateURL validates update server URLs.
//
// Note: Self-hosted update servers are commonly deployed on a local LAN, so we
// allow private RFC1918 IPs. We still block loopback, link-local, and other
// obviously unsafe URL forms to reduce SSRF risk.
func isValidUpdateURL(urlStr string) bool {
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return false
	}

	// Limit URL length
	if len(urlStr) > 2048 {
		return false
	}

	// Parse URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Only allow http and https schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// Require an explicit host.
	if u.Hostname() == "" {
		return false
	}

	// Disallow credentials in URL (userinfo) to avoid leaking secrets.
	if u.User != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())

	// Block localhost
	if host == "localhost" {
		return false
	}

	// If it's an IP address, apply IP-based safety checks.
	if ip := net.ParseIP(host); ip != nil {
		// Block loopback (127.0.0.1/::1), unspecified (0.0.0.0/::), and link-local.
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false
		}

		// Block multicast/broadcast-like destinations.
		if ip.IsMulticast() {
			return false
		}
	}

	return true
}

// getDefaultConfig returns the default update configuration.
func (h *UpdateConfigHandler) getDefaultConfig() *UpdateConfig {
	return &UpdateConfig{
		OSSource:           "tpr_official",
		ModelSource:        "tpr_official",
		SelfHostedOSUrl:    "",
		SelfHostedModelUrl: "",
		CheckFrequency:     "daily",
		WeeklyDay:          "sunday",
	}
}

const (
	updateCronBegin = "# supervisor:update-check BEGIN"
	updateCronEnd   = "# supervisor:update-check END"
)

// applyUpdateCheckSchedule installs/removes a managed cron entry that runs:
//
//	/usr/local/bin/supervisor --check-updates
//
// based on the update configuration.
func applyUpdateCheckSchedule(cfg *UpdateConfig) error {
	cronDir, cronFile, err := detectCronTab()
	if err != nil {
		return err
	}

	jobLine, err := buildCronJobLine(cfg)
	if err != nil {
		return err
	}

	var existing string
	if b, readErr := os.ReadFile(cronFile); readErr == nil {
		existing = string(b)
	}

	updated := removeManagedCronBlock(existing)
	if jobLine != "" {
		if updated != "" && !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}
		updated += updateCronBegin + "\n" + jobLine + "\n" + updateCronEnd + "\n"
	}

	// Ensure directory exists and write the file.
	if err := os.MkdirAll(filepath.Dir(cronFile), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(cronFile, []byte(updated), 0600); err != nil {
		return err
	}

	if err := ensureCronRunning(cronDir); err != nil {
		return err
	}
	return nil
}

func buildCronJobLine(cfg *UpdateConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("nil config")
	}

	// Manual mode means "do not schedule".
	if cfg.CheckFrequency == "manual" {
		return "", nil
	}

	// Use a fixed local time for daily/weekly checks.
	// (UI currently does not expose time-of-day.)
	const hour = 3
	const minute = 0

	cmd := "/usr/local/bin/supervisor --check-updates >/dev/null 2>&1"

	switch cfg.CheckFrequency {
	case "1min":
		return fmt.Sprintf("* * * * * %s", cmd), nil
	case "30min":
		return fmt.Sprintf("*/30 * * * * %s", cmd), nil
	case "daily":
		return fmt.Sprintf("%d %d * * * %s", minute, hour, cmd), nil
	case "weekly":
		dow, ok := map[string]int{
			"sunday":    0,
			"monday":    1,
			"tuesday":   2,
			"wednesday": 3,
			"thursday":  4,
			"friday":    5,
			"saturday":  6,
		}[cfg.WeeklyDay]
		if !ok {
			return "", fmt.Errorf("invalid weekly day: %q", cfg.WeeklyDay)
		}
		return fmt.Sprintf("%d %d * * %d %s", minute, hour, dow, cmd), nil
	default:
		return "", fmt.Errorf("invalid check frequency: %q", cfg.CheckFrequency)
	}
}

func removeManagedCronBlock(s string) string {
	start := strings.Index(s, updateCronBegin)
	if start < 0 {
		return s
	}
	end := strings.Index(s[start:], updateCronEnd)
	if end < 0 {
		// If the block is malformed, drop everything from the begin marker onward.
		return strings.TrimRight(s[:start], "\n") + "\n"
	}
	end = start + end + len(updateCronEnd)
	// Also remove trailing newline after end marker if present.
	if end < len(s) && s[end] == '\n' {
		end++
	}

	out := s[:start] + s[end:]
	return strings.TrimRight(out, "\n") + "\n"
}

// detectCronTab chooses a root crontab location compatible with common embedded distros.
// Returns (cronDir, crontabFilePath).
func detectCronTab() (string, string, error) {
	// Buildroot + BusyBox commonly set CONFIG_FEATURE_CROND_DIR="/etc/cron" and store per-user
	// crontabs under /etc/cron/crontabs/<user>.
	if st, err := os.Stat("/etc/cron"); err == nil && st.IsDir() {
		crontabsDir := "/etc/cron/crontabs"
		if err := os.MkdirAll(crontabsDir, 0755); err != nil {
			return "", "", err
		}
		return "/etc/cron", filepath.Join(crontabsDir, "root"), nil
	}

	// Some systems use /etc/cron/crontabs directly as the cron dir.
	if st, err := os.Stat("/etc/cron/crontabs"); err == nil && st.IsDir() {
		return "/etc/cron/crontabs", "/etc/cron/crontabs/root", nil
	}

	// OpenWrt-style BusyBox layout.
	if st, err := os.Stat("/etc/crontabs"); err == nil && st.IsDir() {
		return "/etc/crontabs", "/etc/crontabs/root", nil
	}

	// dcron/vixie-style layouts.
	if st, err := os.Stat("/var/spool/cron/crontabs"); err == nil && st.IsDir() {
		return "/var/spool/cron", "/var/spool/cron/crontabs/root", nil
	}
	if st, err := os.Stat("/var/spool/cron"); err == nil && st.IsDir() {
		return "/var/spool/cron", filepath.Join("/var/spool/cron", "root"), nil
	}

	// Fallback: create the buildroot-friendly layout.
	fallbackBase := "/etc/cron"
	fallbackCrontabs := "/etc/cron/crontabs"
	if err := os.MkdirAll(fallbackCrontabs, 0755); err != nil {
		return "", "", err
	}
	return fallbackBase, filepath.Join(fallbackCrontabs, "root"), nil
}

func ensureCronRunning(cronDir string) error {
	// If crond is already running, we're done.
	if isCrondRunning() {
		return nil
	}

	// Prefer init scripts when present.
	initCandidates := []string{"/etc/init.d/S50crond", "/etc/init.d/P02crond", "/etc/init.d/S90dcron"}
	for _, script := range initCandidates {
		if _, err := os.Stat(script); err == nil {
			if runErr := exec.Command(script, "start").Run(); runErr != nil {
				logger.Warning("Failed to start cron via %s: %v", script, runErr)
			}
			if isCrondRunning() {
				return nil
			}
			break
		}
	}

	// Best-effort direct start.
	//
	// Important: do NOT rely on daemonizing behavior here, because starting a forking daemon
	// via exec.Command().Start() without Wait() can leak zombies. We start with -f and detach
	// with setsid so the process stays running without needing an init script.
	cronDirsToTry := []string{cronDir}
	if filepath.Base(cronDir) == "crontabs" {
		cronDirsToTry = append(cronDirsToTry, filepath.Dir(cronDir))
	}

	for _, dir := range cronDirsToTry {
		cmd := exec.Command("crond", "-f", "-c", dir, "-L", "/dev/null")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			logger.Warning("Failed to start crond directly (dir=%s): %v", dir, err)
			continue
		}
		_ = cmd.Process.Release()

		// Give the daemon a moment to initialize.
		time.Sleep(200 * time.Millisecond)
		if isCrondRunning() {
			return nil
		}
	}

	return fmt.Errorf("cron daemon (crond) is not running and could not be started")
}

func isCrondRunning() bool {
	// Fast path: pidof is common on embedded images (busybox applet).
	if err := exec.Command("pidof", "crond").Run(); err == nil {
		return true
	}

	// Fallback: check pidfiles used by common init scripts.
	pidFiles := []string{"/var/run/crond.pid", "/var/run/dcron.pid"}
	for _, pf := range pidFiles {
		b, err := os.ReadFile(pf)
		if err != nil {
			continue
		}
		pidStr := strings.TrimSpace(string(b))
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 1 {
			continue
		}
		// kill(pid, 0) checks for existence without sending a signal.
		if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
			return true
		}
	}

	return false
}
