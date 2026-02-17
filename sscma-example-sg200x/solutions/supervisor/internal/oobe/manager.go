// Package oobe provides OOBE (Out-of-Box Experience) management.
package oobe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"supervisor/pkg/logger"
)

const (
	FlagFile         = "/etc/oobe/flag"
	BinaryPath       = "/usr/local/bin/oobe"
	ListenAddr       = "127.0.0.1:8081"
	RootDir          = "/usr/share/oobe/www"
	FlagCheckInterval = 2 * time.Second
	StartupTimeout   = 10 * time.Second
	ShutdownTimeout  = 5 * time.Second
)

// Manager handles OOBE process lifecycle and proxying.
type Manager struct {
	mu       sync.RWMutex
	active   bool
	cmd      *exec.Cmd
	proxy    *httputil.ReverseProxy
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// New creates a new OOBE Manager.
func New() *Manager {
	return &Manager{
		stopChan: make(chan struct{}),
	}
}

// IsActive returns whether OOBE mode is currently active.
func (m *Manager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// Start begins OOBE monitoring. Call this on supervisor startup.
func (m *Manager) Start() error {
	// Check initial state
	if m.flagExists() {
		if err := m.startOOBE(); err != nil {
			return err
		}
	}

	// Start flag file monitor
	m.wg.Add(1)
	go m.monitorFlagFile()

	return nil
}

// Stop terminates OOBE mode and cleanup.
func (m *Manager) Stop(ctx context.Context) error {
	close(m.stopChan)
	m.wg.Wait()
	return m.stopOOBE()
}

// Handler returns an http.Handler that routes requests appropriately.
// When OOBE is active:
//   - /oobe/* routes are proxied to OOBE
//   - /* (root/static) routes are proxied to OOBE
//
// When OOBE is inactive, fallback handler is used.
func (m *Manager) Handler(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.IsActive() && m.proxy != nil {
			// When OOBE is active, proxy all non-API, non-websocket requests to OOBE
			// /oobe/* paths and root paths go to OOBE
			if strings.HasPrefix(r.URL.Path, "/oobe/") || !strings.HasPrefix(r.URL.Path, "/api/") {
				m.proxy.ServeHTTP(w, r)
				return
			}
		}
		fallback.ServeHTTP(w, r)
	})
}

// flagExists checks if the OOBE flag file exists.
func (m *Manager) flagExists() bool {
	_, err := os.Stat(FlagFile)
	return err == nil
}

// monitorFlagFile periodically checks the flag file status.
func (m *Manager) monitorFlagFile() {
	defer m.wg.Done()
	ticker := time.NewTicker(FlagCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			flagExists := m.flagExists()
			m.mu.RLock()
			wasActive := m.active
			m.mu.RUnlock()

			if flagExists && !wasActive {
				// Flag appeared, start OOBE
				if err := m.startOOBE(); err != nil {
					logger.Error("Failed to start OOBE: %v", err)
				}
			} else if !flagExists && wasActive {
				// Flag removed, stop OOBE
				if err := m.stopOOBE(); err != nil {
					logger.Error("Failed to stop OOBE: %v", err)
				}
			}
		}
	}
}

// startOOBE starts the OOBE process and sets up the reverse proxy.
func (m *Manager) startOOBE() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		return nil // Already running
	}

	// Start OOBE process with localhost binding (HTTP, not HTTPS - supervisor handles TLS)
	m.cmd = exec.Command(BinaryPath,
		"--listen", "http://"+ListenAddr,
		"--root", RootDir,
	)
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start OOBE: %w", err)
	}

	logger.Info("Started OOBE process (PID: %d) on %s", m.cmd.Process.Pid, ListenAddr)

	// Wait for OOBE to be ready (health check) and detect whether it's serving
	// HTTP or HTTPS on the loopback interface.
	target, err := m.waitForOOBEReady()
	if err != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
		m.cmd = nil
		return err
	}

	// Create reverse proxy
	m.proxy = httputil.NewSingleHostReverseProxy(target)
	// If the OOBE server is running HTTPS on loopback (e.g., older binaries or
	// default config), allow the proxy to connect even with self-signed certs.
	if target.Scheme == "https" {
		m.proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Custom director to set forwarding headers and strip /oobe prefix
	originalDirector := m.proxy.Director
	m.proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		if clientIP := getClientIP(req); clientIP != "" {
			req.Header.Set("X-Real-IP", clientIP)
			req.Header.Set("X-Forwarded-For", clientIP)
		}
		
		// Strip /oobe prefix from path for the OOBE server
		// The OOBE server's web root is already the OOBE directory
		if strings.HasPrefix(req.URL.Path, "/oobe/") {
			req.URL.Path = "/" + strings.TrimPrefix(req.URL.Path, "/oobe/")
		} else if req.URL.Path == "/oobe" {
			req.URL.Path = "/"
		}
	}

	m.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("OOBE proxy error for %s %s: %v", r.Method, r.URL.String(), err)
		http.Error(w, "OOBE temporarily unavailable", http.StatusBadGateway)
	}

	m.active = true

	// Monitor process for unexpected termination
	go m.monitorProcess()

	logger.Info("OOBE mode activated")
	return nil
}

// stopOOBE gracefully stops the OOBE process.
func (m *Manager) stopOOBE() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active || m.cmd == nil {
		m.active = false
		return nil
	}

	logger.Info("Stopping OOBE process")

	// Send SIGTERM for graceful shutdown
	if m.cmd.Process != nil {
		m.cmd.Process.Signal(syscall.SIGTERM)

		// Wait with timeout
		done := make(chan error, 1)
		go func() { done <- m.cmd.Wait() }()

		select {
		case <-done:
			// Process exited
		case <-time.After(ShutdownTimeout):
			// Force kill
			m.cmd.Process.Kill()
			<-done
		}
	}

	m.active = false
	m.proxy = nil
	m.cmd = nil

	logger.Info("OOBE process stopped, resuming normal operation")
	return nil
}


// waitForOOBEReady performs a health check to ensure OOBE is ready.
// It returns the base URL (scheme + host) to use for the reverse proxy.
func (m *Manager) waitForOOBEReady() (*url.URL, error) {
	httpClient := &http.Client{Timeout: time.Second}
	httpsClient := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	deadline := time.Now().Add(StartupTimeout)

	httpHealthURL := "http://" + ListenAddr + "/api/health"
	httpsHealthURL := "https://" + ListenAddr + "/api/health"

	for time.Now().Before(deadline) {
		// Try HTTP first (preferred; supervisor already terminates TLS)
		if resp, err := httpClient.Get(httpHealthURL); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return url.Parse("http://" + ListenAddr)
			}
		}

		// Fallback: some images/binaries may run OOBE as HTTPS on loopback.
		if resp, err := httpsClient.Get(httpsHealthURL); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return url.Parse("https://" + ListenAddr)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("OOBE failed to start within timeout")
}

// monitorProcess watches for unexpected OOBE process termination.
func (m *Manager) monitorProcess() {
	if m.cmd == nil || m.cmd.Process == nil {
		return
	}

	// Wait for process to exit
	m.cmd.Wait()

	m.mu.Lock()
	wasActive := m.active
	m.active = false
	m.proxy = nil
	m.cmd = nil
	m.mu.Unlock()

	if wasActive {
		logger.Warning("OOBE process terminated unexpectedly")
		// Check if flag still exists - if so, try to restart
		if m.flagExists() {
			logger.Info("OOBE flag still exists, attempting restart...")
			time.Sleep(time.Second)
			if err := m.startOOBE(); err != nil {
				logger.Error("Failed to restart OOBE: %v", err)
			}
		}
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	if r.RemoteAddr != "" {
		// Remove port if present
		if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
			return r.RemoteAddr[:idx]
		}
		return r.RemoteAddr
	}
	return ""
}
