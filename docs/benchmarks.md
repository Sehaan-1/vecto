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
