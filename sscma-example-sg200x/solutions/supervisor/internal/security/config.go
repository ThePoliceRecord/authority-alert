// Package security provides configuration for security features.
//
// Security features can be enabled/disabled via /etc/recamera.conf/security.json
// This allows gradual rollout of security features without firmware updates.
package security

import (
	"encoding/json"
	"os"
	"sync"

	"supervisor/internal/timestamp"
	"supervisor/internal/upgrade"
	"supervisor/pkg/logger"
)

// ConfigPath is the path to the security configuration file.
const ConfigPath = "/etc/recamera.conf/security.json"

// Config holds the security feature flags.
type Config struct {
	// FirmwareSigningEnabled enables ECDSA signature verification for firmware updates.
	// When enabled, firmware packages must have a valid .sig file.
	// Default: false (disabled for initial rollout)
	FirmwareSigningEnabled bool `json:"firmware_signing_enabled"`

	// TSATimestampingEnabled enables RFC 3161 trusted timestamps via FreeTSA.
	// When enabled, detection uploads include cryptographic timestamp tokens.
	// Note: Frame hashes are always computed by camera-streamer (cheap operation),
	// but TSA tokens are only requested when this is enabled.
	// Default: false (disabled for initial rollout)
	TSATimestampingEnabled bool `json:"tsa_timestamping_enabled"`

	// EvidenceChainEnabled enables the full trusted evidence chain.
	// When enabled, frame_hash and timestamp fields are included in detection uploads.
	// This is separate from TSATimestampingEnabled to allow testing hash propagation
	// without the overhead of TSA requests.
	// Default: false (disabled for initial rollout)
	EvidenceChainEnabled bool `json:"evidence_chain_enabled"`
}

var (
	config     Config
	configOnce sync.Once
)

// Load reads the security configuration from disk and applies it.
// Safe to call multiple times; only loads once.
func Load() {
	configOnce.Do(func() {
		loadAndApply()
	})
}

// Reload forces a reload of the security configuration.
// Use this after modifying the config file.
func Reload() {
	loadAndApply()
}

func loadAndApply() {
	// Start with defaults (all disabled)
	config = Config{
		FirmwareSigningEnabled: false,
		TSATimestampingEnabled: false,
		EvidenceChainEnabled:   false,
	}

	// Try to load from file
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning("Failed to read security config: %v", err)
		}
		// Use defaults - all security features disabled
		applyConfig()
		return
	}

	if err := json.Unmarshal(data, &config); err != nil {
		logger.Warning("Failed to parse security config: %v (using defaults)", err)
		config = Config{} // Reset to defaults
	}

	applyConfig()
}

func applyConfig() {
	// Apply to upgrade package
	upgrade.SetFirmwareSigningEnabled(config.FirmwareSigningEnabled)

	// Apply to timestamp package
	timestamp.SetEnabled(config.TSATimestampingEnabled)

	logger.Info("Security config: firmware_signing=%v, tsa_timestamping=%v, evidence_chain=%v",
		config.FirmwareSigningEnabled,
		config.TSATimestampingEnabled,
		config.EvidenceChainEnabled)
}

// Get returns the current security configuration.
func Get() Config {
	return config
}

// EvidenceChainEnabled returns whether the evidence chain is enabled.
// When true, frame_hash and timestamp fields are included in detection uploads.
func EvidenceChainEnabled() bool {
	return config.EvidenceChainEnabled
}

// Save writes the current configuration to disk.
func Save() error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath, data, 0644)
}

// Update modifies the security configuration and saves it.
func Update(c Config) error {
	config = c
	applyConfig()
	return Save()
}
