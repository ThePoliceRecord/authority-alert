package auth

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT token claims
type Claims struct {
	CameraID       string   `json:"camera_id,omitempty"`
	UserID         string   `json:"user_id,omitempty"`
	Role           string   `json:"role"` // "camera" or "viewer"
	AllowedCameras []string `json:"allowed_cameras,omitempty"`
	jwt.RegisteredClaims
}

// Validator validates JWT tokens
type Validator struct {
	publicKey *rsa.PublicKey
}

// New Public creates a new JWT validator
func NewValidator(publicKeyPath string) (*Validator, error) {
	keyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &Validator{publicKey: publicKey}, nil
}

// Validate validates a JWT token string
func (v *Validator) Validate(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// CanAccessCamera checks if the claims allow access to a camera
func (c *Claims) CanAccessCamera(cameraID string) bool {
	// Cameras can only access themselves
	if c.Role == "camera" {
		return c.CameraID == cameraID
	}

	// Viewers must have camera in allowed list
	if c.Role == "viewer" {
		for _, allowed := range c.AllowedCameras {
			if allowed == "*" || allowed == cameraID {
				return true
			}
		}
	}

	return false
}
