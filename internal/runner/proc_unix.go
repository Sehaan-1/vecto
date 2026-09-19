//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// setProcAttrs places the child process in its own process group so that
// killProcessGroup can cleanly terminate all descendants (compilers, test
// runners, etc.) and not just the top-level /bin/sh wrapper.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the entire process group of cmd.
// Using the negative PID targets all processes in the group, preventing
// orphaned zombie children when a build task spawns sub-processes.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
