# Handoff: Week 2 One proof of real use

**For a building agent.** Do not start this from Cuecards. Cuecards sitting ends when this file is written.

## Provenance
- Board: [Week 2: One proof of real use](https://github.com/Sehaan-1/vecto/issues/16)
- For-you cards decided by human: 3 of 3
- Last human answer recorded: see issue #17, #18, #19 (user Week 2 brief, 2026-09-20)
- Handoff written by: agent

## Where we're headed
One believable story: vecto shares finished work between machines through a tiny remote server, builds itself in CI, and compares itself honestly against tools seniors actually use.

## How we'll know we're there
- Walk: start stub, cold run uploads, wipe local cache, hot run restores everything without re-running commands.
- Proof: `TestE2E_RemoteCache_ColdCleanHot` green; CI dogfood step green on 3 OSes; benchmarks doc shows measured turbo + just rows; `go test -race ./...` green.
- Enforced: CI `go test -race ./...` + `go vet ./...`; benchmark test skips (never fails) when turbo/just absent.

## Constraints (from Notes)
- Stdlib first; race-detector green; no emojis in new code; additive only.
- No S3/cloud; external tools local-only, never required by tests.
- Reporter glyphs (`[✓]`, `[CACHED]`) are pre-existing test-pinned output, not new code — untouched.

## ADRs this implements (in force)
- [ADR-0013 What proves the shared cache is real](../adr/0013-disk-remote-stub-and-proof.md)
- [ADR-0014 How our pipeline shows we trust our tool](../adr/0014-dogfood-ci-build.md)
- [ADR-0015 How the speed comparison stays honest](../adr/0015-honest-benchmark-comparison.md)

## Not this effort
- S3/GCS backends; hosted cache service; beating turbo on speed claims.

## Sequence

### Slice 1: Disk remote stub
- **ADRs:** ADR-0013
- **Does:** `cmd/vecto-server/main.go` (~120 lines): `PUT/GET/HEAD /:hash` to disk files, `--addr`/`--dir` flags, `run() int` shape, startup line with bound address for tests.
- **Check:** `go build ./cmd/vecto-server`; manual PUT then GET round-trip via test client.
- **Depends on:** none

### Slice 2: Remote E2E
- **ADRs:** ADR-0013
- **Does:** `test/e2e/remote_e2e_test.go`: builds both binaries, cold run uploads, wipes local cache + outputs, hot run restores all from remote with zero re-execution.
- **Check:** `go test -race ./test/e2e/ -run TestE2E_RemoteCache` green.
- **Depends on:** Slice 1 (consumes server binary contract only)

### Slice 3: Dogfood CI
- **ADRs:** ADR-0014
- **Does:** root `vecto.yaml` with portable `build` task; ci.yml build step becomes `go run ./cmd/vecto run build`.
- **Check:** CI green on all three OSes with vecto in the build log.
- **Depends on:** none (disjoint files; parallel safe)

### Slice 4: Honest benchmarks
- **ADRs:** ADR-0015
- **Does:** `turbo.json` + `package.json` scripts + `justfile` in multi-lang project; `TestBenchmark_ExternalRunners` measuring hot replay for turbo/just with skip-if-absent; docs rows + reframed headline.
- **Check:** test green locally with tools installed, skipped without; docs table updated.
- **Depends on:** none (owns benchmarks/ + docs/benchmarks.md; parallel safe)
