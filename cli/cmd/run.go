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
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/hardware"
	"github.com/bloc-org/bloc/internal/pipeline"
	"github.com/bloc-org/bloc/internal/recipe"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

// preRunCmdRe is an allowlist for pre-run commands.
// SEC-02: We cannot parse shell syntax in Go, so we reject any command containing
// shell metacharacters that could cause injection (;|&`$(){}><\r\n\\).
// Authors who need complex commands should put them in a script file and call that.
var preRunCmdBannedRe = regexp.MustCompile(`[;|&` + "`" + `$(){}><\r\n\\]`)

var runDryRun bool
var runNoTelemetry bool
var runRuntime string // --runtime flag: overrides recipe's engine.runtime
var runYes bool      // --yes flag: auto-confirm all prompts (CI-safe opt-in)

// MED-7: Package-level compiled regex for sanitizeLogSlug.
// Avoids re-compiling on every log file creation.
var logSlugRe = regexp.MustCompile(`[^a-zA-Z0-9\-.]+`)



var runCmd = &cobra.Command{
	Use:     "run [author/recipe]",
	Aliases: []string{"deploy"},
	Short:   "Fetch and run a recipe from the Bloc registry",
	Long: `Fetch a recipe from bloc-theta.vercel.app, probe your hardware and runtime
capabilities, download the model weights if needed, and launch the server.

Examples:
  bloc run arnav080/qwen3-30b-moe-8gb-cpu-offload
  bloc run arnav080/qwen3-30b-moe-8gb-cpu-offload --dry-run
  bloc run arnav080/step-3.7-flash --runtime docker
  bloc run arnav080/qwen3-30b --yes   # auto-confirm all prompts (CI use)`,
	Args: cobra.ExactArgs(1),
	RunE: runRecipe,
}

func init() {
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Show the server command without running it")
	runCmd.Flags().BoolVar(&runNoTelemetry, "no-telemetry", false, "Disable telemetry for this run")
	runCmd.Flags().StringVar(&runRuntime, "runtime", "", "Override recipe's declared runtime (native|docker)")
	runCmd.Flags().BoolVarP(&runYes, "yes", "y", false, "Auto-confirm all prompts (for CI / non-interactive use)")
}

func isLocalRecipe(path string) bool {
	// SEC-10: only treat as a local recipe if it has a YAML extension OR is an
	// explicit file path (contains a path separator). Prevents any arbitrary
	// existing filesystem path from being silently loaded as a recipe.
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return true
	}
	// Accept explicit relative/absolute paths (e.g. ./my-recipe or /home/user/recipe)
	if strings.Contains(path, "/") || strings.Contains(path, "\\") {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func runRecipe(cmd *cobra.Command, args []string) error {
	// Top-level context: cancelled by SIGINT/SIGTERM (Ctrl+C).
	// All stages and the engine goroutine honour this context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cacheDir, err := config.CacheDir()
	if err != nil {
		return err
	}

	pipeline.SetVersion(Version)

	state := &pipeline.RunState{
		RecipeID:        args[0],
		IsDryRun:        runDryRun,
		IsYes:           runYes,
		RuntimeOverride: runRuntime,
		NoTelemetry:     runNoTelemetry,
		APIBase:         hubAPIBase,
		CacheDir:        cacheDir,
	}

	p := pipeline.New(
		&pipeline.FetchRecipeStage{},
		&pipeline.ResolveEngineStage{},
		&pipeline.HardwareProbeStage{},
		&pipeline.CapabilityProbeStage{},
		&pipeline.DownloadModelStage{},
		&pipeline.PreRunStage{},
		&pipeline.SecurityGateStage{},
		&pipeline.BuildFlagsStage{},
		&pipeline.LaunchStage{},
	)

	if err := p.Execute(ctx, state); err != nil {
		if pipeline.IsDryRunDone(unwrapSentinel(err)) {
			return nil // --dry-run: clean exit
		}
		return err
	}
	return nil
}

// unwrapSentinel unwraps one level of pipeline stage error wrapping
// ("[StageName] <err>") to expose the underlying sentinel for IsDryRunDone.
func unwrapSentinel(err error) error {
	if uw, ok := err.(interface{ Unwrap() error }); ok {
		return uw.Unwrap()
	}
	return err
}


// fetchRecipe downloads and parses the recipe YAML from the Hub API.
func fetchRecipe(author, name string) (*recipe.Recipe, error) {
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

	// SEC-09 (M-5): Only send auth token to the official Hub API to prevent token leakage.
	// If the user overrides BLOC_HUB_API_URL to a third-party server, we must not
	// silently transmit their official authentication token.
	isOfficialAPI := strings.HasPrefix(apiURL, "https://hub.bloc.ai/") || strings.HasPrefix(apiURL, "https://api.bloc.ai/")
	if isOfficialAPI {
		if auth, authErr := config.LoadAuth(); authErr == nil && auth != nil && auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		}
	}

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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var envelope struct {
		YAMLContent string `json:"yaml_content"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.YAMLContent != "" {
		return recipe.Parse([]byte(envelope.YAMLContent))
	}

	return recipe.Parse(body)
}

// confirm shows prompt and returns true if the user answers y/yes/enter.
//
// FIX-1: Non-TTY safety.
//   - If --yes is set, returns true immediately (CI-safe explicit opt-in).
//   - If stdin is not a terminal (piped, redirected, GitHub Actions), returns
//     false and prints a clear message. The old behaviour was to return true on
//     EOF, silently auto-approving trust_remote_code and pre-run commands in CI.
//   - If stdin IS a terminal and the user hits Ctrl+D (EOF), returns false
//     (treat as "no", matching user intent).
func confirm(prompt string) bool {
	if runYes {
		fmt.Printf("%s [auto-confirmed via --yes]\n", prompt)
		return true
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "  [non-interactive] Prompt skipped — use --yes to auto-confirm in CI\n")
		return false
	}
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return ans == "y" || ans == "yes" || ans == ""
	}
	// EOF on a real TTY means the user hit Ctrl+D — treat as "no".
	return false
}

// confirmYesExplicit is like confirm but requires an explicit "y" or "yes";
// an empty Enter is treated as "no". Used for high-risk gates (trust_remote_code).
func confirmYesExplicit(prompt string) bool {
	if runYes {
		fmt.Printf("%s [auto-confirmed via --yes]\n", prompt)
		return true
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "  [non-interactive] Security prompt skipped — use --yes to auto-confirm in CI\n")
		return false
	}
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return ans == "y" || ans == "yes"
	}
	return false
}

// openEngineLogFile creates a named log file under cacheDir/logs/.
// FIX-4: Replaces os.CreateTemp("", "bloc-engine-*.log") which put logs in
// an ephemeral /tmp directory, making post-crash debugging nearly impossible.
func openEngineLogFile(cacheDir, recipeName string) (*os.File, error) {
	logDir := filepath.Join(cacheDir, "logs")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("cannot create log dir: %w", err)
	}
	slug := sanitizeLogSlug(recipeName)
	logName := fmt.Sprintf("engine-%s-%s.log", slug, time.Now().Format("20060102-150405"))
	logPath := filepath.Join(logDir, logName)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return nil, fmt.Errorf("cannot create log file: %w", err)
	}
	return f, nil
}

// sanitizeLogSlug converts a recipe name into a safe filename component.
func sanitizeLogSlug(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	// Replace any character that is not alphanumeric, dash, or dot with a dash.
	slug := logSlugRe.ReplaceAllString(name, "-")
	// Cap at 48 chars to keep filenames reasonable.
	if len(slug) > 48 {
		slug = slug[:48]
	}
	return slug
}

// pruneEngineLogs removes all but the `keep` most recent engine-*.log files
// from logDir. Silent on errors — log rotation failure is never fatal.
func pruneEngineLogs(logDir string, keep int) error {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "engine-") && strings.HasSuffix(e.Name(), ".log") {
			logs = append(logs, filepath.Join(logDir, e.Name()))
		}
	}
	// Sort ascending by name (which embeds a timestamp), then delete the oldest.
	sort.Strings(logs)
	for len(logs) > keep {
		_ = os.Remove(logs[0])
		logs = logs[1:]
	}
	return nil
}

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

func shortDesc(s string, maxLen int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// NOTE: runShellCommand was removed (SEC-01 / SEC-02).
// The function used exec.Command("sh", "-c", command) which is a shell
// injection vector, and injected recipe env vars without key or value
// sanitization. The safe implementation lives in pipeline/stage_prerun.go
// which uses strings.Fields + direct exec.Command (no shell) and a
// minimal safe environment. That version is the only one called at runtime.

func checkDiskSpace(cacheDir string, sizeGB float64) error {
	freeBytes, err := hardware.FreeSpaceBytes(cacheDir)
	if err != nil {
		return nil
	}

	freeGB := float64(freeBytes) / 1e9
	requiredGB := sizeGB * 1.1

	if freeGB < requiredGB {
		fmt.Println()
		fmt.Printf("  \033[33m⚠  Warning:\033[0m This model is ~%.1f GB. You have %.1f GB free.\n", sizeGB, freeGB)
		fmt.Println("     Run 'bloc models prune' to free space.")
		if !confirm("     Continue anyway? [y/N]: ") {
			return fmt.Errorf("cancelled due to low disk space")
		}
		fmt.Println()
	}
	return nil
}
