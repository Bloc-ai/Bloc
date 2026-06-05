// Package process provides a single, shared subprocess supervisor for all
// Bloc engine runtimes. It replaces the copy-pasted scanner+signal+wg pattern
// that existed identically in llama_cpp.go, vllm_docker.go, vllm_native.go,
// and sglang_docker.go.
//
// Responsibilities:
//   - Start an *exec.Cmd and fan its stdout/stderr to the log file (and TTY if !Silent)
//   - Call the engine-specific LogParser on every stdout line to extract metrics
//   - Handle SIGINT/SIGTERM gracefully with a process-group kill fallback
//   - Block until the process exits, then return a Stats summary
//
// Security properties:
//   - Accepts only a pre-built *exec.Cmd — no shell string evaluation
//   - All goroutines are guaranteed to exit (SEC-08: done channel, no leaks)
//   - Stats are protected by a mutex (SEC-00: concurrent stdout/stderr scanners)
//   - Scanner buffer is 256 KB (PERF-05: prevents bufio.ErrTooLong on long lines)
package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// ─── LogParser ────────────────────────────────────────────────────────────────

// Metrics holds performance data extracted from one engine log line.
// Zero values mean "not present in this line" and are skipped during merge.
type Metrics struct {
	// TokensPerSecGen is the generation throughput (tokens/s).
	TokensPerSecGen float64
	// TokensPerSecPrefill is the prompt-eval throughput (tokens/s).
	TokensPerSecPrefill float64
	// PeakVRAMMB is the peak VRAM used in MB (0 = not reported).
	PeakVRAMMB int64
}

// LogParser is implemented by each engine to extract Metrics from its specific
// log format. It is called once per stdout line (never per stderr line — avoids
// data races by keeping parsing to a single goroutine).
type LogParser interface {
	// ParseLine parses one log line and returns any metrics embedded in it.
	// Must be safe to call concurrently (though Supervisor only calls it from
	// the stdout goroutine). Return zero-value Metrics for lines without data.
	ParseLine(line string) Metrics
}

// ─── Config ───────────────────────────────────────────────────────────────────

// Config configures a single Supervisor run. All fields are optional except Cmd.
type Config struct {
	// Cmd is the fully constructed subprocess to run. Required.
	// The Supervisor calls cmd.Start() / cmd.Wait() — caller must not.
	Cmd *exec.Cmd

	// LogPath is the absolute path to the named log file produced by Phase 1
	// Fix-4. Both stdout and stderr are written here. May be empty (logs
	// go only to TTY if !Silent).
	LogPath string

	// LogWriter is an already-open file at LogPath. If non-nil, the Supervisor
	// writes to it directly instead of opening LogPath itself. This allows the
	// caller to hold the file open for the process lifetime (RACE-4 fix).
	LogWriter io.Writer

	// Parser extracts Metrics from stdout lines. nil = no metric parsing.
	Parser LogParser

	// Silent suppresses TTY output (stdout/stderr are only written to the log).
	// Set to true when the TUI is active so engine noise does not corrupt it.
	Silent bool

	// KillFunc is called when SIGINT/SIGTERM is received or ctx is cancelled.
	// It is responsible for cleanly stopping the subprocess (e.g. killing a
	// process group on Unix, or docker stop on Docker-based runtimes).
	// If nil, the Supervisor sends SIGTERM to cmd.Process then os.Process.Kill
	// after 5 seconds.
	KillFunc func()
}

// ─── Stats ────────────────────────────────────────────────────────────────────

// Stats is the result of a completed Supervisor run.
type Stats struct {
	mu                  sync.Mutex
	TokensPerSecGen     float64
	TokensPerSecPrefill float64
	PeakVRAMMB          int64
	Duration            time.Duration
	Success             bool
}

// merge atomically merges non-zero Metrics into Stats.
// Called from the stdout-scanning goroutine only — but mutex ensures
// concurrent Snapshot() calls see a consistent view.
func (s *Stats) merge(m Metrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.TokensPerSecGen > 0 {
		s.TokensPerSecGen = m.TokensPerSecGen
	}
	if m.TokensPerSecPrefill > 0 {
		s.TokensPerSecPrefill = m.TokensPerSecPrefill
	}
	if m.PeakVRAMMB > s.PeakVRAMMB {
		s.PeakVRAMMB = m.PeakVRAMMB
	}
}

// Snapshot returns a copy of the current stats values, safe to read outside
// the mutex. Mirrors runtime.Stats.Snapshot() for a compatible return shape.
func (s *Stats) Snapshot() (gen, prefill float64, peakVRAMMB int64, duration time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TokensPerSecGen, s.TokensPerSecPrefill, s.PeakVRAMMB, s.Duration, s.Success
}

// ─── Supervisor ───────────────────────────────────────────────────────────────

// Supervisor manages a single engine subprocess. Create one with New(), then
// call Run() once. A Supervisor is not reusable after Run() returns.
type Supervisor struct {
	cfg      Config
	killDone chan struct{} // closed when process exits cleanly; cancels force-kill timer
}

// New validates cfg and returns a Supervisor ready to Run().
// Returns an error if cfg.Cmd is nil.
func New(cfg Config) (*Supervisor, error) {
	if cfg.Cmd == nil {
		return nil, fmt.Errorf("process.Supervisor: cfg.Cmd must not be nil")
	}
	return &Supervisor{cfg: cfg, killDone: make(chan struct{})}, nil
}

// Run starts the subprocess, fans out stdout/stderr, waits for the process to
// exit (or ctx to be cancelled), then returns a Stats summary.
//
// Goroutine lifecycle (SEC-08 guarantee):
//   - stdout goroutine: exits when the pipe is closed (process exit or kill)
//   - stderr goroutine: same
//   - signal goroutine: exits via the done channel, which is closed after
//     wg.Wait() returns — guaranteed to exit regardless of signal receipt
func (sv *Supervisor) Run(ctx context.Context) (*Stats, error) {
	cfg := sv.cfg
	cmd := cfg.Cmd

	// Wire up stdout/stderr pipes before starting.
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

	// Determine the log writer. If the caller supplied an open file, use it
	// directly (RACE-4: caller must not close it until after Run returns).
	// Otherwise open LogPath if provided.
	var logWriter io.Writer
	if cfg.LogWriter != nil {
		logWriter = cfg.LogWriter
	} else if cfg.LogPath != "" {
		f, openErr := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "  ⚠  Cannot open log file %s: %v\n", cfg.LogPath, openErr)
		} else {
			logWriter = f
			defer f.Close()
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// ── Fan-out goroutines (PERF-05: 256 KB scanner buffer prevents ErrTooLong)

	var wg sync.WaitGroup
	wg.Add(2)

	// stdout goroutine — the ONLY goroutine that calls cfg.Parser.ParseLine().
	// SEC-00: keeping parse in one goroutine eliminates the data race between
	// concurrent writers that existed in the original copy-pasted code.
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			writeLine(logWriter, cfg.Silent, false, line)
			if cfg.Parser != nil {
				stats.merge(cfg.Parser.ParseLine(line))
			}
		}
	}()

	// stderr goroutine — writes to log and TTY, never parses metrics.
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			writeLine(logWriter, cfg.Silent, true, line)
		}
	}()

	// ── Signal handler (SEC-08: done channel guarantees goroutine exit)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	// PERF-17 (PH-8): Stop intercepting signals as soon as Run returns to close
	// the race condition where a signal arrives while cmd.Wait() is blocking.
	defer signal.Stop(sigCh)
	done := make(chan struct{})

	go func() {
		select {
		case <-sigCh:
			sv.kill(cmd)
		case <-ctx.Done():
			sv.kill(cmd)
		case <-done:
			// Process exited cleanly — nothing to do.
		}
	}()

	wg.Wait()
	close(done)     // SEC-08: unblock signal goroutine
	close(sv.killDone) // M-6/PH-7: unblock any pending force-kill timer goroutine
	runErr := cmd.Wait()

	stats.mu.Lock()
	stats.Duration = time.Since(startTime)
	stats.Success = runErr == nil
	stats.mu.Unlock()

	return stats, runErr
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// kill stops the subprocess using cfg.KillFunc if provided, otherwise sends
// SIGTERM and force-kills after 5 seconds using a select-cancellable goroutine.
// M-6/PH-7: The kill goroutine is now joined via a done channel so it cannot
// outlive the Supervisor.Run() call or call Kill() on a process that has
// already exited cleanly.
func (sv *Supervisor) kill(cmd *exec.Cmd) {
	if sv.cfg.KillFunc != nil {
		sv.cfg.KillFunc()
		return
	}
	if cmd.Process == nil {
		return
	}
	// Windows does not support SIGTERM; cmd.Process.Signal(syscall.SIGTERM) returns
	// "not supported" and does nothing. This causes Windows to always wait the full
	// 5-second timeout. We must fallback to Kill() immediately on Windows.
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
		return
	}
	// Graceful: SIGTERM
	_ = cmd.Process.Signal(syscall.SIGTERM)
	// Force: after 5 seconds if still alive. The goroutine listens on sv.killDone
	// so Supervisor.Run() can cancel it when the process exits cleanly.
	go func() {
		select {
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		case <-sv.killDone:
			// Process exited cleanly before the timeout — skip force-kill.
		}
	}()
}

// writeLine writes a log line to the log file and/or TTY depending on cfg.Silent.
// isStderr = true → writes to os.Stderr on TTY; false → os.Stdout.
func writeLine(w io.Writer, silent, isStderr bool, line string) {
	// PERF-25 (PL-5): Use io.WriteString instead of fmt.Fprintln for high-throughput log lines
	outLine := line + "\n"
	if w != nil {
		io.WriteString(w, outLine)
	}
	if !silent {
		if isStderr {
			io.WriteString(os.Stderr, outLine)
		} else {
			io.WriteString(os.Stdout, outLine)
		}
	}
}
