# ADR-0017: How CI keeps its green promise

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How does CI keep its green promise?](https://github.com/Sehaan-1/vecto/issues/22)
- **Board:** [Week 3: Packaging](https://github.com/Sehaan-1/vecto/issues/20)
- **Supersedes:** none
- **Superseded by:** none

## Context
CI ran `go test -race` on three OSes plus benchmarks and the dogfood build, but had no lint and no coverage number. Seniors expect both as baseline packaging signals. We chose between tests plus fast lint plus a coverage percent and tests-only.

## Decision
CI checks tests, lint, and shows coverage. Every push runs `go test -race ./...` (matrix), `golangci-lint` (pinned action, fast path), and a coverage aggregation step that reports a percent without gating the build on a hard threshold yet. The status stays green only when tests and lint are green.

## Consequences
What this means from now on:
- **People notice:** the same green badge, plus a lint run and a coverage number in the log.
- **Later cards/ADRs must:** lint config stays strict enough to be honest but not flaky; coverage is reported, not yet a blocking gate; any new workflow must keep the 3-OS matrix.
- **We give up:** nothing; the push stays green unless real issues land.
- **Look/CI/proof:** CI run with `golangci-lint` step green and a coverage percent printed/aggregated.

## History
- 2026-09-20 accepted
