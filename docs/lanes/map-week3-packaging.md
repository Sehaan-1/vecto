# Map: Week 3 Packaging

**For the team.** Cuecards decided. This file is how we ship it without colliding.

- **Handoff:** [handoff-week3-packaging](../cuecards/handoff-week3-packaging.md)
- **Board:** [Week 3: Packaging](https://github.com/Sehaan-1/vecto/issues/20)
- **Integration owner:** Sehaan
- **Status:** Destination Check passed
- **Time budget:** none
- **Spend ceiling:** none
- **Round:** 2
- **Agents in flight:** none

## Where we're headed
A recruiter opens the repo and in 30 seconds knows how to install it, sees it run, and trusts the diagram and badges without scrolling through a manual.

## How we'll know we're there
- **Walk:** open README on GitHub, see install then vecto init and vecto run trace, see the architecture diagram, click CI badge and it is green.
- **Proof:** README top fits one screen with install + trace + diagram; CI runs go test -race plus lint and a coverage percent on all three OSes; resume numbers match docs/benchmarks.md.
- **Enforced:** CI fails if lint fails; README table numbers match measured docs.

This is the locked target. Rounds do not rewrite it.

## Decisions we honor
- [ADR-0016 What the top of the README shows](../adr/0016-readme-top-packaging.md)
- [ADR-0017 How CI keeps its green promise](../adr/0017-ci-lint-and-coverage.md)
- [ADR-0018 What resume facts we can honestly claim](../adr/0018-resume-bullets.md)

## Not this effort
- New runtime features; hosted cache; rewriting benchmarks this week.

## Seams (shared contracts)
No shared contract this round. README and CI own disjoint files.

## Lanes

### Lane A — README packaging
- **Check:** README renders install + 30s trace + diagram above the fold; numbers cited match docs/benchmarks.md and feed the two resume bullets.
- **Does:** cut README top to install, 30-second vecto init + run trace (placeholder for GIF), keep architecture diagram, push longer tables/flags lower (ADR-0016, ADR-0018).
- **Handoff slices:** Slice 1
- **Owns (files/packages):** `README.md`, `docs/benchmarks.md` (numbers only if they drift), `docs/assets/` (GIF placeholder if needed)
- **Does not touch:** `.github/workflows/ci.yml`, `.golangci.yml`
- **Needs seams:** none
- **Parallel with:** Lane B
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [What does the top of the README show?](https://github.com/Sehaan-1/vecto/issues/21)
- **Brief:** Lane A packet only — ADRs: ADR-0016, ADR-0018.
- **Status:** unclaimed

### Lane B — CI packaging
- **Check:** CI green on 3 OSes with lint and coverage percent visible; go test -race still the gate.
- **Does:** ci.yml adds golangci-lint step and coverage aggregation; repo adds .golangci.yml; 3-OS matrix and dogfood build stay green (ADR-0017).
- **Handoff slices:** Slice 2
- **Owns (files/packages):** `.github/workflows/ci.yml`, `.golangci.yml`
- **Does not touch:** `README.md`, `docs/`
- **Needs seams:** none
- **Parallel with:** Lane A
- **Waits on:** none
- **Claimed by:** agent
- **Ticket:** [How does CI keep its green promise?](https://github.com/Sehaan-1/vecto/issues/22)
- **Brief:** Lane B packet only — ADRs: ADR-0017.
- **Status:** unclaimed

## Now / Next / Then
- **Now, in parallel:** A, B (disjoint files)
- **Next:** destination walk
- **Then:** done

## Integration
- Merge to the default branch after every lane Check, not only at the end
- After each merge, walk as far as the destination currently allows
- Final: full walk + proof + enforced

## Sitting profile (this round)
- **Takeable now:** none (destination shipped)
- **Idle / waiting on:** none
- **Bottleneck:** lint config used wrong top-level key plus gofmt/lint noise; fixed with strict .golangci.yml plus errcheck exclusions and bash coverage on Windows.
- **Duplicate work or duplicate context:** none
- **Walk before → after this round:** before: manual-heavy README top, no lint/coverage. After: README top is install plus 30s trace plus arch; CI lint green plus coverage 64.1% on all 3 OSes.

## What landed
- Lane A: README packaging cut (commit 2cd1acc) — install plus 30s trace plus arch above the fold, resume bullets pinned.
- Lane B: CI packaging — .golangci.yml plus ci.yml lint plus coverage report (commit bf54eb4, fixed in 1c0d68e and a18978c).
- Destination proof: CI green on 3 OSes with lint plus coverage (run 35516298860, coverage 64.1 percent).

## Blocked
nothing
