// Package detector provides camera-detector process lifecycle management.
package detector

import (
	"context"
	"os"
	"sync"
	"time"

	"supervisor/internal/system"
	"supervisor/pkg/logger"
)

const (
	InitScript    = "S96camera-detector"
	ModelFile     = "/userdata/Models/model.cvimodel"
	ProcessName   = "camera-detector"
	StreamerName  = "camera-streamer"
	CheckInterval = 5 * time.Second
	RestartDelay  = 3 * time.Second
)

// Manager handles camera-detector process lifecycle.
type Manager struct {
	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// GetManager returns the singleton detector manager.
func GetManager() *Manager {
	managerOnce.Do(func() {
		globalManager = &Manager{
			stopChan: make(chan struct{}),
		}
	})
	return globalManager
}

// Start begins the detector monitoring loop.
func (m *Manager) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.monitorLoop(ctx)
	logger.Info("Detector manager: Started monitoring")
}

// Stop halts the detector and cleanup.
func (m *Manager) Stop(ctx context.Context) {
	close(m.stopChan)
	m.wg.Wait()

	// Stop the detector if running
	m.mu.Lock()
	if m.running {
		logger.Info("Detector manager: Stopping camera-detector")
		system.RunService(InitScript, "stop")
		m.running = false
	}
	m.mu.Unlock()

	logger.Info("Detector manager: Stopped")
}

// RestartWithNewModel restarts the detector for a new model version.
func (m *Manager) RestartWithNewModel(version float64) {
	m.mu.Lock()
	wasRunning := m.running
	m.mu.Unlock()

	if wasRunning {
		logger.Info("Detector manager: Stopping detector for model update to v%.1f", version)
		system.RunService(InitScript, "stop")

		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}

	// Wait before restart
	time.Sleep(RestartDelay)

	// Only restart if dependencies are still met
	if m.checkDependencies() {
		logger.Info("Detector manager: Starting detector with new model v%.1f", version)
		if err := system.RunService(InitScript, "start"); err == nil {
			m.mu.Lock()
			m.running = true
			m.mu.Unlock()
		} else {
			logger.Warning("Detector manager: Failed to start detector: %v", err)
		}
	}
}

// checkDependencies verifies that model file exists and camera-streamer is running.
func (m *Manager) checkDependencies() bool {
	// Check if model file exists
	if _, err := os.Stat(ModelFile); os.IsNotExist(err) {
		return false
	}

	// Check if camera-streamer is running
	return system.IsProcessRunning(StreamerName)
}

// isDetectorRunning checks if camera-detector process is running.
func (m *Manager) isDetectorRunning() bool {
	return system.IsProcessRunning(ProcessName)
}

// monitorLoop periodically checks dependencies and manages detector lifecycle.
func (m *Manager) monitorLoop(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAndManage()
		}
	}
}

// checkAndManage checks dependencies and starts/stops detector accordingly.
func (m *Manager) checkAndManage() {
	depsReady := m.checkDependencies()
	processRunning := m.isDetectorRunning()

	m.mu.Lock()
	defer m.mu.Unlock()

	if depsReady && !processRunning {
		// Dependencies ready but detector not running - start it
		logger.Info("Detector manager: Dependencies ready, starting camera-detector")
		if err := system.RunService(InitScript, "start"); err != nil {
			logger.Warning("Detector manager: Failed to start detector: %v", err)
		} else {
			m.running = true
		}
	} else if !depsReady && processRunning {
		// Dependencies not ready but detector running - stop it
		logger.Info("Detector manager: Dependencies not ready, stopping camera-detector")
		if err := system.RunService(InitScript, "stop"); err != nil {
			logger.Warning("Detector manager: Failed to stop detector: %v", err)
		}
		m.running = false
	} else if depsReady && processRunning {
		// Everything OK, update running state
		m.running = true
	} else {
		// Not ready and not running
		m.running = false
	}
}
