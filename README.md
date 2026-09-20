# Vecto

A language-agnostic Directed Acyclic Graph (DAG) task runner and content-addressed build cache written in Go.

[![CI](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml/badge.svg)](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Sehaan-1/vecto)](https://goreportcard.com/report/github.com/Sehaan-1/vecto)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Overview

In multi-language projects, build scripts and CI pipelines frequently repeat work whose inputs have not changed, or run independent steps serially due to rigid phase definitions.

**Vecto** provides a single, zero-dependency binary that:
1. **Dispatches tasks as dependencies resolve:** Tasks fire as soon as their direct prerequisites finish, rather than waiting for an entire execution phase or sibling task to complete.
2. **Computes content-addressed fingerprints:** Deterministic SHA-256 hashes are calculated across task commands, input file contents (supporting `**` recursive globs and ignore files), declared environment variables, and transitive upstream hashes.
3. **Guarantees cache integrity:**
   - **Schema versioning:** Cache metadata tracks schema versions, cleanly invalidating older or incompatible formats.
   - **Integrity validation:** Output logs and artifacts are verified against recorded SHA-256 hashes and byte sizes on restore, preventing truncated or corrupted cache entries from entering workspaces.
   - **Atomic staging:** Cache entries are written into isolated staging directories on the same filesystem and committed via atomic filesystem rename (`os.Rename`), protected by cross-platform file locking (`flock` on Unix, Win32 `LockFileEx` on Windows).
   - **Remote cache support:** Built-in HTTP REST remote cache client for sharing cache entries across team members and CI pipelines.
4. **Manages subprocess lifecycles:** Subprocesses are launched in dedicated process groups. On cancellation, Vecto broadcasts termination signals across the entire process tree (`SIGTERM` with 1-second `SIGKILL` escalation on Unix, process-tree termination on Windows).
5. **Surfaces developer tooling:** Native graph export to Mermaid and Graphviz DOT, predictive `--dry-run` inspection, dynamic flag forwarding (`--`), structured logging (`slog`), and machine-readable JSON summary output (`--json`) for CI.

---

## Empirical Benchmarks

The following benchmarks were measured on a 13th Gen Intel Core i7-1355U (12 threads) running on a realistic multi-language repository containing Go, Python, and JavaScript services ([detailed methodology & reproduction steps in `docs/benchmarks.md`](docs/benchmarks.md)):

### Vecto vs. GNU Make

| Scenario | GNU Make (`mingw32-make`) | Vecto | Comparison / Impact |
|---|---|---|---|
| **Clean Cold Build** | `3.63s` | `1.14s` | **3.18x faster** (automatic dependency parallelism) |
| **Incremental Build (1 file modified)** | `1.50s` | `1.10s` | **1.37x faster** (26.9% wall-clock time saved) |
| **Hot Replay (0 files modified)** | `773 ms` | `17.5 ms` | **44.2x faster** (content-addressed cache hit) |
| **Parallel Cold Build (`make -j`)** | `1.06s` | `1.18s` | **0.90x** (within 120ms of Make `-j`, with caching enabled) |

### Micro-Benchmarks & Latency

| Benchmark | Latency / Throughput | Notes |
|---|---|---|
| **Cache Hit Latency (7 tasks)** | **~8.0 ms** median ($p_{50}$), $10.7\text{ ms}$ ($p_{95}$) | Measured over 100 consecutive full replays |
| **Topological Sort (1,000 tasks)** | **< 0.5 ms** | In-place min-heap Kahn's algorithm |
| **Topological Sort (10,000 tasks)** | **~2.8 ms** | O((V+E) log V) with zero interface allocations |
| **Cancellation Teardown** | **~565 ms** | Subprocesses terminated promptly on sibling failure |

To reproduce locally:
```bash
go test -v ./benchmarks -run TestBenchmark_MultiLang_Comparison
go test -run='^$' -bench=. -benchmem ./...
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
                         │ internal/config │  (Schema Validation & Typo Diagnostics)
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
│ (SHA-256 + Glob) │                              │ (Atomic & Lock)  │
└────────┬─────────┘                              └────────┬─────────┘
         │                                                 │
         └────────────────────────┬────────────────────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │ internal/runner │  (Event-Driven Dispatch & Tree Teardown)
                         └────────┬────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │   internal/ui   │  (Dashboard, JSON Summary & Slog)
                         └─────────────────┘
```

### 1. Event-Driven Scheduling
Unlike layer-based task runners that block until all tasks in an entire phase finish, Vecto tracks a `pendingDeps[task]` counter. The instant a task completes, it decrements the counter for all its direct dependents. When a task's counter reaches zero, it enters the ready queue immediately.

### 2. Atomic Directory Staging and File Locking
To ensure cache entries are never written partially:
- Outputs, metadata, and logs are written into `.vecto/cache/tmp-<hash>-<pid>-<timestamp>` on the same filesystem volume.
- Output artifacts and logs have their SHA-256 hashes recorded in `meta.json`.
- A cross-platform file lock is held while committing the directory via an atomic filesystem rename (`os.Rename`).
- During `Restore()`, artifact sizes and checksums are verified before files are restored to the workspace.

### 3. Subprocess Group Teardown
When a task fails or the run context is cancelled:
1. On Unix, `syscall.SIGTERM` is broadcast to `-pgid` (the process group), allowing compilers and child processes a 1-second grace window to flush buffers and release file locks before escalating to `syscall.SIGKILL`.
2. On Windows, process tree termination is performed via `taskkill /F /T`, terminating child processes and releasing standard I/O handles promptly.

---

## Comparison

| Feature | GNU Make | Vecto | Turborepo | Bazel |
|---|---|---|---|---|
| **Language Ecosystem** | Any | Any (Language-Agnostic) | Node.js / JS focus | Multi-language (Starlark) |
| **Dependencies** | C compiler / binary | Single binary (Go, zero deps) | Node.js / Rust binary | Java, Python, C++ |
| **Cache Key Calculation** | File timestamps (`mtime`) | SHA-256 content hashes | Hash across inputs & deps | Content-addressed CAS |
| **Atomic Cache Commits** | No | Yes (`os.Rename` + staging) | Yes (tarball archive) | Yes |
| **Corruption Detection** | No | Yes (SHA-256 verification) | Archive checksums | CAS merkle-tree |
| **Remote Cache** | No | Yes (HTTP REST) | Yes (Vercel remote cache) | Yes (gRPC RBE) |
| **CI Integration** | Text output | JSON summary (`--json`) | JSON / Turborepo cloud | BEP (Build Event Protocol) |

---

## Quickstart

### 1. Install
```bash
go install github.com/Sehaan-1/vecto/cmd/vecto@latest
```

Prebuilt binaries for Windows, macOS, and Linux are attached to each git tag via GoReleaser (see `.goreleaser.yml`).

### 2. Build from Source
```bash
git clone https://github.com/Sehaan-1/vecto.git
cd vecto
go build -o bin/vecto ./cmd/vecto
```

### 2. Configuration (`vecto.yaml`)
Create `vecto.yaml` in your repository root:
```yaml
version: "1"

tasks:
  codegen:
    command: "go run ./scripts/generate.go"
    inputs: ["schema.json"]
    outputs: ["generated/routes.go"]

  lint:
    command: "golangci-lint run"
    inputs: ["**/*.go"]

  test:
    command: "go test ./..."
    deps: ["codegen"]
    inputs: ["**/*.go", "generated/routes.go"]

  build:
    command: "go build -o bin/app ."
    deps: ["test", "lint"]
    inputs: ["**/*.go"]
    outputs: ["bin/app"]
```

### 3. CLI Usage

```bash
# Run a target task and its dependencies
vecto run build

# Preview execution plan and cache status without executing commands
vecto run --dry-run build

# Output machine-readable JSON summary for CI pipelines
vecto run --json build

# Forward dynamic arguments to underlying task commands
vecto run test -- -v -run TestUserAuth

# Export graph topology as Mermaid.js or Graphviz DOT
vecto graph --format=mermaid
vecto graph --format=dot build

# Run with custom concurrency (default: CPU threads)
vecto run --concurrency 4

# Keep unaffected independent tasks running on failure
vecto run --keep-going

# Bypass cache and force re-execution
vecto run --force

# Stream command stdout/stderr for all tasks
vecto run --verbose build

# Clear or prune local cache
vecto clean
vecto clean --max-age 24h
```

---

## Configuration Diagnostics

Vecto validates task definitions at startup:
- **Typo suggestions:** If a task references an unknown dependency, Vecto calculates Levenshtein distances and suggests the closest match (e.g. `Task "test" depends on "compyle". Did you mean "compile"?`).
- **Self-dependencies & cycles:** Cycle paths are reported explicitly (`Cycle detected: A -> B -> C -> A`).
- **Group tasks:** Tasks that define dependencies without commands are supported as logical group targets.
- **Overlapping paths:** Warnings are emitted if an output file is declared as an input to the same task.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
