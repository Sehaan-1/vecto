# ADR-0013: What proves the shared cache is real

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [What proves the shared cache is real?](https://github.com/Sehaan-1/vecto/issues/17)
- **Board:** [Week 2: One proof of real use](https://github.com/Sehaan-1/vecto/issues/16)
- **Supersedes:** none
- **Superseded by:** none

## Context
The HTTP remote backend exists but no server speaks its protocol, so sharing is unproven. We chose between a tiny disk server plus a real two-run test (A) and a test-only fake (B). S3 is explicitly out for this spike.

## Decision
A cold run uploads, a wiped machine restores, no commands re-ran. `cmd/vecto-server` (~120 lines, stdlib only) implements exactly the three calls the client makes: `PUT /:hash`, `GET /:hash`, `HEAD /:hash`, stored as files on disk. `test/e2e/remote_e2e_test.go` runs both real binaries: cold run, wipe local cache, hot run fully restored from remote.

## Consequences
What this means from now on:
- **People notice:** a runnable sharing story with inspectable disk files.
- **Later cards/ADRs must:** keep the server protocol byte-compatible with `RemoteBundle`; any protocol change needs a versioned ADR first.
- **We give up:** hosted/cloud blob storage this week.
- **Look/CI/proof:** `TestE2E_RemoteCache_ColdCleanHot` green; server follows the `run() int` shape from ADR-0011.

## History
- 2026-09-20 accepted
