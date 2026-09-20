# Handoff: Merkle File Index Hardening

**For a building agent.** Do not start this from Cuecards. Cuecards sitting ends when this file is written.

## Provenance
- Board: [Merkle File Index Pre-Merge Hardening](https://github.com/Sehaan-1/vecto/tree/arena/01a0bf54-vecto)
- Slices decided: 3 of 3
- Last human answer recorded: proceed with plan using cuecards and /lanes on arena/01a0bf54-vecto branch
- Handoff written by: agent

## Where we're headed
Harden the Merkle File Index (ADR-0019) implementation on branch `arena/01a0bf54-vecto` across Windows, concurrency, scheduler bounding, dry-run safety, and test execution speed, ensuring clean CI on 3 OSes (Linux, Windows, macOS) before merging into `main`.

## How we'll know we're there
- **Walk:**
  - `go build ./cmd/vecto` and `go build ./internal/fileindex/...` compile cleanly on Windows, Linux, and macOS without missing syscall fields.
  - `go test -race ./...` passes without data race detections on `indexDirty`.
  - Phase B re-hashing uses a bounded worker semaphore instead of unbounded goroutines.
  - `vecto run --dry-run` performs accurate fingerprint previews without writing `.vecto/fileindex.json`.
  - `go test -short ./internal/fileindex/...` completes in <1s (down from ~15s) using deterministic snapshot backdating.
- **Proof:**
  - `stat_unix.go` and `stat_windows.go` platform split compiles on Windows (`GOOS=windows go build ./internal/fileindex/...`).
  - Concurrent `runTask` race test passes with `-race`.
  - Dry-run inspection verifies `.vecto/fileindex.json` is not created.
  - `fileindex_test.go` suite executes in < 2s with all assertions passing.
- **Enforced:** GitHub Actions CI matrix running on `ubuntu-latest`, `windows-latest`, `macos-latest` passes with race detector enabled.

## Constraints (from Notes)
- Standard library first: `sync/atomic`, build tags (`//go:build !windows`, `//go:build windows`). Zero external dependencies.
- Race-detector clean: must pass `go test -race ./...`.
- Deterministic: same inputs produce exact same fingerprints.
- Dry-run must be strictly read-only.
- Do NOT modify `TestRacyTimestampRehashes` in `fileindex_test.go` (deliberate racy boundary test).

## ADRs this implements (in force)
- [ADR-0019: Merkle File Index — Incremental Fingerprinting](../adr/0019-merkle-file-index-incremental-fingerprinting.md)

## Not this effort
- Changing the disk format or switching to binary index serialization (deferred to future ADR per Note A).
- Distributed or network-based remote file indexes.

## Sequence

### Slice 1: Cross-Platform Stat Engine & Scheduler Bounding (Lane A)
- **ADRs:** ADR-0019
- **Does:**
  - Splits `fileindex.go` syscall stat retrieval into `stat_unix.go` (`!windows`) and `stat_windows.go` (`windows`). Removes `syscall` import from `fileindex.go`.
  - Bounds Phase B parallel re-hash in `fileindex.go` using a worker semaphore (`chan struct{}`).
  - Replaces `time.Sleep(1100ms)` in `fileindex_test.go` with deterministic `ix.Snapshot` backdating.
- **Check:** `GOOS=windows go build ./internal/fileindex/...` compiles; `go test -v -race ./internal/fileindex/...` passes in < 2s.
- **Depends on:** none

### Slice 2: Thread-Safe Index Dirty Flag (Lane B)
- **ADRs:** ADR-0019
- **Does:**
  - Changes `Runner.indexDirty` from `bool` to `atomic.Bool` in `internal/runner/runner.go`.
  - Updates all reads and writes to `.Store(true)` and `.Load()`.
- **Check:** `go test -race -v ./internal/runner/...` passes without race conditions.
- **Depends on:** none (disjoint file: `internal/runner/runner.go`)

### Slice 3: Read-Only Dry-Run Isolation (Lane C)
- **ADRs:** ADR-0019
- **Does:**
  - Removes `ix.Save(cwd)` from `cmd/vecto/handlers.go` in `handleDryRun`.
  - Retains `i.Sync` for preview accuracy with an explicit documentation comment.
- **Check:** Running `--dry-run` does not create or modify `.vecto/fileindex.json`.
- **Depends on:** none (disjoint file: `cmd/vecto/handlers.go`)
