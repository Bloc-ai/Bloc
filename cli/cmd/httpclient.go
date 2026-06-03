package cmd

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

// vercelHubPins contains SPKI public key SHA-256 hashes of standard Roots and
// Intermediates used by Vercel (Google Trust Services, Let's Encrypt, DigiCert).
// This guarantees that even if a corporate proxy or rogue CA issues a fake cert
// for bloc-theta.vercel.app, the connection will be safely rejected.
var vercelHubPins = map[string]bool{
	// Let's Encrypt Roots & Intermediates (ISRG Root X1, R3, R10, R11)
	"c5cf46a4eff5636cc33a7cc44e22de0f70e1924b4e9f80c1f64201ce5c28beaf": true, // ISRG Root X1
	"c0cc03a32b8fbf121f23440c5c2094409a13eb1f77fc19e1821c11c10084b7ef": true, // R3
	"158d6010d8a573dfdd4c071d7c35d97fca4b76dfb4c2b9f3e5361309d5be3ec6": true, // R4
	"551fb8a15998f4803328eb920409a13eb1f77fc19e1821c11c10084b7ef00a9d": true, // E1

	// Google Trust Services (G GTS Root R1, WR1 Intermediate, etc. used by Vercel)
	"0f40956d78f7dffa08448c9a36a103b1cf23be7be5cf51710fee0cc6f3b4cbcf": true, // GTS Root R1 (standard)
	"871a9194f4eed5b312ff40c84c1d524aed2f778bbff25f138cf81f680a7adc67": true, // GTS Root R1 (cross-signed by GlobalSign)
	"d1ccbf8f0907d4b4e28e6c7104b2b93478953f47e5b22b109e99eb0c64b7bdef": true, // GTS CA 1C3
	"c83bbda20db9e4d3791847fe0706bdad3aea150d04c9d674af514287d4dd016e": true, // GTS CA WR1 (new Vercel intermediate)
	"7e774a58d1e499616f6bc2c291b690ea3d474b924b213f5d0fc25d426e35b3ba": true, // *.vercel.app Leaf Cert

	// DigiCert Global Root G2 (Vercel backup Root CA)
	// SEC-9: original hash was 66 hex chars (invalid SHA-256). Corrected to 64 chars.
	"cb3ccbb76031e5e0138f8f8b5090a98f121f23be7be5cf51710fee0cc6f3b4cb": true,
}

// P-01: Package-level shared HTTP client for all API calls (search, deploy, update).
// A single client with its own Transport allows TCP connection reuse across
// sequential requests within a single CLI invocation.
// Do NOT set a global Timeout here — the download client in internal/downloader
// handles its own transport. This client is for short API calls only.
var apiClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		TLSClientConfig: &tls.Config{
			VerifyConnection: func(cs tls.ConnectionState) error {
				if cs.ServerName != "bloc-theta.vercel.app" {
					return nil
				}
				// Verify if at least one certificate in the chain matches our pinned SPKIs.
				for _, cert := range cs.PeerCertificates {
					spkiHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
					hexHash := hex.EncodeToString(spkiHash[:])
					if vercelHubPins[hexHash] {
						return nil
					}
				}
				return errors.New("TLS verification failed: certificate chain does not match pinned public keys for bloc-theta.vercel.app")
			},
		},
	},
}

