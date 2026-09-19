# ADR-0001: Where task definitions live

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [Where do task definitions live?](https://github.com/Sehaan-1/vecto/issues/2)
- **Board:** [Vecto Task Runner](https://github.com/Sehaan-1/vecto/issues/1)
- **Supersedes:** none
- **Superseded by:** none

## Context
Vecto needs to parse task commands, dependency graphs, and declared file inputs/outputs. We evaluated three approaches:
1. Single root configuration file (`vecto.yaml`).
2. Auto-detecting existing build tool files (`package.json`, `Makefile`).
3. Distributed per-folder configurations.

## Decision
Tasks and their dependencies are declared in a single, explicit `vecto.yaml` file located in the project root.

## Consequences
What this means from now on:
- **People notice:** Clear, single-file overview of the entire repository pipeline without hunting through subdirectories.
- **Later cards/ADRs must:** Build configuration parsers, hash calculations, and task loaders targeting `vecto.yaml`.
- **We give up:** Multi-format guessing heuristics across foreign file types (`Makefile`, `package.json`).
- **Look/CI/proof:** Validated by `internal/config` YAML decoder and sample `vecto.yaml` fixtures.

## History
- 2026-09-20 accepted
