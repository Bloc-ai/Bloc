package pipeline

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/telemetry"

	"dashboard/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// engineReadyTimeout is the maximum time to wait for the engine /health endpoint.
// 5 minutes is generous enough for 30 GB+ models on slow hardware.
const engineReadyTimeout = 5 * time.Minute

// engineResult carries the outcome of a completed engine subprocess.
type engineResult struct {
	stats        *process.Stats
	runErr       error
	wasCancelled bool
}

// LaunchStage is the final stage: it opens the log file, starts the engine in
// a background goroutine, polls /health until the engine is ready, launches
// the TUI, and blocks until the TUI exits (user pressed Ctrl+C).
//
// After the TUI exits, it cancels the engine context, waits for the engine
// goroutine to finish, closes the log file, and optionally emits telemetry.
//
// Sets: state.LogFile, state.LogPath, state.Port, state.Stats
type LaunchStage struct{}

func (s *LaunchStage) Name() string { return "Launching engine" }

func (s *LaunchStage) Run(ctx context.Context, state *RunState) error {
	r := state.Recipe
	eng := state.Engine
	cacheDir := state.CacheDir

	// ── Open named log file (Fix 4) ──────────────────────────────────────────
	logFile, logErr := openEngineLogFile(cacheDir, r.Metadata.Name)
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "  ⚠  Warning: Could not create log file: %v\n", logErr)
	} else {
		state.LogFile = logFile
		state.LogPath = logFile.Name()
		_ = pruneEngineLogs(filepath.Join(cacheDir, "logs"), 10)
	}

	// ── Build LaunchConfig ────────────────────────────────────────────────────
	port := state.ResolvedPort()
	state.Port = port

	// Check if the port is already in use by another process
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return fmt.Errorf("port %d is already in use: %w", port, err)
	}
	ln.Close()

	launchCfg := engine.LaunchConfig{
		ModelPath: state.ModelPath,
		Flags:     state.Flags,
		EnvVars:   r.PreRun.Env,
		Port:      port,
		Recipe:    r,
		Silent:    true, // silence engine logs during TUI — they go to the log file
		LogWriter: logFile,
	}

	// Touch the model file to update its access time (cache eviction heuristic).
	if state.ModelPath != "" {
		now := time.Now()
		_ = os.Chtimes(state.ModelPath, now, now)
	}

	// ── Build supervisor ─────────────────────────────────────────────────────
	sv, err := eng.NewSupervisor(launchCfg)
	if err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return fmt.Errorf("cannot create engine supervisor: %w", err)
	}

	// Create a local cancellable context for managing the engine's lifecycle cleanly
	launchCtx, launchCancel := context.WithCancel(ctx)
	defer launchCancel()

	// ── Start engine goroutine (RACE-1: typed channel, not bare variables) ───
	engineDone := make(chan engineResult, 1)
	go func() {
		s, e := sv.Run(launchCtx)
		wasCancelled := launchCtx.Err() != nil
		engineDone <- engineResult{stats: s, runErr: e, wasCancelled: wasCancelled}
	}()

	// ── Poll /health (Fix 5) ─────────────────────────────────────────────────
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	if err := waitForEngineReady(ctx, healthURL, engineReadyTimeout, state.LogPath, engineDone); err != nil {
		// Engine crashed or timed out — clean up and surface the error.
		// Drain engineDone defensively; waitForEngineReady may have consumed it.
		select {
		case <-engineDone:
		default:
		}
		if logFile != nil {
			logFile.Close()
		}
		fmt.Fprintf(os.Stderr, "\n  Engine logs: %s\n", state.LogPath)
		return fmt.Errorf("engine failed to become ready: %w", err)
	}

	// ── TUI ──────────────────────────────────────────────────────────────────
	modelName := r.Metadata.Name
	if modelName == "" {
		if r.Model.HFRepo != "" {
			modelName = r.Model.HFRepo
		} else {
			modelName = r.Model.File
		}
	}

	hwString := "CPU"
	if state.Hardware != nil {
		switch state.Hardware.Platform {
		case "metal":
			hwString = "Metal"
		case "cuda":
			hwString = "CUDA"
		case "rocm":
			hwString = "ROCm"
		}
	}

	prog := tea.NewProgram(
		tui.NewApp(version(), state.LogPath, port, eng.Name(), modelName, hwString),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting dashboard: %v\n", err)
	}

	// ── Shutdown ─────────────────────────────────────────────────────────────
	// Cancel the local engine context so sv.Run() returns, then wait for the goroutine.
	launchCancel()
	result := <-engineDone
	if logFile != nil {
		logFile.Close()
	}
	state.Stats = result.stats

	// ── Telemetry (opt-in, post-run) ─────────────────────────────────────────
	if !state.NoTelemetry && !state.IsLocal && state.Stats != nil {
		emitTelemetry(state)
	}

	if state.Stats != nil {
		gen, _, _, _, _ := state.Stats.Snapshot()
		if gen > 0 {
			_, prefill, _, _, _ := state.Stats.Snapshot()
			fmt.Printf("\n📈 Session summary: %.1f t/s generation, %.1f t/s prefill\n", gen, prefill)
		}
	}

	fmt.Fprintf(os.Stderr, "\n  Engine logs: %s\n", state.LogPath)
	return nil
}

// emitTelemetry handles the post-run consent prompt and sends benchmark data.
func emitTelemetry(state *RunState) {
	t, _ := config.LoadTelemetry()
	if t != nil && t.Enabled {
		telemetry.Send(state.RecipeID, state.Stats)
		fmt.Println("\n📊 Anonymous benchmark shared with the community. Thank you!")
		return
	}
	if t != nil && t.ConsentGiven {
		return // user previously opted out
	}
	// First run — prompt once.
	fmt.Println()
	if confirmPrompt("📊 Share anonymous benchmark with the community? [Y/n]: ", state.IsYes) {
		t2, _ := config.LoadTelemetry()
		if t2 != nil {
			t2.Enabled = true
			t2.ConsentGiven = true
			config.SaveTelemetry(t2)
			telemetry.Send(state.RecipeID, state.Stats)
		}
	}
}

// waitForEngineReady polls healthURL every 2 seconds until the engine responds
// HTTP 200, ctx is cancelled, or timeout elapses.
// engineDone is monitored so a crashed engine is detected immediately.
func waitForEngineReady(
	ctx context.Context,
	healthURL string,
	timeout time.Duration,
	logPath string,
	engineDone <-chan engineResult,
) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "  Waiting for engine to be ready (timeout %s)...\n", timeout.Round(time.Second))

	for {
		// Check for early engine exit (crash at startup).
		select {
		case res := <-engineDone:
			printEngineLogs(logPath, 30)
			if res.runErr != nil {
				return fmt.Errorf("engine exited before becoming ready: %w", res.runErr)
			}
			return fmt.Errorf("engine exited unexpectedly before /health responded")
		default:
		}

		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  Engine ready at %s\n", healthURL)
				return nil
			}
		}

		if time.Now().After(deadline) {
			printEngineLogs(logPath, 30)
			return fmt.Errorf("timed out after %s waiting for engine at %s",
				timeout.Round(time.Second), healthURL)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// printEngineLogs tails the last n lines of the log file to stderr.
func printEngineLogs(logPath string, n int) {
	if logPath == "" {
		return
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return
	}
	lines := splitLines(string(data))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	fmt.Fprintln(os.Stderr, "\n\033[33m── Last engine log lines ────────────────────────────────────────\033[0m")
	for _, l := range lines {
		fmt.Fprintln(os.Stderr, l)
	}
	fmt.Fprintln(os.Stderr, "\033[33m─────────────────────────────────────────────────────────────────\033[0m")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// version returns the CLI version string. Deferred to avoid circular imports
// with the cmd package — populated via build-time injection in cmd/root.go.
// Falls back to "dev" if not set.
func version() string {
	if launchVersion != "" {
		return launchVersion
	}
	return "dev"
}

// LaunchVersion is set by cmd/run.go before calling pipeline.Execute so the
// TUI shows the correct version string.
var launchVersion string

// SetVersion allows cmd/run.go to inject the version without a circular import.
func SetVersion(v string) { launchVersion = v }
