# Map: Vecto Task Runner

**For the team.** Cuecards decided. This file is how we ship it without colliding.

- **Handoff:** [handoff](../cuecards/handoff-vecto.md)
- **Board:** [Vecto Task Runner](https://github.com/Sehaan-1/vecto/issues/1)
- **Integration owner:** Antigravity (integration agent)
- **Status:** Destination Check passed
- **Time budget:** none
- **Spend ceiling:** none
- **Round:** 2
- **Agents in flight:** none

## Where we're headed
A single-binary CLI tool in Go that executes interdependent build and script tasks concurrently as a Directed Acyclic Graph (DAG), caches artifact outputs using cryptographic file content hashes, and skips unchanged work in 0ms with streaming terminal progress.

## How we'll know we're there
- **Walk:** A developer clones a sample multi-task project, runs `vecto run build`, watches independent tasks execute concurrently with real-time log output, runs it a second time without touching files, and sees all tasks finish instantly with `[CACHED] (0.00s)`.
- **Proof:** Automated end-to-end integration test suite verifying graph cycle detection, concurrent execution order, cache hits, and cache invalidation on file edits.
- **Enforced:** GitHub Actions CI pipeline running `go test -race ./...` and an automated E2E benchmark check that fails if cache replay exceeds 50ms.

This is the locked target. Rounds do not rewrite it.

## Decisions we honor
- [ADR-0001 Where task definitions live](../adr/0001-task-definitions-in-root-yaml.md)
- [ADR-0002 How parallel progress looks on screen](../adr/0002-live-terminal-status-dashboard.md)
- [ADR-0003 Where saved build output lives](../adr/0003-hybrid-cache-storage-directory.md)
- [ADR-0004 Cryptographic input hashing](../adr/0004-cryptographic-input-hashing.md)
- [ADR-0005 Task failure lifecycle and keep-going execution](../adr/0005-fail-fast-with-keep-going.md)

## Not this effort
- Distributed cloud build farms or multi-machine worker daemons.
- Remote S3/GCS bucket synchronization.

## Seams (shared contracts)

### Seam 1: Task Configuration Contract
- **Contract:** `internal/config/config.go` (`TaskConfig`, `Config`, `LoadConfig`, `BuildGraph`)
- **Owned by:** Lane A
- **Consumed by:** Lane C
- **Status:** both sides on it

### Seam 2: Cache Store Contract
- **Contract:** `internal/cache/cache.go` (`Store`, `Restore`, `Has`, `Clean`)
- **Owned by:** Lane B
- **Consumed by:** Lane C
- **Status:** both sides on it

### Seam 3: UI Event Contract
- **Contract:** `internal/ui/ui.go` (`Reporter`, `TaskStarted`, `TaskCompleted`, `TaskCached`, `TaskFailed`, `Summary`)
- **Owned by:** Lane B
- **Consumed by:** Lane C
- **Status:** both sides on it

## Lanes

### Lane A — Config & Hashing Core
- **Check:** `go test -race ./internal/config ./internal/hash` passed.
- **Does:** Manifest loader, DAG graph validator, and deterministic SHA-256 fingerprint engine.
- **Handoff slices:** Slice 1, Slice 2
- **Owns:** `internal/config/`, `internal/hash/`
- **Status:** Check passed · Merged

### Lane B — Cache Store & Terminal UI
- **Check:** `go test -race ./internal/cache ./internal/ui` passed.
- **Does:** Local and env-configurable cache store/restore engine and thread-safe TUI reporter.
- **Handoff slices:** Slice 3, Slice 5
- **Owns:** `internal/cache/`, `internal/ui/`
- **Status:** Check passed · Merged

### Lane C — Concurrent Runner & CLI Integration
- **Check:** Destination Walk passed (`test/e2e/e2e_test.go` and live walk in `examples/sample_project`).
- **Does:** Concurrent worker pool, process group cancellation, `--keep-going` support, CLI flags and commands.
- **Handoff slices:** Slice 4, Slice 6
- **Owns:** `internal/runner/`, `cmd/vecto/`, `test/e2e/`, `examples/sample_project/`
- **Status:** Check passed · Merged

## Now / Next / Then
- **Now:** All lanes landed
- **Next:** Destination shipped
- **Then:** None

## Integration
- Full test suite passing race detection: `go test -v -race ./...`
- Production binary compiled to `bin/vecto.exe`
- Destination Walk verified:
  - Cold run: 0.05s total (all 3 tasks executed)
  - Hot run: 0.00s total (all 3 tasks `[⚡ CACHED]`)

## What landed
- `internal/config/`: `vecto.yaml` parser, task validator, and DAG builder.
- `internal/hash/`: Content-addressable SHA-256 fingerprint engine over inputs, command, and env.
- `internal/cache/`: `.vecto/cache/` storage, artifact snapshotting and replay engine.
- `internal/ui/`: Thread-safe terminal reporter with spinners, elapsed timing, and CI pipe fallback.
- `internal/runner/`: Concurrent DAG layer dispatcher, worker pool, context cancellation, and keep-going mode.
- `cmd/vecto/`: Complete CLI commands (`run`, `list`, `init`, `clean`, `version`).
- `test/e2e/`: Automated E2E test suite.
- `examples/sample_project/`: End-to-end demo project.

## Blocked
nothing
