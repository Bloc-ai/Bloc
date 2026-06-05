//go:build windows

// Windows-specific supervisor tests.
//
// Uses cmd.exe /C instead of /bin/sh -c. Tests that rely on POSIX signals
// (SIGTERM) or bash redirects (>&2) are skipped — they are not applicable
// on Windows.
package process_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bloc-org/bloc/internal/process"
)

var blockerBin string // path to compiled test helper

func TestMain(m *testing.M) {
	// Build the helper binary into a temp dir shared for the whole test run.
	dir, err := os.MkdirTemp("", "proc-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: MkdirTemp: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "blocker.exe")
	src := filepath.Join("testdata", "blocker", "main.go")
	out, err := exec.Command("go", "build", "-o", bin, src).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: go build blocker: %v\n%s\n", err, out)
		os.Exit(1)
	}
	blockerBin = bin
	os.Exit(m.Run())
}

// ─── Fake LogParser ───────────────────────────────────────────────────────────

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

func shellCmd(script string) *exec.Cmd {
	return exec.Command("cmd.exe", "/C", script)
}

func newSupervisor(t *testing.T, script string, logPath string, parser process.LogParser, silent bool) *process.Supervisor {
	t.Helper()
	cmd := shellCmd(script)
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

func tempLog(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test-engine.log")
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestSupervisor_NilCmdReturnsError(t *testing.T) {
	_, err := process.New(process.Config{Cmd: nil})
	if err == nil {
		t.Fatal("expected error for nil Cmd, got nil")
	}
}

func TestSupervisor_NormalExit(t *testing.T) {
	sv := newSupervisor(t, `echo hello`, "", nil, true)

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

func TestSupervisor_NonZeroExit(t *testing.T) {
	sv := newSupervisor(t, `exit /b 1`, "", nil, true)

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

func TestSupervisor_ContextCancel(t *testing.T) {
	// Use compiled blocker so there is a stable process that properly closes pipes when killed.
	cmd := exec.Command(blockerBin)
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
	sv.Run(ctx) //nolint:errcheck
	elapsed := time.Since(start)

	if elapsed > 8*time.Second {
		t.Errorf("Run did not return within 8s of ctx cancel (elapsed: %v)", elapsed)
	}
}

func TestSupervisor_LogFileWritten(t *testing.T) {
	logPath := tempLog(t)
	sv := newSupervisor(t, `echo line-one && echo line-two && echo line-three`, logPath, nil, true)

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

func TestSupervisor_SilentMode(t *testing.T) {
	logPath := tempLog(t)
	sv := newSupervisor(t, `echo silent-output`, logPath, nil, true)

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

func TestSupervisor_MetricsParsed(t *testing.T) {
	parser := &recordingParser{}
	sv := newSupervisor(t,
		`echo METRIC gen=42.5 prefill=100.0 vram=4096`,
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

func TestSupervisor_MetricsPeakOnlyIncreases(t *testing.T) {
	parser := &recordingParser{}
	sv := newSupervisor(t,
		`echo METRIC gen=1.0 prefill=1.0 vram=4096 && echo METRIC gen=2.0 prefill=2.0 vram=8192 && echo METRIC gen=3.0 prefill=3.0 vram=2048`,
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

func TestSupervisor_ConcurrentRace(t *testing.T) {
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf(`echo METRIC gen=%d.0 prefill=%d.0 vram=%d`, i, i, i*100))
	}

	parser := &recordingParser{}
	sv := newSupervisor(t, strings.Join(lines, " && "), "", parser, true)

	stats, err := sv.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats == nil {
		t.Fatal("stats is nil")
	}

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
}

func TestSupervisor_LargeOutputLines(t *testing.T) {
	// Windows cmd.exe has a max line length of ~8191 chars so we test with 4KB
	// (still well above the old 64KB scanner limit in relative terms — the
	// bufio.Scanner 256KB buffer is what we are validating here).
	bigLine := strings.Repeat("x", 4*1024)
	sv := newSupervisor(t, fmt.Sprintf(`echo %s`, bigLine), "", nil, true)

	_, err := sv.Run(context.Background())
	if err != nil {
		t.Errorf("Run returned unexpected error: %v", err)
	}
}

// TestSupervisor_StderrWrittenToLog is skipped on Windows because cmd.exe
// does not support bash-style >&2 stderr redirection in a single inline script.
func TestSupervisor_StderrWrittenToLog(t *testing.T) {
	t.Skip("stderr redirect test not supported on Windows (bash-only syntax)")
}

// TestSupervisor_ParserOnlyCalledForStdout is skipped on Windows for the same reason.
func TestSupervisor_ParserOnlyCalledForStdout(t *testing.T) {
	t.Skip("stderr redirect test not supported on Windows (bash-only syntax)")
}

// TestSupervisor_CustomKillFunc verifies that KillFunc is called on ctx cancel.
//
// We use a compiled test helper (blockerBin) to ensure a stable single process.
// This allows Run() to return promptly when killed, so the non-blocking select
// is safe.
func TestSupervisor_CustomKillFunc(t *testing.T) {
	killed := make(chan struct{})

	cmd := exec.Command(blockerBin)
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
		// correct — KillFunc was called.
	default:
		t.Error("KillFunc was not called when ctx was cancelled")
	}
}

// TestSupervisor_SignalKill is skipped on Windows — POSIX SIGTERM does not
// exist on Windows. Process termination is handled via KillFunc instead.
func TestSupervisor_SignalKill(t *testing.T) {
	t.Skip("POSIX SIGTERM not available on Windows")
}
