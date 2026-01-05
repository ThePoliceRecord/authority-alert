package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"encoding/json"

	"github.com/authority-alert/video-relay-server/internal/auth"
	"github.com/authority-alert/video-relay-server/internal/config"
	"github.com/authority-alert/video-relay-server/internal/relay"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: Implement proper origin checking
	},
}

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Setup logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.Log.Format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Setup authentication
	var authValidator *auth.Validator
	if cfg.Auth.Enabled {
		authValidator, err = auth.NewValidator(cfg.Auth.JWTPublicKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize authentication")
		}
	}

	// Create relay server
	relayServer := relay.NewServer(cfg, authValidator, log.Logger)

	// Add logging middleware
	loggingMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote", r.RemoteAddr).
				Str("user_agent", r.Header.Get("User-Agent")).
				Msg("Incoming HTTP request")
			next(w, r)
		}
	}

	// Setup HTTP handlers
	http.HandleFunc("/ws", loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(relayServer, w, r)
	}))

	http.HandleFunc("/health", loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := relayServer.GetStats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Simple JSON response
		w.Write([]byte("{"))
		first := true
		for k, v := range stats {
			if !first {
				w.Write([]byte(","))
			}
			first = false
			switch v := v.(type) {
			case int:
				w.Write([]byte("\"" + k + "\":" + string(rune(v))))
			}
		}
		w.Write([]byte("}"))
	})

	// Serve test page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "test/viewer.html")
	})

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "test/viewer.html")
	})

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Server shutdown error")
		}
	}()

	log.Info().Str("addr", addr).Msg("Starting video relay server")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func handleWebSocket(relayServer *relay.Server, w http.ResponseWriter, r *http.Request) {
	log.Info().
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.Header.Get("User-Agent")).
		Str("camera_id_header", r.Header.Get("Camera-ID")).
		Str("role_header", r.Header.Get("Role")).
		Str("auth_header", r.Header.Get("Authorization")).
		Msg("WebSocket upgrade request received")

	var claims *auth.Claims
	var err error

	// Extract authentication token if auth is enabled
	token := r.Header.Get("Authorization")
	cameraID := r.Header.Get("Camera-ID")
	role := r.Header.Get("Role")

	if relayServer.AuthEnabled() {
		log.Info().Msg("Auth is enabled, validating token")
		if token == "" {
			log.Warn().Msg("Missing authorization token")
			http.Error(w, "Missing authorization", http.StatusUnauthorized)
			return
		}

		// Validate token
		claims, err = relayServer.ValidateToken(token)
		if err != nil {
			log.Warn().Err(err).Msg("Invalid token")
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
	} else {
		log.Info().Msg("Auth is disabled, using headers")
		// Auth disabled - use headers directly
		claims = &auth.Claims{}

		if role == "supervisor" && cameraID != "" {
			claims.Role = "supervisor"
			claims.CameraID = cameraID
			log.Info().Str("camera_id", cameraID).Msg("Accepting supervisor connection")
		} else if cameraID != "" {
			claims.Role = "camera"
			claims.CameraID = cameraID
			log.Info().Str("camera_id", cameraID).Msg("Accepting camera connection")
		} else {
			claims.Role = "viewer"
			claims.UserID = "anonymous_" + r.RemoteAddr
			claims.AllowedCameras = []string{"*"} // Allow all cameras when auth disabled
			log.Info().Str("user_id", claims.UserID).Msg("Accepting viewer connection")
		}
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}

	// Register connection based on role
	var connection *relay.Connection
	if claims.Role == "supervisor" {
		connection, err = relayServer.RegisterSupervisor(conn, claims)
		if err != nil {
			conn.Close()
			return
		}
		go handleSupervisorConnection(relayServer, connection)
	} else if claims.Role == "camera" {
		connection, err = relayServer.RegisterCamera(conn, claims)
		if err != nil {
			conn.Close()
			return
		}
		go handleCameraConnection(relayServer, connection)
	} else if claims.Role == "viewer" {
		connection, err = relayServer.RegisterViewer(conn, claims)
		if err != nil {
			conn.Close()
			return
		}
		go handleViewerConnection(relayServer, connection)
	} else {
		conn.Close()
		return
	}

	// Start write pump
	go connection.WritePump(context.Background())
}

func handleCameraConnection(relayServer *relay.Server, conn *relay.Connection) {
	defer relayServer.UnregisterCamera(conn.CameraID())

	log.Info().Str("camera_id", conn.CameraID()).Msg("Camera connection handler started")

	frameCount := 0
	for {
		messageType, data, err := conn.Read()
		if err != nil {
			log.Error().Err(err).Str("camera_id", conn.CameraID()).Msg("Read error")
			return
		}

		if messageType == websocket.BinaryMessage {
			frameCount++
			if frameCount%30 == 0 { // Log every 30 frames
				log.Info().Str("camera_id", conn.CameraID()).Int("frames", frameCount).Int("bytes", len(data)).Msg("Received frames from camera")
			}
			// Forward video data to viewers
			relayServer.BroadcastToViewers(conn.CameraID(), data)
		}
		// Ignore text messages from camera for now
	}
}

func handleViewerConnection(relayServer *relay.Server, conn *relay.Connection) {
	defer relayServer.UnregisterViewer(conn.UserID())

	for {
		messageType, data, err := conn.Read()
		if err != nil {
			log.Error().Err(err).Str("user_id", conn.UserID()).Msg("Read error")
			return
		}

		if messageType == websocket.TextMessage {
			// Parse JSON command
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Warn().Err(err).Msg("Failed to parse viewer message")
				continue
			}

			msgType, ok := msg["type"].(string)
			if !ok {
				continue
			}

			switch msgType {
			case "subscribe":
				cameraID, ok := msg["camera_id"].(string)
				if !ok {
					log.Warn().Msg("Missing camera_id in subscribe message")
					continue
				}

				if err := relayServer.SubscribeViewer(conn, cameraID); err != nil {
					log.Error().Err(err).Str("camera_id", cameraID).Msg("Failed to subscribe viewer")
				} else {
					log.Info().Str("user_id", conn.UserID()).Str("camera_id", cameraID).Msg("Viewer subscribed")
				}

			case "unsubscribe":
				// TODO: Implement unsubscribe
			}
		}
	}
}

func handleSupervisorConnection(relayServer *relay.Server, conn *relay.Connection) {
	defer relayServer.UnregisterSupervisor(conn.CameraID())

	log.Info().Str("camera_id", conn.CameraID()).Msg("Supervisor connection handler started")

	for {
		messageType, data, err := conn.Read()
		if err != nil {
			log.Error().Err(err).Str("camera_id", conn.CameraID()).Msg("Supervisor read error")
			return
		}

		if messageType == websocket.TextMessage {
			// Parse JSON status message from supervisor
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Warn().Err(err).Msg("Failed to parse supervisor message")
				continue
			}

			msgType, ok := msg["type"].(string)
			if !ok {
				continue
			}

			// Log supervisor status updates
			switch msgType {
			case "streaming_started":
				log.Info().
					Str("camera_id", conn.CameraID()).
					Interface("channels", msg["channels_active"]).
					Msg("Supervisor confirmed streaming started")
			case "streaming_stopped":
				log.Info().
					Str("camera_id", conn.CameraID()).
					Msg("Supervisor confirmed streaming stopped")
			default:
				log.Debug().
					Str("camera_id", conn.CameraID()).
					Str("type", msgType).
					Msg("Received supervisor message")
			}
		}
	}
}
