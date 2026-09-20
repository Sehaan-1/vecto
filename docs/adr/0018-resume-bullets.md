# ADR-0018: What resume facts we can honestly claim

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [What resume facts can we honestly claim?](https://github.com/Sehaan-1/vecto/issues/23)
- **Board:** [Week 3: Packaging](https://github.com/Sehaan-1/vecto/issues/20)
- **Supersedes:** none
- **Superseded by:** none

## Context
The requested resume closes Week 3 with two bullets that name the stack and four measured numbers. We chose between exactly those two bullets with a named rerun and softened buzzwords without numbers.

## Decision
Both bullets stand as written and anyone can rerun them. Bullet 1: "Built content-addressed DAG runner in Go: event-driven scheduler, SHA-256 transitive hashing, atomic cache with cross-platform locks". Bullet 2: "17ms hot replay (7 tasks), 2.8ms topo-sort on 10k nodes, 3-way CI". Numbers are pinned to `docs/benchmarks.md` and reproducible via `go test -run` / `go test -bench` commands documented there.

## Consequences
What this means from now on:
- **People notice:** two refusable claims, not vague speed.
- **Later cards/ADRs must:** any number in README or resume must have a rerun command in `docs/benchmarks.md`; no new speed claim without a measurement on the same project.
- **We give up:** a punchier but vaguer claim.
- **Look/CI/proof:** `docs/benchmarks.md` sections 3-4 and `TestBenchmark_*` tests back the numbers; README table matches.

## History
- 2026-09-20 accepted
