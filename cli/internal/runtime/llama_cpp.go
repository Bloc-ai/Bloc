package runtime

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// setSysProcAttr configures platform-specific process group attributes.
// Implemented in llama_cpp_unix.go (sets Setpgid=true) and
// llama_cpp_windows.go (no-op — Windows has no process groups).


// LlamaCppRuntime implements Runtime for llama.cpp (llama-server binary).
// It consolidates the logic that was previously split across internal/probe
// and internal/runner, giving the llama.cpp engine a single cohesive struct.
type LlamaCppRuntime struct{}

// Name returns the display label used in CLI step headers.
func (r *LlamaCppRuntime) Name() string { return "llama-server" }

// Probe checks whether llama-server is in PATH and whether all flags
// required by the recipe are supported by the detected binary version.
// This is a direct migration of the probe.CheckRecipeCompatibility logic.
func (r *LlamaCppRuntime) Probe(required map[string]struct{}) (*ProbeResult, error) {
	path, err := exec.LookPath("llama-server")
	if err != nil {
		return nil, fmt.Errorf("llama-server not found in PATH")
	}

	// Run --help; llama-server exits with code 1 but prints to stdout+stderr
	cmd := exec.Command(path, "--help")
	out, _ := cmd.CombinedOutput() // intentionally ignore exit code
	if len(out) == 0 {
		return nil, fmt.Errorf("llama-server --help returned no output")
	}

	supported := parseLlamaFlags(string(out))
	res := &ProbeResult{BinaryPath: path}
	for flag := range required {
		if _, ok := supported[flag]; !ok {
			res.Missing = append(res.Missing, flag)
		}
	}
	return res, nil
}

// OfferInstall prompts the user to install llama.cpp via Homebrew (macOS)
// or prints manual instructions (Linux/other). Returns true if llama-server
// is available after the call.
// Migrated from probe.OfferInstall.
func (r *LlamaCppRuntime) OfferInstall() bool {
	switch runtime.GOOS {
	case "darwin":
		fmt.Print("\n  Would you like to install llama.cpp via Homebrew now? [Y/n]: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if ans != "" && ans != "y" && ans != "yes" {
				fmt.Println("  Skipped. Re-run after installing manually:")
				fmt.Println("    brew install llama.cpp")
				return false
			}
		}
		fmt.Println("  Running: brew install llama.cpp ...")
		// SEC-BREW: Validate brew binary path is in a known safe location to prevent
		// binary hijack via a modified PATH pointing to a malicious brew binary.
		brewPath, brewErr := exec.LookPath("brew")
		if brewErr != nil {
			fmt.Fprintln(os.Stderr, "\033[31m✗  brew not found in PATH\033[0m")
			return false
		}
		if !strings.HasPrefix(brewPath, "/opt/homebrew/") && !strings.HasPrefix(brewPath, "/usr/local/") {
			fmt.Fprintf(os.Stderr, "\033[31m✗  brew binary at unexpected path %q — refusing to execute\033[0m\n", brewPath)
			return false
		}
		cmd := exec.Command(brewPath, "install", "llama.cpp")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "\n\033[31m✗  brew install failed: %v\033[0m\n", err)
			return false
		}
		if _, err := exec.LookPath("llama-server"); err != nil {
			fmt.Fprintln(os.Stderr, "\033[33m⚠  llama-server still not found after install. Try opening a new terminal.\033[0m")
			return false
		}
		fmt.Println("  \033[32m✓\033[0m  llama.cpp installed successfully.")
		return true

	case "linux":
		fmt.Println("  Auto-install is not supported on Linux.")
		fmt.Println("  Download a prebuilt binary from:")
		fmt.Println("    https://github.com/ggml-org/llama.cpp/releases")
		fmt.Println("  or build from source: https://bloc-theta.vercel.app/install")
		return false

	default:
		fmt.Println("  Auto-install is not supported on this platform.")
		fmt.Println("  Install guide: https://bloc-theta.vercel.app/install")
		return false
	}
}

// Run launches llama-server with the given RunConfig and blocks until it exits.
// Streams stdout/stderr to the terminal, captures stats, and kills the entire
// process group on SIGINT/SIGTERM or ctx cancel.
//
// Key changes from the old runner.Run package function:
//   - Binary name comes from exec.LookPath("llama-server"), not hardcoded
//   - Port printed in the startup message comes from cfg.Port (Fix #5)
//   - Signal handler and process-group kill are isolated to this struct
func (r *LlamaCppRuntime) Run(ctx context.Context, cfg RunConfig) (*Stats, error) {
	binary, err := exec.LookPath("llama-server")
	if err != nil {
		return nil, fmt.Errorf("llama-server not found in PATH: %w", err)
	}

	allArgs := append([]string{"-m", cfg.ModelPath}, cfg.Flags...)
	cmd := exec.CommandContext(ctx, binary, allArgs...)

	// Inherit env + inject recipe env vars
	cmd.Env = os.Environ()
	for k, v := range cfg.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set process group so we can kill all children on Ctrl+C.
	// The actual SysProcAttr fields are platform-specific (see llama_cpp_unix.go).
	setSysProcAttr(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot pipe stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot pipe stderr: %w", err)
	}

	stats := &Stats{}
	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start llama-server: %w", err)
	}

	// Resolve port for display — default to 8080 if not set (Fix #5)
	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	fmt.Fprintf(os.Stderr, "\n\033[32m✅ llama-server started (PID %d)\033[0m\n", cmd.Process.Pid)
	fmt.Fprintf(os.Stderr, "\033[36m   Chat UI: http://127.0.0.1:%d\033[0m\n", port)
	fmt.Fprintf(os.Stderr, "\033[90m   Press Ctrl+C to stop\033[0m\n\n")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024) // PERF-05: 256 KB prevents ErrTooLong
		for scanner.Scan() {
			line := scanner.Text()
			if cfg.LogWriter != nil {
				fmt.Fprintln(cfg.LogWriter, line)
			} else if !cfg.Silent {
				fmt.Println(line)
			}
			parseLlamaStats(line, stats) // stdout only — SEC-00: single writer goroutine
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024) // PERF-05
		for scanner.Scan() {
			line := scanner.Text()
			if cfg.LogWriter != nil {
				fmt.Fprintln(cfg.LogWriter, line)
			} else if !cfg.Silent {
				fmt.Fprintln(os.Stderr, line)
			}
			// SEC-00: stderr goroutine does NOT call parseLlamaStats.
		}
	}()

	// Handle SIGINT/SIGTERM — kill the entire process group.
	// SEC-08: done channel guarantees the goroutine exits instead of leaking.
	// killProcessGroup is implemented in llama_cpp_unix.go / llama_cpp_windows.go.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			if cmd.Process != nil {
				killProcessGroup(cmd.Process)
			}
		case <-ctx.Done():
		case <-done:
		}
	}()

	wg.Wait()
	close(done) // SEC-08: unblock signal goroutine
	err = cmd.Wait()
	signal.Stop(sigCh)

	stats.Duration = time.Since(startTime)
	stats.Success = err == nil

	return stats, nil
}

// ─── Internal helpers ──────────────────────────────────────────────────────

// flagRe matches --long-flag and -s (short flag) tokens on option lines.
// P-12: Compiled once at package init, not per call.
var flagRe = regexp.MustCompile(`(?m)^\s+(-{1,2}[a-zA-Z][a-zA-Z0-9\-]*)`)

func parseLlamaFlags(helpText string) map[string]struct{} {
	flags := make(map[string]struct{})
	for _, m := range flagRe.FindAllStringSubmatch(helpText, -1) {
		flags[m[1]] = struct{}{}
	}
	// SEC-08 (regex): use package-level inlineFlagRe instead of compiling per call.
	for _, line := range strings.Split(helpText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			for _, m := range inlineFlagRe.FindAllString(trimmed, -1) {
				flags[m] = struct{}{}
			}
		}
	}
	return flags
}

// P-10: Hoist regexes to package level — compiled once, not per log line.
var (
	llamaGenRe    = regexp.MustCompile(`eval time\s*=.*?([\d.]+)\s*tokens per second`)
	llamaPromptRe = regexp.MustCompile(`prompt eval time\s*=.*?([\d.]+)\s*tokens per second`)
	// SEC-00 / PERF-16: Use (?i) case-insensitive flag instead of strings.ToUpper(line)
	// on every log line — avoids a full string allocation per line.
	llamaVRAMRe = regexp.MustCompile(`(?i)VRAM\s+USED\s*[=:]\s*([\d.]+)\s*(MB|MIB|GB|GIB)`)

	// SEC-08 (regex): hoisted from inside parseLlamaFlags to avoid per-call compilation.
	inlineFlagRe = regexp.MustCompile(`(-{1,2}[a-zA-Z][a-zA-Z0-9\-]*)`)
)

func parseLlamaStats(line string, s *Stats) {
	if m := llamaGenRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			s.TokensPerSecGeneration = val
		}
	}
	if m := llamaPromptRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			s.TokensPerSecPrefill = val
		}
	}
	// SEC-00: Use (?i) regex instead of strings.ToUpper — no per-line allocation.
	if m := llamaVRAMRe.FindStringSubmatch(line); len(m) > 1 {
		val, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			unit := strings.ToUpper(m[2])
			if unit == "GB" || unit == "GIB" {
				val *= 1024
			}
			if int64(val) > s.PeakVRAMMB {
				s.PeakVRAMMB = int64(val)
			}
		}
	}
}
