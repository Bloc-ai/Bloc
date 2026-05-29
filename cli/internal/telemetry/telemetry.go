package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/runner"
)

// CLIVersion is set by the cmd package at startup to avoid circular imports.
var CLIVersion = "dev"

// P-02: Dedicated package-level telemetry client — not http.DefaultClient.
var telemetryClient = &http.Client{
	Timeout: 3 * time.Second,
}

// getEndpoint returns the telemetry API URL.
// F-02: BLOC_API_URL is validated to be https:// by the cmd package before this is called.
var endpoint = getEndpoint()

func getEndpoint() string {
	if url := os.Getenv("BLOC_API_URL"); url != "" {
		return url + "/telemetry"
	}
	return "https://bloc-theta.vercel.app/api/telemetry"
}

// Payload is what we send to the Hub telemetry endpoint.
// No PII, no file paths, no model content.
// F-12: session_id removed entirely — was a persistent pseudonymous device identifier.
// A random per-invocation nonce is used instead for deduplication if ever needed.
type Payload struct {
	Event                  string  `json:"event"`
	CLIVersion             string  `json:"cli_version"`
	OS                     string  `json:"os"`
	Arch                   string  `json:"arch"`
	RecipeID               string  `json:"recipe_id"`
	Success                bool    `json:"success"`
	TokensPerSecGeneration float64 `json:"tokens_per_sec_generation,omitempty"`
	TokensPerSecPrefill    float64 `json:"tokens_per_sec_prefill,omitempty"`
	PeakVRAMMB             int64   `json:"peak_vram_mb,omitempty"`
	DurationSeconds        float64 `json:"duration_seconds,omitempty"`
}

// MaybePromptConsent shows the first-run telemetry prompt if the user hasn't been asked yet.
func MaybePromptConsent() error {
	t, err := config.LoadTelemetry()
	if err != nil {
		return nil // non-fatal
	}
	if t.ConsentGiven {
		return nil // already decided
	}

	// Check BLOC_NO_TELEMETRY env override
	noTel := false
	for _, env := range []string{"BLOC_NO_TELEMETRY", "DO_NOT_TRACK"} {
		if val, ok := os.LookupEnv(env); ok && val == "1" {
			noTel = true
			break
		}
	}
	if noTel {
		t.Enabled = false
		t.ConsentGiven = true
		return config.SaveTelemetry(t)
	}

	fmt.Println()
	fmt.Println("🔬 Help improve Bloc by sharing anonymous usage data.")
	fmt.Println("   We collect: CLI version, OS, recipe success/failure, tokens/sec.")
	fmt.Println("   We never collect: file paths, model content, or personal data.")
	fmt.Println("   You can opt out anytime with: bloc telemetry off")
	fmt.Println()
	fmt.Print("   Enable telemetry? [y/N]: ")

	var answer string
	fmt.Scanln(&answer)

	t.ConsentGiven = true
	t.Enabled = (answer == "y" || answer == "Y" || answer == "yes")
	return config.SaveTelemetry(t)
}

// Send fires a telemetry event with a 3-second timeout (fire-and-forget).
// Safe to call even if telemetry is disabled — it will silently no-op.
func Send(recipeID string, stats *runner.Stats) {
	t, err := config.LoadTelemetry()
	if err != nil || !t.Enabled {
		return
	}

	payload := Payload{
		Event:                  "recipe_run_complete",
		CLIVersion:             CLIVersion,
		OS:                     runtime.GOOS,
		Arch:                   runtime.GOARCH,
		RecipeID:               recipeID,
		Success:                stats.Success,
		TokensPerSecGeneration: stats.TokensPerSecGeneration,
		TokensPerSecPrefill:    stats.TokensPerSecPrefill,
		PeakVRAMMB:             stats.PeakVRAMMB,
		DurationSeconds:        stats.Duration.Seconds(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "bloc-cli/"+CLIVersion)

	// P-02: Use dedicated telemetryClient, not http.DefaultClient
	resp, err := telemetryClient.Do(req)
	if err != nil {
		return // silently ignore — network errors must not affect UX
	}
	resp.Body.Close()
}

// newInvocationID generates a random per-invocation identifier for deduplication.
// Not stored to disk — not a persistent tracker.
func newInvocationID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck // crypto/rand.Read never errors on supported platforms
	return hex.EncodeToString(b)
}
