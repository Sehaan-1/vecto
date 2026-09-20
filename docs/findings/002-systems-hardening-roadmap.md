# Finding 002: Systems Hardening and Industrial-Grade Reliability Roadmap

- **Status:** Complete
- **Date:** 2026-09-20
- **Author:** Vecto Engine Team

## Objective
Identify and evaluate the architectural upgrades required to transition Vecto from an algorithmic prototype into an industrial-grade systems tool (targeting 9.5+ systems complexity and adoption standards).

---

## 1. Operating System Interactions & Signal Lifecycle

### Primary Source Investigation
- **POSIX Signal Semantics (`signal(7)`, `kill(2)`):**
  - Standard `syscall.SIGKILL` (signal 9) cannot be caught, blocked, or ignored by child processes.
  - When a process running an open transaction (e.g. SQLite database, Go compiler writing `a.out`, or a test harness with active socket descriptors) is killed with `SIGKILL`, OS buffers remain unflushed, locks remain held, and file artifacts are left truncated.
  - POSIX specifies `SIGTERM` (signal 15) as the standard graceful termination signal. Sane build tools and daemons trap `SIGTERM` to perform cleanup routines before exiting.
- **Process Group Targeting (`setpgid(2)`, `kill(-pgid, sig)`):**
  - When a runner executes shell commands (`sh -c ...`), the shell is merely the parent of the actual compiler or test runner.
  - Signaling only `cmd.Process.Pid` kills the wrapper shell, leaving grandchild processes orphaned and running as background zombies attached to init (`PID 1`).
  - Negating the PID (`-pgid`) broadcasts the signal across the entire process tree created under `Setpgid: true`.

### Recommended Systems Fix
Implement a **Two-Phase Signal Escalation Ladder**:
1. Broadcast `syscall.SIGTERM` to `-pgid`.
2. Start an escalation timer (grace window: 1–2 seconds).
3. If `cmd.Wait()` returns before the timer expires, release resources cleanly.
4. If the timer expires and the process group is still alive, drop the hammer with `syscall.SIGKILL` to prevent hanging builds.

---

## 2. Filesystem Atomicity & Cache Staging

### Primary Source Investigation
- **POSIX `rename(2)` & NTFS `MoveFileEx` Atomicity:**
  - In Linux/POSIX, `rename(oldpath, newpath)` within the same filesystem mount point is atomic. At no point can another process observe an intermediate, partially written state.
  - Directly writing files (logs, metadata, copied artifacts) into `.vecto/cache/<hash>/` creates a race window:
    - If user presses `Ctrl+C` (SIGINT) mid-write, partial artifacts remain.
    - If two concurrent Vecto processes run the same task, one may read `meta.json` while the other is still copying artifacts.
  - `Has(hash)` checks only for `meta.json`. A run interrupted after `meta.json` was written but before artifacts finished will poison all future runs.

### Recommended Systems Fix
Implement **Atomic Directory Staging**:
1. Write all logs, copied artifacts, and `meta.json` into an isolated staging directory on the same filesystem: `.vecto/cache/tmp-<hash>-<pid>-<timestamp>`.
2. Commit the entry with a single `os.Rename(stagingDir, finalDir)`.
3. If `finalDir` already exists (due to a concurrent run finishing first), discard the staging directory and return success.
4. If any error occurs prior to rename, clean up the staging directory immediately.

---

## 3. Developer Experience (DevEx) & Observability

### Primary Source Investigation
- **Graph Inspection Standards:**
  - Build engines (Turborepo, Nx, Bazel) provide visual inspection commands (`nx graph`, `turbo run ... --dry-run`).
  - GitHub natively renders Mermaid.js code blocks in markdown (`graph TD`). Providing `vecto graph --format=mermaid` allows developers to paste their build topology directly into pull requests, architecture docs, and CI summaries.
  - Graphviz DOT format (`digraph G { ... }`) provides compatibility with standard UNIX graph utilities (`dot -Tsvg`).
- **Dry-Run Validation:**
  - Developers need to verify task invalidation and cache status without executing heavy shell commands.
  - `--dry-run` performs DAG resolution, content hashing, and cache lookups, printing which tasks hit cache and which will execute.
- **Dynamic Argument Passthrough (`--`):**
  - Developers need to pass flags directly to underlying commands (e.g. `vecto run test -- -run TestFoo -v`).
  - Standard POSIX `--` delimiter separates runner flags from target command arguments.

---

## Conclusion & Implementation Priority

| Feature | Category | Effort | Target Impact |
|---|---|---|---|
| **Atomic Cache Staging (`os.Rename`)** | Reliability | Low (1–2 hrs) | Eliminates cache corruption / race hazards |
| **Two-Phase Signal Teardown (`SIGTERM` $\to$ `SIGKILL`)** | OS Architecture | Medium (2–3 hrs) | Clean process termination and resource deallocation |
| **Graph Visualizer (`mermaid`/`dot`) & `--dry-run`** | Developer Experience | Low-Med (2–3 hrs) | Visual verification, CI documentation, plan preview |
| **Argument Passthrough (`--`)** | Developer Experience | Low (1 hr) | Real-world CLI ergonomics |
