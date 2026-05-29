//go:build windows

package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Run launches llama-server with the given args and model path.
// Windows variant: no Setpgid (unsupported); uses cmd.Process.Kill() on Ctrl+C.
func Run(ctx context.Context, modelPath string, flags []string, envVars map[string]string) (*Stats, error) {
	// Prepend model path
	allArgs := append([]string{"-m", modelPath}, flags...)

	cmd := exec.CommandContext(ctx, "llama-server", allArgs...)

	// Inherit env + add recipe env vars
	cmd.Env = os.Environ()
	for k, v := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Note: Windows does not support Setpgid / process groups the same way.
	// Child processes are terminated via cmd.Process.Kill().

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

	fmt.Fprintf(os.Stderr, "\nllama-server started (PID %d)\n", cmd.Process.Pid)
	fmt.Fprintf(os.Stderr, "   Chat UI: http://127.0.0.1:8080\n")
	fmt.Fprintf(os.Stderr, "   Press Ctrl+C to stop\n\n")

	// Goroutine: pipe stdout to terminal + parse stats
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			parseStats(line, stats)
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintln(os.Stderr, scanner.Text())
		}
	}()

	// Handle SIGINT/SIGTERM — kill the process on Windows
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			if cmd.Process != nil {
				cmd.Process.Kill() //nolint:errcheck
			}
		case <-ctx.Done():
		}
	}()

	wg.Wait()
	err = cmd.Wait()
	signal.Stop(sigCh)

	stats.Duration = time.Since(startTime)
	stats.Success = err == nil

	return stats, nil
}
