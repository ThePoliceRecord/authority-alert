// Package handler provides HTTP handlers for the supervisor API.
package handler

import (
	"encoding/json"
	"net/http"

	"supervisor/internal/api"
	"supervisor/internal/security"
	"supervisor/pkg/logger"
)

// SecurityHandler handles security configuration endpoints.
type SecurityHandler struct{}

// NewSecurityHandler creates a new security handler.
func NewSecurityHandler() *SecurityHandler {
	return &SecurityHandler{}
}

// GetSecurityConfig returns the current security configuration.
func (h *SecurityHandler) GetSecurityConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	cfg := security.Get()
	api.WriteSuccess(w, cfg)
}

// SetSecurityConfig updates the security configuration.
func (h *SecurityHandler) SetSecurityConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var cfg security.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		api.WriteError(w, -1, "Invalid JSON: "+err.Error())
		return
	}

	if err := security.Update(cfg); err != nil {
		logger.Error("Failed to update security config: %v", err)
		api.WriteError(w, -1, "Failed to save configuration")
		return
	}

	logger.Info("Security configuration updated: firmware_signing=%v, tsa_timestamping=%v, evidence_chain=%v",
		cfg.FirmwareSigningEnabled,
		cfg.TSATimestampingEnabled,
		cfg.EvidenceChainEnabled)

	api.WriteSuccess(w, cfg)
}
