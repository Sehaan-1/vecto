# ADR-0007: Process Group Isolation and Two-Phase Teardown

**Status:** Accepted (Updated)  
**Date:** 2026-09-20  
**Author:** Vecto Engine Team

## Context

Task commands are executed via `exec.Command("sh", "-c", cmdStr)` (Unix) or `cmd.exe /C` (Windows). When a sibling task fails and the run context is cancelled, Go's default behaviour sends `SIGKILL` **only to the top-level shell process** (`/bin/sh` or `cmd.exe`). Any child processes that the shell spawned — compilers, test runners, language servers, bundlers — are reparented to PID 1 (init) and continue consuming CPU, memory, and port locks as orphaned zombies.

Furthermore, immediately firing `SIGKILL` prevents child processes from trapping termination signals, flushing unwritten disk buffers, closing SQLite handles, or releasing network sockets, leaving corrupted state behind.

## Decision

Use **OS process groups** combined with a **Two-Phase Signal Escalation Ladder**:

### Unix (`//go:build !windows` — `internal/runner/proc_unix.go`)
```go
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

// Phase 1: Polite termination broadcast to process group (-pgid)
_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

// Phase 2: Escalation timer (grace period: 1s)
go func() {
    select {
    case <-time.After(gracePeriod):
        // Force kill if child failed to terminate
        _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
    case <-done:
        // Child exited cleanly within grace period
    }
}()
```
`Setpgid: true` places the child shell in a new process group (PGID = child PID). Signalling the **negative PGID** delivers `SIGTERM` to every process in the group. If the process tree does not exit before `gracePeriod` expires, `SIGKILL` is delivered to guarantee termination.

### Windows (`//go:build windows` — `internal/runner/proc_windows.go`)
```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
}

// Phase 1: Attempt polite console break event
_ = syscall.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))

// Phase 2: Escalation timer followed by Process.Kill()
go func() {
    select {
    case <-time.After(gracePeriod):
        if cmd.Process != nil {
            _ = cmd.Process.Kill()
        }
    case <-done:
    }
}()
```

### Cancellation watcher goroutine in `executeCommand`
```go
watchDone := make(chan struct{})
go func() {
    select {
    case <-ctx.Done():
        terminateProcessGroup(cmd, 1*time.Second, watchDone)
    case <-watchDone:
    }
}()

err := cmd.Wait()
close(watchDone)
```

## Consequences

**Positive:**
- Build tasks that spawn compilers or test runners no longer leave zombie processes after pipeline cancellation.
- Two-phase teardown allows compilers and test suites to trap `SIGTERM`, flush disk buffers, and cleanly release file/port locks before hard termination.
- Unresponsive or hanging processes are guaranteed to be terminated when the grace timer expires.

**Neutral:**
- In fast cancellations where child processes exit immediately upon `SIGTERM`, `watchDone` closes quickly and the escalation timer goroutine exits without allocating.
