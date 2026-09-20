# ADR-0010: How people install it

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How do people install it?](https://github.com/Sehaan-1/vecto/issues/13)
- **Board:** [Week 1: Stop losing points](https://github.com/Sehaan-1/vecto/issues/12)
- **Supersedes:** none
- **Superseded by:** none

## Context
Install means building from source, and a stray `vecto.exe` sits in the folder. A committed binary reads as sloppy to recruiters. We chose between one-line install plus automatic releases (A) and build-from-source only (B).

## Decision
People install vecto with one line, and tags make the files. We add `.goreleaser.yml` (tag-triggered Windows/Mac/Linux builds with checksums) plus a `go install github.com/Sehaan-1/vecto/cmd/vecto@latest` line in README. `vecto.exe` and `bin/` stay untracked via `.gitignore`.

## Consequences
What this means from now on:
- **People notice:** one copy-paste install; a Releases page with files per tag.
- **Later cards/ADRs must:** keep the release config working when build flags change; version is tag-as-source-of-truth.
- **We give up:** hand-building files per machine.
- **Look/CI/proof:** `.goreleaser.yml` in repo; README install line; `git ls-files` shows no `*.exe` or `bin/vecto`.

## History
- 2026-09-20 accepted
