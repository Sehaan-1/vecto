# ADR-0007: Process Group Isolation for Subprocess Cleanup

**Status:** Accepted  
**Date:** 2026-09-20

## Context

Task commands are executed via `exec.Command("sh", "-c", cmdStr)` (Unix) or `cmd.exe /C` (Windows). When a sibling task fails and the run context is cancelled, Go's default behaviour sends `SIGKILL` **only to the top-level shell process** (`/bin/sh` or `cmd.exe`). Any child processes that the shell spawned — compilers, test runners, language servers, bundlers — are reparented to PID 1 (init) and continue consuming CPU, memory, and port locks as orphaned zombies.

This is a well-known problem in build systems and CI runners (see: Bazel's process-wrapper, Turborepo's cross-platform kill, GitHub Actions' `cancel-in-progress`).

## Decision

Use **OS process groups** to ensure the full subprocess tree is terminated:

### Unix (`//go:build !windows` — `internal/runner/proc_unix.go`)
```go
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
// On cancellation:
syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
```
`Setpgid: true` places the child shell in a new process group (PGID = child PID). Signalling the **negative PGID** delivers the signal to every process in the group.

### Windows (`//go:build windows` — `internal/runner/proc_windows.go`)
```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
}
// On cancellation:
cmd.Process.Kill()
```
On Windows, `cmd.exe /C` already runs inside a job object when invoked via Go's `exec.Command`. `CREATE_NEW_PROCESS_GROUP` isolates the group; `Process.Kill()` terminates it.

### Cancellation watcher goroutine
`executeCommand` uses `cmd.Start()` + `cmd.Wait()` (not `exec.CommandContext`) so it can control the kill signal precisely:
```go
go func() {
    select {
    case <-ctx.Done():
        killProcessGroup(cmd)
    case <-watchDone:
    }
}()
```

## Consequences

**Positive:**
- Build tasks that spawn compilers or test runners no longer leave zombie processes after pipeline cancellation.
- Port conflicts and file lock contention from orphaned processes are eliminated.

**Neutral:**
- The `watchDone` goroutine is always spawned, even for tasks that complete normally. It exits immediately via the `<-watchDone` case at zero cost.

**Negative / Trade-offs:**
- `SIGKILL` is abrupt — no cleanup for the child. For tasks that need graceful shutdown (e.g., a dev server), a two-phase `SIGTERM → SIGKILL` with a timeout would be preferable. This is deferred as a future enhancement.
- Windows behaviour differs: `cmd.exe /C` job propagation is less deterministic than Unix PGID signalling for deeply nested child trees.
