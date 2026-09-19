# ADR-0003: Where saved build output lives

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [Where does saved build output live?](https://github.com/Sehaan-1/vecto/issues/4)
- **Board:** [Vecto Task Runner](https://github.com/Sehaan-1/vecto/issues/1)
- **Supersedes:** none
- **Superseded by:** none

## Context
Vecto promises instant 0-second cache replay by caching command metadata, logs, and declared output files. We evaluated:
1. Purely local project-root directory (`.vecto/cache/`).
2. Machine-wide global directory (`~/.cache/vecto/`).
3. Configurable hybrid (defaults to `.vecto/cache/`, respects `XDG_CACHE_HOME` or `VECTO_CACHE_DIR`).

## Decision
Caches and build proofs are stored in `.vecto/cache/` by default for self-contained repos, but can be redirected to custom or shared cache directories via `VECTO_CACHE_DIR` / `XDG_CACHE_HOME`.

## Consequences
What this means from now on:
- **People notice:** Standard projects remain 100% self-contained and cleanable with `rm -rf .vecto`, while CI pipelines can easily point the cache to persistent shared runner mounts.
- **Later cards/ADRs must:** Query the cache root resolver in `internal/cache` before creating or retrieving cache tarballs.
- **We give up:** Hardcoded assumptions about fixed filesystem paths.
- **Look/CI/proof:** Validated by directory creation and retrieval tests in `internal/cache`.

## History
- 2026-09-20 accepted
