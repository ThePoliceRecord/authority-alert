package ble

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"supervisor/internal/auth"
	"supervisor/pkg/logger"
)

var errUnauthorized = errors.New("unauthorized")

// validateSession checks that token matches the active session and resets the idle timer.
func (s *Service) validateSession(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil || s.session.token != token {
		return errUnauthorized
	}
	s.session.lastActive = time.Now()
	return nil
}

// handleAuth processes the "auth" command on the Session characteristic (FC01).
func (s *Service) handleAuth(req Request) Response {
	username := auth.GetUsername()

	// During OOBE, no password exists yet — issue a token without verification.
	// BLE is already gated to OOBE-only (advertising stops when OOBE ends).
	if isOOBEActive() {
		token, err := s.authMgr.GenerateToken(username)
		if err != nil {
			logger.Error("BLE OOBE token generation failed: %v", err)
			return Response{Code: 500, Msg: "token generation failed"}
		}

		s.mu.Lock()
		s.session = &bleSession{
			token:      token,
			lastActive: time.Now(),
		}
		s.mu.Unlock()

		logger.Info("BLE session authenticated (OOBE bypass)")
		return Response{
			Code: 200,
			Msg:  "authenticated",
			Data: map[string]string{
				"token": token,
				"ap_ip": "192.168.16.1",
			},
		}
	}

	var data struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(req.Data, &data); err != nil || data.Password == "" {
		return Response{Code: 400, Msg: "password required"}
	}

	token, err := s.authMgr.Authenticate(username, data.Password)
	if err != nil {
		logger.Warning("BLE auth failed: %v", err)
		return Response{Code: 401, Msg: "invalid password"}
	}

	// One session at a time — invalidate any previous session.
	s.mu.Lock()
	s.session = &bleSession{
		token:      token,
		lastActive: time.Now(),
	}
	s.mu.Unlock()

	logger.Info("BLE session authenticated")
	return Response{
		Code: 200,
		Msg:  "authenticated",
		Data: map[string]string{
			"token": token,
			"ap_ip": "192.168.16.1",
		},
	}
}

// handleScan processes the "scan" command on the WiFi Scan characteristic (FC02).
func (s *Service) handleScan(req Request) Response {
	if err := s.validateSession(req.Token); err != nil {
		return Response{Code: 401, Msg: "unauthorized"}
	}

	type scanResult struct {
		SSID     string `json:"ssid"`
		Signal   int    `json:"signal"`
		Security string `json:"security"`
		Channel  int    `json:"channel"`
	}

	// Scan with a timeout.
	type scanOut struct {
		results []scanResult
		err     error
	}
	ch := make(chan scanOut, 1)
	go func() {
		networks, err := s.wifiMgr.Scan()
		if err != nil {
			ch <- scanOut{err: err}
			return
		}
		var res []scanResult
		for _, n := range networks {
			if n.SSID == "" {
				continue
			}
			res = append(res, scanResult{
				SSID:     n.SSID,
				Signal:   n.Signal,
				Security: n.Security,
				Channel:  freqToChannel(n.Frequency),
			})
		}
		sort.Slice(res, func(i, j int) bool { return res[i].Signal > res[j].Signal })
		ch <- scanOut{results: res}
	}()

	select {
	case out := <-ch:
		if out.err != nil {
			return Response{Code: 500, Msg: "scan failed: " + out.err.Error()}
		}
		return Response{Code: 200, Msg: "scan complete", Data: map[string]interface{}{"networks": out.results}}
	case <-time.After(15 * time.Second):
		return Response{Code: 408, Msg: "scan timeout"}
	}
}

// handleConnect processes the "connect" command on the WiFi Config characteristic (FC03).
// It sends an immediate "connecting" notification, then polls for success.
func (s *Service) handleConnect(req Request, notify func(Response)) {
	if err := s.validateSession(req.Token); err != nil {
		notify(Response{Code: 401, Msg: "unauthorized"})
		return
	}

	var data struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
		Security string `json:"security"`
	}
	if err := json.Unmarshal(req.Data, &data); err != nil || data.SSID == "" {
		notify(Response{Code: 400, Msg: "ssid required"})
		return
	}

	// Acknowledge immediately.
	notify(Response{Code: 200, Msg: "connecting"})

	// Start connection.
	if err := s.wifiMgr.Connect(data.SSID, data.Password, -1); err != nil {
		notify(Response{Code: 500, Msg: "connect failed: " + err.Error()})
		return
	}

	// Poll for connection result.
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			notify(Response{Code: 408, Msg: "connection timeout"})
			return
		case <-ticker.C:
			status, err := s.wifiMgr.GetStatus()
			if err != nil {
				continue
			}
			if status.Connected && status.SSID == data.SSID {
				notify(Response{
					Code: 200,
					Msg:  "connected",
					Data: map[string]string{"ip": status.IP, "ssid": status.SSID},
				})
				return
			}
		}
	}
}

// handleStatus processes the "status" command on the WiFi Config characteristic (FC03).
func (s *Service) handleStatus(req Request) Response {
	if err := s.validateSession(req.Token); err != nil {
		return Response{Code: 401, Msg: "unauthorized"}
	}

	status, err := s.wifiMgr.GetStatus()
	if err != nil {
		return Response{Code: 500, Msg: "status unavailable"}
	}

	var wifiState string
	switch {
	case status.State == "COMPLETED" && status.Connected:
		wifiState = "connected"
	case status.State == "SCANNING" || status.State == "ASSOCIATING" || status.State == "ASSOCIATED" ||
		status.State == "4WAY_HANDSHAKE" || status.State == "GROUP_HANDSHAKE":
		wifiState = "connecting"
	default:
		wifiState = "disconnected"
	}

	return Response{
		Code: 200,
		Msg:  "ok",
		Data: map[string]string{
			"wifi_state": wifiState,
			"ssid":       status.SSID,
			"ip":         status.IP,
		},
	}
}

// freqToChannel converts a WiFi frequency string (MHz) to a channel number.
func freqToChannel(freq string) int {
	f, err := strconv.Atoi(strings.TrimSpace(freq))
	if err != nil || f == 0 {
		return 0
	}
	switch {
	case f == 2484:
		return 14
	case f >= 2412 && f <= 2472:
		return (f - 2407) / 5
	case f >= 5170 && f <= 5825:
		return (f - 5000) / 5
	default:
		return 0
	}
}
