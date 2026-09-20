//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcAttrs places the child process in its own process group so that
// terminateProcessGroup can cleanly signal or terminate all descendants
// (compilers, test runners, sub-processes) and not just the top-level shell.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup performs a two-phase escalation teardown:
// 1. Sends SIGTERM to the entire process group (-pgid) so child processes
//    (compilers, tests, database handles) can catch the signal, flush buffers,
//    and shut down cleanly (ADR-0007).
// 2. Starts an escalation timer for gracePeriod.
// 3. If cmd has not exited when the grace period expires, delivers SIGKILL.
func terminateProcessGroup(cmd *exec.Cmd, gracePeriod time.Duration, done <-chan struct{}) {
	if cmd.Process == nil {
		return
	}
	pgid := -cmd.Process.Pid

	// Phase 1: Polite termination request to the entire process group
	_ = syscall.Kill(pgid, syscall.SIGTERM)

	// Phase 2: Escalation timer
	go func() {
		select {
		case <-time.After(gracePeriod):
			// Escalation: force kill if still alive
			if cmd.Process != nil {
				_ = syscall.Kill(pgid, syscall.SIGKILL)
			}
		case <-done:
			// Process exited cleanly within grace period
		}
	}()
}
