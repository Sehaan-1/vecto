# ADR-0004: Cryptographic input hashing for cache fingerprints

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How do we detect changed files?](https://github.com/Sehaan-1/vecto/issues/5)
- **Board:** [Vecto Task Runner](https://github.com/Sehaan-1/vecto/issues/1)
- **Supersedes:** none
- **Superseded by:** none

## Context
A caching task runner must reliably detect whether task inputs have changed. We evaluated:
1. Full cryptographic SHA-256 hashing (command line + input file contents + environment variables).
2. Modification timestamp (`mtime`) and file size checks.
3. Git tree commit object hashing.

## Decision
Task cache keys are calculated using deterministic SHA-256 digests of the exact command string, sorted input file contents matched via globs, and specified environment variables.

## Consequences
What this means from now on:
- **People notice:** 100% deterministic, zero false skips or false rebuilds. Modifying file contents always invalidates cache; touching a file without changing bytes always hits cache.
- **Later cards/ADRs must:** Implement chunked streaming SHA-256 file hashing in `internal/hash`.
- **We give up:** Dependency on Git CLI or fragile filesystem timestamps.
- **Look/CI/proof:** Validated by `internal/hash` unit tests verifying hash mutation on single-byte changes.

## History
- 2026-09-20 accepted
