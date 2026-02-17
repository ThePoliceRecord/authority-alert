package handler

import (
	"net/http"
	"supervisor/internal/api"
)

// ChannelInfo represents metadata for a video channel
type ChannelInfo struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Resolution   string `json:"resolution"`
	FPS          int    `json:"fps"`
	Bitrate      string `json:"bitrate"`
	Codec        string `json:"codec"`
	UseCase      string `json:"use_case"`
	WebSocketURL string `json:"websocket_url"`
	ShmPath      string `json:"shm_path"`
	Active       bool   `json:"active"`
}

// ChannelsResponse represents the response for /api/channels
type ChannelsResponse struct {
	Channels       []ChannelInfo `json:"channels"`
	DefaultChannel int           `json:"default_channel"`
}

// GetChannels returns metadata for all available video channels
func GetChannels(w http.ResponseWriter, r *http.Request) {
	// Get the host from the request to construct WebSocket URLs
	host := r.Host

	channels := []ChannelInfo{
		{
			ID:           0,
			Name:         "High Resolution",
			Resolution:   "1920x1080",
			FPS:          30,
			Bitrate:      "2-4 Mbps",
			Codec:        "H.264",
			UseCase:      "Recording, archival, high-quality streaming",
			WebSocketURL: "ws://" + host + ":8765/?channel=0",
			ShmPath:      "/video_stream_ch0",
			Active:       true,
		},
		{
			ID:           1,
			Name:         "Medium Resolution",
			Resolution:   "1280x720",
			FPS:          30,
			Bitrate:      "1-2 Mbps",
			Codec:        "H.264",
			UseCase:      "Web viewing, remote monitoring",
			WebSocketURL: "ws://" + host + ":8765/?channel=1",
			ShmPath:      "/video_stream_ch1",
			Active:       true,
		},
		{
			ID:           2,
			Name:         "Low Resolution",
			Resolution:   "640x480",
			FPS:          15,
			Bitrate:      "0.5-1 Mbps",
			Codec:        "H.264",
			UseCase:      "AI/ML processing, motion detection",
			WebSocketURL: "ws://" + host + ":8765/?channel=2",
			ShmPath:      "/video_stream_ch2",
			Active:       true,
		},
	}

	response := ChannelsResponse{
		Channels:       channels,
		DefaultChannel: 1, // Default to medium resolution
	}

	api.WriteSuccess(w, response)
}
