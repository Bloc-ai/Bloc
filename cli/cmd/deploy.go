package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/downloader"
	"github.com/bloc-org/bloc/internal/hardware"
	"github.com/bloc-org/bloc/internal/probe"
	"github.com/bloc-org/bloc/internal/recipe"
	"github.com/bloc-org/bloc/internal/runner"
	"github.com/bloc-org/bloc/internal/telemetry"
	"github.com/spf13/cobra"
)

// F-02: hubAPIBase is resolved once at startup.
// BLOC_API_URL is validated to be a safe https:// URL — plain http:// is rejected.
var hubAPIBase = getHubAPIBase()

func getHubAPIBase() string {
	if rawURL := os.Getenv("BLOC_API_URL"); rawURL != "" {
		if err := validateAPIURL(rawURL); err != nil {
			fmt.Fprintf(os.Stderr, "\033[33m⚠  BLOC_API_URL ignored: %v\033[0m\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "\033[33m⚠  Using BLOC_API_URL override: %s\033[0m\n", rawURL)
			return rawURL
		}
	}
	return "https://bloc-theta.vercel.app/api"
}

// validateAPIURL ensures the override URL is safe to use.
// F-02: Prevents SSRF via plain-HTTP downgrade or non-HTTPS schemes.
// Exception: http://localhost and http://127.0.0.1 are allowed for local dev.
func validateAPIURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "http" {
		host := u.Hostname()
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return fmt.Errorf("http:// URLs are not allowed (use https://); got: %s", rawURL)
		}
	} else if u.Scheme != "https" {
		return fmt.Errorf("only https:// URLs are allowed; got scheme %q", u.Scheme)
	}
	return nil
}

// recipeIDRe validates author and recipe name segments.
// F-09: Prevents path traversal via crafted recipe IDs like "../../etc/passwd/foo".
var recipeIDRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,99}$`)

var deployDryRun bool
var deployNoTelemetry bool

var deployCmd = &cobra.Command{
	Use:   "deploy [author/recipe]",
	Short: "Fetch and run a recipe from the Bloc registry",
	Long: `Fetch a recipe from bloc-theta.vercel.app, probe your hardware and llama-server
capabilities, download the model weights if needed, and launch the server.

Examples:
  bloc deploy arnav080/qwen3-30b-moe-8gb-cpu-offload
  bloc deploy arnav080/qwen3-30b-moe-8gb-cpu-offload --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show the llama-server command without running it")
	deployCmd.Flags().BoolVar(&deployNoTelemetry, "no-telemetry", false, "Disable telemetry for this run")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	recipeID := args[0]
	parts := strings.SplitN(recipeID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid recipe ID %q — expected format: author/recipe-name", recipeID)
	}

	// F-09: Validate both path segments against the allowlist regex before use.
	author, name := parts[0], parts[1]
	if !recipeIDRe.MatchString(author) {
		return fmt.Errorf("invalid author name %q — only alphanumeric, dash, dot and underscore allowed", author)
	}
	if !recipeIDRe.MatchString(name) {
		return fmt.Errorf("invalid recipe name %q — only alphanumeric, dash, dot and underscore allowed", name)
	}

	printStep("Fetching recipe")
	r, err := fetchRecipe(author, name)
	if err != nil {
		return fmt.Errorf("cannot fetch recipe: %w", err)
	}
	fmt.Printf("  \033[32m✓\033[0m  %s — %s\n", r.Metadata.Name, shortDesc(r.Metadata.Description, 72))

	// ── Step 2: Hardware Probe ─────────────────────────────────────────────────
	printStep("Probing hardware")
	hw, err := hardware.Probe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠  Could not probe hardware: %v\n", err)
	} else {
		fmt.Printf("  \033[32m✓\033[0m  %s\n", hw.Summary())
		ok, detectedGB, requiredGB := hw.CheckVRAMRequirement(r.Hardware.MinVRAM)
		if !ok {
			fmt.Printf("\n  \033[33m⚠  VRAM warning:\033[0m This recipe requires %.0f GB VRAM.\n", requiredGB)
			fmt.Printf("     Your system has %.1f GB available.\n", detectedGB)
			if !confirm("     Continue anyway? [y/N]: ") {
				return fmt.Errorf("aborted by user")
			}
		}
	}

	// ── Step 3: Capability Check ───────────────────────────────────────────────
	printStep("Checking llama-server capabilities")
	requiredFlags := r.RequiredFlags()
	probeResult, err := probe.CheckRecipeCompatibility(requiredFlags)
	if err != nil {
		// llama-server not found — offer to install it
		fmt.Fprintf(os.Stderr, "\n\033[31m✗  llama-server not found\033[0m\n")
		if probe.OfferInstall() {
			// Re-probe after a successful install
			probeResult, err = probe.CheckRecipeCompatibility(requiredFlags)
			if err != nil {
				return fmt.Errorf("llama-server still unavailable after install: %w", err)
			}
		} else {
			return fmt.Errorf("llama-server is required but not installed")
		}
	}
	if len(probeResult.Missing) > 0 {
		fmt.Fprintf(os.Stderr, "\n\033[31m✗  Incompatible llama-server binary:\033[0m\n")
		fmt.Fprintf(os.Stderr, "   Missing flags required by this recipe:\n")
		for _, f := range probeResult.Missing {
			fmt.Fprintf(os.Stderr, "     %s\n", f)
		}
		fmt.Fprintf(os.Stderr, "\n   Update llama.cpp to a newer build.\n")
		fmt.Fprintf(os.Stderr, "   Install guide: https://bloc-theta.vercel.app/install\n\n")
		return fmt.Errorf("llama-server is missing required capabilities")
	}
	fmt.Printf("  \033[32m✓\033[0m  %s — all required flags supported\n", probeResult.BinaryPath)

	// ── Step 4: Download Model ─────────────────────────────────────────────────
	cacheDir, err := config.CacheDir()
	if err != nil {
		return err
	}
	dm, err := downloader.NewManager(cacheDir)
	if err != nil {
		return err
	}

	printStep("Checking model cache")
	cached, _ := dm.IsAlreadyCached(r.Model.File, r.Model.SHA256)
	var modelPath string
	if cached {
		modelPath = dm.ModelPath(r.Model.File)
		fmt.Printf("  \033[32m✓\033[0m  Already cached: %s\n", modelPath)
	} else {
		fmt.Printf("  Downloading %s (%.1f GB)...\n", r.Model.File, r.Model.SizeGB)
		modelPath, err = dm.EnsureDownloaded(
			context.Background(),
			r.Model.File,
			r.Model.DownloadURL,
			r.Model.SHA256,
			r.Model.SizeGB,
			func(downloaded, total int64, speedMBs float64) {
				pct := float64(downloaded) / float64(total) * 100
				bar := progressBar(int(pct), 30)
				fmt.Printf("\r  %s %.1f/%.1f GB  [%s] %.0f%% @ %.1f MB/s",
					r.Model.File,
					float64(downloaded)/1e9,
					float64(total)/1e9,
					bar,
					pct,
					speedMBs,
				)
			},
		)
		fmt.Println() // newline after progress bar
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		fmt.Printf("  \033[32m✓\033[0m  Saved to %s\n", modelPath)
	}

	// ── Step 5: Pre-run commands ───────────────────────────────────────────────
	if len(r.PreRun.Commands) > 0 {
		printStep("Pre-run setup")
		fmt.Println("  This recipe will execute the following commands before starting:")
		for _, c := range r.PreRun.Commands {
			fmt.Printf("    \033[33m%s\033[0m\n", c)
		}
		if !confirm("  Allow? [Y/n]: ") {
			return fmt.Errorf("pre-run commands rejected by user")
		}
		for _, c := range r.PreRun.Commands {
			// F-01: runShellCommand now actually executes the command.
			if err := runShellCommand(c, r.PreRun.Env); err != nil {
				return fmt.Errorf("pre-run command failed: %w", err)
			}
		}
	}

	// ── Step 6+7: Build command and run (or dry-run) ───────────────────────────
	flags := r.BuildFlags()
	if deployDryRun {
		fmt.Println("\n\033[36m── Dry run: llama-server command ─────────────────────────────────\033[0m")
		fmt.Printf("llama-server -m %s \\\n", modelPath)
		for i, f := range flags {
			if i < len(flags)-1 {
				fmt.Printf("  %s \\\n", f)
			} else {
				fmt.Printf("  %s\n", f)
			}
		}
		return nil
	}

	// Telemetry consent (first run only)
	if !deployNoTelemetry {
		telemetry.MaybePromptConsent()
	}

	printStep("Launching llama-server")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stats, err := runner.Run(ctx, modelPath, flags, r.PreRun.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  llama-server exited with error: %v\033[0m\n", err)
	}

	// ── Step 8: Shutdown + telemetry ──────────────────────────────────────────
	if !deployNoTelemetry && stats != nil {
		t, _ := config.LoadTelemetry()
		if t != nil && t.Enabled {
			telemetry.Send(recipeID, stats)
			fmt.Println("\n📊 Anonymous benchmark shared with the community. Thank you!")
		} else if t != nil && t.ConsentGiven && !t.Enabled {
			// user opted out — show summary but don't send
		} else {
			// Never asked — prompt once
			fmt.Println()
			if confirm("📊 Share anonymous benchmark with the community? [Y/n]: ") {
				t2, _ := config.LoadTelemetry()
				if t2 != nil {
					t2.Enabled = true
					t2.ConsentGiven = true
					config.SaveTelemetry(t2)
					telemetry.Send(recipeID, stats)
				}
			}
		}
	}

	if stats != nil && stats.TokensPerSecGeneration > 0 {
		fmt.Printf("\n📈 Session summary: %.1f t/s generation, %.1f t/s prefill\n",
			stats.TokensPerSecGeneration, stats.TokensPerSecPrefill)
	}

	return nil
}

// fetchRecipe downloads and parses the recipe YAML from the Hub API.
// F-09: Path segments are URL-encoded and validated before use.
// F-14: Response body is limited to 1 MB to prevent memory DoS.
func fetchRecipe(author, name string) (*recipe.Recipe, error) {
	// F-09: url.PathEscape ensures special characters don't create path traversal
	apiURL := fmt.Sprintf("%s/recipes/%s/%s",
		hubAPIBase,
		url.PathEscape(author),
		url.PathEscape(name),
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/yaml, application/json")
	req.Header.Set("User-Agent", "bloc-cli/"+Version)

	// P-01: Use shared package-level apiClient (not a new client per call)
	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("recipe %q not found — check spelling or visit https://bloc-theta.vercel.app/registry", author+"/"+name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	// F-14: Limit response to 1 MB — a valid recipe YAML is never this large.
	// Prevents memory DoS if a malicious Hub returns a huge payload.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	// Try to unwrap JSON envelope {"yaml_content": "..."} from the Hub API
	var envelope struct {
		YAMLContent string `json:"yaml_content"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.YAMLContent != "" {
		return recipe.Parse([]byte(envelope.YAMLContent))
	}

	// Fallback: treat the response body as raw YAML
	return recipe.Parse(body)
}

// printStep prints a numbered step header.
var stepN int

func printStep(label string) {
	stepN++
	fmt.Printf("\n\033[1m[%d] %s\033[0m\n", stepN, label)
}

// confirm reads a y/n answer from stdin.
func confirm(prompt string) bool {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return ans == "y" || ans == "yes" || ans == ""
	}
	return false
}

// progressBar renders an ASCII progress bar of given width.
func progressBar(pct, width int) string {
	filled := width * pct / 100
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("=", filled)
	if filled < width {
		bar += ">"
		bar += strings.Repeat(" ", width-filled-1)
	}
	return bar
}

// shortDesc truncates a description to maxLen characters.
func shortDesc(s string, maxLen int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// runShellCommand executes a shell command with the given env vars.
// F-01: Previously a no-op stub — now actually executes the command.
// Uses exec.Command("sh", "-c", command) to avoid the fmt.Sprintf("%q") injection
// that was present in the original stub (Go %q != shell quoting).
func runShellCommand(command string, env map[string]string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	// #nosec G204 — commands are from recipe YAML; user reviewed and confirmed the list above.
	// We use "sh -c command" as a single argument so the shell handles quoting,
	// NOT fmt.Sprintf which uses Go string quoting (different from shell quoting).
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd.Run()
}
