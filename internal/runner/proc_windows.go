//go:build windows

package runner

import (
	"os/exec"
	"syscall"
)

// setProcAttrs creates a new process group on Windows.
// This is the closest Windows equivalent to Unix Setpgid: it isolates the
// child job so that TerminateProcess (cmd.Process.Kill) affects the group.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcessGroup terminates the process on Windows.
// Windows job objects propagate termination to children when the parent
// cmd.exe is killed, so Process.Kill() is sufficient here.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
