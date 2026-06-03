//go:build !windows

package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// defaultVLLMVersion is used when a recipe does not pin engine.version.
// Updated with each bloc release as the latest well-tested stable vLLM version.
const defaultVLLMVersion = "0.9.1"

// installedMeta is written to ~/.cache/bloc/runtimes/vllm/<version>/installed.json
// after a successful venv install so we can confirm the environment later.
type installedMeta struct {
	Version       string    `json:"version"`
	PythonVersion string    `json:"python_version"`
	InstalledAt   time.Time `json:"installed_at"`
}

// NativeVLLMRuntime implements Runtime for engine.name=vllm, runtime=native.
// It manages fully isolated, per-version Python virtual environments at
// ~/.cache/bloc/runtimes/vllm/<version>/venv/ so recipes pinning different
// vLLM versions never conflict.
type NativeVLLMRuntime struct {
	// version is the resolved vLLM version string (e.g. "0.9.1").
	// Set by Probe or OfferInstall; consumed by Run.
	version string

	// venvDir is the resolved absolute path to the versioned venv root.
	venvDir string
}

// resolveVersion determines the vLLM version to use.
// Priority: recipe pin → defaultVLLMVersion constant.
func resolveVLLMVersion(recipePinned string) string {
	if v := strings.TrimSpace(recipePinned); v != "" {
		return v
	}
	return defaultVLLMVersion
}

// venvPath returns the absolute path to the versioned venv root directory.
// ~/.cache/bloc/runtimes/vllm/<version>/venv
func venvPath(cacheDir, version string) string {
	return filepath.Join(cacheDir, "runtimes", "vllm", version, "venv")
}

// installedMetaPath returns the path to the installed.json marker file.
func installedMetaPath(cacheDir, version string) string {
	return filepath.Join(cacheDir, "runtimes", "vllm", version, "installed.json")
}

// pythonBin returns the path to the python3 binary inside the versioned venv.
func pythonBin(venv string) string {
	return filepath.Join(venv, "bin", "python3")
}

// Name returns the display name used in CLI step headers.
func (r *NativeVLLMRuntime) Name() string {
	if r.version != "" {
		return fmt.Sprintf("vLLM %s (native)", r.version)
	}
	return "vLLM (native)"
}

// Probe checks whether the versioned venv exists and that the correct vLLM
// version is importable inside it. It populates r.version and r.venvDir on success.
//
// For the native vLLM runtime there are no per-recipe "required flags" to probe
// (vLLM accepts arbitrary keyword args, not a fixed binary flag set), so the
// required map is ignored — the probe focuses purely on environment availability.
func (r *NativeVLLMRuntime) Probe(required map[string]struct{}) (*ProbeResult, error) {
	// Resolve version and venv path from env
	cacheDir, err := defaultCacheDir()
	if err != nil {
		return nil, fmt.Errorf("cannot locate cache dir: %w", err)
	}

	venv := venvPath(cacheDir, r.version)
	py := pythonBin(venv)

	if _, err := os.Stat(py); os.IsNotExist(err) {
		return nil, fmt.Errorf("vLLM %s venv not found at %s", r.version, venv)
	}

	// Verify vLLM imports cleanly and reports the expected version
	versionOut, err := exec.Command(py, "-c",
		"import vllm; print(vllm.__version__)").Output()
	if err != nil {
		return nil, fmt.Errorf("vLLM %s is not importable in venv %s: %w", r.version, venv, err)
	}

	reportedVersion := strings.TrimSpace(string(versionOut))
	// Loose match: recipe pins "0.9.0", vLLM may report "0.9.0.post1" — accept both
	if !strings.HasPrefix(reportedVersion, r.version) && reportedVersion != r.version {
		return nil, fmt.Errorf(
			"vLLM version mismatch: expected %s, venv has %s",
			r.version, reportedVersion,
		)
	}

	r.venvDir = venv
	return &ProbeResult{BinaryPath: py}, nil
}

// OfferInstall creates the versioned venv and installs vLLM==<version> inside it.
// It prints platform warnings (macOS = CPU-only), then runs:
//  1. python3 -m venv <venv>
//  2. <venv>/bin/pip install --upgrade pip
//  3. <venv>/bin/pip install vllm==<version>
//  4. Writes installed.json
//
// Returns true if vLLM is available after the call.
func (r *NativeVLLMRuntime) OfferInstall() bool {
	cacheDir, err := defaultCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  Cannot locate cache dir: %v\033[0m\n", err)
		return false
	}

	venv := venvPath(cacheDir, r.version)
	metaPath := installedMetaPath(cacheDir, r.version)

	// ── Platform warning ──────────────────────────────────────────────────────
	if runtime.GOOS == "darwin" {
		fmt.Println()
		fmt.Println("  \033[33m⚠  macOS detected:\033[0m vLLM runs on CPU on Apple Silicon.")
		fmt.Println("     GPU acceleration requires Linux + CUDA.")
		fmt.Println("     Performance will be significantly reduced.")
		fmt.Print("  Continue with CPU-only install? [y/N]: ")
		var ans string
		fmt.Scanln(&ans)
		if strings.ToLower(strings.TrimSpace(ans)) != "y" {
			fmt.Println("  Cancelled. Use 'engine: llama.cpp' for GPU-accelerated inference on macOS.")
			return false
		}
	}

	// Verify python3 is available on the host
	python3, err := exec.LookPath("python3")
	if err != nil {
		fmt.Fprintln(os.Stderr, "\033[31m✗  python3 not found in PATH — install Python 3.9+ first.\033[0m")
		fmt.Fprintln(os.Stderr, "   https://www.python.org/downloads/")
		return false
	}

	fmt.Printf("\n  Installing vLLM %s into: %s\n", r.version, venv)
	fmt.Println("  This may take several minutes (large CUDA dependencies)...")
	fmt.Println()

	if err := os.MkdirAll(filepath.Dir(venv), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  Cannot create runtimes dir: %v\033[0m\n", err)
		return false
	}

	// ── Step 1: Create venv ───────────────────────────────────────────────────
	fmt.Println("  [1/3] Creating Python virtual environment...")
	if err := runVerbose(python3, "-m", "venv", venv); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  venv creation failed: %v\033[0m\n", err)
		os.RemoveAll(venv)
		return false
	}

	pipBin := filepath.Join(venv, "bin", "pip")

	// ── Step 2: Upgrade pip ───────────────────────────────────────────────────
	fmt.Println("  [2/3] Upgrading pip...")
	if err := runVerbose(pipBin, "install", "--upgrade", "pip"); err != nil {
		fmt.Fprintf(os.Stderr, "\033[33m⚠  pip upgrade failed (non-fatal): %v\033[0m\n", err)
		// non-fatal — continue with older pip
	}

	// ── Step 3: Install vLLM ─────────────────────────────────────────────────
	fmt.Printf("  [3/3] Installing vllm==%s...\n", r.version)
	pkg := fmt.Sprintf("vllm==%s", r.version)
	if err := runVerbose(pipBin, "install", pkg); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  vLLM install failed: %v\033[0m\n", err)
		os.RemoveAll(venv)
		return false
	}

	// ── Write installed.json ──────────────────────────────────────────────────
	pyVersionOut, _ := exec.Command(python3, "--version").Output()
	meta := installedMeta{
		Version:       r.version,
		PythonVersion: strings.TrimPrefix(strings.TrimSpace(string(pyVersionOut)), "Python "),
		InstalledAt:   time.Now(),
	}
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(metaPath, data, 0644)
	}

	r.venvDir = venv
	fmt.Printf("\n  \033[32m✓\033[0m  vLLM %s installed at %s\n", r.version, venv)
	return true
}

// Run launches the vLLM OpenAI-compatible API server inside the versioned venv.
// Command: <venv>/bin/python3 -m vllm.entrypoints.openai.api_server
//
//	--model <modelPath>
//	<flags from BuildVLLMFlags>
//
// Streams stdout/stderr to terminal, parses vLLM-specific log lines for stats,
// and kills the process on SIGINT/SIGTERM or ctx cancel.
func (r *NativeVLLMRuntime) Run(ctx context.Context, cfg RunConfig) (*Stats, error) {
	py := pythonBin(r.venvDir)

	// Build full argument list
	allArgs := []string{"-m", "vllm.entrypoints.openai.api_server",
		"--model", cfg.ModelPath}
	allArgs = append(allArgs, cfg.Flags...)

	cmd := exec.CommandContext(ctx, py, allArgs...)

	// Inherit env + inject recipe env vars
	cmd.Env = os.Environ()
	for k, v := range cfg.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set process group so we can kill all children on Ctrl+C
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
		return nil, fmt.Errorf("failed to start vLLM: %w", err)
	}

	port := cfg.Port
	if port == 0 {
		port = 8000 // vLLM default
	}
	fmt.Fprintf(os.Stderr, "\n\033[32m✅ vLLM started (PID %d)\033[0m\n", cmd.Process.Pid)
	fmt.Fprintf(os.Stderr, "\033[36m   OpenAI API: http://127.0.0.1:%d/v1\033[0m\n", port)
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
			parseVLLMStats(line, stats) // stdout only (SEC-00: one goroutine writes stats)
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
			// SEC-00: stderr goroutine does NOT call parseVLLMStats to avoid the data race.
		}
	}()

	// SEC-08: done channel ensures the goroutine exits cleanly even if it
	// is still blocked on select when wg.Wait() returns.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			if cmd.Process != nil {
				syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) //nolint:errcheck
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

// ─── vLLM log stat parsing ────────────────────────────────────────────────────

// P-10: All regexes compiled once at package init.
var (
	// "Avg prompt throughput: 123.4 tokens/s"
	vllmPromptRe = regexp.MustCompile(`(?i)avg prompt throughput:\s*([\d.]+)\s*tokens/s`)
	// "Avg generation throughput: 56.7 tokens/s"
	vllmGenRe = regexp.MustCompile(`(?i)avg generation throughput:\s*([\d.]+)\s*tokens/s`)
	// "GPU KV cache usage: 45.2%"
	vllmKVRe = regexp.MustCompile(`(?i)gpu kv cache usage:\s*([\d.]+)%`)
)

// parseVLLMStats extracts performance metrics from a vLLM log line.
// vLLM logs these via its built-in stats logger approximately every 5 seconds.
func parseVLLMStats(line string, s *Stats) {
	if m := vllmPromptRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			s.TokensPerSecPrefill = val
		}
	}
	if m := vllmGenRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			s.TokensPerSecGeneration = val
		}
	}
	// Convert KV cache % to a rough VRAM MB estimate placeholder
	// (actual VRAM is not directly reported in the log line)
	if m := vllmKVRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			// Store as percentage × 100 as a proxy (e.g. 45.2% → 4520)
			// Phase 4 (Docker) can refine this with nvidia-smi output.
			proxy := int64(val * 100)
			if proxy > s.PeakVRAMMB {
				s.PeakVRAMMB = proxy
			}
		}
	}
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// runVerbose runs a command and streams its stdout+stderr to the terminal.
// Used during OfferInstall to let the user see pip progress.
func runVerbose(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// defaultCacheDir returns the platform cache directory for bloc
// (~/.cache/bloc on Linux/macOS). Duplicated here to avoid an import cycle
// with the config package.
func defaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "bloc"), nil
}
