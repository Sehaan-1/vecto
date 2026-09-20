# ADR-0014: How our pipeline shows we trust our tool

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How do we show we trust our own tool?](https://github.com/Sehaan-1/vecto/issues/18)
- **Board:** [Week 2: One proof of real use](https://github.com/Sehaan-1/vecto/issues/16)
- **Supersedes:** none
- **Superseded by:** none

## Context
CI built vecto with a plain `go build` line while the README claims vecto builds projects. We chose between one pipeline step running our own build file (A) and docs-only claims (B).

## Decision
Our pipeline builds itself with vecto. A root `vecto.yaml` defines a `build` task compiling `./cmd/vecto`, and the CI build step runs it via `go run ./cmd/vecto run build` so no vendored binary is needed and all three OS runners work unchanged.

## Consequences
What this means from now on:
- **People notice:** build logs show vecto building vecto.
- **Later cards/ADRs must:** keep the root build task portable (no shell-only commands); CI must keep passing on Windows, macOS, Linux.
- **We give up:** the plain direct build command for that step.
- **Look/CI/proof:** CI run with the dogfood step green on all three OSes.

## History
- 2026-09-20 accepted
