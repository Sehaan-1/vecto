# Map: Vecto Task Runner

**For the team.** Cuecards decided. This file is how we ship it without colliding.

- **Handoff:** [handoff](../cuecards/handoff-vecto.md)
- **Board:** [Vecto Task Runner](https://github.com/Sehaan-1/vecto/issues/1)
- **Integration owner:** Antigravity (integration agent)
- **Status:** Locked
- **Time budget:** none
- **Spend ceiling:** none
- **Round:** 0
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
- **Status:** written

### Seam 2: Cache Store Contract
- **Contract:** `internal/cache/cache.go` (`Store`, `Retrieve`, `Has`, `Clear`)
- **Owned by:** Lane B
- **Consumed by:** Lane C
- **Status:** written

### Seam 3: UI Event Contract
- **Contract:** `internal/ui/ui.go` (`Reporter`, `TaskStarted`, `TaskCompleted`, `TaskCached`, `TaskFailed`)
- **Owned by:** Lane B
- **Consumed by:** Lane C
- **Status:** written

## Lanes

### Lane A — Config & Hashing Core
- **Check:** `go test -race ./internal/config ./internal/hash` passes, correctly parsing `vecto.yaml` and computing deterministic SHA-256 keys.
- **Does:** `internal/config` (YAML loader, DAG builder, dependency validator) and `internal/hash` (lexicographical glob file walker, SHA-256 hasher).
- **Handoff slices:** Slice 1, Slice 2
- **Owns (files/packages):** `internal/config/`, `internal/hash/`
- **Does not touch:** `internal/cache/`, `internal/ui/`, `internal/runner/`, `cmd/vecto/`
- **Needs seams:** none
- **Parallel with:** Lane B
- **Waits on:** none
- **Claimed by:** Antigravity
- **Ticket:** [#9 Lane A — Config & Hashing Core](https://github.com/Sehaan-1/vecto/issues)
- **Status:** in progress

### Lane B — Cache Store & Terminal UI
- **Check:** `go test -race ./internal/cache ./internal/ui` passes, storing/restoring artifacts and rendering live terminal output without race conditions.
- **Does:** `internal/cache` (metadata, log, and artifact tar storage in `.vecto/cache` or custom env) and `internal/ui` (thread-safe multi-task status dashboard with TTY and pipe modes).
- **Handoff slices:** Slice 3, Slice 5
- **Owns (files/packages):** `internal/cache/`, `internal/ui/`
- **Does not touch:** `internal/config/`, `internal/hash/`, `internal/runner/`, `cmd/vecto/`
- **Needs seams:** none
- **Parallel with:** Lane A
- **Waits on:** none
- **Claimed by:** Antigravity
- **Ticket:** [#10 Lane B — Cache Store & Terminal UI](https://github.com/Sehaan-1/vecto/issues)
- **Status:** ready

### Lane C — Concurrent Runner & CLI Integration
- **Check:** Destination Walk passes: `vecto run build` executes concurrently, detects cycles, honors `--keep-going`, and replays in 0.00s from cache.
- **Does:** `internal/runner` (worker pool, execution layer dispatcher, process group cancellation) and `cmd/vecto` (CLI commands `run`, `list`, `init`, `clean`).
- **Handoff slices:** Slice 4, Slice 6
- **Owns (files/packages):** `internal/runner/`, `cmd/vecto/`, `test/e2e/`
- **Does not touch:** `internal/config/`, `internal/hash/`, `internal/cache/`, `internal/ui/`
- **Needs seams:** Seam 1 (Task Config), Seam 2 (Cache Store), Seam 3 (UI Event)
- **Parallel with:** none
- **Waits on:** Lane A and Lane B
- **Claimed by:** Antigravity
- **Ticket:** [#11 Lane C — Concurrent Runner & CLI Integration](https://github.com/Sehaan-1/vecto/issues)
- **Status:** waiting on A and B

## Now / Next / Then
- **Now, in parallel:** Lane A, Lane B
- **Next:** Lane C
- **Then:** Full destination walk & enforcement check

## Integration
- Merge to master branch after every lane Check
- After each merge, verify `go test -race ./...` remains green
- Final: run full end-to-end multi-task demo walk verifying 0.00s replay

## Sitting profile (this round)
- **Takeable now:** Lane A, Lane B
- **Idle / waiting on:** Lane C (waits on A & B)
- **Bottleneck:** none
- **Duplicate work or duplicate context:** none
- **Walk before → after this round:** Skeleton compiled → Core subsystems (Config, Hashing, Cache, UI) functional

## What landed
- Baseline Go 1.27 module and `internal/dag` with topological sort and cycle detection.

## Blocked
nothing
