//go:build windows

package runner

import (
	"os"
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
	if cmd.Process != nil && cmd.Process.Pid > 0 {
		taskkill := "taskkill"
		if p, err := exec.LookPath("taskkill.exe"); err == nil {
			taskkill = p
		} else if _, err := os.Stat(`C:\Windows\System32\taskkill.exe`); err == nil {
			taskkill = `C:\Windows\System32\taskkill.exe`
		}
		killCmd := exec.Command(taskkill, "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		_ = killCmd.Run()
		_ = cmd.Process.Kill()
	}
}
