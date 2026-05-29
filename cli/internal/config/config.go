package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// homeDir is memoized once at first use (P-16: avoid repeated syscalls).
var (
	cachedHome    string
	cachedHomeErr error
	homeDirOnce   sync.Once
)

func getHomeDir() (string, error) {
	homeDirOnce.Do(func() {
		cachedHome, cachedHomeErr = os.UserHomeDir()
	})
	return cachedHome, cachedHomeErr
}

// ConfigDir returns the platform config directory for bloc.
// macOS/Linux: ~/.config/bloc
func ConfigDir() (string, error) {
	home, err := getHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "bloc"), nil
}

// CacheDir returns the platform cache directory for bloc.
// macOS/Linux: ~/.cache/bloc
func CacheDir() (string, error) {
	home, err := getHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "bloc"), nil
}

// AuthData holds the saved credentials.
type AuthData struct {
	Token       string `json:"token"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// LoadAuth reads the saved auth token from ~/.config/bloc/auth.json.
// Returns nil, nil if not logged in.
// F-15: Verifies file permissions are 0600 to detect external tampering.
func LoadAuth() (*AuthData, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "auth.json")

	// F-15: Check permissions before reading — warn if too permissive.
	if stat, statErr := os.Stat(path); statErr == nil {
		if perm := stat.Mode().Perm(); perm > 0o600 {
			return nil, fmt.Errorf(
				"auth file %s has insecure permissions %04o (expected 0600) — run: chmod 600 %s",
				path, perm, path,
			)
		}
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read auth file: %w", err)
	}

	var auth AuthData
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("malformed auth file: %w", err)
	}
	return &auth, nil
}

// SaveAuth writes credentials to ~/.config/bloc/auth.json.
func SaveAuth(auth *AuthData) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, "auth.json")
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// DeleteAuth removes the auth file (logout).
func DeleteAuth() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "auth.json")
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// TelemetrySettings holds telemetry consent.
// F-12: session_id removed — it was a persistent pseudonymous device identifier.
// Per-invocation IDs are generated in-memory by the telemetry package when needed.
type TelemetrySettings struct {
	Enabled      bool `json:"enabled"`
	ConsentGiven bool `json:"consent_given"` // true once user answered the prompt
}

// LoadTelemetry reads ~/.config/bloc/telemetry.json.
func LoadTelemetry() (*TelemetrySettings, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "telemetry.json")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &TelemetrySettings{Enabled: false, ConsentGiven: false}, nil
	}
	if err != nil {
		return nil, err
	}

	var t TelemetrySettings
	if err := json.Unmarshal(data, &t); err != nil {
		return &TelemetrySettings{Enabled: false, ConsentGiven: false}, nil
	}
	return &t, nil
}

// SaveTelemetry writes telemetry settings.
func SaveTelemetry(t *TelemetrySettings) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "telemetry.json")
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
