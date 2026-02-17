package handler

import (
	"io"
	"net/http"
	"supervisor/pkg/logger"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	// Optimize buffer sizes for video streaming
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
}

// Buffer pool for zero-copy transfers
var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64*1024)
		return &buf
	},
}

// CameraWebSocketProxy proxies WebSocket connections from browser (WSS) to camera-streamer (WS)
func CameraWebSocketProxy(w http.ResponseWriter, r *http.Request) {
	// Upgrade browser connection to WebSocket
	browserConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade browser connection: %v", err)
		return
	}
	defer browserConn.Close()

	// Extract channel parameter from query string (required by camera-streamer)
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		channel = "1" // Default to channel 1 (medium resolution)
	}

	// Connect to camera-streamer on localhost with channel parameter
	cameraURL := "ws://localhost:8765/?channel=" + channel
	cameraConn, _, err := websocket.DefaultDialer.Dial(cameraURL, nil)
	if err != nil {
		logger.Error("Failed to connect to camera-streamer (channel %s): %v", channel, err)
		browserConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Camera unavailable"))
		return
	}
	defer cameraConn.Close()

	logger.Info("WebSocket proxy established for channel %s", channel)

	// Bidirectional relay with zero-copy optimization
	done := make(chan struct{}, 2)

	// Camera -> Browser (optimized for binary frames)
	go func() {
		defer func() { done <- struct{}{} }()
		buf := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(buf)

		for {
			messageType, reader, err := cameraConn.NextReader()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
					return
				}
				logger.Warning("Camera read error: %v", err)
				return
			}

			// Get writer for browser
			writer, err := browserConn.NextWriter(messageType)
			if err != nil {
				return
			}

			// Zero-copy transfer using pooled buffer
			_, err = io.CopyBuffer(writer, reader, *buf)
			writer.Close()

			if err != nil {
				return
			}
		}
	}()

	// Browser -> Camera (control messages, typically small)
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			messageType, message, err := browserConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
					return
				}
				logger.Warning("Browser read error: %v", err)
				return
			}
			if err := cameraConn.WriteMessage(messageType, message); err != nil {
				return
			}
		}
	}()

	// Wait for either direction to close
	<-done
	logger.Info("WebSocket proxy closed")
}
