//go:build !windows

package vllm

import (
	"os/exec"
	"syscall"
	"time"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) //nolint:errcheck
		go func() {
			time.Sleep(5 * time.Second)
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) //nolint:errcheck
		}()
	}
}
