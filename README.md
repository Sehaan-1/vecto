# Vecto

A high-performance, language-agnostic Directed Acyclic Graph (DAG) task runner and content-addressable build caching engine written in Go.

[![CI](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml/badge.svg)](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Sehaan-1/vecto)](https://goreportcard.com/report/github.com/Sehaan-1/vecto)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Overview

In modern software projects, build and test pipelines often waste significant time running independent tasks sequentially, or re-executing steps whose inputs have not changed.

**Vecto** solves this by:
1. **Parallel Execution via DAG:** Parsing tasks and their dependencies into an execution graph, partitioning independent tasks into concurrent execution waves across available CPU cores.
2. **Cryptographic Content-Addressed Caching:** Calculating a deterministic SHA-256 fingerprint from the command string, input file contents, environment variables, and **transitive upstream dependency fingerprints**.
3. **Sub-Millisecond Replay (`[⚡ CACHED]`):** Restoring logs and output artifacts directly from content-addressable storage when inputs have not changed.
4. **Resilient Failure Lifecycle:** Failing fast on the first broken step by gracefully terminating sibling processes, while supporting `--keep-going` (`-k`) for CI test collection.

---

## Benchmarks & Performance

Measured on an Intel Core i7-1355U (12 threads) with Go 1.22+ (raw benchmark output committed in [docs/benchmarks.txt](docs/benchmarks.txt) and automated in CI on every push):

| Benchmark | Operations / Iterations | Latency per Op | Memory / Allocs |
|---|---|---|---|
| **Cache Replay / Restore** | 12,488 ops | **~0.09 ms** (`93 µs`) | 2.8 KB / 16 allocs |
| **Topological Sort (1,000 nodes)** | 3,810 ops | **~0.31 ms** (`315 µs`) | 155 KB / 377 allocs |
| **Execution Layer Partitioning (1,000 nodes)** | 4,719 ops | **~0.25 ms** (`252 µs`) | 165 KB / 527 allocs |
| **SHA-256 Streaming Hash (100 files)** | 132 ops | **~10.3 ms** | 3.4 MB / 1,438 allocs |

To run benchmarks locally:
```bash
go test -run='^$' -bench=. -benchmem ./internal/...
```

---

## Architecture & Design Decisions

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
                         │  internal/dag   │  (Kahn's Algorithm & Cycle Detection)
                         └────────┬────────┘
                                  │
         ┌────────────────────────┴────────────────────────┐
         │                                                 │
         ▼                                                 ▼
┌──────────────────┐                              ┌──────────────────┐
│  internal/hash   │                              │  internal/cache  │
│ (SHA-256 Finger) │                              │ (.vecto/cache/)  │
└────────┬─────────┘                              └────────┬─────────┘
         │                                                 │
         └────────────────────────┬────────────────────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │ internal/runner │  (Worker Pool & Process Lifecycle)
                         └────────┬────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │   internal/ui   │  (Thread-Safe Terminal Progress)
                         └─────────────────┘
```

### 1. Kahn's Algorithm & Layer Partitioning
Rather than using generic external graph libraries, Vecto implements a zero-dependency Kahn's algorithm with indegree tracking in `internal/dag`:
- **Cycle Detection:** If an unresolved dependency cycle exists (e.g., `A → B → C → A`), a depth-first search backtracker formats the exact offending chain in the error output.
- **Concurrent Waves:** Tasks are partitioned into `ExecutionLayers` — tasks in Layer 0 run in parallel; once all complete, Layer 1 executes.

### 2. Transitive Dependency Hashing
A fundamental design requirement in build systems: if library `A` changes, application `B` (which depends on `A`) must rebuild, even if `B`'s own source files did not change.
Vecto implements **transitive fingerprinting**:
$$\text{Fingerprint}(B) = \text{SHA-256}(\text{Cmd}_B + \text{Inputs}_B + \text{Env}_B + \text{Fingerprint}(A))$$

### 3. File Permissions Preservation
Cached artifacts are archived with their exact POSIX file permissions (`info.Mode()`), ensuring restored compiled binaries retain executable bits (`0755`) upon cache replay.

---

## What Vecto Is vs. What It Is Not

| Feature | Vecto | Turborepo | Bazel |
|---|---|---|---|
| **Language Ecosystem** | Language-Agnostic | Node.js / TS focus | Multi-language (Starlark) |
| **Runtime Dependencies** | **None** (single standalone binary) | Node.js / Rust | Java / Python / C++ |
| **Configuration** | Flat `vecto.yaml` | `turbo.json` | `WORKSPACE` + `BUILD` |
| **Hermetic Sandboxing** | No (runs native processes) | Partial | Full (chroot/sandbox) |
| **Distributed Remote Workers** | No (Single-machine focus) | Vercel Remote Cache | Remote Build Execution (RBE) |

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

### 3. Commands
```bash
# Run a target task and all of its dependencies
vecto run build

# Run with custom concurrency (default: CPU cores)
vecto run --concurrency 4

# Keep unaffected independent tasks running on failure
vecto run --keep-going

# Bypass cache and force re-execution
vecto run --force

# Stream command stdout/stderr for all tasks (including successful and cached)
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

## License
MIT License. See [LICENSE](LICENSE) for details.
