//go:build windows

package runner

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// setProcAttrs creates a new process group on Windows.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// terminateProcessGroup performs process tree teardown on Windows.
// Using taskkill /F /T ensures that child processes started by cmd.exe (e.g.
// compilers, test suites, background scripts) are terminated alongside the shell,
// releasing stdio pipes and terminating in milliseconds.
func terminateProcessGroup(cmd *exec.Cmd, _ time.Duration, _ <-chan struct{}) {
	if cmd.Process != nil {
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		_ = killCmd.Run()
		_ = cmd.Process.Kill()
	}
}
