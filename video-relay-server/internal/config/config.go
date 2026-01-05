package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration
type Config struct {
	Server ServerConfig `yaml:"server"`
	Auth   AuthConfig   `yaml:"auth"`
	Limits LimitsConfig `yaml:"limits"`
	Log    LogConfig    `yaml:"log"`
}

// ServerConfig contains server settings
type ServerConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	ReadTimeout  int    `yaml:"read_timeout"`  // seconds
	WriteTimeout int    `yaml:"write_timeout"` // seconds
	TLSCert      string `yaml:"tls_cert"`
	TLSKey       string `yaml:"tls_key"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	JWTPublicKey  string `yaml:"jwt_public_key_path"`
	JWTPrivateKey string `yaml:"jwt_private_key_path"`
	Enabled       bool   `yaml:"enabled"`
}

// LimitsConfig contains rate limiting and connection limits
type LimitsConfig struct {
	MaxCamerasPerViewer   int `yaml:"max_cameras_per_viewer"`
	MaxViewersPerCamera   int `yaml:"max_viewers_per_camera"`
	MaxMessageSizeBytes   int `yaml:"max_message_size_bytes"`
	ConnectionIdleTimeout int `yaml:"connection_idle_timeout"` // seconds
}

// LogConfig contains logging settings
type LogConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8443
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 60
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 60
	}
	if cfg.Limits.MaxMessageSizeBytes == 0 {
		cfg.Limits.MaxMessageSizeBytes = 1024 * 1024 // 1MB
	}
	if cfg.Limits.MaxViewersPerCamera == 0 {
		cfg.Limits.MaxViewersPerCamera = 10
	}
	if cfg.Limits.ConnectionIdleTimeout == 0 {
		cfg.Limits.ConnectionIdleTimeout = 300 // 5 minutes
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}

	return &cfg, nil
}
