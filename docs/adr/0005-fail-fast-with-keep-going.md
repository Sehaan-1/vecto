# ADR-0005: Task failure lifecycle and keep-going execution

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How should task failures be handled?](https://github.com/Sehaan-1/vecto/issues/6)
- **Board:** [Vecto Task Runner](https://github.com/Sehaan-1/vecto/issues/1)
- **Supersedes:** none
- **Superseded by:** none

## Context
When running parallel tasks, failures are inevitable. We evaluated:
1. Immediate fail-fast: cancelling all other active tasks instantly.
2. Draining independent tasks: completing unaffected parallel work.
3. Hybrid: fail-fast by default, with an explicit `--keep-going` flag.

## Decision
Vecto will fail-fast by default, sending graceful termination signals to active sibling tasks and skipping pending work. An optional `--keep-going` / `-k` CLI flag allows independent tasks to finish.

## Consequences
What this means from now on:
- **People notice:** Instant feedback when iterating locally, with the power to collect full test failure suites in CI.
- **Later cards/ADRs must:** Wire `context.WithCancelCause` and process tree signal handling in `internal/runner`.
- **We give up:** Arbitrary unhandled zombie child processes running in the background.
- **Look/CI/proof:** Validated by runner lifecycle tests in `internal/runner` with mock failing tasks.

## History
- 2026-09-20 accepted
