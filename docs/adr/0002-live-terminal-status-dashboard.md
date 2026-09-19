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
The terminal displays structured status rows per task (`[-] ... running`, `[✓] ... duration`, `[⚡ CACHED] 0.00s`), capturing output and cleanly printing full command stdout/stderr only on task failure.

## Consequences
What this means from now on:
- **People notice:** Snappy developer feedback with immediate visibility of task start, completion, cache hits, and compiler error dumps on failure.
- **Later cards/ADRs must:** Route task stdout/stderr through thread-safe buffers in `internal/ui`.
- **We give up:** Direct raw unbounded dumping of child process stdout straight to the OS terminal handle.
- **Look/CI/proof:** Validated by `internal/ui` unit tests verifying thread-safe reporting and error output capture.

## History
- 2026-09-20 accepted
