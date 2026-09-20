# Vecto

A high-performance, language-agnostic Directed Acyclic Graph (DAG) task runner and content-addressable build caching engine written in Go.

[![CI](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml/badge.svg)](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Sehaan-1/vecto)](https://goreportcard.com/report/github.com/Sehaan-1/vecto)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Overview

In modern software projects, build and test pipelines often waste significant time running independent tasks sequentially, or re-executing steps whose inputs have not changed.

**Vecto** solves this by combining formal graph theory with industrial-grade systems engineering:
1. **Reactive Event-Driven Scheduler:** Tasks fire the instant their dependencies complete via a dependency-counter queue, eliminating head-of-line blocking from slow sibling tasks.
2. **Cryptographic Content-Addressed Caching:** Deterministic SHA-256 fingerprinting calculated across commands, input file contents, environment variables, and **transitive upstream dependency fingerprints**.
3. **Atomic Cache Staging (`os.Rename`):** Cache writes are isolated in temporary staging directories on the same filesystem and committed atomically, preventing cache poisoning from `SIGINT` cancellations or concurrent runs.
4. **Two-Phase Process Teardown (`SIGTERM` $\to$ `SIGKILL`):** Subprocesses run in dedicated OS process groups (`Setpgid: true`). On cancellation, `SIGTERM` is broadcast across the process tree giving compilers a grace period to flush disk buffers before escalating to `SIGKILL`.
5. **Developer Experience & Inspection:** Native graph visualization export (`mermaid` and `dot`), predictive `--dry-run` execution planning, and dynamic CLI argument passthrough (`--`).

---

## Benchmarks & Performance

Measured on an Intel Core i7-1355U (12 threads) with Go 1.22+ (raw benchmark output committed in [docs/benchmarks.txt](docs/benchmarks.txt) and automated in CI on every push):

| Benchmark | Operations / Iterations | Latency per Op | Memory / Allocs |
|---|---|---|---|
| **Cache Replay / Restore** | 13,464 ops | **~0.08 ms** (`84 µs`) | 2.8 KB / 16 allocs |
| **Topological Sort (1,000 nodes)** | 7,290 ops | **~0.16 ms** (`163 µs`) | 87 KB / 7 allocs |
| **Execution Layer Partitioning (1,000 nodes)** | 4,596 ops | **~0.24 ms** (`243 µs`) | 165 KB / 527 allocs |
| **SHA-256 Streaming Hash (100 files)** | 139 ops | **~9.1 ms** | 3.5 MB / 1,438 allocs |

To run benchmarks locally:
```bash
go test -run='^$' -bench=. -benchmem ./internal/...
```

---

## Architecture & Systems Design

```
                           ┌──────────────┐
                           │  vecto.yaml  │
                           └──────┬───────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │ internal/config │
                         └────────┬────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │  internal/dag   │  (Kahn's Min-Heap Topo Sort & Visualizer)
                         └────────┬────────┘
                                  │
         ┌────────────────────────┴────────────────────────┐
         │                                                 │
         ▼                                                 ▼
┌──────────────────┐                              ┌──────────────────┐
│  internal/hash   │                              │  internal/cache  │
│ (SHA-256 Finger) │                              │ (Atomic Staging) │
└────────┬─────────┘                              └────────┬─────────┘
         │                                                 │
         └────────────────────────┬────────────────────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │ internal/runner │  (Reactive Scheduler & Two-Phase Teardown)
                         └────────┬────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │   internal/ui   │  (Thread-Safe Terminal Dashboard)
                         └─────────────────┘
```

### 1. Reactive Event-Driven Scheduler (No Head-of-Line Blocking)
Unlike naive batch/layer task runners where an entire layer must finish before any task in the next layer begins, Vecto tracks a `pendingDeps[task]` counter. When a task completes, it decrements the counter for all its direct dependents. The instant a task's counter hits zero, it enters the ready queue and is dispatched to an available worker goroutine.

### 2. Atomic Directory Staging (`os.Rename`)
To eliminate cache corruption if a build is killed mid-write:
- Output logs, metadata, and artifacts are written into `.vecto/cache/tmp-<hash>-<pid>-<timestamp>` on the same filesystem volume.
- Once all writes and checksums succeed, the entry is committed via an atomic filesystem rename (`os.Rename`).
- A cache entry is either 100% complete or does not exist at all.

### 3. Two-Phase Signal Escalation Ladder
When a sibling task fails or the runner context is cancelled:
1. `terminateProcessGroup` broadcasts `syscall.SIGTERM` to `-pgid` (the entire process tree, not just `/bin/sh`).
2. Child compilers and test suites can catch `SIGTERM`, flush disk buffers, and cleanly shut down.
3. If the process does not terminate within a 1-second grace window, Vecto escalates to `syscall.SIGKILL`.

### 4. Transitive Content Fingerprinting
If library `A` changes, application `B` (which depends on `A`) must rebuild even if `B`'s own source files did not change:
$$\text{Fingerprint}(B) = \text{SHA-256}(\text{Cmd}_B + \text{Inputs}_B + \text{Env}_B + \text{Fingerprint}(A))$$

---

## What Vecto Is vs. What It Is Not

| Feature | Vecto | Turborepo | Bazel |
|---|---|---|---|
| **Language Ecosystem** | Language-Agnostic | Node.js / TS focus | Multi-language (Starlark) |
| **Runtime Dependencies** | **None** (single standalone binary) | Node.js / Rust | Java / Python / C++ |
| **Configuration** | Flat `vecto.yaml` | `turbo.json` | `WORKSPACE` + `BUILD` |
| **Process Isolation** | OS Process Groups & Two-Phase Kill | Cross-platform kill | Chroot / sandbox containers |
| **Cache Consistency** | Atomic Directory Staging (`os.Rename`) | Tarball unpacking | Content-addressed CAS |
| **Distributed Workers** | No (Single-machine focus) | Vercel Remote Cache | Remote Build Execution (RBE) |

**Vecto is designed for:** Teams that want fast, reproducible, and cached task pipelines across mixed languages (Go, Python, Rust, Node, C++) without the setup complexity of Bazel or the JavaScript-centric coupling of Turborepo.

---

## Quickstart

### 1. Installation
Clone and build the standalone binary:
```bash
git clone https://github.com/Sehaan-1/vecto.git
cd vecto
go build -o bin/vecto ./cmd/vecto
```

### 2. Configuration (`vecto.yaml`)
Run `vecto init` or create a `vecto.yaml` in your project root:
```yaml
version: "1"

tasks:
  codegen:
    command: "go run ./scripts/generate.go"
    inputs: ["schema.json"]

  lint:
    command: "golangci-lint run"
    inputs: ["**/*.go"]

  build:
    command: "go build -o bin/app ."
    deps: ["codegen", "lint"]
    inputs: ["**/*.go"]
    outputs: ["bin/app"]
```

### 3. CLI Commands

```bash
# Run a target task and all of its dependencies
vecto run build

# Preview execution plan and cache hits without running shell commands
vecto run --dry-run build

# Pass dynamic flags directly through to the underlying task command
vecto run test -- -v -run TestUserAuth

# Export graph topology as Mermaid.js (for GitHub markdown) or Graphviz DOT
vecto graph --format=mermaid
vecto graph --format=dot build

# Run with custom concurrency (default: CPU cores)
vecto run --concurrency 4

# Keep unaffected independent tasks running on failure
vecto run --keep-going

# Bypass cache and force re-execution
vecto run --force

# Stream command stdout/stderr for all tasks
vecto run --verbose build

# List declared tasks and dependency relationships
vecto list

# Clear entire local cache storage (.vecto/cache)
vecto clean

# Prune cache entries older than a duration (e.g. 24h, 7d)
vecto clean --max-age 24h
```

---

## Testing & Quality

Vecto enforces strict race detection across all internal packages:
```bash
go test -v -race ./...
```
Continuous integration is automated via GitHub Actions on every push and pull request.

---

## Architecture Decision Records (ADRs)

Key architectural choices are formally documented in [`docs/adr/`](docs/adr/):
- [ADR-0001: Task definitions in root YAML](docs/adr/0001-task-definitions-in-root-yaml.md)
- [ADR-0002: Live terminal status dashboard](docs/adr/0002-live-terminal-status-dashboard.md)
- [ADR-0003: Hybrid cache storage directory](docs/adr/0003-hybrid-cache-storage-directory.md)
- [ADR-0004: Cryptographic input hashing](docs/adr/0004-cryptographic-input-hashing.md)
- [ADR-0005: Task failure lifecycle and keep-going](docs/adr/0005-fail-fast-with-keep-going.md)
- [ADR-0006: Reactive event-driven scheduler](docs/adr/0006-reactive-event-driven-scheduler.md)
- [ADR-0007: Process group isolation and two-phase teardown](docs/adr/0007-process-group-isolation.md)
- [ADR-0008: Atomic cache staging via staging directory and rename](docs/adr/0008-atomic-cache-staging.md)
- [ADR-0009: Graph inspection and dry-run execution mode](docs/adr/0009-graph-inspection-and-dry-run.md)

---

## License
MIT License. See [LICENSE](LICENSE) for details.
