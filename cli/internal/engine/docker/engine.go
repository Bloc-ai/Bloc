// Package docker provides a unified DockerEngine that replaces both
// DockerVLLMRuntime and SGLangDockerRuntime from the legacy runtime package.
//
// The two old runtimes were 94% identical — they differed only in:
//   - The in-container entry command (vllm serve vs python3 -m sglang.launch_server)
//   - The log parser (vLLM stats format vs SGLang stats format)
//   - The display name
//
// DockerEngine captures those differences through injected fields (EntryCmd,
// Parser, displayName) so the shared infrastructure — docker info/pull/run args,
// container name generation, graceful stop, GPU passthrough, volume mount — lives
// in exactly one place.
//
// Container lifecycle is guaranteed clean through four independent paths (F-20):
//  1. --rm flag: Docker daemon auto-removes on clean exit
//  2. SIGINT/SIGTERM handler: docker stop + docker rm -f  (via KillFunc)
//  3. ctx cancel: same as SIGINT path
//  4. defer + recover in NewSupervisor caller: docker rm -f even on panic
package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── DockerEngine ─────────────────────────────────────────────────────────────

// dockerEnvKeyRe is the allowlist for environment variable keys passed to
// `docker run -e`. Compiled once at package init (not per-launch).
var dockerEnvKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DockerEngine implements engine.Engine for any Docker-based inference backend.
//
// Construct via the engine-specific New() functions in the vllm/ and sglang/
// packages — they set EntryCmd, FlagBuilder, Parser, and displayName before
// returning a DockerEngine to the registry.
type DockerEngine struct {
	// Image is the pinned Docker image tag from the recipe.
	// e.g. "vllm/vllm-openai:v0.9.0" or "lmsysorg/sglang:v0.5.12.post1"
	Image string

	// EntryCmd returns the in-container command tokens that precede the model
	// path argument. Called at NewSupervisor() time with the resolved container
	// model path.
	//
	// vLLM: func(m) []string{"vllm", "serve", m, "--port", ...}
	// SGLang: func(m) []string{"python3", "-m", "sglang.launch_server", "--model-path", m, ...}
	EntryCmd func(containerModelPath string, port int) []string

	// FlagBuilder converts EngineConfig fields into engine-specific CLI flags.
	// It is injected by vllm/ and sglang/ packages. The flags are appended
	// after EntryCmd in the docker run argument list.
	FlagBuilder func(cfg recipe.EngineConfig) []string

	// Parser extracts structured metrics from the engine's log lines.
	// Injected by the engine-specific constructor.
	Parser process.LogParser

	// DisplayName is what Name() returns (e.g. "vLLM Docker (image:tag)").
	// Set by the constructor.
	DisplayName string

	// capsMu / caps: cached Capabilities() result.
	capsMu sync.Mutex
	caps   *engine.CapabilitySet

	// containerName is set at NewSupervisor() time and used by all cleanup
	// paths. Format: bloc-<slug>-<8hex>
	// Guarded by containerMu.
	containerMu   sync.Mutex
	containerName string
}

// Name returns a human-readable label for CLI output.
// e.g. "vLLM Docker (vllm/vllm-openai:v0.9.0)"
func (e *DockerEngine) Name() string {
	return e.DisplayName
}

// Capabilities probes Docker and the image to confirm the engine is available.
//
// For Docker engines the image is pinned in the recipe, so flag churn is not
// a risk. Rather than running --help inside a throwaway container (slow, requires
// pulling), we return a pre-built CapabilitySet that marks all features as
// supported. This is correct because:
//   - vLLM / SGLang recipes declare exactly which flags they need, and
//   - those flags are stable within a pinned image version.
//
// The probe does run three real checks:
//  1. `docker info` — verifies the daemon is running.
//  2. `docker pull <image>` — ensures the image is locally cached.
//  3. CUDA smoke test (Linux only, non-fatal).
//
// The result is cached via sync.Once — the pull runs at most once per run.
func (e *DockerEngine) Capabilities(ctx context.Context) (*engine.CapabilitySet, error) {
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
func (e *DockerEngine) probe(ctx context.Context) (*engine.CapabilitySet, error) {
	// ── macOS warning ─────────────────────────────────────────────────────────
	if runtime.GOOS == "darwin" {
		fmt.Println()
		fmt.Println("  \033[33m⚠  macOS + Docker warning:\033[0m Docker cannot access the GPU on macOS.")
		fmt.Printf("     %s requires GPU passthrough to run at full speed.\n", e.DisplayName)
		fmt.Println("     For GPU-accelerated inference on Apple Silicon, use:")
		fmt.Println("       engine: llama.cpp")
		fmt.Println()
	}

	// ── Step 1: docker info ───────────────────────────────────────────────────
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf(
			"docker not found in PATH\n" +
				"  Install Docker Desktop: https://www.docker.com/products/docker-desktop/",
		)
	}

	// PERF-14 (PH-5): Use a 10s timeout so a stuck Docker socket doesn't hang the CLI.
	infoCtx, cancelInfo := context.WithTimeout(ctx, 10*time.Second)
	defer cancelInfo()
	infoOut, err := exec.CommandContext(infoCtx, dockerPath, "info", "--format", "{{.ServerVersion}}").Output()
	if err != nil {
		return nil, fmt.Errorf(
			"Docker daemon is not running (docker info failed)\n" +
				"  Start Docker Desktop and try again.",
		)
	}
	dockerVersion := strings.TrimSpace(string(infoOut))

	// ── Step 2: image validation ───────────────────────────────────────────────
	if e.Image == "" {
		return nil, fmt.Errorf(
			"recipe is missing engine.image — required for Docker runtime\n" +
				"  Example: image: vllm/vllm-openai:v0.9.0",
		)
	}

	// ── Step 3: docker pull ───────────────────────────────────────────────────
	fmt.Printf("  Pulling image %s (may take a while for first run)...\n", e.Image)
	pullCtx, pullCancel := context.WithTimeout(ctx, 2*time.Hour)
	defer pullCancel()
	pullCmd := exec.CommandContext(pullCtx, dockerPath, "pull", e.Image)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		return nil, fmt.Errorf("docker pull %s failed: %w", e.Image, err)
	}

	// ── Step 4: CUDA smoke test (Linux only, non-fatal) ───────────────────────
	// PM-8: Gate behind host nvidia-smi to avoid pulling ~100MB nvidia/cuda
	// image on machines that have no NVIDIA GPU (e.g. CI, CPU-only servers).
	if runtime.GOOS == "linux" {
		// Check if nvidia-smi is available on the host first.
		nvidiaSMI, lookErr := exec.LookPath("nvidia-smi")
		if lookErr != nil {
			// No nvidia-smi — skip GPU verification silently.
			fmt.Println("  \033[33m⚠  nvidia-smi not found — skipping GPU verification.\033[0m")
			fmt.Println("     If you have an NVIDIA GPU, install the NVIDIA driver first.")
		} else {
			// nvidia-smi exists: run a quick local check (no Docker, no image pull).
			smiCtx, smiCancel := context.WithTimeout(ctx, 5*time.Second)
			defer smiCancel()
			if err := exec.CommandContext(smiCtx, nvidiaSMI, "--query", "--display=MEMORY").Run(); err != nil {
				fmt.Println()
				fmt.Println("  \033[33m⚠  nvidia-smi check failed — GPU may not be accessible.\033[0m")
				fmt.Println("     The NVIDIA Container Toolkit may still work; proceeding.")
				fmt.Println()
			} else {
				// GPU confirmed via host nvidia-smi — now validate Docker can see it.
				smokeArgs := []string{
					"run", "--rm", "--gpus", "all",
					"--entrypoint", "nvidia-smi",
					"nvidia/cuda:12.0-base-ubuntu20.04",
				}
				smokeCtx, smokeCancel := context.WithTimeout(ctx, 15*time.Second)
				defer smokeCancel()
				if err := exec.CommandContext(smokeCtx, dockerPath, smokeArgs...).Run(); err != nil {
					fmt.Println()
					fmt.Println("  \033[33m⚠  NVIDIA Container Toolkit smoke test failed.\033[0m")
					fmt.Println("     GPU passthrough may not be available.")
					fmt.Println("     Install guide:")
					fmt.Println("       https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html")
					fmt.Println("     Continuing without GPU passthrough verification.")
					fmt.Println()
				} else {
					fmt.Println("  \033[32m✓\033[0m  NVIDIA Container Toolkit verified")
				}
			}
		}
	}

	// Return a pre-built stub CapabilitySet. Docker images are pinned so every
	// flag the recipe declares is assumed supported.
	caps := engine.BuildCapabilities(
		fmt.Sprintf("docker/%s", e.Image),
		dockerVersion,
		nil, // nil rawFlags → stub (all features marked supported)
	)
	return caps, nil
}

// BuildArgs delegates to the injected FlagBuilder to convert EngineConfig into
// CLI flags. caps is intentionally unused here (the stub always says "supported")
// but is kept in the signature to satisfy the engine.Engine interface.
func (e *DockerEngine) BuildArgs(_ *engine.CapabilitySet, cfg recipe.EngineConfig) ([]string, error) {
	if e.FlagBuilder == nil {
		return nil, fmt.Errorf("docker engine %q: FlagBuilder is nil", e.DisplayName)
	}
	return e.FlagBuilder(cfg), nil
}

// NewSupervisor constructs and returns a process.Supervisor that runs
// `docker run ... <image> <entryCmd> <engineFlags>`.
//
// Command shape:
//
//	docker run --rm
//	  --name bloc-<slug>-<8hex>
//	  [--gpus all]                 (Linux only)
//	  --ipc host
//	  --shm-size 64g
//	  -v <cacheDir>/repos:/bloc-models:ro
//	  -p <hostPort>:<containerPort>
//	  [-e KEY=VALUE ...]
//	  <image>
//	  <entryCmd(containerModelPath, port)>
//	  <cfg.Flags...>
func (e *DockerEngine) NewSupervisor(cfg engine.LaunchConfig) (*process.Supervisor, error) {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker not found in PATH: %w", err)
	}

	// ── Container name: sanitized, unique (F-20) ──────────────────────────────
	recipeName := ""
	if cfg.Recipe != nil {
		recipeName = cfg.Recipe.Metadata.Name
	}
	slug := SanitizeContainerSlug(recipeName)
	containerName := fmt.Sprintf("bloc-%s-%s", slug, RandomHex(4))
	e.containerMu.Lock()
	e.containerName = containerName
	e.containerMu.Unlock()

	// Volume mount: host cache → /bloc-models:ro (F-16)
	cacheDir, err := engine.DefaultCacheDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve cache dir: %w", err)
	}
	// LOW-1: Use filepath.Join instead of string concatenation.
	reposMountSrc := filepath.Join(cacheDir, "repos")
	// HIGH-3: Check MkdirAll error; use 0700 consistent with downloader SEC-03 policy.
	if err := os.MkdirAll(reposMountSrc, 0700); err != nil {
		return nil, fmt.Errorf("cannot create models mount dir: %w", err)
	}

	// Map host model path → container model path
	containerModelPath := "/bloc-models/" + strings.TrimPrefix(cfg.ModelPath, reposMountSrc+"/")

	// ── Port ──────────────────────────────────────────────────────────────────
	hostPort := cfg.Port
	if hostPort == 0 {
		hostPort = 8000
	}
	containerPort := hostPort // same port inside container for simplicity

	// ── Build docker run args (no shell — each token is a separate element) ───
	dockerArgs := []string{
		"run", "--rm",
		"--name", containerName,
		"--ipc", "host",
		"--shm-size", "64g",
		"-v", fmt.Sprintf("%s:/bloc-models:ro", reposMountSrc),
		"-p", fmt.Sprintf("%d:%d", hostPort, containerPort),
	}

	// GPU passthrough — Linux only
	if runtime.GOOS == "linux" {
		dockerArgs = append(dockerArgs, "--gpus", "all")
	}

	// Inject recipe env vars
	// SEC-12 (M-3): Validate Docker env keys. An attacker controlling env keys
	// could inject arbitrary Docker run flags (e.g. key="-v /:/host" value="").
	for k, v := range cfg.EnvVars {
		if !dockerEnvKeyRe.MatchString(k) {
			return nil, fmt.Errorf("security: pre_run.env key %q is invalid", k)
		}
		dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Image
	dockerArgs = append(dockerArgs, e.Image)

	// In-container command: entryCmd builds all tokens including model path and
	// port so each engine (vLLM vs SGLang) can structure them differently.
	if e.EntryCmd != nil {
		dockerArgs = append(dockerArgs, e.EntryCmd(containerModelPath, containerPort)...)
	}

	// Engine-specific flags from BuildArgs
	dockerArgs = append(dockerArgs, cfg.Flags...)

	// ── Build cmd ─────────────────────────────────────────────────────────────
	cmd := exec.Command(dockerPath, dockerArgs...)

	fmt.Fprintf(os.Stderr, "\n\033[32m✅ Container starting: %s\033[0m\n", containerName)
	fmt.Fprintf(os.Stderr, "\033[36m   OpenAI API: http://127.0.0.1:%d/v1\033[0m\n", hostPort)
	fmt.Fprintf(os.Stderr, "\033[90m   Press Ctrl+C to stop and remove container\033[0m\n\n")

	return process.New(process.Config{
		Cmd:       cmd,
		LogWriter: cfg.LogWriter,
		Parser:    e.Parser,
		Silent:    cfg.Silent,
		// F-20 path 2+3: graceful docker stop on signal/ctx cancel.
		KillFunc: func() { e.gracefulStop(dockerPath) },
	})
}

// OfferInstall points to Docker Desktop — we cannot install Docker on behalf of the user.
func (e *DockerEngine) OfferInstall() bool {
	fmt.Println()
	fmt.Println("  Docker is required to use this runtime.")
	fmt.Println("  Install Docker Desktop: \033[36mhttps://www.docker.com/products/docker-desktop/\033[0m")
	fmt.Println()
	fmt.Println("  After installing Docker, re-run your command.")
	return false
}

// ─── Container lifecycle helpers (F-20) ──────────────────────────────────────

// gracefulStop issues `docker stop <name>` (10 s grace) then `docker rm -f`.
// Called from the KillFunc goroutine — must not block the main goroutine.
func (e *DockerEngine) gracefulStop(dockerPath string) {
	e.containerMu.Lock()
	name := e.containerName
	e.containerMu.Unlock()
	if name == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "\n\033[33m  Stopping container %s...\033[0m\n", name)
	// PERF-15 (PH-6): Use a 20s timeout so a stuck Docker socket doesn't hang cleanup.
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelStop()
	_ = exec.CommandContext(stopCtx, dockerPath, "stop", "--time", "10", name).Run()
	e.forceRemoveContainer(dockerPath)
}

// forceRemoveContainer issues `docker rm -f <name>`. Safe to call multiple
// times — docker rm -f exits 0 when the container is already gone.
func (e *DockerEngine) forceRemoveContainer(dockerPath string) {
	e.containerMu.Lock()
	name := e.containerName
	e.containerMu.Unlock()
	if name == "" {
		return
	}
	// PERF-15 (PH-6): Use a 20s timeout so a stuck Docker socket doesn't hang cleanup.
	rmCtx, cancelRm := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelRm()
	_ = exec.CommandContext(rmCtx, dockerPath, "rm", "-f", name).Run()
}

// ─── Shared helpers (exported for use by vllm/ and sglang/ packages) ─────────

// SanitizeContainerSlug converts a recipe name into a Docker-safe lowercase
// alphanumeric+hyphen string (max 40 chars). Docker container names must match
// [a-zA-Z0-9][a-zA-Z0-9_.-]* — we restrict further to [a-z0-9-] only to
// prevent shell injection through the container name (F-20).
func SanitizeContainerSlug(name string) string {
	// PERF-21 (PM-4): Dedup hyphens in a single pass to avoid O(N^2) strings.ReplaceAll.
	var b strings.Builder
	lastWasDash := true // initialize to true to skip leading dashes
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastWasDash = false
		} else if !lastWasDash {
			b.WriteRune('-')
			lastWasDash = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	if slug == "" {
		slug = "model"
	}
	return slug
}

// RandomHex returns n random bytes encoded as a 2n-character hex string.
func RandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// IsInterruptExit returns true for exit codes that indicate a user-initiated
// stop (SIGINT → 130, SIGTERM → 143) rather than a process crash.
func IsInterruptExit(err error) bool {
	if err == nil {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "exit status 130") || strings.Contains(s, "exit status 143")
}

