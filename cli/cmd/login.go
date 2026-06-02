package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)


var loginHF bool // --hf flag: authenticate with HuggingFace instead of Bloc Hub

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Bloc Hub (or HuggingFace with --hf)",
	Long: `Authenticate with Bloc Hub using the OAuth device flow.
Your credentials are saved locally in ~/.config/bloc/auth.json.

To authenticate with HuggingFace (required for gated models like Llama, Gemma):
  bloc login --hf

Examples:
  bloc login
  bloc login --hf
  bloc logout`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().BoolVar(&loginHF, "hf", false, "Authenticate with HuggingFace (for gated models)")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Route to HF token flow when --hf flag is set
	if loginHF {
		return runLoginHF()
	}

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
	fmt.Println("\033[90m  Connecting to bloc-theta.vercel.app...\033[0m")
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

	// Create a cancellable context that listens to Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	// ── Step 3: Poll for the token ─────────────────────────────────────────────
	result, err := pollDeviceToken(ctx, dr.DeviceCode, dr.ExpiresIn)
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
		return nil, fmt.Errorf("network error reaching bloc-theta.vercel.app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("bloc-theta.vercel.app is temporarily unavailable — try again shortly")
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

func pollDeviceToken(ctx context.Context, deviceCode string, expiresIn int) (*tokenPollResponse, error) {
	payload, _ := json.Marshal(map[string]string{"device_code": deviceCode})
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	consecutiveErrors := 0
	consecutivePendings := 0

	// PERF-10: Single reader hoisted outside the loop and reset via Seek on each iteration.
	reader := bytes.NewReader(payload)

	for time.Now().Before(deadline) {
		// PERF-NEW-H: Replace time.Sleep with a cancellable select block.
		// LOW-SEC-13: Exponential backoff with jitter on consecutive pending responses.
		backoffFactor := math.Pow(1.2, float64(consecutivePendings))
		backoffDuration := time.Duration(float64(5*time.Second) * backoffFactor)
		if backoffDuration > 30*time.Second {
			backoffDuration = 30 * time.Second
		}
		// Add small random jitter up to 1 second
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		sleepTime := backoffDuration + jitter

		timer := time.NewTimer(sleepTime)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("login cancelled")
		}

		fmt.Print(".")

		reader.Seek(0, io.SeekStart)
		req, err := http.NewRequestWithContext(ctx, "POST",
			hubAPIBase+"/auth/device/token",
			reader,
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
			consecutivePendings++
			// still waiting — keep polling
		}
	}

	return nil, fmt.Errorf("authorization timed out — run 'bloc login' again")
}

// ── HuggingFace Login ─────────────────────────────────────────────────────────

// runLoginHF implements the `bloc login --hf` flow.
// Prompts for a HF personal access token, verifies it against
// https://huggingface.co/api/whoami, and saves it to ~/.config/bloc/hf_auth.json.
// F-21: Token is never printed, logged, or included in error messages.
func runLoginHF() error {
	fmt.Println("\033[1m🤗 HuggingFace Login\033[0m")
	fmt.Println()
	fmt.Println("  Generate a token at: \033[36mhttps://huggingface.co/settings/tokens\033[0m")
	fmt.Println("  Required scope: \033[33mRead\033[0m (for public + gated models)")
	fmt.Println()
	fmt.Print("  Enter your HuggingFace token: ")

	// SEC-01: Read token securely using x/term when os.Stdin is a TTY.
	var token string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to read token securely: %w", err)
		}
		token = strings.TrimSpace(string(tokenBytes))
		fmt.Println() // print newline since ReadPassword doesn't echo Enter
	} else {
		// Non-terminal fallback (e.g. pipes, CI)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("no token entered")
		}
		token = strings.TrimSpace(scanner.Text())
	}

	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Verify the token against HF whoami (non-fatal if HF is down)
	fmt.Println("\n  Verifying token...")
	username := verifyHFToken(token)
	if username == "" {
		// Non-fatal: save anyway — HF may be temporarily unreachable
		fmt.Println("  \033[33m⚠  Could not verify token (HF may be down). Saving anyway.\033[0m")
	} else {
		fmt.Printf("  \033[32m✓\033[0m  Authenticated as \033[1m%s\033[0m\n", username)
	}

	if err := config.SaveHFAuth(&config.HFCredentials{
		Token:    token,
		Username: username,
	}); err != nil {
		return fmt.Errorf("failed to save HF credentials: %w", err)
	}

	fmt.Println("  Credentials saved to ~/.config/bloc/hf_auth.json")
	fmt.Println()
	fmt.Println("  You can now run gated models:")
	fmt.Println("    bloc run your-username/llama3-8b-gated")
	fmt.Println()
	return nil
}

// verifyHFToken calls https://huggingface.co/api/whoami with the given token
// and returns the username on success, or "" if the call fails.
// F-21: Token is only in the Authorization header — never logged.
func verifyHFToken(token string) string {
	req, err := http.NewRequest("GET", "https://huggingface.co/api/whoami", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "bloc-cli/"+Version)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return ""
	}

	var who struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &who); err != nil {
		return ""
	}
	return who.Name
}

// ── Logout ───────────────────────────────────────────────────────────────────

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved authentication credentials",
	Long: `Clear saved Bloc Hub credentials. Use --hf to clear HuggingFace credentials.

Examples:
  bloc logout        # clears Bloc Hub auth
  bloc logout --hf   # clears HuggingFace token`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hf, _ := cmd.Flags().GetBool("hf")
		if hf {
			if err := config.DeleteHFAuth(); err != nil {
				return fmt.Errorf("HF logout failed: %w", err)
			}
			fmt.Println("✓ HuggingFace token cleared from ~/.config/bloc/hf_auth.json")
			return nil
		}
		if err := config.DeleteAuth(); err != nil {
			return fmt.Errorf("logout failed: %w", err)
		}
		fmt.Println("✓ Logged out. Credentials cleared from ~/.config/bloc/auth.json")
		return nil
	},
}

func init() {
	logoutCmd.Flags().Bool("hf", false, "Clear HuggingFace credentials instead of Bloc Hub credentials")
}
