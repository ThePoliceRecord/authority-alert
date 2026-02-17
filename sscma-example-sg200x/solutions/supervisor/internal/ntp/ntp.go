// Package ntp provides reliable NTP time synchronization for the supervisor.
package ntp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"supervisor/pkg/logger"
)

const (
	InitialDelay         = 3 * time.Second  // Brief wait for network stack
	NtpdateTimeout       = 30 * time.Second // Per-server timeout
	RetryInterval        = 10 * time.Second // Retry every 10s until synced
	PeriodicSyncInterval = 30 * time.Minute // Re-check interval after sync
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
		// Try to sync
		if m.trySync(ctx) {
			m.mu.Lock()
			m.synced = true
			m.lastSync = time.Now()
			m.mu.Unlock()
			logger.Info("NTP sync: Time synchronized successfully")

			// After success, wait for periodic re-sync
			select {
			case <-ctx.Done():
				return
			case <-time.After(PeriodicSyncInterval):
				logger.Info("NTP sync: Periodic re-sync")
			}
			continue
		}

		// Sync failed - retry in 10 seconds
		select {
		case <-ctx.Done():
			return
		case <-time.After(RetryInterval):
		}
	}
}

// trySync attempts to sync time, returns true on success.
// We skip HTTPS connectivity checks because TLS fails with wrong system time.
// Just try ntpdate directly - if network isn't ready, it fails fast and we retry.
func (m *Manager) trySync(ctx context.Context) bool {
	return m.doSync() == nil
}

func (m *Manager) doSync() error {
	// Stop ntpd to free port 123
	exec.Command("/etc/init.d/S49ntp", "stop").Run()
	time.Sleep(500 * time.Millisecond)

	// Try each server
	for _, server := range m.servers {
		ctx, cancel := context.WithTimeout(context.Background(), NtpdateTimeout)
		cmd := exec.CommandContext(ctx, "/usr/bin/ntpdate", "-u", "-b", server)
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
			// Restart ntpd for ongoing drift correction
			exec.Command("/etc/init.d/S49ntp", "start").Run()
			return nil
		}
		logger.Debug("NTP sync: Server %s failed: %v", server, err)
	}

	// Restart ntpd even on failure
	exec.Command("/etc/init.d/S49ntp", "start").Run()
	return fmt.Errorf("all NTP servers failed")
}
