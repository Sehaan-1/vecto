# Handoff: Week 3 Packaging

**For a building agent.** Do not start this from Cuecards. Cuecards sitting ends when this file is written.

## Provenance
- Board: [Week 3: Packaging](https://github.com/Sehaan-1/vecto/issues/20)
- For-you cards decided by human: 3 of 3
- Last human answer recorded: see issue #21, #22, #23 (user Week 3 brief, 2026-09-20)
- Handoff written by: agent

## Where we're headed
A recruiter opens the repo and in 30 seconds knows how to install it, sees it run, and trusts the diagram and badges without scrolling through a manual.

## How we'll know we're there
- Walk: open README on GitHub, see install then vecto init and vecto run trace, see the architecture diagram, click CI badge and it is green.
- Proof: README top fits one screen of PHI? no — one screen of scrolling with install + trace + diagram; CI runs go test -race plus lint and a coverage percent on all three OSes; resume numbers match docs/benchmarks.md.
- Enforced: CI fails if lint fails; README table numbers match measured docs.

## Constraints (from Notes)
- Stdlib first; race-detector green; no emojis in new code; additive only; badges must stay green.

## ADRs this implements (in force)
- [ADR-0016 What the top of the README shows](../adr/0016-readme-top-packaging.md)
- [ADR-0017 How CI keeps its green promise](../adr/0017-ci-lint-and-coverage.md)
- [ADR-0018 What resume facts we can honestly claim](../adr/0018-resume-bullets.md)

## Not this effort
- New runtime features; hosted cache; rewriting benchmarks this week.

## Sequence

### Slice 1: README packaging
- **ADRs:** ADR-0016, ADR-0018
- **Does:** cut README top to install, 30-second vecto init + run code trace (placeholder for GIF), keep architecture diagram, push longer tables/flags lower, ensure numbers match docs/benchmarks.md and feed the two resume bullets.
- **Check:** README renders install + trace + diagram above the fold; numbers cited match docs/benchmarks.md.
- **Depends on:** none

### Slice 2: CI packaging
- **ADRs:** ADR-0017
- **Does:** ci.yml adds golangci-lint step (pinned) and a coverage aggregation/report step; repo adds .golangci.yml; existing 3-OS matrix and dogfood build stay green.
- **Check:** CI green on 3 OSes with lint and coverage percent visible; go test -race still the gate.
- **Depends on:** none (disjoint files from Slice 1; parallel safe)
