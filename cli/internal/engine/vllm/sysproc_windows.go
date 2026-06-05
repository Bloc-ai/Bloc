//go:build windows

package vllm

import "os/exec"

func setSysProcAttr(cmd *exec.Cmd) {
	// No-op on Windows
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
