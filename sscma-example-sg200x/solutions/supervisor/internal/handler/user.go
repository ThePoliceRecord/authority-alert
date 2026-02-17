// Package handler provides HTTP request handlers for the supervisor API.
package handler

import (
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"supervisor/internal/api"
	"supervisor/internal/auth"
	"supervisor/pkg/logger"
)

// Default username for the system
const DefaultUsername = "recamera"

// UserHandler handles user management API requests.
type UserHandler struct {
	authManager *auth.AuthManager
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(am *auth.AuthManager) *UserHandler {
	return &UserHandler{
		authManager: am,
	}
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `json:"userName"`
	Password string `json:"password"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Token      string `json:"token"`
	RetryCount int    `json:"retryCount,omitempty"`
}

// Login handles user login.
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		api.WriteError(w, -1, "Username and password required")
		return
	}

	token, err := h.authManager.Authenticate(req.Username, req.Password)
	if err != nil {
		logger.Warn("Login failed for user %s: %v", req.Username, err)
		api.WriteError(w, -1, "Authentication failed")
		return
	}

	logger.Info("User %s logged in successfully", req.Username)
	api.WriteSuccess(w, LoginResponse{Token: token})
}

// QueryUserInfo returns user information.
func (h *UserHandler) QueryUserInfo(w http.ResponseWriter, r *http.Request) {
	username := auth.GetUsernameFromRequest(r)
	logger.Info("[SSH DEBUG] QueryUserInfo called, username from auth: %s", username)

	// Use default username if not set
	if username == "" {
		username = DefaultUsername
		logger.Warn("[SSH DEBUG] Username was empty, using default: %s", username)
	}

	// Check SSH status
	sshEnabled := isSSHEnabled()
	logger.Info("[SSH DEBUG] SSH enabled status: %v", sshEnabled)

	// Get SSH keys
	sshKeys := getSSHKeys(username)
	logger.Info("[SSH DEBUG] Retrieved %d SSH keys for user %s", len(sshKeys), username)

	api.WriteSuccess(w, map[string]interface{}{
		"userName":   username,
		"sshEnabled": sshEnabled,
		"sshkeyList": sshKeys,
	})
}

// UpdatePasswordRequest represents a password update request.
type UpdatePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// UpdatePassword handles password updates.
// This endpoint requires authentication.
func (h *UserHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	username := auth.GetUsernameFromRequest(r)

	var req UpdatePasswordRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		api.WriteError(w, -1, "Old and new passwords required")
		return
	}

	// Validate password strength
	if len(req.NewPassword) < 8 {
		api.WriteError(w, -1, "Password must be at least 8 characters")
		return
	}

	// Verify old password first
	_, err := h.authManager.Authenticate(username, req.OldPassword)
	if err != nil {
		api.WriteError(w, -1, "Current password is incorrect")
		return
	}

	// Change password using passwd command (same as original shell script)
	// The passwd command expects the new password twice via stdin
	cmd := exec.Command("passwd", username)
	cmd.Stdin = strings.NewReader(req.NewPassword + "\n" + req.NewPassword + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to change password: %v, output: %s", err, string(output))
		api.WriteError(w, -1, "Failed to update password")
		return
	}

	logger.Info("Password updated for user %s", username)
	api.WriteSuccess(w, map[string]interface{}{"message": "Password updated successfully"})
}

// SetSSHStatusRequest represents an SSH status update request.
type SetSSHStatusRequest struct {
	Enable  bool `json:"enable"`
	Enabled bool `json:"enabled"` // Support both for backwards compatibility
}

// SetSSHStatus enables or disables SSH.
func (h *UserHandler) SetSSHStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req SetSSHStatusRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		logger.Error("[SSH DEBUG] Failed to parse SetSSHStatus request body: %v", err)
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	// Support both 'enable' and 'enabled' fields
	enable := req.Enable || req.Enabled
	logger.Info("[SSH DEBUG] SetSSHStatus called with Enable=%v", enable)

	var err error
	if enable {
		// Try SysV init script first, fall back to systemctl
		if _, statErr := os.Stat("/etc/init.d/S50dropbear"); statErr == nil {
			err = exec.Command("/etc/init.d/S50dropbear", "start").Run()
		} else {
			err = exec.Command("systemctl", "start", "dropbear").Run()
			if err == nil {
				err = exec.Command("systemctl", "enable", "dropbear").Run()
			}
		}
	} else {
		// Try SysV init script first, fall back to systemctl
		if _, statErr := os.Stat("/etc/init.d/S50dropbear"); statErr == nil {
			err = exec.Command("/etc/init.d/S50dropbear", "stop").Run()
		} else {
			err = exec.Command("systemctl", "stop", "dropbear").Run()
			if err == nil {
				err = exec.Command("systemctl", "disable", "dropbear").Run()
			}
		}
	}

	if err != nil {
		logger.Error("Failed to change SSH status: %v", err)
		api.WriteError(w, -1, "Failed to update SSH status")
		return
	}

	logger.Info("[SSH DEBUG] SetSSHStatus completed successfully, Enable=%v", enable)
	api.WriteSuccess(w, map[string]interface{}{"enabled": enable})
}

// AddSSHKeyRequest represents an SSH key addition request.
type AddSSHKeyRequest struct {
	Key   string `json:"key"`   // Legacy field
	Value string `json:"value"` // New field from frontend
	Name  string `json:"name"`
	Time  string `json:"time"` // Optional field from frontend
}

// AddSSHKey adds an SSH public key.
func (h *UserHandler) AddSSHKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req AddSSHKeyRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		logger.Error("[SSH DEBUG] Failed to parse AddSSHKey request body: %v", err)
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	// Support both 'key' and 'value' fields
	sshKey := req.Key
	if sshKey == "" {
		sshKey = req.Value
	}

	logger.Info("[SSH DEBUG] AddSSHKey called with Key length=%d, Name=%s", len(sshKey), req.Name)

	if sshKey == "" {
		logger.Error("[SSH DEBUG] SSH key is empty")
		api.WriteError(w, -1, "SSH key required")
		return
	}

	// Validate SSH key format
	if !isValidSSHKey(sshKey) {
		logger.Error("[SSH DEBUG] Invalid SSH key format: %s", sshKey[:min(50, len(sshKey))])
		api.WriteError(w, -1, "Invalid SSH key format")
		return
	}

	username := auth.GetUsernameFromRequest(r)
	logger.Info("[SSH DEBUG] Username from auth context: %s", username)

	// Use default username if not set
	if username == "" {
		username = DefaultUsername
		logger.Warn("[SSH DEBUG] Username was empty, using default: %s", username)
	}

	// Validate username to prevent path traversal
	if !isValidUsername(username) {
		logger.Error("Invalid username detected: %s", username)
		api.WriteError(w, -1, "Invalid username")
		return
	}

	sshDir := "/home/" + username + "/.ssh"
	authKeysFile := sshDir + "/authorized_keys"

	// Verify the path is within expected home directory
	absSSHDir, err := filepath.Abs(sshDir)
	if err != nil || !strings.HasPrefix(absSSHDir, "/home/"+username+"/") {
		logger.Error("SSH directory path validation failed")
		api.WriteError(w, -1, "Failed to add SSH key")
		return
	}

	// Ensure .ssh directory exists
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		logger.Error("Failed to create .ssh directory: %v", err)
		api.WriteError(w, -1, "Failed to add SSH key")
		return
	}

	// Set ownership of .ssh directory to the user
	if err := chownToUser(sshDir, username); err != nil {
		logger.Error("Failed to set .ssh directory ownership: %v", err)
		// Continue anyway - the key might still work
	}

	// Append key to authorized_keys
	f, err := os.OpenFile(authKeysFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		logger.Error("Failed to open authorized_keys: %v", err)
		api.WriteError(w, -1, "Failed to add SSH key")
		return
	}
	defer f.Close()

	keyLine := sshKey
	if req.Name != "" {
		keyLine = keyLine + " " + req.Name
	}
	if _, err := f.WriteString(keyLine + "\n"); err != nil {
		logger.Error("Failed to write SSH key: %v", err)
		api.WriteError(w, -1, "Failed to add SSH key")
		return
	}

	// Set ownership of authorized_keys file to the user
	if err := chownToUser(authKeysFile, username); err != nil {
		logger.Error("Failed to set authorized_keys ownership: %v", err)
		// Continue anyway - the key might still work
	}

	logger.Info("[SSH DEBUG] SSH key added successfully for user %s", username)
	api.WriteSuccess(w, map[string]interface{}{"message": "SSH key added successfully"})
}

// DeleteSSHKeyRequest represents an SSH key deletion request.
type DeleteSSHKeyRequest struct {
	ID string `json:"id"`
}

// DeleteSSHKey removes an SSH public key.
func (h *UserHandler) DeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, -1, "Method not allowed")
		return
	}

	var req DeleteSSHKeyRequest
	if err := api.ParseJSONBody(r, &req); err != nil {
		logger.Error("[SSH DEBUG] Failed to parse DeleteSSHKey request body: %v", err)
		api.WriteError(w, -1, "Invalid request body")
		return
	}

	logger.Info("[SSH DEBUG] DeleteSSHKey called with ID=%s", req.ID)

	if req.ID == "" {
		logger.Error("[SSH DEBUG] Key ID is empty")
		api.WriteError(w, -1, "Key ID required")
		return
	}

	username := auth.GetUsernameFromRequest(r)
	logger.Info("[SSH DEBUG] Username from auth context: %s", username)

	// Use default username if not set
	if username == "" {
		username = DefaultUsername
		logger.Warn("[SSH DEBUG] Username was empty, using default: %s", username)
	}

	// Validate username to prevent path traversal
	if !isValidUsername(username) {
		logger.Error("Invalid username detected: %s", username)
		api.WriteError(w, -1, "Invalid username")
		return
	}

	authKeysFile := "/home/" + username + "/.ssh/authorized_keys"

	// Read current keys
	data, err := os.ReadFile(authKeysFile)
	if err != nil {
		api.WriteError(w, -1, "No SSH keys found")
		return
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	found := false
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// ID is the line number (1-indexed)
		if strconv.Itoa(i+1) != req.ID {
			newLines = append(newLines, line)
		} else {
			found = true
		}
	}

	if !found {
		api.WriteError(w, -1, "SSH key not found")
		return
	}

	// Write back
	if err := os.WriteFile(authKeysFile, []byte(strings.Join(newLines, "\n")+"\n"), 0600); err != nil {
		logger.Error("Failed to write authorized_keys: %v", err)
		api.WriteError(w, -1, "Failed to delete SSH key")
		return
	}

	// Ensure ownership is set correctly after writing
	if err := chownToUser(authKeysFile, username); err != nil {
		logger.Error("Failed to set authorized_keys ownership: %v", err)
	}

	logger.Info("[SSH DEBUG] SSH key deleted successfully for user %s", username)
	api.WriteSuccess(w, map[string]interface{}{"message": "SSH key deleted successfully"})
}

// Helper functions

func isSSHEnabled() bool {
	// Method 1: Check if dropbear process is running using pidof
	cmd := exec.Command("pidof", "dropbear")
	if err := cmd.Run(); err == nil {
		logger.Info("[SSH DEBUG] isSSHEnabled: dropbear process found via pidof")
		return true
	}

	// Method 2: Check the PID file
	if pidData, err := os.ReadFile("/var/run/dropbear.pid"); err == nil {
		pidStr := strings.TrimSpace(string(pidData))
		if pidStr != "" {
			// Verify the process exists
			if _, err := os.Stat("/proc/" + pidStr); err == nil {
				logger.Info("[SSH DEBUG] isSSHEnabled: dropbear PID file exists and process running")
				return true
			}
		}
	}

	// Method 3: Fall back to systemctl for systemd-based systems
	cmd = exec.Command("systemctl", "is-active", "dropbear")
	if err := cmd.Run(); err == nil {
		logger.Info("[SSH DEBUG] isSSHEnabled: dropbear active via systemctl")
		return true
	}

	logger.Info("[SSH DEBUG] isSSHEnabled: dropbear not running")
	return false
}

func getSSHKeys(username string) []map[string]interface{} {
	// Validate username to prevent path traversal
	if !isValidUsername(username) {
		logger.Error("Invalid username detected in getSSHKeys: %s", username)
		return []map[string]interface{}{}
	}

	authKeysFile := "/home/" + username + "/.ssh/authorized_keys"
	data, err := os.ReadFile(authKeysFile)
	if err != nil {
		return []map[string]interface{}{}
	}

	var keys []map[string]interface{}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		key := map[string]interface{}{
			"id":      strconv.Itoa(i + 1),
			"type":    "",
			"value":   "", // Changed from "key" to "value" to match frontend
			"name":    "",
			"addTime": "", // Optional field that frontend may use
		}
		if len(parts) >= 1 {
			key["type"] = parts[0]
		}
		if len(parts) >= 2 {
			// Truncate key for display
			fullKey := parts[1]
			if len(fullKey) > 20 {
				key["value"] = fullKey[:10] + "..." + fullKey[len(fullKey)-10:]
			} else {
				key["value"] = fullKey
			}
		}
		if len(parts) >= 3 {
			key["name"] = strings.Join(parts[2:], " ")
		}
		keys = append(keys, key)
	}
	return keys
}

func isValidSSHKey(key string) bool {
	parts := strings.Fields(key)
	if len(parts) < 2 {
		return false
	}
	validTypes := []string{"ssh-rsa", "ssh-ed25519", "ssh-dss", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521"}
	for _, t := range validTypes {
		if parts[0] == t {
			return true
		}
	}
	return false
}

func isValidUsername(username string) bool {
	if username == "" || len(username) > 32 {
		return false
	}
	// Check for path traversal patterns
	if strings.Contains(username, "..") || strings.Contains(username, "/") || strings.Contains(username, "\\") {
		return false
	}
	// Check for special characters that could be used in attacks
	if strings.ContainsAny(username, "<>:\"|?*;`$&()[]{}'\n\r\t") {
		return false
	}
	// Prevent using system directory names
	systemNames := []string{".", "..", "etc", "home", "root", "tmp", "var", "bin", "sbin", "usr", "dev", "proc", "sys"}
	for _, sysName := range systemNames {
		if username == sysName {
			return false
		}
	}
	// Only allow alphanumeric, underscore, and hyphen
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// chownToUser changes ownership of a file or directory to the specified user
func chownToUser(path, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	return os.Chown(path, uid, gid)
}
