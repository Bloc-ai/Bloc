//go:build windows

package llamacpp

import (
	"os"
	"os/exec"
)

// setSysProcAttr is a no-op on Windows.
// Windows does not support POSIX process groups; exec.CommandContext cancellation
// and os.Process.Kill() are sufficient to stop the server process.
func setSysProcAttr(cmd *exec.Cmd) {}

// killProcessGroup on Windows falls back to a plain process kill.
// Windows has no POSIX process group concept; child processes started by
// llama-server are cleaned up by the OS when the parent exits because they
// share the same Job Object.
func killProcessGroup(p *os.Process) {
	if p == nil {
		return
	}
	_ = p.Kill()
}
