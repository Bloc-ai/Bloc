//go:build !windows

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
// It streams stdout/stderr to the terminal, captures stats, and
// kills the entire process group on SIGINT/SIGTERM.
// Returns stats collected from the run.
func Run(ctx context.Context, modelPath string, flags []string, envVars map[string]string) (*Stats, error) {
	// Prepend model path
	allArgs := append([]string{"-m", modelPath}, flags...)

	cmd := exec.CommandContext(ctx, "llama-server", allArgs...)

	// Inherit env + add recipe env vars
	cmd.Env = os.Environ()
	for k, v := range envVars {
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
		return nil, fmt.Errorf("failed to start llama-server: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n\033[32m✅ llama-server started (PID %d)\033[0m\n", cmd.Process.Pid)
	fmt.Fprintf(os.Stderr, "\033[36m   Chat UI: http://127.0.0.1:8080\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[90m   Press Ctrl+C to stop\033[0m\n\n")

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

	// Handle SIGINT/SIGTERM — kill the whole process group
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			// Kill the process group (negative PID = entire group)
			if cmd.Process != nil {
				syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) //nolint:errcheck
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
