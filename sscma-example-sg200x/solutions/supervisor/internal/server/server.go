// Package server provides the HTTP server for the supervisor.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"supervisor/internal/api"
	"supervisor/internal/auth"
	"supervisor/internal/config"
	"supervisor/internal/handler"
	"supervisor/internal/middleware"
	"supervisor/internal/ntp"
	"supervisor/internal/oobe"
	"supervisor/internal/system"
	"supervisor/internal/tls"
	"supervisor/pkg/logger"
)

// Server represents the HTTP server.
type Server struct {
	cfg              *config.Config
	httpServer       *http.Server
	httpsServer      *http.Server
	detectionWsServer *http.Server
	authManager      *auth.AuthManager
	wifiHandler      *handler.WiFiHandler
	qrHandler        *handler.QRHandler
	tlsManager       *tls.Manager
	oobeManager      *oobe.Manager
	ntpManager       *ntp.Manager
	serverCtx        context.Context
	serverCancel     context.CancelFunc
}

// New creates a new Server.
func New(cfg *config.Config, ntpManager *ntp.Manager) *Server {
	// Build certificate subject from config
	certSubject := tls.CertSubject{
		Organization: cfg.CertOrganization,
		Country:      cfg.CertCountry,
		Province:     cfg.CertProvince,
		Locality:     cfg.CertLocality,
		Issuer:       cfg.CertIssuer,
		// CommonName is left empty to use device name from file
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		cfg:          cfg,
		authManager:  auth.NewAuthManager(cfg),
		tlsManager:   tls.NewManagerWithSubject(cfg.CertDir, certSubject),
		oobeManager:  oobe.New(),
		ntpManager:   ntpManager,
		serverCtx:    ctx,
		serverCancel: cancel,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	// Ensure TLS certificates exist
	if err := s.tlsManager.EnsureCertificates(); err != nil {
		return fmt.Errorf("failed to ensure TLS certificates: %w", err)
	}

	// OTA uploads can be hundreds of MB to multiple GB; 30s is too small and causes connection resets.
	// Use longer timeouts so the browser can finish uploading over slower links.
	readTimeout := 10 * time.Minute
	writeTimeout := 10 * time.Minute

	// Get TLS configuration
	tlsConfig, err := s.tlsManager.GetTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to get TLS config: %w", err)
	}

	// Start OOBE manager (checks flag and starts OOBE if needed)
	if err := s.oobeManager.Start(); err != nil {
		logger.Warning("OOBE initialization failed: %v", err)
		// Non-fatal - continue with normal operation
	}

	// Start detection WebSocket server (receives from camera-detector)
	s.startDetectionWsServer()

	apiMux := s.setupRoutes()

	// Create top-level mux that routes WebSocket directly (bypass middleware)
	rootMux := http.NewServeMux()

	// WebSocket camera proxy - direct connection (no middleware)
	rootMux.HandleFunc("/ws/camera", handler.CameraWebSocketProxy)

	// Apply middleware chain to the base handler
	baseHandler := middleware.Chain(
		apiMux,
		middleware.Recovery,
		middleware.SecureHeaders,
		middleware.Logging,
		middleware.CORS,
	)

	// Wrap with OOBE handler - proxies to OOBE when active, otherwise uses base handler
	rootMux.Handle("/", s.oobeManager.Handler(baseHandler))

	// HTTP server - redirects all traffic to HTTPS
	// Only start if HTTPPort is configured (non-empty)
	if s.cfg.HTTPPort != "" && s.cfg.HTTPPort != "0" {
		s.httpServer = &http.Server{
			Addr:         ":" + s.cfg.HTTPPort,
			Handler:      s.httpHandler(rootMux),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			logger.Info("HTTP server starting on port %s (AP clients served over HTTP; others redirected to HTTPS)", s.cfg.HTTPPort)
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTP server error: %v", err)
			}
		}()
	}

	// HTTPS server (required) - use rootMux instead of wrapped handler
	s.httpsServer = &http.Server{
		Addr:         ":" + s.cfg.HTTPSPort,
		Handler:      rootMux,
		TLSConfig:    tlsConfig,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("HTTPS server starting on port %s", s.cfg.HTTPSPort)
		if err := s.httpsServer.ListenAndServeTLS(s.tlsManager.CertFile(), s.tlsManager.KeyFile()); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTPS server error: %v", err)
		}
	}()

	return nil
}

func (s *Server) httpHandler(rootHandler http.Handler) http.Handler {
	redirectHandler := s.httpsRedirectHandler()

	// AP clients are on wlan1 and should get a plain HTTP captive portal experience.
	apNet := &net.IPNet{IP: net.IPv4(192, 168, 16, 0), Mask: net.CIDRMask(24, 32)}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := clientIPFromRequest(r); ip != nil {
			if ip4 := ip.To4(); ip4 != nil && apNet.Contains(ip4) {
				rootHandler.ServeHTTP(w, r)
				return
			}
		}

		redirectHandler.ServeHTTP(w, r)
	})
}

func clientIPFromRequest(r *http.Request) net.IP {
	// Prefer X-Forwarded-For when present (first hop).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip
		}
	}

	// RemoteAddr is typically ip:port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}

	// Fallback: try parsing the full RemoteAddr (some servers may omit port).
	return net.ParseIP(r.RemoteAddr)
}

// httpsRedirectHandler returns a handler that redirects all HTTP requests to HTTPS.
func (s *Server) httpsRedirectHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if already HTTPS (via X-Forwarded-Proto header or TLS)
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			// Already HTTPS, don't redirect
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Build the HTTPS URL
		host := r.Host
		// Remove port if present
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}

		// Add HTTPS port if not default
		var targetURL string
		if s.cfg.HTTPSPort == "443" {
			targetURL = fmt.Sprintf("https://%s%s", host, r.RequestURI)
		} else {
			targetURL = fmt.Sprintf("https://%s:%s%s", host, s.cfg.HTTPSPort, r.RequestURI)
		}

		http.Redirect(w, r, targetURL, http.StatusMovedPermanently)
	})
}

// Stop gracefully stops the server.
func (s *Server) Stop(ctx context.Context) error {
	// Cancel server context to stop background goroutines
	if s.serverCancel != nil {
		s.serverCancel()
	}

	// Stop detection WebSocket server
	if s.detectionWsServer != nil {
		if err := s.detectionWsServer.Shutdown(ctx); err != nil {
			logger.Error("Detection WS server shutdown error: %v", err)
		}
	}

	// Stop OOBE manager first
	if s.oobeManager != nil {
		if err := s.oobeManager.Stop(ctx); err != nil {
			logger.Error("OOBE manager shutdown error: %v", err)
		}
	}

	// Stop WiFi handler
	if s.wifiHandler != nil {
		s.wifiHandler.Stop()
	}

	// Stop QR handler
	if s.qrHandler != nil {
		s.qrHandler.Close()
	}

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			logger.Error("HTTP server shutdown error: %v", err)
		}
	}

	// Shutdown HTTPS server
	if s.httpsServer != nil {
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			logger.Error("HTTPS server shutdown error: %v", err)
		}
	}

	return nil
}

// setupRoutes configures the HTTP routes.
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Create handlers
	userHandler := handler.NewUserHandler(s.authManager)
	deviceHandler := handler.NewDeviceHandler(s.ntpManager)
	s.wifiHandler = handler.NewWiFiHandler()
	fileHandler := handler.NewFileHandler()
	ledHandler := handler.NewLEDHandler()
	s.qrHandler = handler.NewQRHandler()
	recordingHandler := handler.NewRecordingHandler()
	updateConfigHandler := handler.NewUpdateConfigHandler()
	detectionHandler := handler.NewDetectionHandler()
	storageHandler := handler.NewStorageHandler()
	securityHandler := handler.NewSecurityHandler()

	// Paths that don't require authentication
	// Only include endpoints needed before login
	noAuthPaths := map[string]bool{
		"/api/userMgr/login":             true,
		"/api/deviceMgr/queryDeviceInfo": true, // Needed before login to get device SN
	}

	// Auth middleware
	authMiddleware := middleware.Auth(s.authManager, noAuthPaths)

	// API routes - wrap with auth middleware
	apiHandler := http.NewServeMux()

	// Version endpoint
	apiHandler.HandleFunc("/api/version", s.handleVersion)

	// User management
	apiHandler.HandleFunc("/api/userMgr/login", userHandler.Login)
	apiHandler.HandleFunc("/api/userMgr/queryUserInfo", userHandler.QueryUserInfo)
	apiHandler.HandleFunc("/api/userMgr/updatePassword", userHandler.UpdatePassword)
	apiHandler.HandleFunc("/api/userMgr/setSShStatus", userHandler.SetSSHStatus)
	apiHandler.HandleFunc("/api/userMgr/addSShkey", userHandler.AddSSHKey)
	apiHandler.HandleFunc("/api/userMgr/deleteSShkey", userHandler.DeleteSSHKey)

	// Device management
	apiHandler.HandleFunc("/api/deviceMgr/queryDeviceInfo", deviceHandler.QueryDeviceInfo)
	apiHandler.HandleFunc("/api/deviceMgr/getDeviceInfo", deviceHandler.GetDeviceInfo)
	apiHandler.HandleFunc("/api/deviceMgr/getDeviceList", deviceHandler.GetDeviceList)
	apiHandler.HandleFunc("/api/deviceMgr/updateDeviceName", deviceHandler.UpdateDeviceName)
	apiHandler.HandleFunc("/api/deviceMgr/getCameraWebsocketUrl", deviceHandler.GetCameraWebsocketUrl)
	apiHandler.HandleFunc("/api/deviceMgr/queryServiceStatus", deviceHandler.QueryServiceStatus)
	apiHandler.HandleFunc("/api/deviceMgr/getSystemStatus", deviceHandler.GetSystemStatus)
	apiHandler.HandleFunc("/api/deviceMgr/getInternetStatus", deviceHandler.GetInternetStatus)
	apiHandler.HandleFunc("/api/deviceMgr/setPower", deviceHandler.SetPower)
	apiHandler.HandleFunc("/api/deviceMgr/getModelList", deviceHandler.GetModelList)
	apiHandler.HandleFunc("/api/deviceMgr/getModelInfo", deviceHandler.GetModelInfo)
	apiHandler.HandleFunc("/api/deviceMgr/getModelFile", deviceHandler.GetModelFile)
	apiHandler.HandleFunc("/api/deviceMgr/uploadModel", deviceHandler.UploadModel)
	apiHandler.HandleFunc("/api/deviceMgr/uploadUpdatePackage", deviceHandler.UploadUpdatePackage)
	apiHandler.HandleFunc("/api/deviceMgr/getUploadedUpdatePackage", deviceHandler.GetUploadedUpdatePackage)
	apiHandler.HandleFunc("/api/deviceMgr/applyUploadedUpdatePackage", deviceHandler.ApplyUploadedUpdatePackage)
	apiHandler.HandleFunc("/api/deviceMgr/setTimestamp", deviceHandler.SetTimestamp)
	apiHandler.HandleFunc("/api/deviceMgr/getTimestamp", deviceHandler.GetTimestamp)
	apiHandler.HandleFunc("/api/deviceMgr/setTimezone", deviceHandler.SetTimezone)
	apiHandler.HandleFunc("/api/deviceMgr/getTimezone", deviceHandler.GetTimezone)
	apiHandler.HandleFunc("/api/deviceMgr/getTimezoneList", deviceHandler.GetTimezoneList)
	apiHandler.HandleFunc("/api/deviceMgr/updateChannel", deviceHandler.UpdateChannel)
	apiHandler.HandleFunc("/api/deviceMgr/getSystemUpdateVersion", deviceHandler.GetSystemUpdateVersion)
	apiHandler.HandleFunc("/api/deviceMgr/getUpdateCheckProgress", deviceHandler.GetUpdateCheckProgress)
	apiHandler.HandleFunc("/api/deviceMgr/updateSystem", deviceHandler.UpdateSystem)
	apiHandler.HandleFunc("/api/deviceMgr/getUpdateProgress", deviceHandler.GetUpdateProgress)
	apiHandler.HandleFunc("/api/deviceMgr/cancelUpdate", deviceHandler.CancelUpdate)
	apiHandler.HandleFunc("/api/deviceMgr/factoryReset", deviceHandler.FactoryReset)
	apiHandler.HandleFunc("/api/deviceMgr/formatSDCard", deviceHandler.FormatSDCard)
	apiHandler.HandleFunc("/api/deviceMgr/getPlatformInfo", deviceHandler.GetPlatformInfo)
	apiHandler.HandleFunc("/api/deviceMgr/savePlatformInfo", deviceHandler.SavePlatformInfo)
	apiHandler.HandleFunc("/api/deviceMgr/getAnalyticsConfig", deviceHandler.GetAnalyticsConfig)
	apiHandler.HandleFunc("/api/deviceMgr/setAnalyticsConfig", deviceHandler.SetAnalyticsConfig)
	apiHandler.HandleFunc("/api/deviceMgr/reRegisterCamera", deviceHandler.ReRegisterCamera)

	// Code-based camera registration (OOBE)
	apiHandler.HandleFunc("/api/deviceMgr/startCodeRegistration", deviceHandler.StartCodeRegistration)
	apiHandler.HandleFunc("/api/deviceMgr/codeRegistrationStatus", deviceHandler.GetCodeRegistrationStatus)
	apiHandler.HandleFunc("/api/deviceMgr/cancelCodeRegistration", deviceHandler.CancelCodeRegistration)

	// Model update management
	apiHandler.HandleFunc("/api/deviceMgr/getModelUpdateStatus", deviceHandler.GetModelUpdateStatus)
	apiHandler.HandleFunc("/api/deviceMgr/checkModelUpdates", deviceHandler.CheckModelUpdates)

	// Camera/Video management
	apiHandler.HandleFunc("/api/channels", handler.GetChannels)

	// WiFi management
	apiHandler.HandleFunc("/api/wifiMgr/getWiFiInfoList", s.wifiHandler.GetWiFiInfoList)
	apiHandler.HandleFunc("/api/wifiMgr/getConnectionStatus", s.wifiHandler.GetConnectionStatus)
	apiHandler.HandleFunc("/api/wifiMgr/connectWiFi", s.wifiHandler.ConnectWiFi)
	apiHandler.HandleFunc("/api/wifiMgr/disconnectWiFi", s.wifiHandler.DisconnectWiFi)
	apiHandler.HandleFunc("/api/wifiMgr/forgetWiFi", s.wifiHandler.ForgetWiFi)
	apiHandler.HandleFunc("/api/wifiMgr/switchWiFi", s.wifiHandler.SwitchWiFi)

	// File management
	apiHandler.HandleFunc("/api/fileMgr/list", fileHandler.List)
	apiHandler.HandleFunc("/api/fileMgr/mkdir", fileHandler.Mkdir)
	apiHandler.HandleFunc("/api/fileMgr/remove", fileHandler.Remove)
	apiHandler.HandleFunc("/api/fileMgr/upload", fileHandler.Upload)
	apiHandler.HandleFunc("/api/fileMgr/download", fileHandler.Download)
	apiHandler.HandleFunc("/api/fileMgr/rename", fileHandler.Rename)
	apiHandler.HandleFunc("/api/fileMgr/info", fileHandler.Info)
	apiHandler.HandleFunc("/api/fileMgr/storageInfo", fileHandler.StorageInfo)

	// LED management
	apiHandler.HandleFunc("/api/ledMgr/getLEDs", ledHandler.GetLEDs)
	apiHandler.HandleFunc("/api/ledMgr/getLED", ledHandler.GetLED)
	apiHandler.HandleFunc("/api/ledMgr/setLED", ledHandler.SetLED)
	apiHandler.HandleFunc("/api/ledMgr/getLEDTriggers", ledHandler.GetLEDTriggers)

	// Recording management
	apiHandler.HandleFunc("/api/recordingMgr/getConfig", recordingHandler.GetConfig)
	apiHandler.HandleFunc("/api/recordingMgr/setConfig", recordingHandler.SetConfig)

	// Update configuration management
	apiHandler.HandleFunc("/api/updateMgr/getConfig", updateConfigHandler.GetConfig)
	apiHandler.HandleFunc("/api/updateMgr/setConfig", updateConfigHandler.SetConfig)

	// QR Code management
	apiHandler.HandleFunc("/api/qr/scan", s.qrHandler.StartScan)
	apiHandler.HandleFunc("/api/qr/scan/", s.qrHandler.GetScanStatus)
	apiHandler.HandleFunc("/api/qr/health", s.qrHandler.GetHealth)
	// Note: Cancel uses DELETE on /api/qr/scan/{id}, handled by GetScanStatus/CancelScan based on method

	// Detection management (stats only - actual upload endpoint is unauthenticated)
	apiHandler.HandleFunc("/api/detection/stats", detectionHandler.GetDetectionStats)
	apiHandler.HandleFunc("/api/detection/queue", detectionHandler.GetDetectionQueue)
	apiHandler.HandleFunc("/api/detection/image", detectionHandler.GetDetectionImage)

	// Storage/Database management
	apiHandler.HandleFunc("/api/storage/cleanupStatus", storageHandler.GetCleanupStatus)
	apiHandler.HandleFunc("/api/storage/cleanup", storageHandler.RunCleanup)
	apiHandler.HandleFunc("/api/storage/dbStats", storageHandler.GetDatabaseStats)
	apiHandler.HandleFunc("/api/storage/scanMissing", storageHandler.ScanMissingFiles)

	// Security configuration (firmware signing, TSA timestamping, frame hashing)
	apiHandler.HandleFunc("/api/security/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			securityHandler.GetSecurityConfig(w, r)
		} else if r.Method == http.MethodPost {
			securityHandler.SetSecurityConfig(w, r)
		} else {
			api.WriteError(w, -1, "Method not allowed")
		}
	})

	// Apply auth middleware to API routes
	mux.Handle("/api/", authMiddleware(apiHandler))

	// Detection upload endpoint - no auth required (comes from local camera-detector)
	mux.HandleFunc("/detection", detectionHandler.HandleDetection)

	// Static file server for web UI
	fileServer := http.FileServer(http.Dir(s.cfg.RootDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Handle root path specially to avoid redirect loop
		if r.URL.Path == "/" {
			http.ServeFile(w, r, s.cfg.RootDir+"/index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	return mux
}

// handleVersion handles the version endpoint.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	api.WriteSuccess(w, map[string]interface{}{
		"uptime":    system.GetUptime(),
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0", // Will be set at build time
	})
}

// startDetectionWsServer starts the WebSocket server for receiving detections from camera-detector.
// This runs on a separate port (8089) and only accepts connections from localhost.
func (s *Server) startDetectionWsServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/detection", handler.HandleDetectionWebSocket)

	addr := fmt.Sprintf("127.0.0.1:%d", handler.DetectionWsPort)
	s.detectionWsServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("========================================")
	logger.Info("Detection WebSocket Server")
	logger.Info("Endpoint: ws://%s/ws/detection", addr)
	logger.Info("Accepts: localhost connections only")
	logger.Info("========================================")

	go func() {
		logger.Info("Detection WS server listening on %s", addr)
		if err := s.detectionWsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Detection WS server FAILED: %v", err)
		}
	}()
}
