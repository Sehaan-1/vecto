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

## 7. 100,000-File Incremental Detection (Vecto legacy vs VFI vs Turbo)

Per [ADR-0019](adr/0019-merkle-file-index-incremental-fingerprinting.md): this is the
proof that the Merkle File Index (VFI) makes input fingerprinting O(changed files) and
holds up against Turborepo 2.x at 100k-file scale.

### Test Environment

- **Machine:** linux/amd64, 2 vCPU, ~4 GB RAM, ext4 (`/dev/root`), Go 1.27.1
- **Tools:** Vecto (this repo), Turborepo 2.11.2 (global `turbo`, Node v24)
- **Date:** 2026-09-20
- **Tree:** generated, 500 packages × 200 files = **100,000 files** (~15 MB,
  deterministic contents), plus `vecto.yaml` / `turbo.json` / `package.json`.
- **Workload:** identical 3-task chain — `lint` → `test` → `build`, every task with
  `inputs: ["packages/**"]` and an `echo` command. Vecto tasks and Turbo tasks declare
  the same graph and the same inputs.
- **Warm = second run**, with all files predating the first sync by >1s, so the
  measurement is a pure 0-change pass (no one-time racy re-hash, per git's racy rule).
- **Vecto numbers** measure the runner in-process (identical code path to the CLI,
  minus process startup). **Turbo numbers** are `turbo run build` wall time via the
  installed binary; git mode uses a committed repo with the daemon warmed by the first
  run; non-git mode has no `.git` (Turbo's SCM then falls back to manual walking +
  re-hashing, which is what the source shows: `/tmp/turbo-src`, S1 in
  [findings/003](findings/003-incremental-detection-at-100k-files.md)).

### Results

| Runner / mode | 2nd run, 0 files modified | 2nd run after 1-file edit |
|---|---|---|
| **Vecto legacy** (pre-ADR-0019, `--no-file-index`) | `5.69s` (walk + re-hash every input, per task) | `5.69s` (same full scan, 3 tasks) |
| **Vecto VFI** (Merkle File Index, on by default) | **`0.59s`** (1 stat cascade, 0 file reads) | **`1.21s`** (stat cascade + exactly 1 re-hash) |
| **Turbo 2.11.2 (git + daemon)** | `0.58s` (FULL TURBO) | not measured |
| **Turbo 2.11.2 (non-git)** | `1.02s` (manual SCM: full walk + re-hash, every run) | not measured |

**VFI vs Vecto legacy: 9.6× faster on the 0-change warm run, 4.7× faster after a
1-file edit.** **VFI vs Turbo (non-git): 1.7× faster.** **VFI vs Turbo (git): parity
(0.59s vs 0.58s).**

### How to read this

- **The 9.6×/4.7× vs legacy is the algorithmic claim.** Vecto's legacy path re-walks and
  re-reads all 100k inputs once *per task* (3× duplication here). VFI pays one parallel
  stat cascade per run and re-hashes only what changed — 0 files when nothing changed.
- **Turbo's best mode is git mode.** Its fast path is `.git/index` stat comparison plus
  blob-OID reuse, so without a git repo it degrades to a full walk + re-hash every run —
  exactly the 1.02s row. VFI reaches the same 0-change performance class without git,
  without a daemon process, and without a Node.js runtime: one static Go binary.
- **Parity with Turbo-git is the honest statement, not "beats Turbo".** Turbo's
  git fast path is the same idea (stat cache + content-addressed reuse) with a
  daemon; VFI matches it at 100k files on a 2-vCPU box while remaining dependency-free.
  Where VFI wins outright is the non-git world and every comparison against its own
  previous behavior.
- **The k=1 row is the day-to-day one.** One edited file: VFI re-hashes that file plus
  O(depth) ancestor directories and re-fingerprints the affected tasks; the 99,999
  untouched files are stat-compared only. Legacy re-reads all 100k × 3 tasks.

### Reproduce

```bash
# full 100k matrix (includes Turborepo when `turbo` is on PATH); ~1 min
VECTO_HUNDREDK=1 go test -v ./benchmarks/hundredk -run TestBenchmark_HundredK

# lightweight 500-file smoke matrix (default, CI-friendly, Vecto only)
go test -v ./benchmarks/hundredk -run TestBenchmark_HundredK

# cost-model diagnostics (crawl vs load vs sync decomposition, raw Lstat rates)
VECTO_PROFILE=1 go test -v ./internal/fileindex -run 'TestProfile100k|TestRawLstatCost'
```

The benchmark generator writes the tree to a temp dir per scenario, so each runner
measured above sees identical bytes. Numbers are single-run walls on the machine above;
the spread between repeated runs on this box is <±10% for the warm rows.
