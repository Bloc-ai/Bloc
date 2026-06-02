//go:build !windows

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

// SGLangDockerRuntime implements Runtime for engine.name=sglang, runtime=docker.
// It wraps the Docker CLI (exec.Command("docker", ...)) — no Docker Go SDK
// dependency — to keep the bloc binary lean and avoid SDK version churn.
//
// SGLang is Docker-only in Bloc v1. Native mode is explicitly unsupported
// because SGLang is primarily a multi-GPU CUDA workload that requires a
// managed Python environment beyond the scope of a local dev tool.
//
// Container lifecycle is guaranteed clean through four independent paths (F-20):
//  1. --rm flag: Docker daemon auto-removes on clean exit
//  2. SIGINT/SIGTERM handler: docker stop + docker rm -f
//  3. ctx cancel: same as SIGINT path
//  4. defer + recover: docker rm -f even on panic
type SGLangDockerRuntime struct {
	// image is the Docker image tag to run (e.g. "lmsysorg/sglang:v0.5.12.post1").
	// Set from recipe.Engine.Image by Resolve().
	image string

	// containerName is the sanitized, unique container name registered at Run()
	// start and used by all cleanup paths. Format: bloc-<slug>-<8hex>
	containerName string
}

// Name returns the display label used in CLI step headers.
func (r *SGLangDockerRuntime) Name() string {
	if r.image != "" {
		return fmt.Sprintf("SGLang Docker (%s)", r.image)
	}
	return "SGLang (Docker)"
}

// Probe runs three checks in order:
//  1. `docker info` — verifies daemon is running
//  2. `docker pull <image>` — pulls if not cached locally, streams progress
//  3. CUDA smoke test (Linux only, non-fatal): verifies NVIDIA Container Toolkit
//
// On macOS, a clear GPU-limitation warning is printed: Docker cannot access
// the GPU on macOS, so SGLang (a multi-GPU CUDA workload) will not function.
// The required map is not used (Docker pulls the image; no flag capability check).
func (r *SGLangDockerRuntime) Probe(required map[string]struct{}) (*ProbeResult, error) {
	// ── macOS warning (no GPU passthrough) ───────────────────────────────────
	if runtime.GOOS == "darwin" {
		fmt.Println()
		fmt.Println("  \033[33m⚠  macOS + Docker warning:\033[0m Docker cannot access the CUDA GPU on macOS.")
		fmt.Println("     SGLang requires NVIDIA GPU passthrough to function.")
		fmt.Println("     This recipe must be run on a Linux host with NVIDIA GPUs.")
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
				"  Example: image: lmsysorg/sglang:v0.5.12.post1",
		)
	}

	fmt.Printf("  Pulling image %s (may take a while for first run)...\n", r.image)
	// Use exec.CommandContext so Ctrl+C / context cancellation propagates.
	// 2-hour timeout is generous for any realistic image size.
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
			// Non-fatal: user may have a different GPU or toolkit setup
			fmt.Println()
			fmt.Println("  \033[33m⚠  NVIDIA Container Toolkit smoke test failed.\033[0m")
			fmt.Println("     GPU passthrough may not be available.")
			fmt.Println("     SGLang requires the NVIDIA Container Toolkit for GPU access:")
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

// OfferInstall for SGLangDockerRuntime just points to Docker Desktop — we
// cannot install Docker programmatically on behalf of the user.
func (r *SGLangDockerRuntime) OfferInstall() bool {
	fmt.Println()
	fmt.Println("  Docker is required to use the SGLang Docker runtime.")
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
//	  python3 -m sglang.launch_server
//	    --model-path /bloc-models/<modelDir>
//	    --host 0.0.0.0
//	    --port <containerPort>
//	    <sgLangFlags from BuildSGLangFlags>
func (r *SGLangDockerRuntime) Run(ctx context.Context, cfg RunConfig) (stats *Stats, retErr error) {
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
		hostPort = 8000 // SGLang default
	}
	containerPort := hostPort // same port inside container for simplicity

	// ── Shared memory size (required for multi-GPU tensor parallel in SGLang) ──
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

	// Inject recipe env vars (-e KEY=VALUE).
	// CUDA_VISIBLE_DEVICES (from SGLangCUDAVisibleDevices) is injected here
	// by run.go before constructing RunConfig, so it arrives in cfg.EnvVars.
	for k, v := range cfg.EnvVars {
		dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Image
	dockerArgs = append(dockerArgs, r.image)

	// SGLang command inside container
	dockerArgs = append(dockerArgs,
		"python3", "-m", "sglang.launch_server",
		"--model-path", containerModelPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", containerPort),
	)
	dockerArgs = append(dockerArgs, cfg.Flags...)

	// ── F-20: Register cleanup before starting — panic-safe via defer/recover ─
	defer func() {
		if rec := recover(); rec != nil {
			retErr = fmt.Errorf("panic in SGLangDockerRuntime.Run: %v", rec)
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

	// ── Stream logs — parse SGLang stats from output ──────────────────────────
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024) // PERF-05: 256 KB prevents ErrTooLong
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			parseSGLangStats(line, stats) // stdout only (SEC-00: one goroutine writes stats)
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024) // PERF-05
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line)
			// SEC-00: stderr goroutine does NOT call parseSGLangStats to avoid the data race.
			// SGLang emits stats to stdout; errors/warnings go to stderr.
		}
	}()

	// ── F-20 path 2+3: SIGINT/SIGTERM + ctx cancel → graceful docker stop ─────
	// SEC-08: done channel ensures the goroutine always exits (no goroutine leak
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
func (r *SGLangDockerRuntime) gracefulStop(dockerPath string) {
	if r.containerName == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "\n\033[33m  Stopping container %s...\033[0m\n", r.containerName)
	stopCmd := exec.Command(dockerPath, "stop", "--time", "10", r.containerName)
	_ = stopCmd.Run()
	r.forceRemoveContainer(dockerPath)
}

// forceRemoveContainer issues `docker rm -f <name>` — the last-resort cleanup.
// Safe to call multiple times (docker rm -f is idempotent: exits 0 if container
// already gone). Called from both gracefulStop and the defer in Run().
func (r *SGLangDockerRuntime) forceRemoveContainer(dockerPath string) {
	if r.containerName == "" {
		return
	}
	rmCmd := exec.Command(dockerPath, "rm", "-f", r.containerName)
	_ = rmCmd.Run() // intentionally ignore error — container may already be gone
}

// ─── SGLang log stat parser ───────────────────────────────────────────────────

// SGLang emits periodic stats as structured key=value pairs on a single log
// line, approximately every 5 seconds:
//
//	throughput_output_token_per_s=47.3 throughput_input_token_per_s=912.1 ...
var (
	// sglangGenRe matches output token throughput from SGLang's periodic log line.
	sglangGenRe = regexp.MustCompile(`throughput_output_token_per_s=([\d.]+)`)

	// sglangPrefillRe matches input (prefill) token throughput.
	sglangPrefillRe = regexp.MustCompile(`throughput_input_token_per_s=([\d.]+)`)

	// sglangVRAMRe matches GPU memory usage lines emitted at startup.
	// SGLang prints e.g. "Memory pool end size: 81920.00 MB"
	// or via nvidia-smi integration: "GPU memory: 48.0 GB"
	sglangVRAMRe = regexp.MustCompile(`(?i)(?:memory pool end size|gpu\s+mem(?:ory)?)\s*[=:]\s*([\d.]+)\s*(GB|MB|MiB|GiB)`)
)

// parseSGLangStats extracts performance metrics from an SGLang log line.
// SGLang logs throughput stats via its built-in metrics logger approximately
// every 5 seconds as a key=value line.
//
// SEC-00: Must only be called from the single stdout goroutine. Never call
// this from the stderr goroutine to avoid data races on Stats fields.
// Use s.Update() (mutex-protected) to write, never direct field assignment.
func parseSGLangStats(line string, s *Stats) {
	var prefill, gen float64
	var vram int64

	if m := sglangGenRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			gen = val
		}
	}
	if m := sglangPrefillRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			prefill = val
		}
	}
	if m := sglangVRAMRe.FindStringSubmatch(line); len(m) > 2 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			unit := strings.ToLower(m[2])
			switch unit {
			case "gb", "gib":
				vram = int64(val * 1024)
			default: // MB, MiB
				vram = int64(val)
			}
		}
	}

	if gen != 0 || prefill != 0 || vram != 0 {
		s.Update(prefill, gen, vram)
	}
}
