//go:build !windows

// Tests for process.Supervisor.
//
// All tests use real subprocesses (exec.Command("/bin/sh", ...)) — no mocks.
// This guarantees we test the actual goroutine lifecycle, pipe behavior, and
// signal delivery, not a fake stand-in. Tests run with -race by default in CI.
//
// Test strategy:
//   - Small, fast shell one-liners as the subprocess (no external dependencies)
//   - Each test verifies one precise behavior, named exactly after that behavior
//   - All tests must complete in under 10 seconds each
package process_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/bloc-org/bloc/internal/process"
)

// ─── Fake LogParser ───────────────────────────────────────────────────────────

// recordingParser records every line it sees and extracts Metrics from lines
// that start with "METRIC gen=<f> prefill=<f> vram=<i>".
type recordingParser struct {
	mu    sync.Mutex
	lines []string
}

func (p *recordingParser) ParseLine(line string) process.Metrics {
	p.mu.Lock()
	p.lines = append(p.lines, line)
	p.mu.Unlock()

	var m process.Metrics
	fmt.Sscanf(line, "METRIC gen=%f prefill=%f vram=%d",
		&m.TokensPerSecGen, &m.TokensPerSecPrefill, &m.PeakVRAMMB)
	return m
}

func (p *recordingParser) Lines() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]string, len(p.lines))
	copy(cp, p.lines)
	return cp
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// newSupervisor creates a Supervisor for a shell snippet and optional parser.
func newSupervisor(t *testing.T, shellScript string, logPath string, parser process.LogParser, silent bool) *process.Supervisor {
	t.Helper()
	cmd := shellCmd(shellScript)
	sv, err := process.New(process.Config{
		Cmd:     cmd,
		LogPath: logPath,
		Parser:  parser,
		Silent:  silent,
	})
	if err != nil {
		t.Fatalf("process.New: %v", err)
	}
	return sv
}

// shellCmd builds a /bin/sh -c command (tests are unix-only via build tag).
func shellCmd(script string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", script)
}

// tempLog returns a temp log path inside t.TempDir().
func tempLog(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test-engine.log")
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestSupervisor_NilCmdReturnsError verifies New rejects a nil Cmd.
func TestSupervisor_NilCmdReturnsError(t *testing.T) {
	_, err := process.New(process.Config{Cmd: nil})
	if err == nil {
		t.Fatal("expected error for nil Cmd, got nil")
	}
}

// TestSupervisor_NormalExit verifies that a process that exits 0 returns
// stats with Success=true and a non-zero Duration.
func TestSupervisor_NormalExit(t *testing.T) {
	sv := newSupervisor(t, `echo hello; sleep 0`, "", nil, true)

	stats, err := sv.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stats == nil {
		t.Fatal("Run returned nil stats")
	}
	if !stats.Success {
		t.Error("expected stats.Success = true for exit-0 process")
	}
	if stats.Duration <= 0 {
		t.Errorf("expected positive Duration, got %v", stats.Duration)
	}
}

// TestSupervisor_NonZeroExit verifies that a process that exits non-zero
// returns an error AND sets Success=false.
func TestSupervisor_NonZeroExit(t *testing.T) {
	sv := newSupervisor(t, `exit 1`, "", nil, true)

	stats, err := sv.Run(context.Background())
	if err == nil {
		t.Fatal("expected non-nil error for exit-1 process")
	}
	if stats == nil {
		t.Fatal("stats must be non-nil even on error")
	}
	if stats.Success {
		t.Error("expected stats.Success = false for exit-1 process")
	}
}

// TestSupervisor_ContextCancel verifies that cancelling the context kills the
// process and Run returns promptly (within 8 seconds for a 60-second sleeper).
func TestSupervisor_ContextCancel(t *testing.T) {
	// Use direct exec to avoid /bin/sh child process leaks
	cmd := exec.Command("sleep", "60")
	sv, err := process.New(process.Config{
		Cmd:    cmd,
		Silent: true,
	})
	if err != nil {
		t.Fatalf("process.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	sv.Run(ctx) //nolint:errcheck — we only care about timing here
	elapsed := time.Since(start)

	if elapsed > 8*time.Second {
		t.Errorf("Run did not return within 8s of ctx cancel (elapsed: %v)", elapsed)
	}
}

// TestSupervisor_LogFileWritten verifies that stdout lines appear in the log file.
func TestSupervisor_LogFileWritten(t *testing.T) {
	logPath := tempLog(t)
	sv := newSupervisor(t, `echo line-one; echo line-two; echo line-three`, logPath, nil, true)

	if _, err := sv.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("cannot read log file: %v", err)
	}
	content := string(data)
	for _, want := range []string{"line-one", "line-two", "line-three"} {
		if !strings.Contains(content, want) {
			t.Errorf("log file missing %q\nlog content:\n%s", want, content)
		}
	}
}

// TestSupervisor_StderrWrittenToLog verifies that stderr output also appears
// in the log file (both streams must be captured).
func TestSupervisor_StderrWrittenToLog(t *testing.T) {
	logPath := tempLog(t)
	sv := newSupervisor(t, `echo stdout-line; echo stderr-line >&2`, logPath, nil, true)

	if _, err := sv.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("cannot read log file: %v", err)
	}
	content := string(data)
	for _, want := range []string{"stdout-line", "stderr-line"} {
		if !strings.Contains(content, want) {
			t.Errorf("log file missing %q\nlog:\n%s", want, content)
		}
	}
}

// TestSupervisor_SilentMode verifies that Silent=true means no output is written
// to the real stdout/stderr. We can't capture os.Stdout/os.Stderr directly, but
// we can verify the log file still gets written (the core contract).
func TestSupervisor_SilentMode(t *testing.T) {
	logPath := tempLog(t)
	sv := newSupervisor(t, `echo silent-output`, logPath, nil, true /* silent */)

	if _, err := sv.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("cannot read log file: %v", err)
	}
	if !strings.Contains(string(data), "silent-output") {
		t.Errorf("log file should still contain output in silent mode\nlog:\n%s", data)
	}
}

// TestSupervisor_MetricsParsed verifies that stdout lines with embedded metrics
// are passed to LogParser.ParseLine and the results accumulate in Stats.
func TestSupervisor_MetricsParsed(t *testing.T) {
	parser := &recordingParser{}
	sv := newSupervisor(t,
		`echo "METRIC gen=42.5 prefill=100.0 vram=4096"`,
		"", parser, true)

	stats, err := sv.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	gen, prefill, peakVRAM, _, _ := stats.Snapshot()
	if gen != 42.5 {
		t.Errorf("TokensPerSecGen = %v, want 42.5", gen)
	}
	if prefill != 100.0 {
		t.Errorf("TokensPerSecPrefill = %v, want 100.0", prefill)
	}
	if peakVRAM != 4096 {
		t.Errorf("PeakVRAMMB = %v, want 4096", peakVRAM)
	}
}

// TestSupervisor_MetricsPeakOnlyIncreases verifies that Stats only updates
// PeakVRAMMB when the new value is larger (mirrors runtime.Stats semantics).
func TestSupervisor_MetricsPeakOnlyIncreases(t *testing.T) {
	parser := &recordingParser{}
	sv := newSupervisor(t,
		// Emit three lines: 4096, 8192, 2048 — peak should be 8192
		`printf "METRIC gen=1.0 prefill=1.0 vram=4096\nMETRIC gen=2.0 prefill=2.0 vram=8192\nMETRIC gen=3.0 prefill=3.0 vram=2048\n"`,
		"", parser, true)

	stats, err := sv.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	_, _, peakVRAM, _, _ := stats.Snapshot()
	if peakVRAM != 8192 {
		t.Errorf("PeakVRAMMB = %v, want 8192 (peak should only increase)", peakVRAM)
	}
}

// TestSupervisor_ParserOnlyCalledForStdout verifies that stderr lines are
// NOT passed to LogParser.ParseLine (SEC-00: single-goroutine parse contract).
// We do this by emitting a metric-shaped line only on stderr and verifying
// it does NOT appear in the parsed lines list.
func TestSupervisor_ParserOnlyCalledForStdout(t *testing.T) {
	parser := &recordingParser{}
	sv := newSupervisor(t,
		// stdout has no metric; stderr has one
		`echo "stdout-normal"; echo "METRIC gen=99.0 prefill=99.0 vram=9999" >&2`,
		"", parser, true)

	stats, err := sv.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	gen, _, _, _, _ := stats.Snapshot()
	if gen == 99.0 {
		t.Error("stderr metric was incorrectly parsed — parser must only see stdout")
	}

	// The stdout line should have been passed to the parser
	lines := parser.Lines()
	found := false
	for _, l := range lines {
		if l == "stdout-normal" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("stdout line not seen by parser; got lines: %v", lines)
	}
}

// TestSupervisor_CustomKillFunc verifies that Config.KillFunc is called when
// ctx is cancelled instead of the default SIGTERM logic.
//
// We use exec.Command("sleep","60") directly — not shellCmd — to avoid the
// two-process hierarchy (/bin/sh + sleep). With a shell wrapper, SIGKILL only
// kills the shell; sleep keeps the stdout pipe open and Run() never returns.
// With a direct exec there is exactly one process: killing it closes all pipes
// and Run() returns promptly, making the non-blocking select safe.
func TestSupervisor_CustomKillFunc(t *testing.T) {
	killed := make(chan struct{})

	cmd := exec.Command("sleep", "60")
	sv, err := process.New(process.Config{
		Cmd:    cmd,
		Silent: true,
		KillFunc: func() {
			close(killed)
			if cmd.Process != nil {
				cmd.Process.Kill() //nolint:errcheck
			}
		},
	})
	if err != nil {
		t.Fatalf("process.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Run() blocks until KillFunc kills the process and the pipes drain.
	// By the time Run() returns, KillFunc is guaranteed to have been called.
	sv.Run(ctx) //nolint:errcheck

	select {
	case <-killed:
		// KillFunc was called — correct.
	default:
		t.Error("KillFunc was not called when ctx was cancelled")
	}
}

// TestSupervisor_ConcurrentRace is designed to be run with -race.
// It verifies that concurrent Snapshot() calls on the returned Stats are race-free.
// Phase 1: Run the supervisor to completion so stats is populated.
// Phase 2: Call Snapshot() from many goroutines concurrently.
func TestSupervisor_ConcurrentRace(t *testing.T) {
	// Emit 20 metric lines.
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf(`echo "METRIC gen=%d.0 prefill=%d.0 vram=%d"`, i, i, i*100))
	}

	parser := &recordingParser{}
	sv := newSupervisor(t, strings.Join(lines, "; "), "", parser, true)

	// Run to completion first (no goroutine race on the stats pointer itself).
	stats, err := sv.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats == nil {
		t.Fatal("stats is nil")
	}

	// Now call Snapshot concurrently — the Stats mutex must protect all reads.
	const readers = 50
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			stats.Snapshot()
		}()
	}
	wg.Wait()
	// Reaching here without the race detector firing proves SEC-00 holds.
}


// TestSupervisor_LargeOutputLines verifies that lines larger than the default
// bufio.Scanner 64KB buffer do not cause ErrTooLong (PERF-05: 256KB buffer).
func TestSupervisor_LargeOutputLines(t *testing.T) {
	// Generate a line of exactly 200 KB (> default 64 KB scanner limit).
	// To avoid OS argument length limits, we pass the data via stdin to cat
	// instead of using a shell command line argument.
	bigLine := strings.Repeat("x", 200*1024) + "\n"
	cmd := exec.Command("cat")
	// Set stdin to our big string so it's read by cat and written to stdout
	cmd.Stdin = strings.NewReader(bigLine)
	
	sv, err := process.New(process.Config{
		Cmd:    cmd,
		Silent: true,
	})
	if err != nil {
		t.Fatalf("process.New: %v", err)
	}

	_, err = sv.Run(context.Background())
	if err != nil {
		t.Errorf("Run returned error on 200KB line (expected no ErrTooLong): %v", err)
	}
}

// TestSupervisor_SignalKill verifies that sending SIGTERM to a long-running
// process causes Run to return promptly.
func TestSupervisor_SignalKill(t *testing.T) {
	// Direct exec to avoid shell child process leak
	cmd := exec.Command("sleep", "60")
	sv, err := process.New(process.Config{
		Cmd:    cmd,
		Silent: true,
		KillFunc: func() {
			if cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck
			}
		},
	})
	if err != nil {
		t.Fatalf("process.New: %v", err)
	}

	// Start in background, then trigger kill via context cancel.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	sv.Run(ctx) //nolint:errcheck
	if time.Since(start) > 7*time.Second {
		t.Error("Run did not return within 7s after SIGTERM")
	}
}
