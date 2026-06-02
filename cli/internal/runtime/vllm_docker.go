//go:build !windows

package runtime

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DockerVLLMRuntime implements Runtime for engine.name=vllm, runtime=docker.
// It wraps the Docker CLI (exec.Command("docker", ...)) — no Docker Go SDK
// dependency — to keep the bloc binary lean and avoid SDK version churn.
//
// Container lifecycle is guaranteed clean through four independent paths (F-20):
//  1. --rm flag: Docker daemon auto-removes on clean exit
//  2. SIGINT/SIGTERM handler: docker stop + docker rm -f
//  3. ctx cancel: same as SIGINT path
//  4. defer + recover: docker rm -f even on panic
type DockerVLLMRuntime struct {
	// image is the Docker image tag to run (e.g. "vllm/vllm-openai:v0.9.0").
	// Set from recipe.Engine.Image by Resolve().
	image string

	// containerName is the sanitized, unique container name registered at Run()
	// start and used by all cleanup paths. Format: bloc-<slug>-<8hex>
	containerName string
}

// Name returns the display label used in CLI step headers.
func (r *DockerVLLMRuntime) Name() string {
	if r.image != "" {
		return fmt.Sprintf("vLLM Docker (%s)", r.image)
	}
	return "vLLM (Docker)"
}

// Probe runs three checks in order:
//  1. `docker info` — verifies daemon is running
//  2. `docker pull <image>` — pulls if not cached locally, streams progress
//  3. CUDA smoke test (Linux+CUDA only): 1-second nvidia-smi via Docker to
//     verify NVIDIA Container Toolkit is wired up (warning only, not a hard fail)
//
// On macOS with runtime=docker, a clear GPU-limitation warning is printed.
// The required map is not used (Docker pulls the image, no flag capability check).
func (r *DockerVLLMRuntime) Probe(required map[string]struct{}) (*ProbeResult, error) {
	// ── macOS warning (F-16 variant: no GPU passthrough) ─────────────────────
	if runtime.GOOS == "darwin" {
		fmt.Println()
		fmt.Println("  \033[33m⚠  macOS + Docker warning:\033[0m Docker cannot access the Metal/M-series GPU.")
		fmt.Println("     The model will run on CPU only — performance will be very slow.")
		fmt.Println("     For GPU-accelerated inference on Apple Silicon, use:")
		fmt.Println("       engine: llama.cpp  (native Metal via llama.cpp)")
		fmt.Println("       runtime: native    (native vLLM — limited on macOS)")
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

	infoOut, err := exec.Command(dockerPath, "info", "--format", "{{.ServerVersion}}").Output()
	if err != nil {
		return nil, fmt.Errorf(
			"Docker daemon is not running (docker info failed)\n" +
				"  Start Docker Desktop and try again.",
		)
	}
	dockerVersion := strings.TrimSpace(string(infoOut))

	// ── Step 2: docker pull ───────────────────────────────────────────────────
	if r.image == "" {
		return nil, fmt.Errorf(
			"recipe is missing engine.image — required for Docker runtime\n" +
				"  Example: image: vllm/vllm-openai:v0.9.0",
		)
	}

	fmt.Printf("  Pulling image %s (may take a while for first run)...\n", r.image)
	// PERF-NEW-F: Use exec.CommandContext so Ctrl+C / context cancellation
	// propagates to the docker pull subprocess. Without this, a slow pull
	// (10+ GB image) blocks Go's side indefinitely with no cancellation path.
	// We use a 2-hour timeout — generous for any realistic image size — so the
	// process always terminates eventually even if something goes wrong.
	pullCtx, pullCancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer pullCancel()
	pullCmd := exec.CommandContext(pullCtx, dockerPath, "pull", r.image)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		return nil, fmt.Errorf("docker pull %s failed: %w", r.image, err)
	}

	// ── Step 3: CUDA smoke test (Linux only, non-fatal) ───────────────────────
	if runtime.GOOS == "linux" {
		smokeArgs := []string{
			"run", "--rm", "--gpus", "all",
			"--entrypoint", "nvidia-smi",
			"nvidia/cuda:12.0-base-ubuntu20.04",
		}
		smokeCtx, smokeCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer smokeCancel()
		smokeCmd := exec.CommandContext(smokeCtx, dockerPath, smokeArgs...)
		if err := smokeCmd.Run(); err != nil {
			// Non-fatal: user may want CPU mode or a different GPU setup
			fmt.Println()
			fmt.Println("  \033[33m⚠  NVIDIA Container Toolkit smoke test failed.\033[0m")
			fmt.Println("     GPU passthrough may not be available.")
			fmt.Println("     If you have an NVIDIA GPU, install the toolkit:")
			fmt.Println("       https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html")
			fmt.Println("     Continuing without GPU passthrough verification.")
			fmt.Println()
		} else {
			fmt.Println("  \033[32m✓\033[0m  NVIDIA Container Toolkit verified")
		}
	}

	return &ProbeResult{
		BinaryPath: fmt.Sprintf("docker %s (engine: %s)", dockerVersion, r.image),
	}, nil
}

// OfferInstall for DockerVLLMRuntime just points to Docker Desktop — we can't
// install Docker programmatically on behalf of the user.
func (r *DockerVLLMRuntime) OfferInstall() bool {
	fmt.Println()
	fmt.Println("  Docker is required to use the Docker vLLM runtime.")
	fmt.Println("  Install Docker Desktop: \033[36mhttps://www.docker.com/products/docker-desktop/\033[0m")
	fmt.Println()
	fmt.Println("  After installing Docker, re-run your run command.")
	return false
}

// Run constructs the docker run command programmatically (no shell interpolation),
// launches the container, streams logs, and guarantees cleanup through four
// independent paths (F-20). It blocks until the container exits.
//
// Command shape:
//
//	docker run --rm
//	  --name bloc-<slug>-<8hex>
//	  [--gpus all]                 (Linux only)
//	  --ipc host
//	  --shm-size <shmSize>
//	  -v <cacheDir>/repos:/bloc-models:ro
//	  -p <hostPort>:<containerPort>
//	  [-e KEY=VALUE ...]
//	  <image>
//	  vllm serve /bloc-models/<modelDir> <vllmFlags>
func (r *DockerVLLMRuntime) Run(ctx context.Context, cfg RunConfig) (stats *Stats, retErr error) {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker not found in PATH: %w", err)
	}

	// ── Container name: sanitized, unique, registered globally (F-20) ─────────
	slug := sanitizeContainerSlug(cfg.Recipe.Metadata.Name)
	r.containerName = fmt.Sprintf("bloc-%s-%s", slug, randomHex(4))

	// ── Host cache dir for volume mount (F-16: read-only, fixed path) ─────────
	cacheDir, err := defaultCacheDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve cache dir: %w", err)
	}
	reposMountSrc := cacheDir + "/repos"
	// Ensure the repos dir exists so Docker doesn't reject the mount
	_ = os.MkdirAll(reposMountSrc, 0755)

	// ── Model path inside container ───────────────────────────────────────────
	// ModelPath on host is .../repos/org--model/main — strip the cache prefix
	// to get the relative path, then prefix with /bloc-models inside container.
	containerModelPath := "/bloc-models/" + strings.TrimPrefix(cfg.ModelPath, reposMountSrc+"/")

	// ── Port (F-17: already validated 1024-65535 at recipe parse) ─────────────
	hostPort := cfg.Port
	if hostPort == 0 {
		hostPort = 8000 // vLLM default
	}
	containerPort := hostPort // same port inside container for simplicity

	// ── Shared memory size (default 64g for multi-GPU tensor parallel) ─────────
	shmSize := "64g"

	// ── Build docker run args (no shell — each token is a separate element) ───
	dockerArgs := []string{
		"run", "--rm",
		"--name", r.containerName,
		"--ipc", "host",
		"--shm-size", shmSize,
		"-v", fmt.Sprintf("%s:/bloc-models:ro", reposMountSrc), // F-16: ro mount
		"-p", fmt.Sprintf("%d:%d", hostPort, containerPort),
	}

	// GPU passthrough — Linux only (F-20: macOS warning already printed in Probe)
	if runtime.GOOS == "linux" {
		dockerArgs = append(dockerArgs, "--gpus", "all")
	}

	// Inject recipe env vars (-e KEY=VALUE)
	for k, v := range cfg.EnvVars {
		dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Image
	dockerArgs = append(dockerArgs, r.image)

	// vLLM command inside container
	dockerArgs = append(dockerArgs,
		"vllm", "serve", containerModelPath,
		"--port", fmt.Sprintf("%d", containerPort),
	)
	dockerArgs = append(dockerArgs, cfg.Flags...)

	// ── F-20: Register cleanup before starting — panic-safe via defer/recover ─
	defer func() {
		if rec := recover(); rec != nil {
			retErr = fmt.Errorf("panic in DockerVLLMRuntime.Run: %v", rec)
		}
		r.forceRemoveContainer(dockerPath)
	}()

	// ── Start the container ───────────────────────────────────────────────────
	cmd := exec.CommandContext(ctx, dockerPath, dockerArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot pipe stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("cannot pipe stderr: %w", err)
	}

	stats = &Stats{}
	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("docker run failed to start: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n\033[32m✅ Container started: %s\033[0m\n", r.containerName)
	fmt.Fprintf(os.Stderr, "\033[36m   OpenAI API: http://127.0.0.1:%d/v1\033[0m\n", hostPort)
	fmt.Fprintf(os.Stderr, "\033[90m   Press Ctrl+C to stop and remove container\033[0m\n\n")

	// ── Stream logs — parse vLLM stats from output ────────────────────────────
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024) // PERF-05: 256 KB prevents ErrTooLong
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			parseVLLMStats(line, stats) // stdout only (SEC-00: one goroutine writes stats)
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024) // PERF-05
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line)
			// SEC-00: stderr goroutine does NOT call parseVLLMStats to avoid the data race.
			// vLLM emits stats to stdout; errors/warnings go to stderr.
		}
	}()

	// ── F-20 path 2+3: SIGINT/SIGTERM + ctx cancel → graceful docker stop ─────
	// SEC-08: done channel ensures the goroutine always exits (previously leaked
	// if it was blocked on select when wg.Wait() returned).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			r.gracefulStop(dockerPath)
		case <-ctx.Done():
			r.gracefulStop(dockerPath)
		case <-done:
		}
	}()

	wg.Wait()
	close(done) // SEC-08: unblock the signal goroutine if still waiting
	err = cmd.Wait()
	signal.Stop(sigCh)

	stats.Duration = time.Since(startTime)
	// Exit code 0 or 130 (Ctrl+C) both count as a clean user-initiated stop
	stats.Success = err == nil || isInterruptExit(err)

	return stats, nil
}

// ─── Container lifecycle helpers (F-20) ──────────────────────────────────────

// gracefulStop issues `docker stop <name>` (10s grace) then `docker rm -f`.
// Called from the signal handler goroutine — must not block the main goroutine.
func (r *DockerVLLMRuntime) gracefulStop(dockerPath string) {
	if r.containerName == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "\n\033[33m  Stopping container %s...\033[0m\n", r.containerName)
	// docker stop with a 10-second timeout before SIGKILL
	stopCmd := exec.Command(dockerPath, "stop", "--time", "10", r.containerName)
	_ = stopCmd.Run()
	// Ensure removal even if stop timed out
	r.forceRemoveContainer(dockerPath)
}

// forceRemoveContainer issues `docker rm -f <name>` — the last-resort cleanup.
// Safe to call multiple times (docker rm -f is idempotent: exits 0 if container
// already gone). Called from both gracefulStop and the defer in Run().
func (r *DockerVLLMRuntime) forceRemoveContainer(dockerPath string) {
	if r.containerName == "" {
		return
	}
	rmCmd := exec.Command(dockerPath, "rm", "-f", r.containerName)
	_ = rmCmd.Run() // intentionally ignore error — container may already be gone
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// sanitizeContainerSlug converts a recipe name into a Docker-safe lowercase
// alphanumeric+hyphen string (max 40 chars). Docker container names must match
// [a-zA-Z0-9][a-zA-Z0-9_.-]* — we restrict further to [a-z0-9-] only.
// F-20: No shell-injectable characters can appear in the container name.
func sanitizeContainerSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	slug := b.String()
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Collapse consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if len(slug) > 40 {
		slug = slug[:40]
	}
	if slug == "" {
		slug = "model"
	}
	return slug
}

// randomHex returns n random hex bytes as a string (2n characters).
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// isInterruptExit returns true for exit codes that indicate a user-initiated
// stop (SIGINT exit code 130, SIGTERM exit code 143) rather than a crash.
func isInterruptExit(err error) bool {
	if err == nil {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "exit status 130") || strings.Contains(s, "exit status 143")
}
