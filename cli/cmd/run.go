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
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/downloader"
	"github.com/bloc-org/bloc/internal/hardware"
	"github.com/bloc-org/bloc/internal/recipe"
	"github.com/bloc-org/bloc/internal/runtime"
	"github.com/bloc-org/bloc/internal/telemetry"
	"github.com/spf13/cobra"
	
	"dashboard/tui"
	tea "github.com/charmbracelet/bubbletea"
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

var runCmd = &cobra.Command{
	Use:     "run [author/recipe]",
	Aliases: []string{"deploy"},
	Short:   "Fetch and run a recipe from the Bloc registry",
	Long: `Fetch a recipe from bloc-theta.vercel.app, probe your hardware and runtime
capabilities, download the model weights if needed, and launch the server.

Examples:
  bloc run arnav080/qwen3-30b-moe-8gb-cpu-offload
  bloc run arnav080/qwen3-30b-moe-8gb-cpu-offload --dry-run
  bloc run arnav080/step-3.7-flash --runtime docker`,
	Args: cobra.ExactArgs(1),
	RunE: runRecipe,
}

func init() {
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Show the server command without running it")
	runCmd.Flags().BoolVar(&runNoTelemetry, "no-telemetry", false, "Disable telemetry for this run")
	runCmd.Flags().StringVar(&runRuntime, "runtime", "", "Override recipe's declared runtime (native|docker)")
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
	recipeID := args[0]
	var r *recipe.Recipe
	var err error
	isLocal := isLocalRecipe(recipeID)

	var stepN int
	printStep := func(label string) {
		stepN++
		fmt.Printf("\n\033[1m[%d] %s\033[0m\n", stepN, label)
	}

	if isLocal {
		// ── Step 1: Parse local recipe ─────────────────────────────────────────────
		printStep("Loading local recipe")
		r, err = recipe.ParseFileLocal(recipeID)
		if err != nil {
			return fmt.Errorf("cannot parse local recipe: %w", err)
		}
		fmt.Printf("  \033[32m✓\033[0m  Local file loaded: %s (%s)\n", recipeID, r.Metadata.Name)
	} else {
		parts := strings.SplitN(recipeID, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid recipe ID %q — expected author/recipe-name or local path (.yaml)", recipeID)
		}

		// F-09: Validate both path segments against the allowlist regex before use.
		author, name := parts[0], parts[1]
		if !recipeIDRe.MatchString(author) {
			return fmt.Errorf("invalid author name %q — only alphanumeric, dash, dot and underscore allowed", author)
		}
		if !recipeIDRe.MatchString(name) {
			return fmt.Errorf("invalid recipe name %q — only alphanumeric, dash, dot and underscore allowed", name)
		}

		// ── Step 1: Fetch recipe ───────────────────────────────────────────────────
		printStep("Fetching recipe")
		r, err = fetchRecipe(author, name)
		if err != nil {
			return fmt.Errorf("cannot fetch recipe: %w", err)
		}
		fmt.Printf("  \033[32m✓\033[0m  %s — %s\n", r.Metadata.Name, shortDesc(r.Metadata.Description, 72))
	}

	// ── Step 2: Resolve runtime ────────────────────────────────────────────────
	printStep("Resolving runtime")
	rt, err := runtime.Resolve(r, runRuntime)
	if err != nil {
		return fmt.Errorf("cannot resolve runtime: %w", err)
	}
	fmt.Printf("  \033[32m✓\033[0m  Engine: %s\n", rt.Name())

	// ── Step 3: Hardware Probe ─────────────────────────────────────────────────
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
			if runDryRun {
				fmt.Println("     [Dry Run] Proceeding with dry-run command display.")
			} else if !confirm("     Continue anyway? [y/N]: ") {
				return fmt.Errorf("aborted by user")
			}
		}
	}

	// ── Step 4: Runtime Capability Check ──────────────────────────────────────
	var probeResult *runtime.ProbeResult
	if runDryRun {
		probeResult = &runtime.ProbeResult{
			BinaryPath: rt.Name(),
		}
	} else {
		printStep(fmt.Sprintf("Checking %s capabilities", rt.Name()))
		requiredFlags := r.RequiredFlags()
		var err error
		probeResult, err = rt.Probe(requiredFlags)
		if err != nil {
			// Runtime not found — offer to install it
			fmt.Fprintf(os.Stderr, "\n\033[31m✗  %s not found\033[0m\n", rt.Name())
			if rt.OfferInstall() {
				// Re-probe after a successful install
				probeResult, err = rt.Probe(requiredFlags)
				if err != nil {
					return fmt.Errorf("%s still unavailable after install: %w", rt.Name(), err)
				}
			} else {
				return fmt.Errorf("%s is required but not installed", rt.Name())
			}
		}
		if len(probeResult.Missing) > 0 {
			fmt.Fprintf(os.Stderr, "\n\033[31m✗  Incompatible %s binary:\033[0m\n", rt.Name())
			fmt.Fprintf(os.Stderr, "   Missing flags required by this recipe:\n")
			for _, f := range probeResult.Missing {
				fmt.Fprintf(os.Stderr, "     %s\n", f)
			}
			fmt.Fprintf(os.Stderr, "\n   Update %s to a newer build.\n", rt.Name())
			fmt.Fprintf(os.Stderr, "   Install guide: https://bloc-theta.vercel.app/install\n\n")
			return fmt.Errorf("%s is missing required capabilities", rt.Name())
		}
		fmt.Printf("  \033[32m✓\033[0m  %s — all required flags supported\n", probeResult.BinaryPath)
	}

	// ── Context for all long-running operations (downloads + server) ────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Step 5: Download Model ─────────────────────────────────────────────────
	cacheDir, err := config.CacheDir()
	if err != nil {
		return err
	}
	dm, err := downloader.NewManager(cacheDir)
	if err != nil {
		return err
	}

	if hfCreds, hfErr := config.LoadHFAuth(); hfErr == nil && hfCreds != nil {
		dm.SetHFToken(hfCreds.Token)
	}

	printStep("Checking model cache")
	var modelPath string

	if r.Model.HFRepo != "" {
		if dm.IsRepoCached(r.Model.HFRepo, "") {
			modelPath = dm.RepoPath(r.Model.HFRepo, "main")
			fmt.Printf("  \033[32m✓\033[0m  Already cached: %s\n", modelPath)
		} else {
			if runDryRun {
				modelPath = dm.RepoPath(r.Model.HFRepo, "main")
				fmt.Printf("  \033[32m✓\033[0m  [Dry Run] Simulated model cache path: %s\n", modelPath)
			} else {
				if err := checkDiskSpace(cacheDir, r.Model.SizeGB); err != nil {
					return err
				}
				fmt.Printf("  Downloading HF repo %s (%.1f GB)...\n", r.Model.HFRepo, r.Model.SizeGB)
				bw := bufio.NewWriterSize(os.Stdout, 1024)
				modelPath, err = dm.EnsureRepoDownloaded(
					ctx,
					r.Model.HFRepo,
					"", // default revision "main"
					func(downloaded, total int64, speedMBs float64) {
						pct := float64(0)
						if total > 0 {
							pct = float64(downloaded) / float64(total) * 100
						}
						bar := progressBar(int(pct), 30)
						_, _ = fmt.Fprintf(bw, "\r  %s %.1f/%.1f GB  [%s] %.0f%% @ %.1f MB/s",
							r.Model.HFRepo,
							float64(downloaded)/1e9,
							float64(total)/1e9,
							bar,
							pct,
							speedMBs,
						)
						_ = bw.Flush()
					},
				)
				fmt.Println() // newline after progress bar
				if err != nil {
					return fmt.Errorf("repo download failed: %w", err)
				}
				fmt.Printf("  \033[32m✓\033[0m  Saved to %s\n", modelPath)
			}
		}
	} else {
		cached, _ := dm.IsAlreadyCached(r.Model.File, r.Model.SHA256)
		if cached {
			modelPath = dm.ModelPath(r.Model.File)
			fmt.Printf("  \033[32m✓\033[0m  Already cached: %s\n", modelPath)
		} else {
			if runDryRun {
				modelPath = dm.ModelPath(r.Model.File)
				fmt.Printf("  \033[32m✓\033[0m  [Dry Run] Simulated model cache path: %s\n", modelPath)
			} else {
				if err := checkDiskSpace(cacheDir, r.Model.SizeGB); err != nil {
					return err
				}
				fmt.Printf("  Downloading %s (%.1f GB)...\n", r.Model.File, r.Model.SizeGB)
				bw := bufio.NewWriterSize(os.Stdout, 1024)
				modelPath, err = dm.EnsureDownloaded(
					ctx,
					r.Model.File,
					r.Model.DownloadURL,
					r.Model.SHA256,
					r.Model.SizeGB,
					func(downloaded, total int64, speedMBs float64) {
						pct := float64(downloaded) / float64(total) * 100
						bar := progressBar(int(pct), 30)
						_, _ = fmt.Fprintf(bw, "\r  %s %.1f/%.1f GB  [%s] %.0f%% @ %.1f MB/s",
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
		}
	}

	// ── Step 6: Pre-run commands ───────────────────────────────────────────────
	if len(r.PreRun.Commands) > 0 {
		printStep("Pre-run setup")
		for _, c := range r.PreRun.Commands {
			if preRunCmdBannedRe.MatchString(c) {
				return fmt.Errorf("pre-run command %q contains shell metacharacters — use a script instead", c)
			}
		}
		fmt.Println("  This recipe will execute the following commands before starting:")
		for _, c := range r.PreRun.Commands {
			fmt.Printf("    \033[33m%s\033[0m\n", c)
		}
		if runDryRun {
			fmt.Println("  [Dry Run] Skipping pre-run command execution.")
		} else {
			if !confirm("  Allow? [Y/n]: ") {
				return fmt.Errorf("pre-run commands rejected by user")
			}
			for _, c := range r.PreRun.Commands {
				if err := runShellCommand(c, r.PreRun.Env); err != nil {
					return fmt.Errorf("pre-run command failed: %w", err)
				}
			}
		}
	}

	// ── Step 7: trust_remote_code gate (F-19) ─────────────────────────────────
	engineName := r.Engine.Name
	if engineName == "" {
		engineName = "llama.cpp"
	}
	if r.EngineConfig.TrustRemoteCode && engineName == "vllm" {
		printStep("Security confirmation required")
		fmt.Println()
		fmt.Println("  \033[33m⚠  This recipe sets trust_remote_code: true\033[0m")
		fmt.Println()
		fmt.Println("  This allows vLLM to execute custom Python code bundled with the model.")
		fmt.Println("  Only proceed if you trust the model author and have reviewed the code at:")
		fmt.Printf("  \033[36mhttps://huggingface.co/%s/tree/main\033[0m\n", r.Model.HFRepo)
		fmt.Println()
		if runDryRun {
			fmt.Println("  [Dry Run] Skipping trust_remote_code confirmation.")
		} else if !confirmYesExplicit("  Allow execution of custom model code? [y/N]: ") {
			return fmt.Errorf("trust_remote_code rejected by user — aborting")
		}
	}

	// ── Step 8: Build flags (engine-aware) + dry-run ───────────────────────────
	var flags []string
	switch engineName {
	case "vllm":
		flags = r.BuildVLLMFlags()
		if r.EngineConfig.TrustRemoteCode {
			flags = append(flags, "--trust-remote-code")
		}
	case "sglang":
		flags = r.BuildSGLangFlags()
		// Inject CUDA device pinning into the env map so the Docker runtime
		// can pass it as -e CUDA_VISIBLE_DEVICES=... to docker run.
		// This is the canonical place to declare GPU bus IDs for SGLang.
		if devs := r.EngineConfig.SGLangCUDAVisibleDevices; devs != "" {
			if r.PreRun.Env == nil {
				r.PreRun.Env = make(map[string]string)
			}
			r.PreRun.Env["CUDA_VISIBLE_DEVICES"] = devs
		}
	default:
		flags = r.BuildFlags()
	}

	if runDryRun {
		fmt.Printf("\n\033[36m── Dry run: %s command ──────────────────────────────────────────\033[0m\n", rt.Name())
		switch engineName {
		case "vllm":
			fmt.Printf("python3 -m vllm.entrypoints.openai.api_server \\\n")
			fmt.Printf("  --model %s \\\n", modelPath)
		case "sglang":
			fmt.Printf("python3 -m sglang.launch_server \\\n")
			fmt.Printf("  --model-path %s \\\n", modelPath)
			fmt.Printf("  --host 0.0.0.0 \\\n")
			fmt.Printf("  --port %d \\\n", r.EngineConfig.Port)
		default:
			fmt.Printf("%s -m %s \\\n", rt.Name(), modelPath)
		}
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
	if !runNoTelemetry && !isLocal {
		telemetry.MaybePromptConsent()
	}

	printStep(fmt.Sprintf("Launching %s", rt.Name()))

	// SEC-6: use os.CreateTemp to avoid predictable /tmp path (symlink attack)
	// RACE-4: do NOT defer logFile.Close() here — closed explicitly after engineDone
	logFile, err := os.CreateTemp("", "bloc-engine-*.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not create log file: %v\n", err)
	}

	runCfg := runtime.RunConfig{
		ModelPath: modelPath,
		Flags:     flags,
		EnvVars:   r.PreRun.Env,
		Port:      r.EngineConfig.Port,
		Recipe:    r,
		Silent:    true, // Silence engine logs to prevent TUI corruption
		LogWriter: logFile,
	}
	if modelPath != "" {
		now := time.Now()
		_ = os.Chtimes(modelPath, now, now)
	}

	// RACE-1: use a typed result channel instead of bare variables shared across
	// goroutine boundaries. Reading stats/runErr without synchronization is a
	// data race detectable by `go test -race`.
	type engineResult struct {
		stats  *runtime.Stats
		runErr error
	}
	engineDone := make(chan engineResult, 1)

	// RACE-4: do NOT defer logFile.Close() here — the engine goroutine writes
	// to logFile via runCfg.LogWriter. We must wait for the goroutine to exit
	// (signalled by engineDone) before closing the file.
	// Start engine in background
	go func() {
		s, e := rt.Run(ctx, runCfg)
		if e != nil {
			fmt.Fprintf(os.Stderr, "\033[31m✗  %s exited with error: %v\033[0m\n", rt.Name(), e)
		}
		engineDone <- engineResult{stats: s, runErr: e}
	}()

	// Launch TUI
	modelName := r.Metadata.Name
	if modelName == "" {
		if r.Model.HFRepo != "" {
			modelName = r.Model.HFRepo
		} else {
			modelName = r.Model.File
		}
	}
	
	hwString := "CPU"
	if hw != nil {
		switch hw.Platform {
		case "metal":
			hwString = "Metal"
		case "cuda":
			hwString = "CUDA"
		case "rocm":
			hwString = "ROCm"
		}
	}

	port := r.EngineConfig.Port
	if port == 0 {
		port = 8080
	}

	logPath := ""
	if logFile != nil {
		logPath = logFile.Name()
	}
	
	p := tea.NewProgram(tui.NewApp(Version, logPath, port, rt.Name(), modelName, hwString), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting dashboard: %v\n", err)
	}
	
	// Graceful shutdown of the background engine when TUI exits (Ctrl+C)
	cancel()

	// RACE-1+RACE-4: block until the engine goroutine exits so we can safely
	// read stats and close the log file without a data race.
	result := <-engineDone
	if logFile != nil {
		logFile.Close()
	}
	stats := result.stats

	if !runNoTelemetry && !isLocal && stats != nil {
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

	if auth, authErr := config.LoadAuth(); authErr == nil && auth != nil && auth.Token != "" {
		req.Header.Set("Authorization", "Bearer "+auth.Token)
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

func confirm(prompt string) bool {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return ans == "y" || ans == "yes" || ans == ""
	}
	return true
}

func confirmYesExplicit(prompt string) bool {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return ans == "y" || ans == "yes"
	}
	return false
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

func runShellCommand(command string, env map[string]string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// SEC-1: build a minimal, clean environment instead of inheriting os.Environ().
	// Passing the full environment allows $BASH_ENV / $ENV / $LD_PRELOAD to run
	// attacker-controlled code before the command even executes.
	safeEnv := []string{
		"HOME=" + os.Getenv("HOME"),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"USER=" + os.Getenv("USER"),
		"LANG=" + os.Getenv("LANG"),
		"TERM=" + os.Getenv("TERM"),
	}
	for k, v := range env {
		safeEnv = append(safeEnv, k+"="+v)
	}
	cmd.Env = safeEnv
	return cmd.Run()
}

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
