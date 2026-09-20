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

// terminateProcessGroup performs a two-phase teardown on Windows:
// 1. Attempts to send a CTRL_BREAK_EVENT to the process group.
// 2. Starts an escalation timer for gracePeriod, followed by Process.Kill().
func terminateProcessGroup(cmd *exec.Cmd, gracePeriod time.Duration, done <-chan struct{}) {
	if cmd.Process == nil {
		return
	}

	// Phase 1: Attempt polite CTRL_BREAK_EVENT to process group
	_ = syscall.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))

	// Phase 2: Escalation timer
	go func() {
		select {
		case <-time.After(gracePeriod):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		case <-done:
			// Process exited cleanly within grace period
		}
	}()
}
