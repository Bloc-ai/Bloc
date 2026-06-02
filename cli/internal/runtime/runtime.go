package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/bloc-org/bloc/internal/recipe"
)

// Runtime is the common interface all execution engines must implement.
// Each engine (llama.cpp, native vLLM, Docker vLLM) provides its own
// implementation. cmd/run.go interacts only with this interface —
// no engine-specific logic leaks into the command layer.
type Runtime interface {
	// Name returns a human-readable engine label for CLI output.
	// e.g. "llama-server", "vLLM (native)", "vLLM (Docker)"
	Name() string

	// Probe checks whether the runtime's binary/environment is present
	// and compatible with the flags required by the recipe.
	// Returns a ProbeResult on success, or an error if the runtime is
	// wholly unavailable (e.g. llama-server not in PATH).
	Probe(required map[string]struct{}) (*ProbeResult, error)

	// OfferInstall prompts the user to install the missing runtime and
	// attempts the installation. Returns true if the runtime is available
	// after the call (whether newly installed or already present).
	OfferInstall() bool

	// Run launches the model server and blocks until it exits.
	// It is responsible for:
	//   - Streaming stdout/stderr to the terminal
	//   - Parsing engine-specific performance metrics into Stats
	//   - Cleanly stopping the process on SIGINT/SIGTERM or ctx cancel
	Run(ctx context.Context, cfg RunConfig) (*Stats, error)
}

// ProbeResult is returned by Runtime.Probe on a successful check.
type ProbeResult struct {
	// BinaryPath is the resolved absolute path to the engine binary
	// (or the Docker image tag for Docker-based runtimes).
	BinaryPath string

	// Missing contains flags required by the recipe but not supported
	// by the detected binary version. Empty slice = fully compatible.
	Missing []string
}

// RunConfig carries all parameters the runner needs to launch the server.
// It replaces the previous positional (modelPath, flags, envVars) signature
// and adds Port so each runtime can print the correct URL (Fix #5).
type RunConfig struct {
	// ModelPath is the absolute local path to the model file or directory.
	ModelPath string

	// Flags is the ordered list of engine-specific CLI flags built from
	// the recipe's engine_config (e.g. ["-c", "4096", "-ngl", "99"]).
	Flags []string

	// EnvVars are additional environment variables to inject into the
	// subprocess (from recipe.pre_run.env).
	EnvVars map[string]string

	// Port is the resolved server port. Used for the "Server started at
	// http://127.0.0.1:<port>" message so it is always accurate (Fix #5).
	Port int

	// Recipe is the full parsed recipe. Used by runtimes that need metadata
	// beyond model path and flags (e.g. DockerVLLMRuntime reads Metadata.Name
	// to build the container slug). May be nil for runtimes that don't need it.
	Recipe *recipe.Recipe
}

// Stats collects runtime performance metrics captured from engine log output.
// Identical to the old runner.Stats — promoted here so all runtimes share one type.
//
// SEC-00: Fields are protected by a mutex because two goroutines (stdout and
// stderr scanners) call Update() concurrently while the server is running.
type Stats struct {
	mu                     sync.Mutex // protects all fields below
	TokensPerSecGeneration float64
	TokensPerSecPrefill    float64
	PeakVRAMMB             int64
	Duration               time.Duration
	Success                bool
}

// Update atomically writes parsed metric values into Stats.
// Pass 0 for any value that was not found in the log line — 0 values are skipped.
// SEC-00: Must be used by all log-parsing goroutines instead of direct field writes.
func (s *Stats) Update(prefill, gen float64, peakVRAMMB int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prefill > 0 {
		s.TokensPerSecPrefill = prefill
	}
	if gen > 0 {
		s.TokensPerSecGeneration = gen
	}
	if peakVRAMMB > s.PeakVRAMMB {
		s.PeakVRAMMB = peakVRAMMB
	}
}

// Snapshot returns a copy of the current stats values, safe to read outside the mutex.
func (s *Stats) Snapshot() (gen, prefill float64, peakVRAMMB int64, duration time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TokensPerSecGeneration, s.TokensPerSecPrefill, s.PeakVRAMMB, s.Duration, s.Success
}
