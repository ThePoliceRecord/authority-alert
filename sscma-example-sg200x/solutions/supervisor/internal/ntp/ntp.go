// Package ntp provides reliable NTP time synchronization for the supervisor.
package ntp

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"supervisor/pkg/logger"
)

const (
	InitialDelay         = 3 * time.Second  // Brief wait for network stack
	NtpdateTimeout       = 5 * time.Second  // Per-server timeout (NTP responds fast when reachable)
	PeriodicSyncInterval = 30 * time.Minute // Re-check interval after sync
	maxPeriodicFailures  = 3                // Consecutive periodic failures before reverting synced
	initialRetryInterval = 3 * time.Second  // Starting retry backoff
	maxRetryInterval     = 10 * time.Second // Cap for retry backoff
)

// Manager handles NTP time synchronization.
type Manager struct {
	mu       sync.RWMutex
	synced   bool
	lastSync time.Time
	servers  []string
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewManager creates a new NTP manager.
func NewManager() *Manager {
	return &Manager{
		servers: []string{
			"time.google.com",
			"time.cloudflare.com",
			"time.windows.com",
			"time.apple.com",
			"ntp.aliyun.com", // China fallback
		},
		done: make(chan struct{}),
	}
}

// Start begins the NTP synchronization service.
func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	go m.syncLoop(ctx)
}

// Stop halts the NTP synchronization service.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	<-m.done
}

// IsSynced returns whether time has been synchronized.
func (m *Manager) IsSynced() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.synced
}

// LastSync returns the time of the last successful sync.
func (m *Manager) LastSync() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSync
}

func (m *Manager) syncLoop(ctx context.Context) {
	defer close(m.done)
	logger.Info("NTP sync: Starting time synchronization service")

	// Brief initial delay for network stack
	select {
	case <-ctx.Done():
		return
	case <-time.After(InitialDelay):
	}

	for {
		// Initial sync: retry with backoff until successful
		attempt := 0
		retryInterval := initialRetryInterval
		for {
			attempt++
			logger.Info("NTP sync: Attempt %d, trying %d servers...", attempt, len(m.servers))

			if m.trySyncRound(ctx) {
				m.mu.Lock()
				m.synced = true
				m.lastSync = time.Now()
				m.mu.Unlock()
				logger.Info("NTP sync: Time synchronized successfully (attempt %d)", attempt)
				break
			}

			logger.Warn("NTP sync: Round %d failed (all %d servers), retrying in %v", attempt, len(m.servers), retryInterval)

			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
			}

			// Backoff: 3s → 5s → 8s → 10s (capped)
			retryInterval = retryInterval * 5 / 3
			if retryInterval > maxRetryInterval {
				retryInterval = maxRetryInterval
			}
		}

		// Periodic re-sync phase
		periodicFailures := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(PeriodicSyncInterval):
			}

			logger.Info("NTP sync: Periodic re-sync")
			if m.trySyncRound(ctx) {
				periodicFailures = 0
				m.mu.Lock()
				m.lastSync = time.Now()
				m.mu.Unlock()
			} else {
				periodicFailures++
				logger.Warn("NTP sync: Periodic re-sync failed (%d/%d)", periodicFailures, maxPeriodicFailures)
				if periodicFailures >= maxPeriodicFailures {
					m.mu.Lock()
					m.synced = false
					m.mu.Unlock()
					logger.Warn("NTP sync: %d consecutive periodic failures, marking as unsynced", maxPeriodicFailures)
					break // Fall back to initial retry loop
				}
			}
		}
	}
}

// trySyncRound stops ntpd, tries all servers, then restarts ntpd.
// Returns true if any server succeeded.
func (m *Manager) trySyncRound(ctx context.Context) bool {
	// Stop ntpd once to free port 123
	exec.Command("/etc/init.d/S49ntp", "stop").Run()
	time.Sleep(500 * time.Millisecond)

	success := false
	for _, server := range m.servers {
		// Bail early if shutting down
		select {
		case <-ctx.Done():
			return false
		default:
		}

		sctx, cancel := context.WithTimeout(ctx, NtpdateTimeout)
		cmd := exec.CommandContext(sctx, "/usr/bin/ntpdate", "-u", "-b", server)
		output, err := cmd.CombinedOutput()
		cancel()

		if err == nil {
			logger.Info("NTP sync: Synced with %s: %s", server, strings.TrimSpace(string(output)))
			// Sync to hardware clock if available (some devices have no RTC)
			if _, err := exec.LookPath("hwclock"); err == nil {
				if err := exec.Command("hwclock", "-w").Run(); err != nil {
					logger.Debug("NTP sync: hwclock sync failed: %v", err)
				}
			}
			success = true
			break
		}
		logger.Debug("NTP sync: Server %s failed: %v", server, err)
	}

	// Only restart ntpd if not shutting down
	if ctx.Err() == nil {
		exec.Command("/etc/init.d/S49ntp", "start").Run()
	}

	return success
}

