# Handoff: Week 1 Stop losing points

**For a building agent.** Do not start this from Cuecards. Cuecards sitting ends when this file is written.

## Provenance
- Board: [Week 1: Stop losing points](https://github.com/Sehaan-1/vecto/issues/12)
- For-you cards decided by human: 3 of 3
- Last human answer recorded: see issue #13, #14, #15 (user Week 1 brief, 2026-09-20)
- Handoff written by: agent

## Where we're headed
A Go repo a recruiter can trust: clean Go, correct concurrency, tested, documented, released. No committed binaries, install in one line, CLI code a test can call, no RAM blow-up on big files.

## How we'll know we're there
- Walk: clone fresh, `go install`, `vecto run build` twice with second instant cached; a 100MB build output stores and restores without memory spike.
- Proof: `go test -race ./...` green; `TestCache_LargeArtifactSkippedFromMemory` + CLI `run()` test; `.goreleaser.yml` present; `git ls-files` shows no `*.exe` or `bin/vecto`.
- Enforced: CI `go test -race ./...` and `go vet ./...`; GoReleaser config checked in.

## Constraints (from Notes)
- Stdlib first; race-detector clean; deterministic cache keys.
- No committed binaries; no secrets; additive changes only.
- Existing ADRs 0001-0009 stay in force.

## ADRs this implements (in force)
- [ADR-0010 How people install it](../adr/0010-one-line-install-and-releases.md)
- [ADR-0011 How the command line is shaped for tests](../adr/0011-testable-cli-shape.md)
- [ADR-0012 What happens to huge files when saving](../adr/0012-large-artifact-memory-cap.md)

## Not this effort
- Beating Bazel on speed claims; full streaming rewrite of cache; multi-machine workers.

## Sequence
Ordered slices. Each slice is independently checkable. Cite ADRs. Name the walk/proof this slice advances. Do not paste product code.

### Slice 1: Release hygiene
- **ADRs:** ADR-0010
- **Does:** `.goreleaser.yml` for tag builds (Windows/Mac/Linux + checksums); README gains `go install ...@latest`; verify `vecto.exe`/`bin/` untracked.
- **Check:** `git ls-files | grep -E 'exe|bin/vecto'` empty; `goreleaser check` or YAML parse passes; README contains `go install`.
- **Depends on:** none

### Slice 2: Testable CLI
- **ADRs:** ADR-0011
- **Does:** `cmd/vecto/main.go` shrinks to `main(){os.Exit(run(...))}`; new `usage.go` (help text) and `handlers.go` (init/clean/list/graph/run/dry-run); handlers return int; long/short flags share one var cleanly.
- **Check:** `go test ./cmd/vecto -run TestRun_` green; `go vet ./cmd/vecto` clean; only one `os.Exit` in `main.go`.
- **Depends on:** none (disjoint files from Slice 1; parallel safe)

### Slice 3: Memory-capped cache
- **ADRs:** ADR-0012
- **Does:** `MaxRemoteArtifactBytes = 50<<20`; large artifacts stored locally but skipped from in-memory remote map; `copyAndHashFile` streams without full-file buffer for large files; 100MB regression test.
- **Check:** `go test -race ./internal/cache -run TestCache_LargeArtifact` green; `go test -race ./...` green.
- **Depends on:** none (owns `internal/cache/`; parallel safe with 1-2 via seams below)
