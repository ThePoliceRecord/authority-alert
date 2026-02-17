// Package timestamp provides RFC 3161 trusted timestamp services.
//
// This package implements a client for obtaining cryptographically-verifiable
// timestamps from a Time Stamping Authority (TSA) per RFC 3161. These timestamps
// provide third-party attestation of when data existed, which is essential for
// legal evidence integrity.
//
// The primary use case is timestamping detection events from security cameras:
// - The camera captures a frame and computes its SHA256 hash
// - The supervisor combines frame_hash + capture_timestamp + detection_json
// - A SHA-512 hash of this combined data is sent to FreeTSA
// - FreeTSA returns a signed timestamp token proving when the data existed
//
// What the token proves:
//   - Frame integrity: frame_hash binds to actual image data
//   - Capture time: embedded in the hash, can't be changed without invalidating
//   - Detection integrity: detection JSON is bound to the hash
//   - Third-party attestation: FreeTSA's signature proves when this data existed
package timestamp

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"supervisor/pkg/logger"
)

// OID for SHA-512 hash algorithm
var oidSHA512 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}

// TimeStampReq is an RFC 3161 Timestamp Request
type TimeStampReq struct {
	Version        int
	MessageImprint MessageImprint
	ReqPolicy      asn1.ObjectIdentifier `asn1:"optional"`
	Nonce          asn1.RawValue         `asn1:"optional"`
	CertReq        bool                  `asn1:"optional,default:false"`
	Extensions     []Extension           `asn1:"optional,tag:0"`
}

// MessageImprint contains the hash algorithm and hash value
type MessageImprint struct {
	HashAlgorithm AlgorithmIdentifier
	HashedMessage []byte
}

// AlgorithmIdentifier identifies a cryptographic algorithm
type AlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

// Extension is an X.509 extension
type Extension struct {
	ExtnID    asn1.ObjectIdentifier
	Critical  bool `asn1:"optional,default:false"`
	ExtnValue []byte
}

// TimeStampResp is an RFC 3161 Timestamp Response
type TimeStampResp struct {
	Status         PKIStatusInfo
	TimeStampToken asn1.RawValue `asn1:"optional"`
}

// PKIStatusInfo contains the response status
type PKIStatusInfo struct {
	Status       int
	StatusString []string       `asn1:"optional,utf8"`
	FailInfo     asn1.BitString `asn1:"optional"`
}

// Client is an RFC 3161 TSA client
type Client struct {
	httpClient *http.Client
	tsaURL     string
	caCert     *x509.CertPool
}

// TSATimeout is the maximum time to wait for a TSA response.
// Keep this short to avoid blocking detection uploads.
const TSATimeout = 10 * time.Second

// NewClient creates a new TSA client for FreeTSA.org
func NewClient() (*Client, error) {
	// Parse the FreeTSA root CA certificate
	caCert := x509.NewCertPool()
	if !caCert.AppendCertsFromPEM([]byte(FreeTSARootCACert)) {
		return nil, fmt.Errorf("failed to parse FreeTSA CA certificate")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: TSATimeout,
		},
		tsaURL: FreeTSAURL,
		caCert: caCert,
	}, nil
}

// HashDetection creates a combined hash of frame evidence for timestamping.
// This binds together:
//   - frameHash: SHA256 of the raw frame data (from camera-streamer)
//   - captureTime: when the frame was captured (NTP-synced)
//   - detections: JSON array of detection results
//
// The resulting SHA-512 hash is what gets signed by the TSA.
func HashDetection(frameHash string, captureTime time.Time, detections string) ([]byte, error) {
	h := sha512.New()

	// Write frame hash (hex string, normalized to lowercase)
	frameHashBytes, err := hex.DecodeString(frameHash)
	if err != nil {
		return nil, fmt.Errorf("invalid frame hash: %w", err)
	}
	h.Write(frameHashBytes)

	// Write capture timestamp as milliseconds since epoch (big-endian)
	if err := binary.Write(h, binary.BigEndian, captureTime.UnixMilli()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	// Write detections JSON
	h.Write([]byte(detections))

	return h.Sum(nil), nil
}

// RequestTimestamp sends a timestamp request to the TSA and returns the response.
// The hash should be a SHA-512 hash of the data to be timestamped.
// Returns the DER-encoded timestamp token.
func (c *Client) RequestTimestamp(hash []byte) ([]byte, error) {
	if len(hash) != sha512.Size {
		return nil, fmt.Errorf("hash must be SHA-512 (%d bytes), got %d bytes", sha512.Size, len(hash))
	}

	// Generate a random nonce for replay protection
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Build the timestamp request
	req := TimeStampReq{
		Version: 1,
		MessageImprint: MessageImprint{
			HashAlgorithm: AlgorithmIdentifier{
				Algorithm:  oidSHA512,
				Parameters: asn1.RawValue{Tag: asn1.TagNull},
			},
			HashedMessage: hash,
		},
		Nonce: asn1.RawValue{
			Class:      asn1.ClassUniversal,
			Tag:        asn1.TagInteger,
			IsCompound: false,
			Bytes:      nonce,
		},
		CertReq: true, // Request signing certificate to be included
	}

	// Encode the request as DER
	reqBytes, err := asn1.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode timestamp request: %w", err)
	}

	// Send HTTP POST to TSA
	httpReq, err := http.NewRequest("POST", c.tsaURL, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/timestamp-query")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("TSA request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TSA returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Read and parse the response
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read TSA response: %w", err)
	}

	var tsResp TimeStampResp
	rest, err := asn1.Unmarshal(respBytes, &tsResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TSA response: %w", err)
	}
	if len(rest) > 0 {
		logger.Warning("TSA response had %d trailing bytes", len(rest))
	}

	// Check response status
	// Status values: 0 = granted, 1 = grantedWithMods, 2 = rejection, etc.
	if tsResp.Status.Status > 1 {
		statusMsg := "unknown error"
		if len(tsResp.Status.StatusString) > 0 {
			statusMsg = tsResp.Status.StatusString[0]
		}
		return nil, fmt.Errorf("TSA rejected request: status=%d, message=%s",
			tsResp.Status.Status, statusMsg)
	}

	// Return the timestamp token
	if len(tsResp.TimeStampToken.Bytes) == 0 {
		return nil, fmt.Errorf("TSA response contains no timestamp token")
	}

	return tsResp.TimeStampToken.FullBytes, nil
}

// TokenToBase64 encodes a DER timestamp token as base64 for storage/transmission
func TokenToBase64(token []byte) string {
	return base64.StdEncoding.EncodeToString(token)
}

// TokenFromBase64 decodes a base64 timestamp token
func TokenFromBase64(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// defaultEnabled is the compile-time default for TSA timestamping.
const defaultEnabled = false

// enabled controls whether TSA timestamping is active at runtime.
// Can be changed via SetEnabled().
var enabled = defaultEnabled

// Enabled returns whether TSA timestamping is currently active.
func Enabled() bool {
	return enabled
}

// SetEnabled enables or disables TSA timestamping at runtime.
func SetEnabled(v bool) {
	enabled = v
}
