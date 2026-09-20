# ADR-0016: What the top of the README shows

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [What does the top of the README show?](https://github.com/Sehaan-1/vecto/issues/21)
- **Board:** [Week 3: Packaging](https://github.com/Sehaan-1/vecto/issues/20)
- **Supersedes:** none
- **Superseded by:** none

## Context
The README buried the install and first run under tables a newcomer must wade through. The architecture diagram was already praised and must stay. We chose between install plus 30-second run plus diagram at the top and keeping the current front matter.

## Decision
The top shows how to install, how to run once, and the picture. README is cut to: install, 30-second `vecto init && vecto run` trace (placeholder code block until a real GIF lands), then the existing architecture diagram. Longer tables and flag docs move down or to `docs/`.

## Consequences
What this means from now on:
- **People notice:** a 30-second path above the fold.
- **Later cards/ADRs must:** keep numbers in any remaining tables pinned to `docs/benchmarks.md`; any new top-of-README claim must survive a cold read.
- **We give up:** having every flag and comparison at the top.
- **Look/CI/proof:** README renders above the fold with install, trace, and diagram; comparison details still exist lower or in `docs/benchmarks.md`.

## History
- 2026-09-20 accepted
