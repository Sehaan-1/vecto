# Vecto Performance & Benchmark Validation

This document records empirical performance benchmarks comparing **Vecto** against **GNU Make** (`mingw32-make` / `make`) across realistic scenarios, as well as micro-benchmarks measuring scheduler overhead, cache hit latencies, and process cancellation teardown times.

---

## 1. Realistic Multi-Language Repository Comparison

### Test Environment
- **OS:** Windows 11 (amd64)
- **CPU:** 13th Gen Intel(R) Core(TM) i7-1355U (12 threads)
- **Runtimes:** Go 1.22, Python 3.11.9, Node.js v24.15.0
- **Make Implementation:** GNU Make (`mingw32-make` 3.81 / 4.x)
- **Repository Structure:** [`benchmarks/multi_lang_project/`](../benchmarks/multi_lang_project/)
  - Go microservice (`services/api/math.go`, `main.go`, unit tests)
  - Python data processing module (`scripts/data_processor.py`, tests)
  - TypeScript/Node web asset bundler (`web/index.js`, tests)
  - Code generation from schema (`schema/api.json`)
  - Identical task graph modeled in both `Makefile` and `vecto.yaml`

### Results

| Scenario | GNU Make Wall-Clock | Vecto Wall-Clock | Relative Performance |
|---|---|---|---|
| **Clean Cold Build** | `3.63s` | `1.14s` | **3.18x faster** (automatic dependency-level parallelism) |
| **Incremental Build (1 file modified)** | `1.50s` | `1.10s` | **1.37x faster** (26.9% wall-clock time saved) |
| **Hot Replay (0 files modified)** | `773 ms` | `17.5 ms` | **44.2x faster** (sub-20ms content-addressed replay) |
| **Parallel Cold Build (`make -j`)** | `1.06s` | `1.18s` | **0.90x** (within 120ms of Make -j while providing caching) |

> **Reading the Hot Replay row honestly:** GNU Make keeps no cache, so a
> replay comparison against it flatters any caching runner. The fair fight
> is cacher vs cacher — see section 6 for Turbo measured on the same project.

---

## 2. Cache Hit Latency Distribution

Measured over 100 consecutive full-graph cache replays across 7 interdependent tasks:

| Metric | Wall-Clock Latency (7 tasks) | Effective Latency Per Task |
|---|---|---|
| **Minimum** | `6.50 ms` | `0.93 ms` |
| **Median ($p_{50}$)** | `8.04 ms` | `1.15 ms` |
| **$p_{95}$** | `10.67 ms` | `1.52 ms` |
| **$p_{99}$** | `18.40 ms` | `2.63 ms` |
| **Maximum** | `18.40 ms` | `2.63 ms` |

---

## 3. DAG Scalability (100, 1,000, and 10,000 Tasks)

Synthetic layered DAG generation measuring Kahn's min-heap topological sort and parallel execution layer partitioning:

| Graph Size | Topological Sort Latency | Layer Partitioning Latency | Verified Output Layers |
|---|---|---|---|
| **100 tasks** | `< 0.1 ms` | `< 0.1 ms` | 10 layers |
| **1,000 tasks** | `< 0.5 ms` | `0.54 ms` | 100 layers |
| **10,000 tasks** | `2.78 ms` | `3.82 ms` | 1,000 layers |

> [!NOTE]
> Even on a 10,000-node task dependency graph, Vecto computes the complete topological order and execution tiers in under 7 milliseconds total.

---

## 4. Failure and Cancellation Teardown Latency

Measured on a task graph where 1 task fails immediately while 3 independent sibling tasks attempt to sleep for 5 seconds:

| Mode | Wall-Clock Duration | Behavior |
|---|---|---|
| **Fail-Fast (`--keep-going=false`)** | `565 ms` | Sibling processes aborted promptly via process-group escalation / `taskkill /T /F` |
| **Keep-Going (`--keep-going=true`)** | `5.16 s` | Independent unaffected tasks run to completion; dependents of failed task skipped |

---

## 5. How to Reproduce Locally

To execute this benchmark suite on your own machine:

```bash
# Run the multi-language comparison and scalability benchmarks
go test -v ./benchmarks -run TestBenchmark_MultiLang_Comparison

# Run cache hit latency distribution
go test -v ./benchmarks -run TestBenchmark_CacheHit_LatencyDistribution

# Run synthetic scalability and failure cancellation tests
go test -v ./benchmarks -run 'TestBenchmark_(Scalability|FailureTeardown)'

# Run all internal Go benchmarks with memory allocation profiling
go test -run='^$' -bench=. -benchmem ./...
```

---

## 6. Same-Machine Runner Comparison (Vecto vs Turbo vs Just vs Make)

Same project (`benchmarks/multi_lang_project/`), same machine, each runner on a
fresh copy. Turbo and Just run the equivalent task graph defined in
`turbo.json`/`package.json` and `justfile`; Make uses the existing `Makefile`.

### Test Environment
- **OS:** Windows 11 (amd64)
- **CPU:** 13th Gen Intel(R) Core(TM) i7-1355U
- **Runtimes:** Go 1.27.0, Python 3.11.9, Node.js v24.15.0
- **Runners:** Vecto (this repo), Turbo 2.11.2 (`turbo` global install), Just 1.58.0, GNU Make (`mingw32-make`)
- **Date:** 2026-09-20

### Results

| Runner | Cold Build | Second Run (0 files modified) | Cache? |
|---|---|---|---|
| **Vecto** | `7.5s` | `0.09s` (7/7 cached) | content-hash |
| **Turbo** | `4.9s` | `0.44s` wall, `92ms` tasks (7/7 FULL TURBO) | content-hash |
| **Just** | `6.6s` | `3.6s` (re-executes everything) | none |
| **Make** | `5.8s` | `3.1s` (re-executes everything) | none |

### How to read this
- **Cold is toolchain-dominated** (Go test compilation alone is ~4s here) and all
  runners share the warm Go module cache, so cold gaps say little. The
  meaningful column is the second run.
- **Cacher vs cacher is sub-second on both sides.** Vecto restores faster here;
  Turbo pays daemon + archive overhead. No 44x-style headline — that number only
  ever existed against cache-less Make.
- **Just and Make re-execute** (only Go's own test cache shortcuts one step),
  which is exactly the pain caching runners remove.

### Reproduce
```bash
go test -v ./benchmarks -run TestBenchmark_ExternalRunners
```
Requires `turbo`, `just` (plus `sh` on Windows), and `git` on PATH; the test
skips cleanly when any is missing. Turbo hashes git-tracked files, so the test
initializes a scratch repo with caches, outputs, and Python bytecode untracked.
