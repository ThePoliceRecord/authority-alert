package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/authority-alert/video-relay-server/internal/auth"
	"github.com/authority-alert/video-relay-server/internal/config"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// getBaseCameraID extracts the base camera ID by removing channel suffixes (_high, _medium, _low)
func getBaseCameraID(cameraID string) string {
	// Remove common channel suffixes
	for _, suffix := range []string{"_high", "_medium", "_low"} {
		if strings.HasSuffix(cameraID, suffix) {
			return strings.TrimSuffix(cameraID, suffix)
		}
	}
	return cameraID
}

// ConnectionType indicates whether connection is camera or viewer
type ConnectionType string

const (
	ConnectionTypeCamera     ConnectionType = "camera"
	ConnectionTypeViewer     ConnectionType = "viewer"
	ConnectionTypeSupervisor ConnectionType = "supervisor"
)

// Connection represents a WebSocket connection
type Connection struct {
	ws         *websocket.Conn
	cType      ConnectionType
	cameraID   string
	userID     string
	claims     *auth.Claims
	lastActive time.Time
	send       chan []byte
	mu         sync.Mutex
}

// CameraID returns the camera ID for this connection
func (c *Connection) CameraID() string {
	return c.cameraID
}

// UserID returns the user ID for this connection
func (c *Connection) UserID() string {
	return c.userID
}

// Read reads a message from the WebSocket
func (c *Connection) Read() (int, []byte, error) {
	return c.ws.ReadMessage()
}

// Server manages camera and viewer connections
type Server struct {
	config       *config.Config
	auth         *auth.Validator
	cameras      map[string]*Connection            // cameraID -> Connection
	viewers      map[string]*Connection            // userID -> Connection
	supervisors  map[string]*Connection            // cameraID -> supervisor control Connection
	cameraMap    map[string]map[string]*Connection // cameraID -> set of viewer connections
	viewerCounts map[string]int                    // cameraID -> viewer count
	mu           sync.RWMutex
	logger       zerolog.Logger
}

// NewServer creates a new relay server
func NewServer(cfg *config.Config, authValidator *auth.Validator, logger zerolog.Logger) *Server {
	return &Server{
		config:       cfg,
		auth:         authValidator,
		cameras:      make(map[string]*Connection),
		viewers:      make(map[string]*Connection),
		supervisors:  make(map[string]*Connection),
		cameraMap:    make(map[string]map[string]*Connection),
		viewerCounts: make(map[string]int),
		logger:       logger,
	}
}

// RegisterCamera adds a camera connection
func (s *Server) RegisterCamera(conn *websocket.Conn, claims *auth.Claims) (*Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cameraID := claims.CameraID
	if cameraID == "" {
		return nil, fmt.Errorf("camera ID required")
	}

	// Check if camera already connected
	if existing, ok := s.cameras[cameraID]; ok {
		s.logger.Warn().Str("camera_id", cameraID).Msg("Replacing existing camera connection")
		existing.Close()
	}

	connection := &Connection{
		ws:         conn,
		cType:      ConnectionTypeCamera,
		cameraID:   cameraID,
		claims:     claims,
		lastActive: time.Now(),
		send:       make(chan []byte, 256),
	}

	s.cameras[cameraID] = connection
	s.logger.Info().Str("camera_id", cameraID).Msg("Camera registered")

	return connection, nil
}

// RegisterViewer adds a viewer connection
func (s *Server) RegisterViewer(conn *websocket.Conn, claims *auth.Claims) (*Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID := claims.UserID
	if userID == "" {
		return nil, fmt.Errorf("user ID required")
	}

	connection := &Connection{
		ws:         conn,
		cType:      ConnectionTypeViewer,
		userID:     userID,
		claims:     claims,
		lastActive: time.Now(),
		send:       make(chan []byte, 256),
	}

	s.viewers[userID] = connection
	s.logger.Info().Str("user_id", userID).Msg("Viewer registered")

	return connection, nil
}

// RegisterSupervisor adds a supervisor control connection
func (s *Server) RegisterSupervisor(conn *websocket.Conn, claims *auth.Claims) (*Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cameraID := claims.CameraID
	if cameraID == "" {
		return nil, fmt.Errorf("camera ID required for supervisor")
	}

	// Check if supervisor already connected
	if existing, ok := s.supervisors[cameraID]; ok {
		s.logger.Warn().Str("camera_id", cameraID).Msg("Replacing existing supervisor connection")
		existing.Close()
	}

	connection := &Connection{
		ws:         conn,
		cType:      ConnectionTypeSupervisor,
		cameraID:   cameraID,
		claims:     claims,
		lastActive: time.Now(),
		send:       make(chan []byte, 256),
	}

	s.supervisors[cameraID] = connection
	s.logger.Info().Str("camera_id", cameraID).Msg("Supervisor registered")

	return connection, nil
}

// SendControlMessage sends a control message to supervisor
func (s *Server) SendControlMessage(cameraID string, msgType string, channels []string, viewerCount int) error {
	s.mu.RLock()
	supervisor, ok := s.supervisors[cameraID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no supervisor connected for camera %s", cameraID)
	}

	msg := map[string]interface{}{
		"type":         msgType,
		"camera_id":    cameraID,
		"channels":     channels,
		"viewer_count": viewerCount,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case supervisor.send <- data:
		s.logger.Info().
			Str("camera_id", cameraID).
			Str("type", msgType).
			Int("viewer_count", viewerCount).
			Msg("Sent control message to supervisor")
		return nil
	default:
		return fmt.Errorf("supervisor send channel full")
	}
}

// SubscribeViewer subscribes a viewer to a camera stream
func (s *Server) SubscribeViewer(viewer *Connection, cameraID string) error {
	s.mu.Lock()

	// Check authorization
	if !viewer.claims.CanAccessCamera(cameraID) {
		s.mu.Unlock()
		return fmt.Errorf("access denied to camera %s", cameraID)
	}

	// Check viewer limit
	if viewers, ok := s.cameraMap[cameraID]; ok {
		if len(viewers) >= s.config.Limits.MaxViewersPerCamera {
			s.mu.Unlock()
			return fmt.Errorf("camera %s has max viewers", cameraID)
		}
	}

	// Add to camera's viewer list
	if _, ok := s.cameraMap[cameraID]; !ok {
		s.cameraMap[cameraID] = make(map[string]*Connection)
	}
	s.cameraMap[cameraID][viewer.userID] = viewer

	// Update viewer count
	s.viewerCounts[cameraID]++
	viewerCount := s.viewerCounts[cameraID]

	s.logger.Info().
		Str("user_id", viewer.userID).
		Str("camera_id", cameraID).
		Int("viewer_count", viewerCount).
		Msg("Viewer subscribed to camera")

	// Check if this is first viewer (before releasing lock)
	isFirstViewer := viewerCount == 1
	baseCameraID := getBaseCameraID(cameraID)

	// Release lock before sending control message to avoid deadlock
	s.mu.Unlock()

	// If this is the first viewer, send start_streaming to supervisor
	if isFirstViewer {
		channels := []string{"high", "medium", "low"}
		if err := s.SendControlMessage(baseCameraID, "start_streaming", channels, viewerCount); err != nil {
			s.logger.Warn().
				Err(err).
				Str("camera_id", baseCameraID).
				Str("full_camera_id", cameraID).
				Msg("Failed to send start_streaming command")
		}
	}

	return nil
}

// BroadcastToViewers sends data to all viewers of a camera
func (s *Server) BroadcastToViewers(cameraID string, data []byte) {
	s.mu.RLock()
	viewers, ok := s.cameraMap[cameraID]
	s.mu.RUnlock()

	if !ok {
		return
	}

	// Send to all viewers
	for _, viewer := range viewers {
		select {
		case viewer.send <- data:
		default:
			// Channel full, log warning
			s.logger.Warn().
				Str("user_id", viewer.userID).
				Str("camera_id", cameraID).
				Msg("Viewer send channel full, dropping frame")
		}
	}
}

// UnregisterCamera removes a camera connection
func (s *Server) UnregisterCamera(cameraID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, ok := s.cameras[cameraID]; ok {
		conn.Close()
		delete(s.cameras, cameraID)

		// Notify all viewers
		if viewers, ok := s.cameraMap[cameraID]; ok {
			for _, viewer := range viewers {
				// Send camera offline notification
				viewer.send <- []byte(`{"type":"camera_offline","camera_id":"` + cameraID + `"}`)
			}
			delete(s.cameraMap, cameraID)
		}

		s.logger.Info().Str("camera_id", cameraID).Msg("Camera unregistered")
	}
}

// UnregisterViewer removes a viewer connection
func (s *Server) UnregisterViewer(userID string) {
	s.mu.Lock()

	if conn, ok := s.viewers[userID]; ok {
		conn.Close()
		delete(s.viewers, userID)

		// Track which cameras need stop_streaming command
		var camerasToStop []struct {
			baseCameraID string
			fullCameraID string
		}

		// Remove from all camera subscriptions and update viewer counts
		for cameraID, viewers := range s.cameraMap {
			if _, subscribed := viewers[userID]; subscribed {
				delete(viewers, userID)

				// Update viewer count
				if s.viewerCounts[cameraID] > 0 {
					s.viewerCounts[cameraID]--
				}
				viewerCount := s.viewerCounts[cameraID]

				s.logger.Info().
					Str("user_id", userID).
					Str("camera_id", cameraID).
					Int("viewer_count", viewerCount).
					Msg("Viewer unsubscribed from camera")

				// If this was the last viewer, mark for stop command
				if viewerCount == 0 {
					delete(s.cameraMap, cameraID)
					delete(s.viewerCounts, cameraID)

					// Get base camera ID (strip channel suffix) for supervisor lookup
					baseCameraID := getBaseCameraID(cameraID)
					camerasToStop = append(camerasToStop, struct {
						baseCameraID string
						fullCameraID string
					}{baseCameraID, cameraID})
				}
			}
		}

		s.logger.Info().Str("user_id", userID).Msg("Viewer unregistered")

		// Release lock before sending control messages to avoid deadlock
		s.mu.Unlock()

		// Send stop_streaming commands for cameras with no viewers
		channels := []string{"high", "medium", "low"}
		for _, camera := range camerasToStop {
			if err := s.SendControlMessage(camera.baseCameraID, "stop_streaming", channels, 0); err != nil {
				s.logger.Warn().
					Err(err).
					Str("camera_id", camera.baseCameraID).
					Str("full_camera_id", camera.fullCameraID).
					Msg("Failed to send stop_streaming command")
			}
		}
	} else {
		s.mu.Unlock()
	}
}

// UnregisterSupervisor removes a supervisor connection
func (s *Server) UnregisterSupervisor(cameraID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, ok := s.supervisors[cameraID]; ok {
		conn.Close()
		delete(s.supervisors, cameraID)
		s.logger.Info().Str("camera_id", cameraID).Msg("Supervisor unregistered")
	}
}

// Close closes a connection
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ws != nil {
		c.ws.Close()
		close(c.send)
		c.ws = nil
	}
}

// WritePump sends messages from send channel to WebSocket
func (c *Connection) WritePump(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-c.send:
			if !ok {
				// Channel closed
				c.mu.Lock()
				if c.ws != nil {
					c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				}
				c.mu.Unlock()
				return
			}

			c.mu.Lock()
			if c.ws == nil {
				c.mu.Unlock()
				return
			}
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))

			// Choose message type based on connection type
			// Supervisors receive JSON text messages, cameras/viewers receive binary video data
			messageType := websocket.BinaryMessage
			if c.cType == ConnectionTypeSupervisor {
				messageType = websocket.TextMessage
			}

			err := c.ws.WriteMessage(messageType, message)
			c.mu.Unlock()

			if err != nil {
				return
			}
		case <-ticker.C:
			c.mu.Lock()
			if c.ws == nil {
				c.mu.Unlock()
				return
			}
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := c.ws.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()

			if err != nil {
				return
			}
		}
	}
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalViewers := 0
	for _, viewers := range s.cameraMap {
		totalViewers += len(viewers)
	}

	return map[string]interface{}{
		"cameras_connected": len(s.cameras),
		"viewers_connected": len(s.viewers),
		"active_streams":    len(s.cameraMap),
		"total_connections": totalViewers,
	}
}

// ValidateToken validates a JWT token string and returns claims
func (s *Server) ValidateToken(tokenString string) (*auth.Claims, error) {
	if s.auth == nil {
		return &auth.Claims{}, nil // Auth disabled
	}
	return s.auth.Validate(tokenString)
}

// AuthEnabled returns true if authentication is enabled
func (s *Server) AuthEnabled() bool {
	return s.auth != nil
}
