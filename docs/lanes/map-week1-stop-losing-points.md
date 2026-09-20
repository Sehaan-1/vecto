# Map: Week 1 Stop losing points

**For the team.** Cuecards decided. This file is how we ship it without colliding.

- **Handoff:** [handoff-week1-stop-losing-points](../cuecards/handoff-week1-stop-losing-points.md)
- **Board:** [Week 1: Stop losing points](https://github.com/Sehaan-1/vecto/issues/12)
- **Integration owner:** Sehaan
- **Status:** Destination Check passed
- **Time budget:** none
- **Spend ceiling:** none
- **Round:** 2
- **Agents in flight:** none

## Where we're headed
A Go repo a recruiter can trust: clean Go, correct concurrency, tested, documented, released.

## How we'll know we're there
- **Walk:** clone fresh, `go install`, `vecto run build` twice with second instant cached; a 100MB build output stores and restores without memory spike.
- **Proof:** `go test -race ./...` green; `TestCache_LargeArtifactSkippedFromMemory` + CLI `run()` test; `.goreleaser.yml` present; `git ls-files` shows no `*.exe` or `bin/vecto`.
- **Enforced:** CI `go test -race ./...` and `go vet ./...`; GoReleaser config checked in.

This is the locked target. Rounds do not rewrite it.

## Decisions we honor
- [ADR-0010 How people install it](../adr/0010-one-line-install-and-releases.md)
- [ADR-0011 How the command line is shaped for tests](../adr/0011-testable-cli-shape.md)
- [ADR-0012 What happens to huge files when saving](../adr/0012-large-artifact-memory-cap.md)

## Not this effort
- Beating Bazel on speed claims; full streaming rewrite of cache; multi-machine workers.

## Seams (shared contracts)
No new shared contract this round. Existing seams reused as-is:
### Seam: cache Manager API
- **Contract:** `internal/cache/cache.go` (`Store`, `Restore`, `Has`) — signatures unchanged
- **Owned by:** Lane C (no signature change, comment-only if needed)
- **Consumed by:** Lane B (callers unchanged)
- **Status:** written

## Lanes

### Lane A — Release hygiene
- **Check:** `git ls-files` shows no exe/bin; README has `go install`; `.goreleaser.yml` parses.
- **Does:** GoReleaser config + README install line + binary hygiene proof.
- **Handoff slices:** Slice 1
- **Owns (files/packages):** `.goreleaser.yml`, `README.md` (install section only), `.gitignore` (verify only)
- **Does not touch:** `cmd/vecto/`, `internal/cache/`
- **Needs seams:** none
- **Parallel with:** Lane B, Lane C
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [How do people install it?](https://github.com/Sehaan-1/vecto/issues/13)
- **Brief:** Lane A packet only — ADRs: ADR-0010.
- **Status:** unclaimed

### Lane B — Testable command line
- **Check:** `go test ./cmd/vecto` green; single `os.Exit` in `main.go`.
- **Does:** `run() int` + `usage.go` + `handlers.go`; flag alias fixed; handlers return status.
- **Handoff slices:** Slice 2
- **Owns (files/packages):** `cmd/vecto/`
- **Does not touch:** `internal/cache/`, `.goreleaser.yml`, `README.md`
- **Needs seams:** cache Manager API (consume only)
- **Parallel with:** Lane A, Lane C
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [How do we make the command line testable?](https://github.com/Sehaan-1/vecto/issues/14)
- **Brief:** Lane B packet only — ADRs: ADR-0011.
- **Status:** unclaimed

### Lane C — Memory-capped cache
- **Check:** `go test -race ./internal/cache -run TestCache_LargeArtifact` green.
- **Does:** 50MB remote-upload cap; streaming copy-and-hash; 100MB regression test.
- **Handoff slices:** Slice 3
- **Owns (files/packages):** `internal/cache/`
- **Does not touch:** `cmd/vecto/`, `.goreleaser.yml`, `README.md`
- **Needs seams:** cache Manager API (owns, no signature change)
- **Parallel with:** Lane A, Lane B
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [How do we stop huge files blowing up memory?](https://github.com/Sehaan-1/vecto/issues/15)
- **Brief:** Lane C packet only — ADRs: ADR-0012.
- **Status:** unclaimed

## Now / Next / Then
- **Now, in parallel:** A, B, C (disjoint files)
- **Next:** destination walk
- **Then:** done

## Integration
- Merge to the default branch after every lane Check, not only at the end
- After each merge, walk as far as the destination currently allows
- Final: full walk + proof + enforced

## Sitting profile (this round)
- **Takeable now:** none (destination shipped)
- **Idle / waiting on:** none
- **Bottleneck:** Round 2 gap was release plumbing: tag existed in plan but no workflow built binaries and no tag existed for `go install`. Fixed with release.yml + v0.1.0.
- **Duplicate work or duplicate context:** none
- **Walk before → after this round:** before: lanes landed, local-only proof. After: CI green on final code (run 35508359042), Release green with 6 binaries (run 35508423094), `go install @latest` + double-run-cached walk verified.

## What landed
- Lane A: `.goreleaser.yml` (tag builds win/linux/mac + checksums), README `go install` section; verified no tracked `*.exe`/`bin/vecto` (commit b2d66c0).
- Lane A follow-up: `.github/workflows/release.yml` tag-triggered GoReleaser (commit 007a16a); without it tags produced no binaries and `go install @latest` had no backing tag.
- Lane B: `cmd/vecto/main.go` thin starter, new `usage.go` + `handlers.go`, single `os.Exit`, clean long/short flag vars, `main_test.go` with 4 tests (commit 1b5c460).
- Lane C: `MaxRemoteArtifactBytes = 50<<20`, streaming `copyAndHashFile` with nil-bytes disk-only signal, `TestCache_LargeArtifactSkippedFromMemory` 100MB green under `-race` (commit 87b4a5a).
- Destination proof (independent GitHub Actions runs, not a local claim):
  - CI `go test -race ./...` + benchmarks + build, 3 OSes, green on final code: https://github.com/Sehaan-1/vecto/actions/runs/35508359042
  - Release v0.1.0, 6 platform binaries + checksums, green: https://github.com/Sehaan-1/vecto/actions/runs/35508423094
  - Release assets: https://github.com/Sehaan-1/vecto/releases/tag/v0.1.0
  - Local walk: `go install ...@latest` → `vecto version 0.1.0`; sample project `run build` twice → second run 3/3 cached, 0.00s.

## Blocked
nothing
