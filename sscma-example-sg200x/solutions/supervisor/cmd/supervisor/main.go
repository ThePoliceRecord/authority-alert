// Package main provides the entry point for the supervisor.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"supervisor/internal/cameradb"
	"supervisor/internal/config"
	"supervisor/internal/detectionqueue"
	"supervisor/internal/handler"
	"supervisor/internal/modelupdate"
	"supervisor/internal/ntp"
	"supervisor/internal/security"
	"supervisor/internal/server"
	"supervisor/internal/system"
	"supervisor/internal/upgrade"
	"supervisor/pkg/logger"
)

// Build-time variables
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Parse command-line flags
	var (
		showVersion  = flag.Bool("version", false, "Show version information")
		daemon       = flag.Bool("d", false, "Run as daemon")
		httpPort     = flag.String("p", "", "HTTP port for redirect (default: 80)")
		httpsPort    = flag.String("P", "", "HTTPS port (default: 443)")
		rootDir      = flag.String("r", "", "Web root directory")
		scriptPath   = flag.String("s", "", "Script path")
		certDir      = flag.String("c", "", "TLS certificate directory")
		logLevel     = flag.Int("v", -1, "Log level (0-4)")
		checkUpdates = flag.Bool("check-updates", false, "Run a one-shot OS update check (for cron) and exit")
	)
	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("Supervisor version %s\n", Version)
		fmt.Printf("Build time: %s\n", BuildTime)
		fmt.Printf("Git commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Get configuration
	cfg := config.Get()

	// Apply command-line overrides
	if *httpPort != "" {
		cfg.HTTPPort = *httpPort
	}
	if *httpsPort != "" {
		cfg.HTTPSPort = *httpsPort
	}
	if *rootDir != "" {
		cfg.RootDir = *rootDir
	}
	if *scriptPath != "" {
		cfg.ScriptPath = *scriptPath
	}
	if *certDir != "" {
		cfg.CertDir = *certDir
	}
	if *logLevel >= 0 {
		cfg.LogLevel = *logLevel
	}
	if *daemon {
		cfg.DaemonMode = true
	}

	// Initialize logger with appropriate level
	logger.SetLevel(logger.Level(cfg.LogLevel))

	// Load security configuration (firmware signing, TSA timestamping, etc.)
	// These features are disabled by default and can be enabled via /etc/recamera.conf/security.json
	security.Load()

	// Cron mode: run the check and exit without starting the server.
	if *checkUpdates {
		if err := runUpdateCheckOnce(); err != nil {
			logger.Error("Scheduled update check failed: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Daemonize if requested
	if cfg.DaemonMode {
		if err := daemonize(); err != nil {
			logger.Error("Failed to daemonize: %v", err)
			os.Exit(1)
		}
	}

	// Start NTP time synchronization first (needed by server and uploader)
	ntpManager := ntp.NewManager()
	ntpManager.Start(context.Background())

	// Create and start server (pass NTP manager for status API)
	srv := server.New(cfg, ntpManager)
	if err := srv.Start(); err != nil {
		logger.Error("Failed to start server: %v", err)
		os.Exit(1)
	}

	logger.Info("Supervisor started (version %s)", Version)

	// Start external relay forwarders if configured
	go handler.StartRelayControl()

	// Start model update manager
	modelUpdateManager := modelupdate.GetManager()
	modelUpdateManager.Start(context.Background())

	// Start detection uploader (queues detections from camera-detector for platform upload)
	// Pass NTP manager so uploader waits for time sync before uploading
	detectionUploader := detectionqueue.GetUploader()
	detectionUploader.SetNTPManager(ntpManager)
	detectionUploader.Start(context.Background())

	// Open camera database (optional - used for recording/detection correlation)
	if _, err := cameradb.Open(); err != nil {
		logger.Warning("Failed to open camera database: %v (detection correlation disabled)", err)
	} else {
		logger.Info("Camera database opened")
	}

	// Start file verifier (checks for missing recording/detection files)
	fileVerifier := cameradb.NewVerifier(1 * time.Hour)
	fileVerifier.Start(context.Background())

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")

	// Stop file verifier
	fileVerifier.Stop()

	// Close camera database
	cameradb.Close()

	// Stop detection uploader
	detectionUploader.Stop()

	// Stop model update manager
	modelUpdateManager.Stop()

	// Stop NTP manager
	ntpManager.Stop()

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}

	logger.Info("Supervisor stopped")
}

// runUpdateCheckOnce performs a one-shot "check for updates" operation.
//
// This is intended to be invoked by the OS scheduler (cron/systemd timers).
// It updates the cached version metadata used by the UI.
func runUpdateCheckOnce() error {
	const updateConfPath = "/etc/supervisor/update.conf"

	// Prevent multiple concurrent cron invocations from overlapping.
	// (e.g. schedule every minute while a prior check/install is still running.)
	lockFile, locked, err := tryAcquireUpdateLock()
	if err != nil {
		return err
	}
	if !locked {
		logger.Info("Another scheduled update check/install is already running; skipping")
		return nil
	}
	defer lockFile.Close()

	type UpdateConfig struct {
		OSSource        string `json:"os_source"`
		SelfHostedOSUrl string `json:"self_hosted_os_url"`
	}

	cfg := UpdateConfig{OSSource: "tpr_official", SelfHostedOSUrl: ""}
	if data, err := os.ReadFile(updateConfPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	mgr := upgrade.NewUpgradeManager()

	// If an upgrade is currently running, skip the scheduled "check" to avoid
	// interfering with download/flash operations.
	if prog, _ := mgr.GetUpdateProgress(); prog != nil {
		st := strings.TrimSpace(prog.Status)
		if st != "" && st != "idle" && st != "cancelled" {
			logger.Info("Upgrade status is %q; skipping scheduled update check", st)
			return nil
		}
	}

	customURL := ""
	if cfg.OSSource == "self_hosted" {
		customURL = strings.TrimSpace(cfg.SelfHostedOSUrl)
	}

	// Keep upgrade manager state aligned with the UI config.
	if err := mgr.UpdateChannel(0, customURL); err != nil {
		return err
	}

	// Do the actual check (downloads/parses checksum manifest and caches version.json).
	if err := mgr.QueryLatestVersion(); err != nil {
		return err
	}

	// Auto-install: if a newer version is available, start the upgrade.
	current := strings.TrimSpace(system.GetOSVersion())
	ver, err := mgr.GetSystemUpdateVersionWithOptions(false)
	if err != nil || ver == nil {
		return err
	}
	latest := strings.TrimSpace(ver.OSVersion)
	if ver.Error != "" {
		logger.Warning("Update check completed with error: %s", ver.Error)
		return nil
	}
	if current == "" || latest == "" {
		logger.Warning("Cannot determine current/latest OS version (current=%q latest=%q); not auto-installing", current, latest)
		return nil
	}

	switch compareVersionStrings(latest, current) {
	case 1:
		logger.Info("Scheduled check found newer OS version %q (current=%q); starting auto-install", latest, current)
		return mgr.UpdateSystemSync()
	case 0:
		logger.Info("No OS update available (current=%q latest=%q)", current, latest)
		return nil
	default:
		// Do not auto-install downgrades.
		logger.Info("Available OS version %q is not newer than current %q; not auto-installing", latest, current)
		return nil
	}
}

func tryAcquireUpdateLock() (*os.File, bool, error) {
	f, err := os.OpenFile(upgrade.UpgradeMutexFile, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, err
	}
	// Non-blocking exclusive lock; if busy, skip this run.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, false, nil
	}
	return f, true, nil
}

// compareVersionStrings does a best-effort numeric comparison for version strings.
// Returns 1 if a>b, -1 if a<b, 0 if equal/indeterminate.
func compareVersionStrings(a, b string) int {
	ap := parseVersionInts(a)
	bp := parseVersionInts(b)
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		ai := 0
		bi := 0
		if i < len(ap) {
			ai = ap[i]
		}
		if i < len(bp) {
			bi = bp[i]
		}
		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}

func parseVersionInts(v string) []int {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	// Strip a leading 'v' (e.g. v1.2.3).
	if len(v) > 1 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	parts := strings.FieldsFunc(v, func(r rune) bool { return r < '0' || r > '9' })
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	for len(out) > 0 && out[len(out)-1] == 0 {
		out = out[:len(out)-1]
	}
	return out
}

// daemonize forks the process and redirects stdout/stderr to /dev/null.
// Uses unix.Dup2 for riscv64 compatibility (syscall.Dup2 is not available on riscv64).
func daemonize() error {
	// Fork
	pid, err := syscall.ForkExec(os.Args[0], os.Args, &syscall.ProcAttr{
		Dir: "/",
		Env: os.Environ(),
		Sys: &syscall.SysProcAttr{
			Setsid: true,
		},
		Files: []uintptr{0, 1, 2},
	})
	if err != nil {
		return fmt.Errorf("fork failed: %w", err)
	}

	if pid > 0 {
		// Parent exits
		os.Exit(0)
	}

	// Child continues
	// Redirect stdin, stdout, stderr to /dev/null
	nullFile, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/null: %w", err)
	}

	// Use unix.Dup2 instead of syscall.Dup2 for riscv64 compatibility
	unix.Dup2(int(nullFile.Fd()), int(os.Stdin.Fd()))
	unix.Dup2(int(nullFile.Fd()), int(os.Stdout.Fd()))
	unix.Dup2(int(nullFile.Fd()), int(os.Stderr.Fd()))

	return nil
}
