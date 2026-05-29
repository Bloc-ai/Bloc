package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with bloc-hub.com via GitHub OAuth",
	Long: `Authenticate with Bloc Hub using the OAuth device flow.
Your credentials are saved locally in ~/.config/bloc/auth.json.

Examples:
  bloc login
  bloc logout`,
	RunE: runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Already logged in?
	existing, err := config.LoadAuth()
	if err == nil && existing != nil {
		fmt.Printf("Already logged in as \033[32m%s\033[0m.\n", existing.Username)
		fmt.Println("Run 'bloc logout' first to switch accounts.")
		return nil
	}

	fmt.Println("\033[1m🔐 Bloc Login\033[0m")
	fmt.Println()

	// ── Step 1: Request a device code from the Hub ─────────────────────────────
	fmt.Println("\033[90m  Connecting to bloc-hub.com...\033[0m")
	dr, err := requestDeviceCode()
	if err != nil {
		return fmt.Errorf("could not start login: %w", err)
	}

	// ── Step 2: Show instructions ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  Open this URL in your browser:")
	fmt.Printf("  \033[36m\033[1m%s\033[0m\n\n", dr.VerificationURL)
	fmt.Printf("  Enter this code: \033[1m\033[33m%s\033[0m\n", dr.UserCode)
	fmt.Printf("\n  \033[90mWaiting for authorization (expires in %d min)...\033[0m",
		dr.ExpiresIn/60)

	// ── Step 3: Poll for the token ─────────────────────────────────────────────
	result, err := pollDeviceToken(dr.DeviceCode, dr.ExpiresIn)
	if err != nil {
		fmt.Println() // end the dot line
		return err
	}
	fmt.Println() // end the dot line

	// ── Step 4: Save credentials ───────────────────────────────────────────────
	if err := config.SaveAuth(&config.AuthData{
		Token:    result.Token,
		Username: result.Username,
	}); err != nil {
		return fmt.Errorf("login succeeded but failed to save credentials: %w", err)
	}

	fmt.Printf("\n  \033[32m✓\033[0m  Logged in as \033[1m%s\033[0m\n", result.Username)
	fmt.Println("  Credentials saved to ~/.config/bloc/auth.json")
	fmt.Println()
	return nil
}

// ── Device flow types ────────────────────────────────────────────────────────

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"` // seconds
}

type tokenPollResponse struct {
	Status   string `json:"status"`             // "pending" | "expired" | "authorized"
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
}

// ── requestDeviceCode POSTs to /api/auth/device ───────────────────────────────

func requestDeviceCode() (*deviceCodeResponse, error) {
	req, err := http.NewRequest("POST", hubAPIBase+"/auth/device", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "bloc-cli/"+Version)

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error reaching bloc-hub.com: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("bloc-hub.com is temporarily unavailable — try again shortly")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, err
	}

	var dr deviceCodeResponse
	if err := json.Unmarshal(body, &dr); err != nil || dr.DeviceCode == "" {
		return nil, fmt.Errorf("unexpected response from server")
	}
	return &dr, nil
}

// ── pollDeviceToken polls /api/auth/device/token every 5 seconds ─────────────

func pollDeviceToken(deviceCode string, expiresIn int) (*tokenPollResponse, error) {
	payload, _ := json.Marshal(map[string]string{"device_code": deviceCode})
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	consecutiveErrors := 0

	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Second)
		fmt.Print(".")

		req, err := http.NewRequest("POST",
			hubAPIBase+"/auth/device/token",
			bytes.NewReader(payload),
		)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= 5 {
				return nil, fmt.Errorf("too many network errors — check your connection")
			}
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "bloc-cli/"+Version)

		resp, err := apiClient.Do(req)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= 5 {
				return nil, fmt.Errorf("too many network errors — check your connection")
			}
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		consecutiveErrors = 0 // reset on any successful HTTP response

		var result tokenPollResponse
		if json.Unmarshal(body, &result) != nil {
			continue
		}

		switch strings.ToLower(result.Status) {
		case "authorized":
			if result.Token == "" || result.Username == "" {
				return nil, fmt.Errorf("invalid response from server — try 'bloc login' again")
			}
			return &result, nil
		case "expired":
			return nil, fmt.Errorf("code expired — run 'bloc login' again")
		case "pending":
			// still waiting — keep polling
		}
	}

	return nil, fmt.Errorf("authorization timed out — run 'bloc login' again")
}

// ── Logout ───────────────────────────────────────────────────────────────────

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved authentication credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.DeleteAuth(); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}
		fmt.Println("✓ Logged out. Credentials cleared from ~/.config/bloc/auth.json")
		return nil
	},
}
