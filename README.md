# Vecto

A language-agnostic DAG task runner and content-addressed build cache in Go.

[![CI](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml/badge.svg)](https://github.com/Sehaan-1/vecto/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Sehaan-1/vecto)](https://goreportcard.com/report/github.com/Sehaan-1/vecto)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Install

```bash
go install github.com/Sehaan-1/vecto/cmd/vecto@latest
# or: download a tagged binary from Releases (built by GoReleaser)
```

## 30 seconds

```bash
vecto init
# writes vecto.yaml with one sample task

vecto run build
# [-] build ... running
# [✓] build (0.41s)
# Tasks: 1 total (0 cached, 1 executed)

vecto run build
# [CACHED] build (0.02s)
# second run restores from cache, no commands re-run

vecto graph --format=mermaid  # or --json for CI
```
> GIF: `vecto init && vecto run` in a real repo. Placeholder above is the exact terminal trace until the recording lands in `docs/assets/demo.gif`.

## Architecture

```
                           ┌──────────────┐
                           │  vecto.yaml  │
                           └──────┬───────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │ internal/config │  (Schema Validation & Typo Diagnostics)
                         └────────┬────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │  internal/dag   │  (Kahn's Min-Heap Topo Sort & Visualizer)
                         └────────┬────────┘
                                  │
         ┌────────────────────────┴────────────────────────┐
         │                                                 │
         ▼                                                 ▼
┌──────────────────┐                              ┌──────────────────┐
│  internal/hash   │                              │  internal/cache  │
│ (SHA-256 + Glob) │                              │ (Atomic & Lock)  │
└────────┬─────────┘                              └────────┬─────────┘
         │                                                 │
         └────────────────────────┬────────────────────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │ internal/runner │  (Event-Driven Dispatch & Tree Teardown)
                         └────────┬────────┘
                                  │
                                  ▼
                         ┌─────────────────┐
                         │   internal/ui   │  (Dashboard, JSON Summary & Slog)
                         └─────────────────┘
```

* Event-driven scheduler, SHA-256 transitive hashing, atomic cache with cross-platform locks — see [ADR-0006](docs/adr/0006-reactive-event-driven-scheduler.md), [ADR-0004](docs/adr/0004-cryptographic-input-hashing.md), [ADR-0008](docs/adr/0008-atomic-cache-staging.md).
* **Resume bullets:** "Built content-addressed DAG runner in Go: event-driven scheduler, SHA-256 transitive hashing, atomic cache with cross-platform locks" and "17ms hot replay (7 tasks), 2.8ms topo-sort on 10k nodes, 3-way CI" — numbers from [`docs/benchmarks.md`](docs/benchmarks.md).

---

## What it does

* Dispatches tasks as dependencies resolve (event-driven, not phase-blocked).
* Content-addressed fingerprints over commands, input files (`**` globs + ignore files), env vars, and upstream hashes.
* Cache integrity: schema versioning, SHA-256 verification, atomic staging + file locks, optional HTTP remote cache.
* Process-group teardown (`SIGTERM` then `SIGKILL` / `taskkill /F /T`).
* Tooling: `vecto graph`, `--dry-run`, `--` passthrough, `slog` logging, `--json` summary.

## Benchmarks (measured)

| Scenario | Make | Vecto |
|---|---|---|
| Hot replay (7 tasks) | 773 ms | 17.5 ms |
| Topo-sort 10k nodes | — | 2.8 ms |

Same-machine Turbo/Just rows and full method in [`docs/benchmarks.md`](docs/benchmarks.md). Hot vs Make flatters any cacher (Make has no cache); the honest cacher-vs-cacher table is in section 6 there.

## Configuration

```yaml
version: "1"
tasks:
  build:
    command: "go build -o bin/app ."
    deps: ["test"]
    inputs: ["**/*.go"]
    outputs: ["bin/app"]
```

Full flag reference and diagnostics (`did you mean ...?`, cycle paths, overlapping inputs) in [`docs/`](docs/) and `vecto --help`.

## License

MIT — see [LICENSE](LICENSE).
