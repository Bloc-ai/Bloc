package vllm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/engine/docker"
	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/recipe"
)

const defaultVLLMVersion = "0.9.1"

type installedMeta struct {
	Version       string    `json:"version"`
	PythonVersion string    `json:"python_version"`
	InstalledAt   time.Time `json:"installed_at"`
}

type NativeVLLMEngine struct {
	version string
	venvDir string

	// PH-2: Cache Capabilities() result so the Python subprocess is only
	// spawned once per engine instance, matching the pattern in llamacpp.Engine
	// and docker.DockerEngine.
	capsMu sync.Mutex
	caps   *engine.CapabilitySet
}

func resolveVersion(recipePinned string) string {
	if v := strings.TrimSpace(recipePinned); v != "" {
		return v
	}
	return defaultVLLMVersion
}

func venvPath(cacheDir, version string) string {
	return filepath.Join(cacheDir, "runtimes", "vllm", version, "venv")
}

func installedMetaPath(cacheDir, version string) string {
	return filepath.Join(cacheDir, "runtimes", "vllm", version, "installed.json")
}

func pythonBin(venv string) string {
	return filepath.Join(venv, "bin", "python3")
}


func runVerbose(ctx context.Context, name string, args ...string) error {
	// PERF-12 (PH-3): Use exec.CommandContext so that long-running operations
	// like pip install or venv creation can be cancelled.
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// New creates a new vLLM engine, either Docker or Native based on the recipe.
func New(r *recipe.Recipe) (engine.Engine, error) {
	if r.Engine.Runtime == "docker" {
		if r.Engine.Image == "" {
			return nil, fmt.Errorf("engine.image is required for docker runtime")
		}
		entryCmd := func(modelPath string, port int) []string {
			return []string{"vllm", "serve", modelPath, "--port", strconv.Itoa(port)}
		}
		return &docker.DockerEngine{
			Image:       r.Engine.Image,
			EntryCmd:    entryCmd,
			FlagBuilder: BuildFlags,
			Parser:      &VLLMLogParser{},
			DisplayName: fmt.Sprintf("vLLM Docker (%s)", r.Engine.Image),
		}, nil
	}

	return &NativeVLLMEngine{version: resolveVersion(r.Engine.Version)}, nil
}

func (e *NativeVLLMEngine) Name() string {
	if e.version != "" {
		return fmt.Sprintf("vLLM %s (native)", e.version)
	}
	return "vLLM (native)"
}

func (e *NativeVLLMEngine) Capabilities(ctx context.Context) (*engine.CapabilitySet, error) {
	e.capsMu.Lock()
	defer e.capsMu.Unlock()
	if e.caps != nil {
		return e.caps, nil
	}
	caps, err := e.probe(ctx)
	if err != nil {
		return nil, err
	}
	e.caps = caps
	return e.caps, nil
}

// probe is the internal implementation of Capabilities.
func (e *NativeVLLMEngine) probe(ctx context.Context) (*engine.CapabilitySet, error) {
	cacheDir, err := engine.DefaultCacheDir()
	if err != nil {
		return nil, fmt.Errorf("cannot locate cache dir: %w", err)
	}

	venv := venvPath(cacheDir, e.version)
	py := pythonBin(venv)

	if _, err := os.Stat(py); os.IsNotExist(err) {
		return nil, fmt.Errorf("vLLM %s venv not found at %s", e.version, venv)
	}

	versionOut, err := exec.CommandContext(ctx, py, "-c", "import vllm; print(vllm.__version__)").Output()
	if err != nil {
		return nil, fmt.Errorf("vLLM %s is not importable in venv %s: %w", e.version, venv, err)
	}

	reportedVersion := strings.TrimSpace(string(versionOut))
	// SEC-18 (L-6): Use exact match or allow only local version identifiers (+...).
	// Prevents "0.9" from matching "0.9.10".
	if reportedVersion != e.version && !strings.HasPrefix(reportedVersion, e.version+"+") {
		return nil, fmt.Errorf("vLLM version mismatch: expected %s, venv has %s", e.version, reportedVersion)
	}

	e.venvDir = venv
	return engine.BuildCapabilities("vllm", reportedVersion, nil), nil
}

func (e *NativeVLLMEngine) BuildArgs(caps *engine.CapabilitySet, cfg recipe.EngineConfig) ([]string, error) {
	return BuildFlags(cfg), nil
}

func (e *NativeVLLMEngine) OfferInstall() bool {
	cacheDir, err := engine.DefaultCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  Cannot locate cache dir: %v\033[0m\n", err)
		return false
	}

	venv := venvPath(cacheDir, e.version)
	metaPath := installedMetaPath(cacheDir, e.version)

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

	python3, err := exec.LookPath("python3")
	if err != nil {
		fmt.Fprintln(os.Stderr, "\033[31m✗  python3 not found in PATH — install Python 3.9+ first.\033[0m")
		fmt.Fprintln(os.Stderr, "   https://www.python.org/downloads/")
		return false
	}

	fmt.Printf("\n  Installing vLLM %s into: %s\n", e.version, venv)
	fmt.Println("  This may take several minutes (large CUDA dependencies)...")
	fmt.Println()

	if err := os.MkdirAll(filepath.Dir(venv), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  Cannot create runtimes dir: %v\033[0m\n", err)
		return false
	}

	fmt.Println("  [1/3] Creating Python virtual environment...")
	// Installation can take minutes; use a long-lived cancellable context.
	installCtx, cancelInstall := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancelInstall()

	if err := runVerbose(installCtx, python3, "-m", "venv", venv); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  venv creation failed: %v\033[0m\n", err)
		os.RemoveAll(venv)
		return false
	}

	pipBin := filepath.Join(venv, "bin", "pip")

	fmt.Println("  [2/3] Upgrading pip...")
	if err := runVerbose(installCtx, pipBin, "install", "--upgrade", "pip"); err != nil {
		fmt.Fprintf(os.Stderr, "\033[33m⚠  pip upgrade failed (non-fatal): %v\033[0m\n", err)
	}

	fmt.Printf("  [3/3] Installing vllm==%s...\n", e.version)
	// SEC-04 (H-1): Defense-in-depth check. Ensure version string matches semver
	// before passing to pip install, preventing package name injection if the
	// validation in recipe.go is ever bypassed or relaxed.
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?([\-+][a-zA-Z0-9.\-+]{1,50})?$`).MatchString(e.version) {
		fmt.Fprintf(os.Stderr, "\033[31m✗  Security: vLLM version %q is invalid\033[0m\n", e.version)
		os.RemoveAll(venv)
		return false
	}
	pkg := fmt.Sprintf("vllm==%s", e.version)
	if err := runVerbose(installCtx, pipBin, "install", pkg); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗  vLLM install failed: %v\033[0m\n", err)
		os.RemoveAll(venv)
		return false
	}

	// PERF-13 (PH-4): Add 5s timeout to python3 --version to prevent hanging.
	verCtx, cancelVer := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelVer()
	pyVersionOut, _ := exec.CommandContext(verCtx, python3, "--version").Output()
	meta := installedMeta{
		Version:       e.version,
		PythonVersion: strings.TrimPrefix(strings.TrimSpace(string(pyVersionOut)), "Python "),
		InstalledAt:   time.Now(),
	}
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(metaPath, data, 0644)
	}

	e.venvDir = venv
	fmt.Printf("\n  \033[32m✓\033[0m  vLLM %s installed at %s\n", e.version, venv)
	return true
}

func (e *NativeVLLMEngine) NewSupervisor(cfg engine.LaunchConfig) (*process.Supervisor, error) {
	vllmExec := filepath.Join(e.venvDir, "bin", "vllm")

	// SEC-11 (H-2): Ensure ModelPath is contained within the expected cache directory.
	// Prevents a path traversal in the downloader from serving an arbitrary local file.
	cacheDir, err := engine.DefaultCacheDir()
	if err != nil {
		return nil, fmt.Errorf("cannot locate cache dir: %w", err)
	}
	// HIGH-1: Check Abs errors explicitly — a failure produces empty strings that
	// make HasPrefix trivially true, silently bypassing the containment check.
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("security: cannot resolve cache dir: %w", err)
	}
	absModel, err := filepath.Abs(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("security: cannot resolve model path: %w", err)
	}
	if !strings.HasPrefix(absModel, absCache+string(filepath.Separator)) {
		return nil, fmt.Errorf("security: ModelPath %q is outside cache directory", cfg.ModelPath)
	}

	// Modern vLLM entrypoint: `vllm serve <model_path>`
	// Replaces deprecated: `python -m vllm.entrypoints.openai.api_server --model <model_path>`
	allArgs := []string{"serve", cfg.ModelPath}
	allArgs = append(allArgs, cfg.Flags...)

	cmd := exec.Command(vllmExec, allArgs...)

	// M-2: Inherit env minus dangerous loader vars, then inject recipe env vars.
	cmd.Env = engine.SafeEnviron()
	for k, v := range cfg.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	setSysProcAttr(cmd)

	port := cfg.Port
	if port == 0 {
		port = 8000
	}

	return process.New(process.Config{
		Cmd:       cmd,
		LogWriter: cfg.LogWriter,
		Parser:    &VLLMLogParser{},
		Silent:    cfg.Silent,
		KillFunc: func() {
			killProcessGroup(cmd)
		},
	})
}
