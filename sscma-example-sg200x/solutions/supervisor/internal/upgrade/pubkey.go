// Package upgrade provides system upgrade management functionality.
package upgrade

// FirmwareSigningPubKey is the ECDSA P-384 public key used to verify firmware signatures.
// This key is embedded at compile time and should correspond to the private key
// used by the build server to sign firmware packages.
//
// To generate a new key pair:
//   openssl ecparam -name secp384r1 -genkey -noout -out firmware-signing.key
//   openssl ec -in firmware-signing.key -pubout -out firmware-signing.pub
//
// IMPORTANT: The private key (firmware-signing.key) must NEVER be committed to the repository.
// Only the public key is embedded here for verification.
//
// This is a placeholder key. Replace with the actual public key before production deployment.
const FirmwareSigningPubKey = `-----BEGIN PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEPlaceholder+Replace+With+Real+Key
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
-----END PUBLIC KEY-----`

// defaultFirmwareSigningEnabled is the compile-time default.
const defaultFirmwareSigningEnabled = false

// firmwareSigningEnabled controls whether firmware signature verification is enforced at runtime.
var firmwareSigningEnabled = defaultFirmwareSigningEnabled

// FirmwareSigningEnabled returns whether firmware signature verification is active.
func FirmwareSigningEnabled() bool {
	return firmwareSigningEnabled
}

// SetFirmwareSigningEnabled enables or disables firmware signature verification at runtime.
func SetFirmwareSigningEnabled(v bool) {
	firmwareSigningEnabled = v
}
