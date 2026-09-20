# ADR-0015: How the speed comparison stays honest

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How do we compare without looking naive?](https://github.com/Sehaan-1/vecto/issues/19)
- **Board:** [Week 2: One proof of real use](https://github.com/Sehaan-1/vecto/issues/16)
- **Supersedes:** none
- **Superseded by:** none

## Context
The benchmarks page led with 44x hot-replay speedup over GNU Make, which has no cache — seniors read that as a stacked deck. We chose between measuring real rivals on the same project (A) and dropping comparisons (B).

## Decision
Our numbers sit next to the rivals, measured the same way. `docs/benchmarks.md` keeps existing rows and adds measured Turbo (caching rival, via npx) and Just (plain runner) rows on the identical multi-language project, with reproduction steps. The benchmark test skips gracefully when a tool is not installed so CI never depends on external tools.

## Consequences
What this means from now on:
- **People notice:** the headline win is reframed; rival rows are measured, not guessed.
- **Later cards/ADRs must:** new benchmark claims ship with method + machine + rerun command; no rival row without a measurement.
- **We give up:** the punchy 44x headline as the lead.
- **Look/CI/proof:** `TestBenchmark_ExternalRunners` green locally, skipped in CI when tools absent.

## History
- 2026-09-20 accepted
