//go:build !windows

package llamacpp

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// setSysProcAttr puts the child into its own process group (Setpgid=true).
// This allows us to kill the entire group (process + children) with a single
// kill(-pgid, SIGTERM) call when the user presses Ctrl+C.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGTERM to the entire process group of p.
// The negative PID notation (-pid) is a POSIX convention that targets the group.
func killProcessGroup(p *os.Process) {
	if p == nil {
		return
	}
	_ = syscall.Kill(-p.Pid, syscall.SIGTERM)
	go func() {
		time.Sleep(5 * time.Second)
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	}()
}
