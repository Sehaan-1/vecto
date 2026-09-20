//go:build windows

package runner

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcAttrs creates a new process group on Windows.
// This is the closest Windows equivalent to Unix Setpgid: it isolates the
// child job so that process termination signals affect the group.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// terminateProcessGroup performs process teardown on Windows.
// Because Windows lacks POSIX signals (SIGTERM/SIGKILL), process teardown
// is performed via Process.Kill() (which invokes the Win32 TerminateProcess API).
func terminateProcessGroup(cmd *exec.Cmd, _ time.Duration, _ <-chan struct{}) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
