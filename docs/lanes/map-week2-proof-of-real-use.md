# Map: Week 2 One proof of real use

**For the team.** Cuecards decided. This file is how we ship it without colliding.

- **Handoff:** [handoff-week2-proof-of-real-use](../cuecards/handoff-week2-proof-of-real-use.md)
- **Board:** [Week 2: One proof of real use](https://github.com/Sehaan-1/vecto/issues/16)
- **Integration owner:** Sehaan
- **Status:** Destination Check passed
- **Time budget:** none
- **Spend ceiling:** none
- **Round:** 2
- **Agents in flight:** none

## Where we're headed
One believable story: vecto shares finished work between machines through a tiny remote server, builds itself in CI, and compares itself honestly against tools seniors actually use.

## How we'll know we're there
- **Walk:** start stub, cold run uploads, wipe local cache, hot run restores everything without re-running commands.
- **Proof:** `TestE2E_RemoteCache_ColdCleanHot` green; CI dogfood step green on 3 OSes; benchmarks doc shows measured turbo + just rows; `go test -race ./...` green.
- **Enforced:** CI `go test -race ./...` + `go vet ./...`; benchmark test skips (never fails) when turbo/just absent.

This is the locked target. Rounds do not rewrite it.

## Decisions we honor
- [ADR-0013 What proves the shared cache is real](../adr/0013-disk-remote-stub-and-proof.md)
- [ADR-0014 How our pipeline shows we trust our tool](../adr/0014-dogfood-ci-build.md)
- [ADR-0015 How the speed comparison stays honest](../adr/0015-honest-benchmark-comparison.md)

## Not this effort
- S3/GCS backends; hosted cache service; beating turbo on speed claims.

## Seams (shared contracts)
### Seam: remote HTTP protocol
- **Contract:** `internal/cache/remote.go` (`PUT/GET/HEAD /:hash`, JSON `RemoteBundle`) — already written, unchanged
- **Owned by:** nobody this round (frozen)
- **Consumed by:** Lane A (server implements it), Lane B (test exercises it)
- **Status:** written

## Lanes

### Lane A — Disk remote stub
- **Check:** `go build ./cmd/vecto-server`; PUT/GET/HEAD round-trip works.
- **Does:** `cmd/vecto-server/main.go` (~120 lines, stdlib, no emojis), `run() int` shape.
- **Handoff slices:** Slice 1
- **Owns (files/packages):** `cmd/vecto-server/`
- **Does not touch:** `test/e2e/`, `vecto.yaml`, `.github/`, `benchmarks/`, `docs/`
- **Needs seams:** remote HTTP protocol (implement only)
- **Parallel with:** B, C, D
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [What proves the shared cache is real?](https://github.com/Sehaan-1/vecto/issues/17)
- **Brief:** Lane A packet only — ADRs: ADR-0013.
- **Status:** unclaimed

### Lane B — Remote E2E
- **Check:** `go test -race ./test/e2e/ -run TestE2E_RemoteCache` green.
- **Does:** `test/e2e/remote_e2e_test.go` driving both real binaries.
- **Handoff slices:** Slice 2
- **Owns (files/packages):** `test/e2e/remote_e2e_test.go`
- **Does not touch:** `cmd/vecto-server/`, `vecto.yaml`, `.github/`, `benchmarks/`, `docs/`
- **Needs seams:** remote HTTP protocol (consume only; test builds server from source)
- **Parallel with:** A, C, D
- **Waits on:** none (builds server from source, needs no Lane A commit)
- **Claimed by:** agent
- **Ticket:** [What proves the shared cache is real?](https://github.com/Sehaan-1/vecto/issues/17)
- **Brief:** Lane B packet only — ADRs: ADR-0013.
- **Status:** unclaimed

### Lane C — Dogfood CI
- **Check:** CI green on 3 OSes with vecto in the build log.
- **Does:** root `vecto.yaml` + one-line ci.yml build step change.
- **Handoff slices:** Slice 3
- **Owns (files/packages):** `vecto.yaml` (root), `.github/workflows/ci.yml` (build step only)
- **Does not touch:** `cmd/`, `test/`, `benchmarks/`, `docs/`
- **Needs seams:** none
- **Parallel with:** A, B, D
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [How do we show we trust our own tool?](https://github.com/Sehaan-1/vecto/issues/18)
- **Brief:** Lane C packet only — ADRs: ADR-0014.
- **Status:** unclaimed

### Lane D — Honest benchmarks
- **Check:** external-runner test green locally / skipped in CI; docs rows added.
- **Does:** `turbo.json` + `package.json` + `justfile` fixtures, benchmark test addition, `docs/benchmarks.md` (+ README table) update.
- **Handoff slices:** Slice 4
- **Owns (files/packages):** `benchmarks/`, `docs/benchmarks.md`, `README.md` (benchmark table only)
- **Does not touch:** `cmd/`, `test/`, `vecto.yaml`, `.github/`
- **Needs seams:** none
- **Parallel with:** A, B, C
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [How do we compare without looking naive?](https://github.com/Sehaan-1/vecto/issues/19)
- **Brief:** Lane D packet only — ADRs: ADR-0015.
- **Status:** unclaimed

## Now / Next / Then
- **Now, in parallel:** A, B, C, D (disjoint files; A/B share only the frozen protocol)
- **Next:** destination walk
- **Then:** done

## Integration
- Merge to the default branch after every lane Check, not only at the end
- After each merge, walk as far as the destination currently allows
- Final: full walk + proof + enforced

## Sitting profile (this round)
- **Takeable now:** none (destination shipped)
- **Idle / waiting on:** none
- **Bottleneck:** Lane D measurement fought turbo's git-hashing twice (committed `.turbo`, then tracked outputs) plus Python bytecode churning hashes; fixed with untracked caches/outputs/`__pycache__`. Lane B exposed a real lost-upload race, fixed with `WaitForUploads`.
- **Duplicate work or duplicate context:** none
- **Walk before → after this round:** before: unproven remote, plain CI build, Make-only comparison. After: cold-clean-hot green, CI builds itself, measured turbo/just rows.

## What landed
- Lane A: `cmd/vecto-server/main.go` (123 lines, stdlib, no emojis), PUT/GET/HEAD verified live (commit 54ee930).
- Lane B: `test/e2e/remote_e2e_test.go` cold-clean-hot green in 12s + `Manager.WaitForUploads` fix in `internal/cache` + `cmd/vecto/handlers.go` (commit 41252cc).
- Lane C: root `vecto.yaml` + ci.yml dogfood step; CI log shows vecto building vecto on all 3 OSes (commit 887a063).
- Lane D: `turbo.json` + `package.json` + `justfile` fixtures, `TestBenchmark_ExternalRunners` (skips cleanly without tools), docs section 6 same-machine table, 44x reframed (commit d8dcca1).
- Destination proof (independent GitHub Actions runs):
  - CI `go test -race ./...` + benchmarks + dogfood build, 3 OSes, green: https://github.com/Sehaan-1/vecto/actions/runs/35513670737

## Blocked
nothing
