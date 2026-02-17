// Package tls provides TLS certificate management for the supervisor.
// This file contains the embedded TPR platform CA certificate.

package tls

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"
)

// TPR platform CA certificate (tpr-aa-ca.ca.thepolicerecord.com)
// Valid until 2036-01-24
const platformCACert = `-----BEGIN CERTIFICATE-----
MIIDdjCCAl6gAwIBAgIUJo4rnPf/BzH4p1kgGMEWSNOsB9UwDQYJKoZIhvcNAQEL
BQAwKzEpMCcGA1UEAxMgdHByLWFhLWNhLmNhLnRoZXBvbGljZXJlY29yZC5jb20w
HhcNMjYwMTI2MDU0OTI1WhcNMzYwMTI0MDU0OTU1WjArMSkwJwYDVQQDEyB0cHIt
YWEtY2EuY2EudGhlcG9saWNlcmVjb3JkLmNvbTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAOIluswvTFIRn5/wCtUClMQbCAgQybwbIlgvdCG8OY/2SKIX
A3ULEpwRJCZXsj77zThwSRYyZVy/hqYKIg0J0csH6CmZGSQQ6fjHllDQT52Bw3vy
Zpg1SdU8HuIH9o8wSxNeJAyS8kysezniEcnOGotWaedzDMry9Z5JRYiyAOdWahdQ
psXFjkzDGiO9TqbfTs6p1niCLpnoNW03HUY4jUNFDOl1aGJXwjrGq4APjY+g6oQz
wKV+8ef1RNG2vpO83KAfbP0HykLAZaf7hFJmWjMDbgf5IBMK/GDL9Kmkvj0N1Jne
Krnn3/Af95Fh5wC4yaN7nMBLHJlStKYQgl3f09sCAwEAAaOBkTCBjjAOBgNVHQ8B
Af8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUtkReNzrdPQjtsItw
fPRJKoiWmBwwHwYDVR0jBBgwFoAUtkReNzrdPQjtsItwfPRJKoiWmBwwKwYDVR0R
BCQwIoIgdHByLWFhLWNhLmNhLnRoZXBvbGljZXJlY29yZC5jb20wDQYJKoZIhvcN
AQELBQADggEBAKJHLf3HOPTeHa12u78Tatt3j3zUfLveP/GfL+fY4dcf7MHnUysN
cL0FggDsgCu1fB4Xc2O00Qx7GFrSUyl6JnfSRe6weEq/8tEjroQJMnntfAx4rLQ9
aSdJoTWEMts7XmdhbO9o4Va1nVqhkh3bhGyYGYV9qXQwAwOMB7XD4PVW0rOlPw0n
5gNG/SXYQc2ewRZPwBg5GK9wRTvUcBtG7sp3/hwlAWdS4iW+zmomjDFmbrUNkg7n
R6wq7e4KidblM8Pwcm3z6WxFcseJPOKUOk7w60yrKC/hXHJS065c110/ifw/5TDx
9jneKQoZmfkfq5x4Jrc/6J/37gt7kWhrerk=
-----END CERTIFICATE-----`

// platformCertPool contains the system certs plus our custom CA cert.
var platformCertPool *x509.CertPool

func init() {
	// Start with system cert pool if available
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	// Add the TPR platform CA cert
	pool.AppendCertsFromPEM([]byte(platformCACert))
	platformCertPool = pool
}

// PlatformCertPool returns the certificate pool containing both system certs
// and the TPR platform CA certificate.
func PlatformCertPool() *x509.CertPool {
	return platformCertPool
}

// PlatformHTTPClient returns an HTTP client configured with the TPR CA cert
// for verifying connections to platform services.
func PlatformHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: platformCertPool,
			},
		},
	}
}

// PlatformTLSConfig returns a TLS config for use with WebSocket or other
// clients that need to verify platform TLS connections.
func PlatformTLSConfig() *tls.Config {
	return &tls.Config{
		RootCAs: platformCertPool,
	}
}
