# ADR-0011: How the command line is shaped for tests

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How do we make the command line testable?](https://github.com/Sehaan-1/vecto/issues/14)
- **Board:** [Week 1: Stop losing points](https://github.com/Sehaan-1/vecto/issues/12)
- **Supersedes:** none
- **Superseded by:** none

## Context
`cmd/vecto/main.go` called quit directly ~19 times and mixed help text with real work, so tests could not call it. The `clean --max-age` / `-a` alias also shared one setting through two registrations with mismatched defaults. We chose between a small testable core with a thin starter (A) and leaving one file plus a flag patch (B).

## Decision
Tests can run our commands and check the result. `main()` only calls `os.Exit(run(args, stdout, stderr))`. Command handlers return int status, live in `handlers.go`, help text lives in `usage.go`. Long/short flags share one variable registered once per name with the same default.

## Consequences
What this means from now on:
- **People notice:** nothing in daily use; commands behave the same.
- **Later cards/ADRs must:** new CLI behavior returns a status instead of quitting; new help text goes in `usage.go`.
- **We give up:** keeping everything in one big file.
- **Look/CI/proof:** `cmd/vecto` unit test calling `run()` and asserting exit codes; `go vet ./...` clean.

## History
- 2026-09-20 accepted
