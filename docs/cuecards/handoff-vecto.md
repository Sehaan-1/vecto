# Handoff: Vecto Task Runner

**For a building agent.** Do not start this from Cuecards. Cuecards sitting ends when this file is written.

## Where we're headed
A single-binary CLI tool in Go that executes interdependent build and script tasks concurrently as a Directed Acyclic Graph (DAG), caches artifact outputs using cryptographic file content hashes, and skips unchanged work in 0ms with streaming terminal progress.

## How we'll know we're there
- **Walk:** A developer clones a sample multi-task project, runs `vecto run build`, watches independent tasks execute concurrently with real-time log output, runs it a second time without touching files, and sees all tasks finish instantly with `[CACHED] (0.00s)`.
- **Proof:** Automated end-to-end integration test suite verifying graph cycle detection, concurrent execution order, cache hits, and cache invalidation on file edits.
- **Enforced:** GitHub Actions CI pipeline running `go test -race ./...` and an automated E2E benchmark check that fails if cache replay exceeds 50ms.

## Constraints (from Notes)
- **Standard library first:** Rely on `sync`, `context`, `crypto/sha256`, `os/exec`. Zero external dependency bloat.
- **Race-detector clean:** Must pass `go test -race ./...`.
- **Deterministic:** Same inputs must produce the exact same cache key across runs.

## ADRs this implements (in force)
- [ADR-0001: Where task definitions live](../adr/0001-task-definitions-in-root-yaml.md)
- [ADR-0002: How parallel progress looks on screen](../adr/0002-live-terminal-status-dashboard.md)
- [ADR-0003: Where saved build output lives](../adr/0003-hybrid-cache-storage-directory.md)
- [ADR-0004: Cryptographic input hashing for cache fingerprints](../adr/0004-cryptographic-input-hashing.md)
- [ADR-0005: Task failure lifecycle and keep-going execution](../adr/0005-fail-fast-with-keep-going.md)

## Not this effort
- Distributed cloud build farms or multi-machine worker daemons (out of scope for single-machine runner).
- Remote S3/GCS bucket synchronization (deferred to post-v1).

## Sequence

### Slice 1: Configuration Parser & Manifest Validator
- **ADRs:** ADR-0001
- **Does:** Parses `vecto.yaml`, validates task names and commands, checks that all declared dependencies exist, and constructs a populated `dag.Graph`.
- **Check:** `internal/config` unit tests parsing valid manifests and rejecting missing task targets.
- **Depends on:** none (package skeleton and `internal/dag` already exist and pass).

### Slice 2: Content-Addressable Cryptographic Hasher
- **ADRs:** ADR-0004
- **Does:** Evaluates input globs (`src/**/*.go`), streams file contents through SHA-256 in lexicographical order, hashes command strings and specified environment variables, and returns a 64-character hex digest.
- **Check:** `internal/hash` unit tests verifying deterministic output and hash invalidation on 1-byte file edits.
- **Depends on:** Slice 1

### Slice 3: Disk Cache Storage & Replay Engine
- **ADRs:** ADR-0003
- **Does:** Checks `.vecto/cache/<hash>`, stores metadata and compressed/raw output artifacts on task completion, and restores cached outputs and terminal logs during cache hits.
- **Check:** `internal/cache` unit test verifying storage, replay, and `VECTO_CACHE_DIR` environment override.
- **Depends on:** Slice 2

### Slice 4: Concurrent Worker Pool & Process Lifecycle Runner
- **ADRs:** ADR-0005
- **Does:** Dispatches tasks across worker goroutines based on `dag.ExecutionLayers()`, cancels in-flight siblings on error via `context.WithCancelCause`, and supports `-k`/`--keep-going`.
- **Check:** `internal/runner` integration test with concurrent mock jobs and simulated failure aborts.
- **Depends on:** Slice 1, Slice 3

### Slice 5: Interactive Terminal Status Dashboard
- **ADRs:** ADR-0002
- **Does:** Renders live in-place task status rows with animated spinners, elapsed timers, and collapsible error logs, with fallback for non-TTY/CI streams.
- **Check:** `internal/ui` test verifying output formatting in both TTY and pipe modes.
- **Depends on:** Slice 4

### Slice 6: CLI Integration & End-to-End Walk Verification
- **ADRs:** All ADRs
- **Does:** Wires `cmd/vecto` with `vecto run`, `vecto list`, and `vecto clean`. Provides an end-to-end demo project fixture.
- **Check:** Full integration test running `vecto run build` twice (verifying `[CACHED] 0.00s`).
- **Depends on:** Slices 1-5
