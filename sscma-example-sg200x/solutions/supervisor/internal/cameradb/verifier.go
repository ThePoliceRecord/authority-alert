// Package cameradb provides periodic file verification for the camera database.
package cameradb

import (
	"context"
	"sync"
	"time"

	"supervisor/pkg/logger"
)

// Verifier periodically checks for missing files and hash mismatches
type Verifier struct {
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	running  bool
}

// NewVerifier creates a new file verifier
func NewVerifier(interval time.Duration) *Verifier {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	return &Verifier{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic file verification
func (v *Verifier) Start(ctx context.Context) {
	v.mu.Lock()
	if v.running {
		v.mu.Unlock()
		return
	}
	v.running = true
	v.mu.Unlock()

	v.wg.Add(1)
	go v.run(ctx)

	logger.Info("File verifier started (interval: %v)", v.interval)
}

// Stop stops the file verifier
func (v *Verifier) Stop() {
	v.mu.Lock()
	if !v.running {
		v.mu.Unlock()
		return
	}
	v.running = false
	v.mu.Unlock()

	close(v.stopCh)
	v.wg.Wait()

	logger.Info("File verifier stopped")
}

// run is the main verification loop
func (v *Verifier) run(ctx context.Context) {
	defer v.wg.Done()

	// Run initial scan after a short delay
	select {
	case <-time.After(30 * time.Second):
		v.verify()
	case <-ctx.Done():
		return
	case <-v.stopCh:
		return
	}

	ticker := time.NewTicker(v.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-v.stopCh:
			return
		case <-ticker.C:
			v.verify()
		}
	}
}

// verify performs the file verification
func (v *Verifier) verify() {
	logger.Debug("Running file verification scan...")

	missingRecordings, missingDetections, err := ScanForMissingFiles()
	if err != nil {
		logger.Error("File verification scan failed: %v", err)
		return
	}

	if missingRecordings > 0 || missingDetections > 0 {
		logger.Warning("File verification found %d missing recordings, %d missing detection images",
			missingRecordings, missingDetections)
	}
}

// RunNow triggers an immediate verification scan
func (v *Verifier) RunNow() {
	v.mu.Lock()
	if !v.running {
		v.mu.Unlock()
		return
	}
	v.mu.Unlock()

	go v.verify()
}
