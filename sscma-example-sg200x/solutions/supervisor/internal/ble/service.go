package ble

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"supervisor/internal/auth"
	"supervisor/internal/network"
	"supervisor/internal/system"
	"supervisor/pkg/logger"
)

const (
	idleTimeout      = 60 * time.Second
	idleCheckPeriod  = 10 * time.Second
	oobeCheckPeriod  = 5 * time.Second
	hciWaitMax       = 60 * time.Second
	defaultMTU       = 23
	fragmentDelay    = 5 * time.Millisecond
	oobeFlagFile     = "/etc/oobe/flag"
)

// bleSession tracks the single active BLE client session.
type bleSession struct {
	token      string
	lastActive time.Time
}

// RegistrationService is implemented by DeviceHandler to avoid ble→handler import.
type RegistrationService interface {
	StartRegistrationDirect(locationName string, lat, lon float64) (map[string]interface{}, error)
	GetRegistrationStatusDirect() map[string]interface{}
}

// Service manages the BLE GATT server for WiFi provisioning and OOBE setup.
type Service struct {
	authMgr  *auth.AuthManager
	wifiMgr  *network.WiFiManager
	regSvc   RegistrationService

	mu       sync.Mutex
	conn     *dbus.Conn
	session  *bleSession
	chars    [5]*GattCharacteristic // DeviceInfo, Session, WiFiScan, WiFiConfig, Setup
	reasm    [5]Reassembler        // per-characteristic reassemblers
	mtu      int
	stopped  bool

	stopOnce sync.Once
	stopChan chan struct{}
}

// NewService creates a BLE provisioning service.
func NewService(authMgr *auth.AuthManager, wifiMgr *network.WiFiManager, regSvc RegistrationService) *Service {
	return &Service{
		authMgr:  authMgr,
		wifiMgr:  wifiMgr,
		regSvc:   regSvc,
		mtu:      defaultMTU,
		stopChan: make(chan struct{}),
	}
}

// Start initialises the BLE stack, registers the GATT application and
// advertisement with BlueZ, and blocks until Stop is called or OOBE ends.
// It is safe to call from a goroutine.
func (s *Service) Start() error {
	if !isOOBEActive() {
		logger.Info("BLE: OOBE not active, skipping BLE start")
		return nil
	}

	// Wait for hci0 to appear.
	if err := s.waitForHCI(); err != nil {
		return err
	}

	// Make sure bluetoothd is running (needed for BlueZ D-Bus API).
	if err := ensureBluetoothd(); err != nil {
		return fmt.Errorf("BLE: failed to start bluetoothd: %w", err)
	}

	// Connect to system D-Bus.
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("BLE: D-Bus connection failed: %w", err)
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	// Build advertising name: "AA-" + last 6 hex of WiFi MAC.
	advName := advertisingName()

	// Power on the adapter.
	if err := ensureAdapterPowered(conn); err != nil {
		logger.Warning("BLE: could not power adapter: %v", err)
	}

	// Set the adapter alias so clients that read the adapter name
	// (instead of the advertisement LocalName) see the correct name.
	if err := setAdapterAlias(conn, advName); err != nil {
		logger.Warning("BLE: could not set adapter alias: %v", err)
	}

	// Export D-Bus objects.
	chars, err := exportObjects(conn, advName)
	if err != nil {
		return fmt.Errorf("BLE: D-Bus export failed: %w", err)
	}
	s.mu.Lock()
	s.chars = chars
	s.mu.Unlock()

	// Wire characteristic callbacks.
	s.wireCallbacks()

	// Register GATT application with BlueZ.
	if err := registerGATT(conn); err != nil {
		return fmt.Errorf("BLE: GATT registration failed: %w", err)
	}

	// Register advertisement.
	if err := registerAdvertisement(conn); err != nil {
		return fmt.Errorf("BLE: advertisement registration failed: %w", err)
	}

	logger.Info("BLE service started, advertising as %s", advName)

	// Background goroutines for idle-timeout and OOBE monitoring.
	go s.idleLoop()
	go s.oobeLoop()

	// Block until stopped.
	<-s.stopChan
	return nil
}

// Stop unregisters the GATT application and advertisement and closes D-Bus.
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		conn := s.conn
		s.session = nil
		s.mu.Unlock()

		if conn != nil {
			unregisterAdvertisement(conn)
			unregisterGATT(conn)
			conn.Close()
		}

		close(s.stopChan)
		logger.Info("BLE service stopped")
	})
}

// updateMTU updates the negotiated MTU if it changed.
func (s *Service) updateMTU(mtu int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mtu > 514 {
		mtu = 514 // avoid 512-byte ATT values (controller firmware bug)
	}
	if mtu != s.mtu {
		logger.Info("BLE: MTU updated %d -> %d", s.mtu, mtu)
		s.mtu = mtu
	}
}

// ---- characteristic callbacks ----

func (s *Service) wireCallbacks() {
	// Wire MTU extraction on all characteristics.
	for i := range s.chars {
		s.chars[i].onMTU = func(mtu int) { s.updateMTU(mtu) }
	}

	// DeviceInfo (read-only).
	s.chars[0].onRead = func() ([]byte, *dbus.Error) {
		return s.buildDeviceInfo(), nil
	}

	// Session (FC01): auth command.
	s.chars[1].onWrite = func(value []byte) *dbus.Error {
		s.handleCharWrite(1, value)
		return nil
	}

	// WiFi Scan (FC02).
	s.chars[2].onWrite = func(value []byte) *dbus.Error {
		s.handleCharWrite(2, value)
		return nil
	}

	// WiFi Config (FC03).
	s.chars[3].onWrite = func(value []byte) *dbus.Error {
		s.handleCharWrite(3, value)
		return nil
	}

	// Setup (FC05).
	s.chars[4].onWrite = func(value []byte) *dbus.Error {
		s.handleCharWrite(4, value)
		return nil
	}
}

// handleCharWrite reassembles fragments, parses the JSON request, and
// dispatches to the appropriate command handler.
func (s *Service) handleCharWrite(idx int, value []byte) {
	complete := s.reasm[idx].Write(value)
	if complete == nil {
		return // more fragments expected
	}

	var req Request
	if err := json.Unmarshal(complete, &req); err != nil {
		logger.Warning("BLE: bad JSON on char %d: %v", idx, err)
		s.sendResponse(idx, Response{Code: 400, Msg: "invalid request"})
		return
	}

	switch idx {
	case 1: // Session
		resp := s.handleAuth(req)
		s.sendResponse(idx, resp)

	case 2: // WiFi Scan
		resp := s.handleScan(req)
		s.sendResponse(idx, resp)

	case 3: // WiFi Config
		switch req.Cmd {
		case "connect":
			go s.handleConnect(req, func(r Response) { s.sendResponse(idx, r) })
		case "status":
			resp := s.handleStatus(req)
			s.sendResponse(idx, resp)
		default:
			s.sendResponse(idx, Response{Code: 400, Msg: "unknown command"})
		}

	case 4: // Setup (FC05)
		resp := s.handleSetup(req)
		s.sendResponse(idx, resp)
	}
}

// sendResponse JSON-encodes a response, fragments it, and sends each
// fragment as a BLE notification on the given characteristic.
func (s *Service) sendResponse(charIdx int, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		logger.Error("BLE: marshal error: %v", err)
		return
	}

	s.mu.Lock()
	conn := s.conn
	c := s.chars[charIdx]
	mtu := s.mtu
	s.mu.Unlock()

	if conn == nil || c == nil {
		return
	}

	packets := Fragment(data, mtu)
	logger.Debug("BLE: char[%d] response %d bytes -> %d fragment(s), MTU %d", charIdx, len(data), len(packets), mtu)
	for i, pkt := range packets {
		if i > 0 {
			time.Sleep(fragmentDelay)
		}
		if err := c.sendNotify(conn, pkt); err != nil {
			logger.Warning("BLE: notify error: %v", err)
			return
		}
	}
}

// ---- device info ----

func (s *Service) buildDeviceInfo() []byte {
	apCfg := s.wifiMgr.GetAPConfig()
	oobeState := "complete"
	if isOOBEActive() {
		oobeState = "pending"
	}

	info := map[string]interface{}{
		"serial":     system.GetSerialNumber(),
		"firmware":   system.GetOSVersion(),
		"ap_ssid":    apCfg.SSID,
		"ap_ip":      "192.168.16.1",
		"oobe_state": oobeState,
		"proto_ver":  1,
		"model":      "reCamera 200x",
	}
	data, _ := json.Marshal(info)
	return data
}

// ---- advertising name ----

func advertisingName() string {
	mac := system.GetMAC("wlan0")
	// MAC format: "aa:bb:cc:dd:ee:ff"
	mac = strings.ReplaceAll(mac, ":", "")
	if len(mac) >= 6 {
		suffix := strings.ToUpper(mac[len(mac)-6:])
		return "AA-" + suffix
	}
	return "AA-000000"
}

// ---- lifecycle helpers ----

func (s *Service) waitForHCI() error {
	deadline := time.Now().Add(hciWaitMax)
	for time.Now().Before(deadline) {
		if _, err := os.Stat("/sys/class/bluetooth/hci0"); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("BLE: hci0 not found after %s", hciWaitMax)
}

func ensureBluetoothd() error {
	if system.IsProcessRunning("bluetoothd") {
		return nil
	}
	cmd := exec.Command("/usr/libexec/bluetooth/bluetoothd", "--experimental")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	// Give bluetoothd time to register on D-Bus.
	time.Sleep(1 * time.Second)
	return nil
}

func isOOBEActive() bool {
	_, err := os.Stat(oobeFlagFile)
	return err == nil
}

// idleLoop invalidates the BLE session after idleTimeout of inactivity.
func (s *Service) idleLoop() {
	ticker := time.NewTicker(idleCheckPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.session != nil && time.Since(s.session.lastActive) > idleTimeout {
				logger.Info("BLE: session idle timeout, invalidating")
				s.session = nil
			}
			s.mu.Unlock()
		}
	}
}

// oobeLoop stops the BLE service when OOBE is no longer active.
func (s *Service) oobeLoop() {
	ticker := time.NewTicker(oobeCheckPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			if !isOOBEActive() {
				logger.Info("BLE: OOBE completed, stopping BLE service")
				s.Stop()
				return
			}
		}
	}
}
