package handler

import (
	"context"
	"fmt"
	"net/http"
	"supervisor/internal/tls"
	"supervisor/pkg/logger"
	"time"

	"github.com/gorilla/websocket"
)

// ExternalRelayForwarderWithContext forwards camera-streamer video to external relay server with context support
func ExternalRelayForwarderWithContext(ctx context.Context, cameraURL string, externalRelayURL string, cameraID string, token string) {
	// Create dialer with timeout and platform TLS config
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		TLSClientConfig:  tls.PlatformTLSConfig(),
	}

	// Connect to local camera-streamer
	cameraConn, _, err := dialer.Dial(cameraURL, nil)
	if err != nil {
		logger.Error("Failed to connect to camera-streamer: %v", err)
		return
	}
	defer cameraConn.Close()

	// Set read deadline for camera connection
	cameraConn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Connect to external relay server
	headers := http.Header{}
	headers.Set("Camera-ID", cameraID)
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	relayConn, _, err := dialer.Dial(externalRelayURL, headers)
	if err != nil {
		logger.Error("Failed to connect to external relay: %v", err)
		return
	}
	defer relayConn.Close()

	// Set read deadline for relay connection
	relayConn.SetReadDeadline(time.Now().Add(30 * time.Second))

	logger.Info("External relay connected for camera %s", cameraID)

	// Bidirectional relay
	done := make(chan struct{}, 2)
	errChan := make(chan error, 2)

	// Camera -> External Relay
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				cameraConn.SetReadDeadline(time.Now().Add(30 * time.Second))
				messageType, message, err := cameraConn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						logger.Warning("Camera read error: %v", err)
						errChan <- fmt.Errorf("camera read: %v", err)
					}
					return
				}

				relayConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := relayConn.WriteMessage(messageType, message); err != nil {
					logger.Warning("Relay write error: %v", err)
					errChan <- fmt.Errorf("relay write: %v", err)
					return
				}
			}
		}
	}()

	// External Relay -> Camera (for control commands)
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				relayConn.SetReadDeadline(time.Now().Add(90 * time.Second))
				messageType, message, err := relayConn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						logger.Warning("Relay read error: %v", err)
						errChan <- fmt.Errorf("relay read: %v", err)
					}
					return
				}

				// Forward control commands to camera
				cameraConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := cameraConn.WriteMessage(messageType, message); err != nil {
					logger.Warning("Camera write error: %v", err)
					errChan <- fmt.Errorf("camera write: %v", err)
					return
				}
			}
		}
	}()

	// Wait for either direction to close or context cancellation
	select {
	case <-done:
		// Check for error
		select {
		case err := <-errChan:
			logger.Debug("Forwarder closed with error for camera %s: %v", cameraID, err)
		default:
		}
	case <-ctx.Done():
		logger.Debug("Forwarder cancelled for camera %s", cameraID)
	}
}

