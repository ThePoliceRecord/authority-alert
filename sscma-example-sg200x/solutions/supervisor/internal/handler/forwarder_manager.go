package handler

import (
	"context"
	"fmt"
	"supervisor/pkg/logger"
	"sync"
	"time"
)

// ForwarderManager manages dynamic lifecycle of stream forwarders
type ForwarderManager struct {
	forwarders map[string]context.CancelFunc
	mu         sync.Mutex
	cameraURL  string
	relayURL   string
	cameraID   string
	token      string
}

// NewForwarderManager creates a new ForwarderManager
func NewForwarderManager(cameraURL, relayURL, cameraID, token string) *ForwarderManager {
	return &ForwarderManager{
		forwarders: make(map[string]context.CancelFunc),
		cameraURL:  cameraURL,
		relayURL:   relayURL,
		cameraID:   cameraID,
		token:      token,
	}
}

// Start starts a forwarder for the given channel if not already running
func (fm *ForwarderManager) Start(channel string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.forwarders[channel]; exists {
		return nil // Already running
	}

	// Determine channel ID based on name
	channelID := ""
	switch channel {
	case "high":
		channelID = "0"
	case "medium":
		channelID = "1"
	case "low":
		channelID = "2"
	default:
		return fmt.Errorf("unknown channel: %s", channel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fm.forwarders[channel] = cancel

	cameraURL := fmt.Sprintf("%s?channel=%s", fm.cameraURL, channelID)
	cameraIDWithChannel := fmt.Sprintf("%s_%s", fm.cameraID, channel)

	// Start forwarder in goroutine
	go func() {
		logger.Info("Starting forwarder for channel %s", channel)
		runForwarderWithReconnect(ctx, cameraURL, fm.relayURL, cameraIDWithChannel, fm.token)
		logger.Info("Forwarder stopped for channel %s", channel)
	}()

	return nil
}

// Stop stops the forwarder for the given channel
func (fm *ForwarderManager) Stop(channel string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if cancel, exists := fm.forwarders[channel]; exists {
		logger.Info("Stopping forwarder for channel %s", channel)
		cancel()
		delete(fm.forwarders, channel)
	} else {
		logger.Warning("Forwarder for channel %s not running", channel)
	}
}

// StopAll stops all running forwarders
func (fm *ForwarderManager) StopAll() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	logger.Info("Stopping all forwarders (%d active)", len(fm.forwarders))
	for channel, cancel := range fm.forwarders {
		logger.Info("Stopping forwarder for channel %s", channel)
		cancel()
	}
	fm.forwarders = make(map[string]context.CancelFunc)
}

// SetRelayURL updates the relay URL for the forwarder manager
func (fm *ForwarderManager) SetRelayURL(url string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.relayURL = url
}

// runForwarderWithReconnect runs the forwarder with automatic reconnection and exponential backoff
func runForwarderWithReconnect(ctx context.Context, cameraURL, relayURL, cameraID, token string) {
	reconnectAttempt := 0
	minDelay := 1 * time.Second
	maxDelay := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.Debug("Forwarder context cancelled for camera %s", cameraID)
			return
		default:
			// Run forwarder until it exits
			ExternalRelayForwarderWithContext(ctx, cameraURL, relayURL, cameraID, token)

			// Check if context was cancelled
			select {
			case <-ctx.Done():
				return
			default:
				// Calculate exponential backoff delay
				reconnectAttempt++
				delay := minDelay * time.Duration(1<<uint(reconnectAttempt-1))
				if delay > maxDelay {
					delay = maxDelay
				}

				// Add small jitter
				jitter := time.Duration(float64(time.Second) * (0.5 + 0.5*float64(time.Now().UnixNano()%1000)/1000.0))
				totalDelay := delay + jitter

				logger.Debug("Reconnecting forwarder for camera %s in %v (attempt %d)", cameraID, totalDelay, reconnectAttempt)

				// Sleep with context awareness
				select {
				case <-time.After(totalDelay):
					// Continue to next iteration
				case <-ctx.Done():
					return
				}
			}
		}
	}
}
