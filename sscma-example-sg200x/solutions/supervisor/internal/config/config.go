// Package config provides configuration management for the supervisor.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Config holds all configuration for the supervisor.
type Config struct {
	// Server settings
	HTTPPort   string // Optional HTTP port (for redirect to HTTPS only)
	HTTPSPort  string // HTTPS port (required)
	RootDir    string
	ScriptPath string

	// TLS settings
	CertDir string // Directory for TLS certificates

	// Certificate subject fields
	CertOrganization string // Organization name for certificate
	CertCountry      string // Country code (e.g., "US", "CN")
	CertProvince     string // State/Province
	CertLocality     string // City/Locality
	CertIssuer       string // Issuer organization name

	// Security settings
	JWTSecret           []byte
	TokenExpiration     time.Duration
	BCryptCost          int
	MaxLoginAttempts    int
	LoginLockoutMinutes int

	// Storage settings
	LocalDir string
	SDDir    string

	// Logging
	LogLevel int

	// Runtime settings
	DaemonMode bool

	// The Police Record / Authority Alert platform settings
	TPRPlatformURL string // Base URL for platform API (e.g. https://thepolicerecord.com)
}

// LogLevels
const (
	LogError   = 0
	LogWarning = 1
	LogInfo    = 2
	LogDebug   = 3
	LogVerbose = 4
)

var (
	instance *Config
	once     sync.Once
)

// DefaultConfig returns a new Config with default values.
func DefaultConfig() *Config {
	return &Config{
		HTTPPort:   "80",  // HTTP for redirect only
		HTTPSPort:  "443", // HTTPS is required
		RootDir:    "/usr/share/supervisor/www/",
		ScriptPath: "/usr/share/supervisor/scripts/main.sh",

		CertDir:          "/etc/recamera.conf/certs",
		CertOrganization: "Seeed Studio",
		CertCountry:      "CN",
		CertProvince:     "Guangdong",
		CertLocality:     "Shenzhen",
		CertIssuer:       "Seeed Studio",

		JWTSecret:           nil, // Will be generated
		TokenExpiration:     72 * time.Hour,
		BCryptCost:          12,
		MaxLoginAttempts:    5,
		LoginLockoutMinutes: 15,

		LocalDir: "/userdata",
		SDDir:    "/mnt/sd",

		LogLevel: LogInfo,

		DaemonMode: false,

		// Default to development for testing
		// Override via env: TPR_PLATFORM_URL=https://thepolicerecord.com for production
		TPRPlatformURL: "https://thepolicerecord.com",
	}
}

// Get returns the singleton configuration instance.
func Get() *Config {
	once.Do(func() {
		instance = DefaultConfig()
		instance.loadFromEnv()
		if instance.JWTSecret == nil {
			// Try to load JWT secret from file first (for persistence across restarts)
			instance.JWTSecret = instance.loadOrCreateJWTSecret()
		}
	})
	return instance
}

// jwtSecretFile returns the path to the JWT secret file.
func (c *Config) jwtSecretFile() string {
	// Store in the cert directory alongside TLS certs
	return filepath.Join(c.CertDir, "jwt_secret")
}

// loadOrCreateJWTSecret loads the JWT secret from file, or creates a new one if it doesn't exist.
// This ensures tokens remain valid across supervisor restarts.
func (c *Config) loadOrCreateJWTSecret() []byte {
	secretFile := c.jwtSecretFile()

	// Try to read existing secret
	if data, err := os.ReadFile(secretFile); err == nil && len(data) >= 32 {
		// Secret file exists and has valid content
		log.Printf("[config] Loaded existing JWT secret from %s (tokens will persist)", secretFile)
		return data
	}

	// Generate new secret
	secret := generateSecureKey(32)
	log.Printf("[config] Generated new JWT secret (old tokens will be invalid)")

	// Try to save it for future restarts
	// Create directory if needed
	dir := filepath.Dir(secretFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		// Can't create directory - secret won't persist
		log.Printf("[config] WARNING: Cannot create directory %s: %v - JWT secret will not persist", dir, err)
		return secret
	}

	// Write secret with restrictive permissions (owner read/write only)
	if err := os.WriteFile(secretFile, secret, 0600); err != nil {
		// Log warning but continue - secret will just be regenerated on next restart
		log.Printf("[config] WARNING: Cannot save JWT secret to %s: %v - tokens will not persist across restarts", secretFile, err)
	} else {
		log.Printf("[config] Saved JWT secret to %s (tokens will persist across restarts)", secretFile)
	}

	return secret
}

// loadFromEnv loads configuration from environment variables.
func (c *Config) loadFromEnv() {
	if port := os.Getenv("SUPERVISOR_HTTP_PORT"); port != "" {
		c.HTTPPort = port
	}
	if port := os.Getenv("SUPERVISOR_HTTPS_PORT"); port != "" {
		c.HTTPSPort = port
	}
	if dir := os.Getenv("SUPERVISOR_ROOT_DIR"); dir != "" {
		c.RootDir = dir
	}
	if script := os.Getenv("SUPERVISOR_SCRIPT_PATH"); script != "" {
		c.ScriptPath = script
	}
	if certDir := os.Getenv("SUPERVISOR_CERT_DIR"); certDir != "" {
		c.CertDir = certDir
	}
	if org := os.Getenv("SUPERVISOR_CERT_ORG"); org != "" {
		c.CertOrganization = org
	}
	if country := os.Getenv("SUPERVISOR_CERT_COUNTRY"); country != "" {
		c.CertCountry = country
	}
	if province := os.Getenv("SUPERVISOR_CERT_PROVINCE"); province != "" {
		c.CertProvince = province
	}
	if locality := os.Getenv("SUPERVISOR_CERT_LOCALITY"); locality != "" {
		c.CertLocality = locality
	}
	if issuer := os.Getenv("SUPERVISOR_CERT_ISSUER"); issuer != "" {
		c.CertIssuer = issuer
	}
	if secret := os.Getenv("SUPERVISOR_JWT_SECRET"); secret != "" {
		c.JWTSecret = []byte(secret)
	}
	if level := os.Getenv("SUPERVISOR_LOG_LEVEL"); level != "" {
		if l, err := strconv.Atoi(level); err == nil {
			c.LogLevel = l
		}
	}
	if localDir := os.Getenv("SUPERVISOR_LOCAL_DIR"); localDir != "" {
		c.LocalDir = localDir
	}
	if sdDir := os.Getenv("SUPERVISOR_SD_DIR"); sdDir != "" {
		c.SDDir = sdDir
	}
	if platformURL := os.Getenv("TPR_PLATFORM_URL"); platformURL != "" {
		c.TPRPlatformURL = platformURL
	}
}

// generateSecureKey generates a cryptographically secure random key.
func generateSecureKey(length int) []byte {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		// Fallback: this should never happen
		panic("failed to generate secure key: " + err.Error())
	}
	return key
}

// GenerateSecureToken generates a cryptographically secure random token.
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// SetJWTSecret sets the JWT secret key (for testing or loading from secure storage).
func (c *Config) SetJWTSecret(secret []byte) {
	c.JWTSecret = secret
}
