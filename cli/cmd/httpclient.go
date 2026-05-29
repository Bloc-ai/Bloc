package cmd

import (
	"net/http"
	"time"
)

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
	},
}
