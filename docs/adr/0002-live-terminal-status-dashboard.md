# ADR-0002: How parallel progress looks on screen

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How should parallel progress look on screen?](https://github.com/Sehaan-1/vecto/issues/3)
- **Board:** [Vecto Task Runner](https://github.com/Sehaan-1/vecto/issues/1)
- **Supersedes:** none
- **Superseded by:** none

## Context
When executing 4 or 8 tasks concurrently, terminal output can easily collide. We considered:
1. Dynamic live-updating dashboard with animated status spinners per task and collapsible logs.
2. Interleaved prefixed raw stdout streaming.
3. Completely quiet summaries writing logs only to files.

## Decision
The terminal displays a dynamic, in-place updating status dashboard with task spinners and elapsed time, gracefully falling back to clean line-by-line streaming in non-interactive CI environments.

## Consequences
What this means from now on:
- **People notice:** Snappy, high-polish developer feedback with zero visual flicker and clean failure reporting.
- **Later cards/ADRs must:** Route task stdout/stderr through thread-safe buffers in `internal/ui`.
- **We give up:** Direct raw unbounded dumping of child process stdout straight to the OS terminal handle.
- **Look/CI/proof:** Validated by `internal/ui` interactive and non-interactive TTY unit tests.

## History
- 2026-09-20 accepted
