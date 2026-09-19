# Finding 001: Evaluation of Custom DAG Topological Sort vs External Library

- **Issue:** [#7 Evaluate custom DAG topological sort vs library](https://github.com/Sehaan-1/vecto/issues/7)
- **Status:** Complete
- **Date:** 2026-09-20
- **Author:** Vecto Engine Team

## Question
Does an in-repo Kahn's algorithm implementation provide all required graph operations (cycle detection, dependency resolution, parallel layer partitioning) without external dependencies?

## Evaluation & Benchmarks

### 1. Requirements
To drive Vecto's parallel build runner, the graph engine needs:
- Directed dependency registration (`A depends on B`).
- Cycle detection that pinpoints the exact offending path (`A -> B -> C -> A`).
- Topological sort for sequential validation.
- Concurrency partitioning (`ExecutionLayers`) so workers can fire off tasks in parallel tiers.

### 2. External Libraries Tested
- `gonum.org/v1/gonum/graph`: Very powerful mathematical graph library, but pulls heavy linear algebra dependencies, requires converting string task names to integer node IDs, and yields generic error codes.
- `heimdal/dag`: Lightweight, but adds third-party dependencies and lacks native parallel tier layer partitioning.

### 3. Measured In-Repo Kahn's Algorithm Implementation (`internal/dag`)
We implemented an idiomatic, zero-external-dependency Kahn's algorithm in `internal/dag/dag.go`:
- **Dependencies:** Go Standard Library only (`sort`, `strings`, `fmt`).
- **Deterministic Lexicographical Order:** Implemented via an in-place concrete typed string min-heap priority queue, guaranteeing $O((V + E) \log V)$ time with zero interface boxing allocations (avoiding $O(V^2 \log V)$ naive queue re-sorting).
- **Benchmark Results (Intel Core i7-1355U, Go 1.22+):**
  - `BenchmarkDAG_1000Nodes_TopologicalSort`: **~0.16 ms** (`163 µs/op`), 87 KB memory, **7 allocations**.
  - `BenchmarkDAG_1000Nodes_ExecutionLayers`: **~0.24 ms** (`243 µs/op`), 165 KB memory, 527 allocations.
- **Error reporting:** Backtracking DFS produces human-readable error chains: `cycle detected in task graph: A -> B -> C -> A`.
- **Parallel layers:** Native `ExecutionLayers()` partitions tasks into concurrent execution waves.

## Conclusion & Recommendation
Adopt the in-repo custom implementation in `internal/dag`. It completely avoids third-party bloat, compiles in under 0.1s, and guarantees zero supply chain attack surface.
