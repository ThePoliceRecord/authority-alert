// Package upgrade provides system upgrade management functionality.
package upgrade

import (
	"crypto/ecdsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"supervisor/pkg/logger"
)

// SignatureFileSuffix is the extension used for signature files
const SignatureFileSuffix = ".sig"

// VerifyFirmwareSignature verifies an ECDSA P-384 signature over firmware data.
// The signature must be in ASN.1 DER format as produced by OpenSSL.
//
// To create a signature:
//
//	openssl dgst -sha384 -sign firmware-signing.key -out package.zip.sig package.zip
//
// Returns nil if the signature is valid, or an error describing the verification failure.
func VerifyFirmwareSignature(firmwareData, signature []byte) error {
	// Parse the PEM-encoded public key
	block, _ := pem.Decode([]byte(FirmwareSigningPubKey))
	if block == nil {
		return fmt.Errorf("failed to decode PEM public key")
	}

	// Parse the public key
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not ECDSA")
	}

	// Compute SHA-384 hash of the firmware data
	hash := sha512.Sum384(firmwareData)

	// Verify the ECDSA signature (ASN.1 DER format)
	if !ecdsa.VerifyASN1(ecdsaPub, hash[:], signature) {
		return fmt.Errorf("firmware signature verification failed - signature does not match")
	}

	return nil
}

// VerifyFirmwareFile verifies a firmware file using its corresponding signature file.
// The signature file is expected at firmwarePath + ".sig"
func VerifyFirmwareFile(firmwarePath string) error {
	if !FirmwareSigningEnabled() {
		logger.Warning("Firmware signature verification is DISABLED - accepting unsigned firmware")
		return nil
	}

	sigPath := firmwarePath + SignatureFileSuffix

	// Read firmware data
	firmwareData, err := os.ReadFile(firmwarePath)
	if err != nil {
		return fmt.Errorf("failed to read firmware file: %w", err)
	}

	// Read signature
	signature, err := os.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("signature file not found at %s - refusing unsigned firmware: %w", sigPath, err)
	}

	// Verify
	if err := VerifyFirmwareSignature(firmwareData, signature); err != nil {
		return fmt.Errorf("SECURITY: %w - update rejected", err)
	}

	logger.Info("Firmware signature verified successfully: %s", firmwarePath)
	return nil
}

// DownloadSignatureFile downloads the signature file for a firmware package.
// Returns the signature data, or an error if the signature cannot be downloaded.
func (m *UpgradeManager) DownloadSignatureFile(packageURL string) ([]byte, error) {
	sigURL := packageURL + SignatureFileSuffix

	logger.Info("Downloading firmware signature from %s", sigURL)

	client := newHTTPClient(60*time.Second, false, false)
	resp, err := client.Get(sigURL)
	if err != nil {
		// Try fallback for GitHub hosts if TLS fails
		if isTrustedFallbackHost(sigURL) && isTLSUnknownAuthority(err) {
			logger.Warning("TLS verification failed for signature URL; retrying with InsecureSkipVerify")
			insecureClient := newHTTPClient(60*time.Second, false, true)
			resp, err = insecureClient.Get(sigURL)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to download signature: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("signature file not found at %s - unsigned firmware not allowed", sigURL)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download signature: HTTP %d", resp.StatusCode)
	}

	sigData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature response: %w", err)
	}

	if len(sigData) == 0 {
		return nil, fmt.Errorf("signature file is empty")
	}

	// Basic sanity check for ASN.1 DER signature (should start with 0x30 for SEQUENCE)
	if len(sigData) < 2 || sigData[0] != 0x30 {
		// Could be a text error message from a misconfigured server
		if len(sigData) < 100 && !strings.Contains(string(sigData), "\x00") {
			return nil, fmt.Errorf("signature file appears invalid: %s", string(sigData))
		}
		return nil, fmt.Errorf("signature file appears invalid (not ASN.1 DER format)")
	}

	logger.Info("Downloaded signature file (%d bytes)", len(sigData))
	return sigData, nil
}

// VerifyDownloadedFirmware verifies a downloaded firmware package before flashing.
// This downloads the signature file and verifies it against the firmware data.
func (m *UpgradeManager) VerifyDownloadedFirmware(packageURL, localPath string) error {
	if !FirmwareSigningEnabled() {
		logger.Warning("Firmware signature verification is DISABLED - accepting unsigned firmware")
		return nil
	}

	// Download signature file
	sigData, err := m.DownloadSignatureFile(packageURL)
	if err != nil {
		return err
	}

	// Read local firmware file
	firmwareData, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read firmware file: %w", err)
	}

	// Verify signature
	if err := VerifyFirmwareSignature(firmwareData, sigData); err != nil {
		return fmt.Errorf("SECURITY: %w", err)
	}

	logger.Info("Firmware signature verified: %s", localPath)
	return nil
}
