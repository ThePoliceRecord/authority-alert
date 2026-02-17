package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"supervisor/internal/device"
	"supervisor/internal/tls"
	"supervisor/pkg/logger"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	globalForwarderManager *ForwarderManager
	reconnectMutex         sync.Mutex
	isConnected            bool
	reconnectAttempt       int
)

const (
	minReconnectDelay  = 1 * time.Second
	maxReconnectDelay  = 60 * time.Second
	maxReconnectJitter = 2 * time.Second
)

// getPlatformCredentials retrieves camera_uid and secret_key from platform info
func getPlatformCredentials() (cameraUID, secretKey string) {
	platformInfo := device.GetPlatformInfo()
	if platformInfo == "" {
		return "", ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(platformInfo), &data); err != nil {
		return "", ""
	}

	// Get camera UID
	if uid, ok := data["tpr_camera_id"].(string); ok {
		cameraUID = uid
	}

	// Get secret key
	if key, ok := data["secret_key"].(string); ok && key != "" {
		secretKey = key
	} else if key, ok := data["api_key"].(string); ok {
		// Fallback for legacy api_key field
		secretKey = key
	}

	return cameraUID, secretKey
}

// verifyViewerToken validates a viewer's bearer token with the backend API
func verifyViewerToken(viewerToken string) error {
	_, secretKey := getPlatformCredentials()
	if secretKey == "" {
		return fmt.Errorf("camera not registered")
	}

	url := platformURL("/api/v1/cameras/check-token/")
	payload, _ := json.Marshal(map[string]string{"token": viewerToken})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("TPR-API-KEY", secretKey)
	applyPlatformCommonHeaders(req)

	client := tls.PlatformHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token rejected: status %d", resp.StatusCode)
	}
	return nil
}

// StartRelayControl establishes control connection to relay server for on-demand streaming
func StartRelayControl() {
	relayURLOverride := os.Getenv("EXTERNAL_RELAY_URL")

	// Get credentials from platform info (set during device registration)
	cameraID, secretKey := getPlatformCredentials()
	if cameraID == "" || secretKey == "" {
		logger.Info("External relay disabled (camera not registered)")
		return
	}

	logger.Info("Starting on-demand relay for camera %s", cameraID)

	// Initialize forwarder manager with placeholder URL (will be set per-connection)
	cameraBaseURL := "ws://localhost:8765/"
	globalForwarderManager = NewForwarderManager(cameraBaseURL, "", cameraID, secretKey)

	// Connect to relay with supervisor role using SRV discovery
	go maintainControlConnectionWithSRV(cameraID, secretKey, relayURLOverride)
}

// maintainControlConnectionWithSRV maintains connection using SRV-based discovery with failover
func maintainControlConnectionWithSRV(cameraID, token, relayURLOverride string) {
	reconnectAttempt = 0

	for {
		reconnectMutex.Lock()
		isConnected = false
		reconnectMutex.Unlock()

		var err error

		// Use override URL if provided, otherwise discover via SRV
		if relayURLOverride != "" {
			logger.Info("Using relay URL override: %s", relayURLOverride)
			err = connectToRelay(relayURLOverride, cameraID, token)
		} else {
			err = connectWithSRVFailover(cameraID, token)
		}

		if err != nil {
			logger.Error("Control connection error: %v", err)
		}

		// Calculate exponential backoff with jitter
		delay := calculateReconnectDelay()
		logger.Info("Reconnecting control connection in %v... (attempt %d)", delay, reconnectAttempt+1)
		time.Sleep(delay)
	}
}

// connectWithSRVFailover discovers relay servers via SRV and connects with failover
func connectWithSRVFailover(cameraID, token string) error {
	servers, err := discoverRelayServers()
	if err != nil {
		return fmt.Errorf("relay discovery failed: %w", err)
	}

	// Track which priorities we've tried
	triedPriorities := make(map[uint16]bool)

	// Try servers in priority order
	for len(triedPriorities) < len(servers) {
		// Find the lowest untried priority
		var currentPriority uint16 = 65535
		for _, s := range servers {
			if !triedPriorities[s.Priority] && s.Priority < currentPriority {
				currentPriority = s.Priority
			}
		}

		if currentPriority == 65535 {
			break // All priorities tried
		}

		// Get all servers at this priority level
		priorityServers := getServersForPriority(servers, currentPriority)
		logger.Info("Trying relay servers at priority %d (%d servers)", currentPriority, len(priorityServers))

		// Try servers at this priority using weighted selection
		triedAtPriority := make(map[string]bool)
		for len(triedAtPriority) < len(priorityServers) {
			// Filter out already-tried servers for weighted selection
			available := make([]relayServer, 0)
			for _, s := range priorityServers {
				key := fmt.Sprintf("%s:%d", s.Host, s.Port)
				if !triedAtPriority[key] {
					available = append(available, s)
				}
			}

			if len(available) == 0 {
				break
			}

			// Select server by weight
			selected := selectServerByWeight(available)
			key := fmt.Sprintf("%s:%d", selected.Host, selected.Port)
			triedAtPriority[key] = true

			// Build URL and attempt connection
			relayURL := buildRelayURL(selected)
			logger.Info("Attempting connection to %s (priority=%d, weight=%d)", relayURL, selected.Priority, selected.Weight)

			err := connectToRelay(relayURL, cameraID, token)
			if err == nil {
				// Connection was established and then closed normally
				return nil
			}

			logger.Warning("Connection to %s failed: %v", relayURL, err)
		}

		// Mark this priority as fully tried
		triedPriorities[currentPriority] = true
	}

	return fmt.Errorf("all relay servers exhausted")
}

// calculateReconnectDelay calculates exponential backoff delay with jitter
func calculateReconnectDelay() time.Duration {
	reconnectAttempt++

	// Exponential backoff: 2^attempt seconds, capped at maxReconnectDelay
	delay := minReconnectDelay * time.Duration(1<<uint(reconnectAttempt))
	if delay > maxReconnectDelay {
		delay = maxReconnectDelay
	}

	// Add random jitter to prevent thundering herd
	jitter := time.Duration(0)
	if maxReconnectJitter > 0 {
		jitter = time.Duration(float64(maxReconnectJitter) * (0.5 + 0.5*float64(time.Now().UnixNano()%1000)/1000.0))
	}

	return delay + jitter
}

// connectToRelay establishes and maintains a control connection to the relay
func connectToRelay(relayURL, cameraID, token string) error {
	headers := http.Header{}
	headers.Set("Camera-ID", cameraID)
	headers.Set("Role", "camera")
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	logger.Info("Connecting to relay as supervisor for camera %s", cameraID)

	// Create dialer with timeout and platform TLS config
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		TLSClientConfig:  tls.PlatformTLSConfig(),
	}

	conn, _, err := dialer.Dial(relayURL, headers)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	logger.Info("Supervisor control connection established")

	// Reset reconnect counter on successful connection
	reconnectMutex.Lock()
	isConnected = true
	reconnectAttempt = 0
	reconnectMutex.Unlock()

	// Set relay URL on forwarder manager now that we know the actual URL
	if globalForwarderManager != nil {
		globalForwarderManager.SetRelayURL(relayURL)
	}

	// Set up ping/pong to detect dead connections
	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// Start ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Channel for graceful shutdown
	done := make(chan struct{})

	// Send ping messages
	go func() {
		for {
			select {
			case <-pingTicker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					logger.Warning("Failed to send ping: %v", err)
					close(done)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Handle control messages from relay
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			close(done)
			return fmt.Errorf("read error: %v", err)
		}

		if messageType == websocket.TextMessage {
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				logger.Warning("Failed to parse control message: %v", err)
				continue
			}

			msgType, ok := msg["type"].(string)
			if !ok {
				logger.Warning("Control message missing type field")
				continue
			}

			switch msgType {
			case "start_streaming":
				handleStartStreaming(conn, msg)
			case "stop_streaming":
				handleStopStreaming(conn, msg)
			default:
				logger.Debug("Unknown control message type: %s", msgType)
			}
		}
	}
}

// handleStartStreaming handles start_streaming command from relay
func handleStartStreaming(conn *websocket.Conn, msg map[string]interface{}) {
	cameraID, _ := msg["camera_id"].(string)
	viewerToken, _ := msg["viewer_token"].(string)
	channelsRaw, _ := msg["channels"].([]interface{})
	viewerCount, _ := msg["viewer_count"].(float64)

	// Require viewer token
	if viewerToken == "" {
		logger.Warning("Viewer token missing, rejecting stream request")
		sendStatusUpdate(conn, map[string]interface{}{
			"type":      "streaming_rejected",
			"camera_id": cameraID,
			"reason":    "viewer_token_required",
		})
		return
	}

	// Verify token with backend
	if err := verifyViewerToken(viewerToken); err != nil {
		logger.Warning("Viewer token verification failed: %v", err)
		sendStatusUpdate(conn, map[string]interface{}{
			"type":      "streaming_rejected",
			"camera_id": cameraID,
			"reason":    "token_invalid",
		})
		return
	}

	logger.Info("Viewer token verified for camera %s", cameraID)

	channels := make([]string, 0, len(channelsRaw))
	for _, ch := range channelsRaw {
		if chStr, ok := ch.(string); ok {
			channels = append(channels, chStr)
		}
	}

	logger.Info("Received start_streaming command for camera %s, channels: %v, viewers: %d",
		cameraID, channels, int(viewerCount))

	if globalForwarderManager == nil {
		logger.Error("Forwarder manager not initialized")
		return
	}

	// Start forwarders for requested channels
	activeChannels := make([]string, 0, len(channels))
	for _, channel := range channels {
		if err := globalForwarderManager.Start(channel); err != nil {
			logger.Error("Failed to start forwarder for channel %s: %v", channel, err)
		} else {
			activeChannels = append(activeChannels, channel)
		}
	}

	// Send acknowledgment back to relay
	response := map[string]interface{}{
		"type":            "streaming_started",
		"camera_id":       cameraID,
		"channels_active": activeChannels,
	}
	sendStatusUpdate(conn, response)
}

// handleStopStreaming handles stop_streaming command from relay
func handleStopStreaming(conn *websocket.Conn, msg map[string]interface{}) {
	cameraID, _ := msg["camera_id"].(string)
	channelsRaw, _ := msg["channels"].([]interface{})

	channels := make([]string, 0, len(channelsRaw))
	for _, ch := range channelsRaw {
		if chStr, ok := ch.(string); ok {
			channels = append(channels, chStr)
		}
	}

	logger.Info("Received stop_streaming command for camera %s, channels: %v", cameraID, channels)

	if globalForwarderManager == nil {
		logger.Error("Forwarder manager not initialized")
		return
	}

	// Stop forwarders for requested channels
	for _, channel := range channels {
		globalForwarderManager.Stop(channel)
	}

	// Send acknowledgment back to relay
	response := map[string]interface{}{
		"type":      "streaming_stopped",
		"camera_id": cameraID,
	}
	sendStatusUpdate(conn, response)
}

// sendStatusUpdate sends a status message to the relay server
func sendStatusUpdate(conn *websocket.Conn, msg map[string]interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal status update: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		logger.Error("Failed to send status update: %v", err)
	} else {
		logger.Debug("Sent status update: %s", msg["type"])
	}
}
