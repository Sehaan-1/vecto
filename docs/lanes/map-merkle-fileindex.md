# Map: Merkle File Index Hardening

**For the team.** Cuecards decided. This file is how we ship it without colliding.

- **Handoff:** [handoff](../cuecards/handoff-merkle-fileindex.md)
- **Board:** [Merkle File Index Pre-Merge Hardening](https://github.com/Sehaan-1/vecto/tree/arena/01a0bf54-vecto)
- **Integration owner:** Sehaan
- **Status:** Round 1 Complete — All Fixes Implemented & Verified
- **Time budget:** none
- **Spend ceiling:** none
- **Round:** 1
- **Agents in flight:** none

## Where we're headed
Harden the Merkle File Index (ADR-0019) on branch `arena/01a0bf54-vecto` across Windows compilation, concurrent dirty-flag mutation, worker scheduler bounding, dry-run read-only semantics, and test wall-clock speedups so that the branch can be cleanly and safely merged into `main` with 3-OS green CI.

## How we'll know we're there
- **Walk:**
  - `GOOS=windows go build ./cmd/vecto` and `GOOS=windows go build ./internal/fileindex/...` succeed without `syscall.Stat_t` field errors.
  - `go test -race ./...` passes with zero race detections across all packages.
  - Phase B re-hashing uses a bounded worker semaphore.
  - `vecto run --dry-run` previews fingerprints without writing `.vecto/fileindex.json`.
  - `go test -short ./internal/fileindex/...` completes in <1s (down from ~15s).
- **Proof:**
  - `stat_unix.go` and `stat_windows.go` platform split compiles cleanly on Windows, Linux, and macOS.
  - Concurrent `runTask` race test passes with `-race`.
  - Dry-run verification confirms `.vecto/fileindex.json` is not written.
  - `fileindex_test.go` suite executes in < 2.5s with all assertions passing.
- **Enforced:** GitHub Actions CI matrix (`ubuntu-latest`, `windows-latest`, `macos-latest`) passes with race detector enabled.

This is the locked target. Rounds do not rewrite it.

## Decisions we honor
- [ADR-0019: Merkle File Index — Incremental Fingerprinting](../adr/0019-merkle-file-index-incremental-fingerprinting.md)

## Not this effort
- Binary serialization format for `fileindex.json` (Note A, future ADR).
- Network distributed cache indexing.

## Seams (shared contracts)

### Seam 1: FileIndex Package Interface
- **Contract:** `internal/fileindex/fileindex.go` (`New`, `Load`, `Save`, `Sync`, `Coverage`)
- **Owned by:** Lane A
- **Consumed by:** Lane B (`internal/runner`), Lane C (`cmd/vecto`)
- **Status:** written & verified; platform split and scheduler bounding maintain unchanged public API

## Lanes

### Lane A — Cross-Platform Stat Engine, Scheduler Bounding & Test Speedups
- **Check:** `GOOS=windows go build ./internal/fileindex/...` succeeds; `go test -race ./internal/fileindex/...` passes in < 2.5s.
- **Does:** Extract platform stat logic into `stat_linux.go`, `stat_darwin.go`, `stat_windows.go`, and `stat_other.go`; remove `syscall` import from `fileindex.go`; bound Phase B parallel hashing with worker semaphore (`hashSem`); backdate `ix.Snapshot` in `fileindex_test.go` to eliminate wall-clock `time.Sleep`.
- **Handoff slices:** Slice 1
- **Owns (files/packages):** `internal/fileindex/`
- **Does not touch:** `internal/runner/`, `cmd/vecto/`
- **Needs seams:** Seam 1 (FileIndex Interface)
- **Parallel with:** Lane B, Lane C
- **Waits on:** none
- **Claimed by:** Sehaan (agent assisted)
- **Ticket:** [Fix 1, 3, 5: FileIndex Windows build, bounded goroutines, and test speedup](https://github.com/Sehaan-1/vecto/tree/arena/01a0bf54-vecto)
- **Brief:** Lane A packet only — ADR-0019.
- **Status:** Check passed

### Lane B — Thread-Safe Index Dirty Flag
- **Check:** `go test -race -v ./internal/runner/...` passes with zero race detections.
- **Does:** Replace `indexDirty bool` with `indexDirty atomic.Bool` in `internal/runner/runner.go`; update `.Store(true)` and `.Load()`.
- **Handoff slices:** Slice 2
- **Owns (files/packages):** `internal/runner/runner.go`
- **Does not touch:** `internal/fileindex/`, `cmd/vecto/`
- **Needs seams:** Seam 1 (FileIndex Interface)
- **Parallel with:** Lane A, Lane C
- **Waits on:** none
- **Claimed by:** Sehaan (agent assisted)
- **Ticket:** [Fix 2: Runner indexDirty atomic.Bool data race fix](https://github.com/Sehaan-1/vecto/tree/arena/01a0bf54-vecto)
- **Brief:** Lane B packet only — ADR-0019.
- **Status:** Check passed

### Lane C — Read-Only Dry-Run Isolation
- **Check:** Running `vecto run --dry-run` leaves `.vecto/fileindex.json` nonexistent / unmodified.
- **Does:** Remove `ix.Save(cwd)` from `handleDryRun` in `cmd/vecto/handlers.go`; keep `i.Sync` for preview accuracy with explanatory comment.
- **Handoff slices:** Slice 3
- **Owns (files/packages):** `cmd/vecto/handlers.go`
- **Does not touch:** `internal/fileindex/`, `internal/runner/`
- **Needs seams:** Seam 1 (FileIndex Interface)
- **Parallel with:** Lane A, Lane B
- **Waits on:** none
- **Claimed by:** Sehaan (agent assisted)
- **Ticket:** [Fix 4: Dry-run read-only preview side-effect removal](https://github.com/Sehaan-1/vecto/tree/arena/01a0bf54-vecto)
- **Brief:** Lane C packet only — ADR-0019.
- **Status:** Check passed

## Now / Next / Then
- **Now:** All lanes (A, B, C) complete and verified locally
- **Next:** Push branch to `origin/arena/01a0bf54-vecto`, verify GitHub Actions CI matrix
- **Then:** Merge `arena/01a0bf54-vecto` into `main`

## Integration
- Merge to the default branch after every lane Check, not only at the end
- After each merge, walk as far as the destination currently allows
- Final: full walk + proof + enforced

## Sitting profile (this round)
- **Takeable now:** none (all 3 lanes complete)
- **Idle / waiting on:** none
- **Bottleneck:** none
- **Duplicate work or duplicate context:** none
- **Walk before → after this round:**
  - Before: `internal/fileindex` broke Windows compilation (`syscall.Stat_t` fields); `indexDirty` had concurrent data races; Phase B spawned unbounded goroutines; dry-run wrote `fileindex.json`; unit tests took ~20s due to `time.Sleep`.
  - After: Platform split compiles cleanly on Windows, Linux, and macOS; `indexDirty` is race-safe with `atomic.Bool`; Phase B is bounded to worker pool size; `--dry-run` is verified read-only; test suite executes deterministically in < 2.5s; all E2E and benchmark tests pass.

## What landed
- Lane A: Cross-platform Stat Engine (`stat_linux.go`, `stat_darwin.go`, `stat_windows.go`, `stat_other.go`), bounded Phase-B worker semaphore in `fileindex.go`, and test speedup via snapshot backdating in `fileindex_test.go`.
- Lane B: Thread-safe `indexDirty` using `atomic.Bool` in `internal/runner/runner.go` (`Store(true)` and `Load()`), race-clean.
- Lane C: Read-only `--dry-run` preview in `cmd/vecto/handlers.go` without persisting `.vecto/fileindex.json`.
- E2E Test Fix: Windows `.exe` path resolution in `test/e2e/fileindex_e2e_test.go`.

## Blocked
nothing
