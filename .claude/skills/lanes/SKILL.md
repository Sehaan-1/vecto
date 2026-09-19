---
name: lanes
description: "Use when shipping the whole Cuecards destination in one sitting, when a team needs a shared engineering map across parallel workstreams, or when the user asks to build everything, ship it, run all the slices, split work so several people or agents can move at once, loop until shipped, or run against a time or spend budget. Do not use to decide product questions (cuecards) or to implement a single named slice or ticket (oneslice). Do not use as a UI or visual-fidelity skill."
disable-model-invocation: true
---

# Lanes

Ship the **whole** destination. Put a map in the repo so a team can move in parallel without colliding. **Lock the target, then loop** until the walk works. Do not stop because slice one passed.

Cuecards decided. The handoff sequenced. Oneslice is how *one* slice is cut to a hard quality bar. This skill is how the destination actually lands: a shared engineering map, workstreams (lanes) that can run at the same time, and a sitting of measured rounds that ends when the walk works — or when a human decision, not a slice boundary, blocks you.

Announce at start: `Using lanes to [map | cut | lock | round | profile | rebalance | merge | ship].`

## Hard gates

1. **No destination, no ship.** You need where we're headed, **How we'll know we're there** (walk / proof / enforced), ADRs in force, and `docs/cuecards/handoff-<slug>.md` with real slices. Missing, placeholder, or "the closed issues are the plan" → cuecards. Do not invent a destination while coding.
2. **Do not reopen product.** The map is engineering: order, ownership, seams, parallelism. A new product question is a cuecards card, not a clever extra lane.
3. **ADRs are law.** Cited ADRs are closed. If the destination cannot honor one, stop and name it — cuecards sitting, not a workaround spread across lanes.
4. **Map before parallel.** If two people or two agents would cut without a committed map, they will collide. Write the map. Commit it. Then run.
5. **The destination Check is done.** Slice Checks are waypoints. You are not done until the board's walk works, the proof exists, and enforcement would fail if it regressed.
6. **Not this effort still binds.** Shipping the destination is not permission to ship the backlog, the nice-to-haves, or uncited ADRs.
7. **Each lane holds the oneslice bar.** Many lanes is not a license for spaghetti. Read the sibling skill `oneslice/SKILL.md` and apply it inside every lane: Check, quality bar, thin increments, save-point git, contract-first interfaces, threat-model security, primary-source research, spoken-English tickets, no secrets. This skill does not lower that bar.
8. **Fake parallelism is a defect.** Two lanes that must edit the same files, or that only look parallel on a slide, are one lane. Say so. One honest lane beats five that thrash `main`.
9. **Seams before both sides.** If two lanes share an API, schema, event, package, or fixture, the contract is written and committed **before** both implement. Consuming lanes do not start the part that needs an unwritten seam.
10. **Tickets and the map are for people.** A teammate who was not in the room must be able to open the map, pick an unclaimed Now lane, and know what to touch. Agent jargon as the live status is a defect. Never a bare `#42`.
11. **Claim a lane.** One owner per lane. Do not have two writers on the same files. Assign the ticket. Put the name on the map.
12. **This sitting is the destination.** Do not stop merely because Slice 1's Check passed. Stop when the destination Check passes, or when you are blocked on a human (ADR, auth/PII/upload/CORS/integration, access, a missing decision). If the work cannot fit one sitting without dropping the oneslice bar, **do not drop the bar** — commit an honest map, ship every Now lane that still fits at that bar, and leave Next/Then visible. Never "finish" by shipping a mess.
13. **Lock, then loop.** Freeze the destination Check and commit the map before round 1. Rounds change the tree, not the target. Do not move the walk to make a round look done.
14. **Thin briefs.** A lane agent or teammate gets only what their Check needs: their lane, Owns / Does not touch, the seams they consume, the ADRs they honor. Dumping the whole handoff, every other lane, and the chat into every agent is a coordination failure — cost, latency, and drift.
15. **Idle waiting is a bottleneck.** Do not start an agent that cannot cut yet. Profile the sitting, write the blocking seam, then dispatch. An agent sitting on an unwritten contract is wasted spend.
16. **The clock does not lower the bar.** A time or spend budget stops you *starting* more work. It does not buy shortcuts, a weaker Check, or "good enough because we were looping." Better to hit the limit with meaningful landed lanes than with everything roughly present and messy.

## Sort

Say the path out loud so they can override.

- **Still fuzzy / no handoff / no ADRs** — cuecards. Stay out.
- **One named slice, one ticket, one milestone** — oneslice. Stay out.
- **Ship the destination / build the rest / we need a map so the team can move / loop until it ships** — here.

When they say "just start building" and the handoff has many slices: this skill, not a silent loop of oneslice with no map.

This is not a UI or screenshot skill. The target is the destination Check, not a picture.

---

# The map

The map is the shared engineering picture. Chat is not. Standup is not. Closed cuecards issues are not.

`docs/lanes/map-<handoff-slug>.md`

Living file. Commit it before anyone cuts. Update it as lanes land and when ownership or seams change. Git versions it next to the code.

A non-technical person should understand **where we're headed** and **what's in flight**. An engineer who joined this morning should be able to claim a lane without a hallway conversation.

```markdown
# Map: <destination spoken name>

**For the team.** Cuecards decided. This file is how we ship it without colliding.

- **Handoff:** [handoff](../cuecards/handoff-<slug>.md)
- **Board:** [<board name>](url)
- **Integration owner:** <person or agent — merges, runs the destination walk, briefs agents>
- **Status:** Mapping · Locked · Round N · Blocked: <plain reason> · Destination Check passed
- **Time budget:** none · until <local time> (clock started <time>)
- **Spend ceiling:** none · <what they named>
- **Round:** 0
- **Agents in flight:** none

## Where we're headed
<from the board, one or two lines, outcome language>

## How we'll know we're there
- **Walk:**
- **Proof:**
- **Enforced:**

This is the locked target. Rounds do not rewrite it.

## Decisions we honor
- [ADR-NNNN Title](../adr/NNNN-….md)

## Not this effort
- [ADR-00NN …](…) — <gist>
- <anything else from the board that must not land>

## Seams (shared contracts)
A seam is anything two lanes both touch: API, schema, event, package, fixture, env.

### Seam: <spoken name>
- **Contract:** <path to types / schema / proto — exists before both sides cut>
- **Owned by:** <lane that writes it>
- **Consumed by:** <lanes>
- **Status:** not written · written · both sides on it

## Lanes

### Lane A — <spoken name>
- **Check:** <user-visible or integration; must be able to fail>
- **Does:** <what exists when this lane is done>
- **Handoff slices:** Slice 2, Slice 3
- **Owns (files/packages):** `src/billing/`
- **Does not touch:** `src/auth/`
- **Needs seams:** Seam: payments API
- **Parallel with:** Lane B
- **Waits on:** none | Lane C merge
- **Claimed by:** unclaimed | <name>
- **Ticket:** [<spoken name>](url)
- **Brief:** <path or "inline below — only this lane's packet">
- **Status:** unclaimed · in progress · Check passed · blocked: <plain>

## Now / Next / Then
- **Now, in parallel:** A, B
- **Next:** C (needs A and B merged)
- **Then:** destination walk

## Integration
- Merge to the default branch after every lane Check, not only at the end
- After each merge, walk as far as the destination currently allows
- Final: full walk + proof + enforced

## Sitting profile (this round)
- **Takeable now:**
- **Idle / waiting on:**
- **Bottleneck:** <seam, file, merge, or none>
- **Duplicate work or duplicate context:**
- **Walk before → after this round:**

## What landed
- <lane / seam, in spoken English>

## Blocked
nothing
```

Do not skip sections. Shorten them; do not omit them. If you cannot fill Seams and Owns, you are not ready to parallelize — you have a queue, and the map should say so.

## Map quality (offstage, before anyone sees it)

Think first. The timid map is one lane because contracts felt like work. The sloppy map is eight lanes that all edit `src/app.ts`. Aim at a map a strong tech lead would defend.

1. **Real parallelism.** For every "Parallel with," you can say why they will not collide (disjoint files, or a written seam).
2. **Ownership becomes architecture.** The Owns lines are the module boundaries you want in six months, not a snapshot of today's accidents. Cut lanes along the architecture you want.
3. **Vertical over horizontal.** Prefer a lane with a user-visible Check (someone can do X) over a "DB lane" and an "API lane" that cannot land alone — unless the team is actually specialized and the seam is clean.
4. **Seam lanes are rare and honest.** Extract a contract/platform lane only when it unblocks two others. A seam lane with no consumer this sitting is theatre.
5. **Right-sized.** A lane is one to a few handoff slices that share ownership and have one Check. More lanes than people (or agents) → merge lanes. A lane that is "the rest of the product" → cut again.
6. **Now is takeable.** Everything in **Now** has Waits on: none, seams written or owned by a Now lane that writes them first this sitting.
7. **The destination still fits.** Not this effort did not sneak in. Uncited ADRs did not sneak in.
8. **Spoken names.** "Who gets paid" not `ledger-sync-worker`. Same voice as cuecards.
9. **Briefable.** Each lane can be handed to an agent as a packet smaller than the map. If you cannot brief it thinly, the lane is still fuzzy — cut again.

If you cannot defend the map against "is this actually how a larger team ships without colliding?", rewrite it. Do not present a queue in costume.

### Red flags

| Thought | Reality |
| --- | --- |
| "We'll discover ownership in the PR." | You will collide. Put Owns on the map first. |
| "These two lanes are parallel if they rebase carefully." | They are one lane. |
| "A shared util file is fine, both can edit it." | That file is a seam, or it belongs to one owner. |
| "Layer lanes (schema / API / UI) look clean on a slide." | Nothing user-visible lands until the last layer. Cut vertical unless the seam is real. |
| "Eight lanes, two people." | Merge until lanes ≤ people, plus one seam lane if it unblocks. |
| "The handoff order is already the map." | The handoff is a sequence for one builder. The map is parallel + ownership + seams. It may regroup slices. It may not change what gets built. |
| "We'll write the contract when both sides meet." | Both sides will invent a different contract. Write it first. |
| "Ship faster by skipping oneslice." | Then you shipped a mess. Stop. |
| "The destination is huge, so lower the bar." | Honest map + Now lanes at full bar. Never a cheap finish. |
| "Integration at the end, once." | You will spend the last day merging fiction. Integrate after every lane. |
| "New teammate can ask in chat." | Then the map failed. They should pick an unclaimed Now lane from the file. |
| "Give every agent the full context to be safe." | You will burn spend and they will touch the wrong files. Thin brief. |
| "Serialize the agents so merging is easier." | If they don't share files, you paid latency for fear. Dispatch together. |
| "Degrade this lane so we hit the time box." | Hit the limit with less, done well. |
| "Don't measure, just keep cutting." | Then you cannot tell a round from thrashing. |
| "Restart every lane because one failed." | Isolate the failure. The others keep moving. |

---

# Cut lanes

Start from the handoff. Do not start from the org chart, and do not start from a blank architecture wish.

1. List every handoff slice, its **Depends on**, its **Check**, the files it must touch (guess, then verify against the tree).
2. Group slices that share ownership and can share a Check. That group is a candidate lane.
3. Where two candidate lanes would touch the same surface, either **merge them** or **extract a seam** (contract owned by one lane, consumed by the other).
4. Name **Now / Next / Then** from true waits, not from habit. Independent groups go Now, in parallel.
5. Name an **integration owner**. If you invoked this skill, that is you unless the map says otherwise.
6. Depth-review the map. Commit it. File or update one GitHub issue per lane (spoken English, same tracker shape as oneslice, plus Owns / Parallel with / Waits on). Point each issue at the map.

Regrouping slices onto lanes is allowed. Changing **Does**, sneaking past **Not this effort**, or contradicting an ADR is not.

**When the honest map is one lane:** write that. Then run it (still this skill if they asked to ship the destination — you keep going through every slice in that lane until the destination Check, each slice at the oneslice bar). Do not fake Lane B.

---

# Seams

Seams are how parallel workstreams stay honest.

- Write the contract in the repo (types, schema, event shape, fixture) **before** both implement. Oneslice interface rules apply: one error shape, validate at the edge, additive fields, idempotency honoured if you accept a key.
- One owner. Everyone else consumes. A seam with two writers is not a seam.
- Version is "one version." Do not let Lane A ship `v1` and Lane B ship `v2` of the same call "for now."
- If implementing the contract would change a product decision, stop — cuecards.
- When a seam must change after both sides have started: **stop both lanes**, update the contract, unstick the map, then continue. Do not let one side "just adapt."
- Seams are also how you cut latency: a written contract is what lets two agents start instead of waiting on each other.

---

# Tracker (team)

Live status is the map plus GitHub issues. The map is the picture. The issue is the lane's ticket.

Lane ticket — same voice as oneslice, extra lines for the team:

```markdown
## Lane <letter>: <spoken name>

**For you to read.** On the map: [Map: <destination>](docs/lanes/map-<slug>.md)

### What this lane does
### How we'll know
### Owns / does not touch
### Parallel with / waits on
### Decisions this honors
- [ADR-NNNN Title](path)

### Status
In progress — claimed by <name>.

### What landed
### What I didn't touch (on purpose)
### Blocked
nothing
```

After every increment and when a lane Check passes, update **both** the issue and the map (Status, What landed, Now/Next, Sitting profile). Refer to lanes, slices, and ADRs **by name**.

Lanes run in parallel sessions, so expect the map and issues to be edited concurrently. Re-read the map before you write to it.

A teammate reads the map, not the integration owner's head.

If `gh` cannot see this repo: say so, keep the map in `docs/lanes/` anyway, and fall back the same way oneslice/cuecards do. The map is not optional.

---

# The loop

Lock the target. Then round until an exit. Implementation *is* the loop. The map is the scoreboard.

This is not a visual loop. There is no target screenshot, no pixel match, no fidelity-to-a-picture. The target is the destination **walk / proof / enforced**.

## Lock (before round 1)

1. Map committed and depth-reviewed.
2. Destination Check written on the map in the same words as the board. **That is the target.** Do not move it later to make a round look done.
3. If they gave a **time budget**, record the clock on the map now (after lock, not before). Check it between rounds.
4. If they named a **spend ceiling**, performance goal, or "Now lanes only," write it on the map. Constraints do not rewrite ADRs or the oneslice bar.
5. If they gave **no** time budget, warn once that this sitting may run until the destination Check — then run until an exit.
6. Map Status = Locked. Say out loud: `Target locked: <walk in one line>. Then I loop.`

## A round

Every round, in order:

1. **Profile the sitting** (five minutes, on the map — not a product APM tour unless a lane Check says so).
   - Which Now lanes are actually takeable?
   - Where would an agent sit idle (unwritten seam, unmerged wait, missing claim)?
   - Which seam or file is the bottleneck?
   - Duplicate work, or the same context about to be pasted into two agents?
   - Is `main` green?
2. **Brief, don't dump.** For each takeable lane, a thin packet: lane block, Owns / Does not touch, seam contract paths, ADRs, Check. Not the whole handoff. Not every other lane's diff. Not the chat. If it isn't needed to pass that Check, it isn't in the brief.
3. **Dispatch independent work together.** One owner per lane. No second agent on a claimed lane. Do not start a blocked lane "to look busy." If this harness has subagents or worktrees, independent Now lanes in parallel (one worktree per lane). If not, run them in sequence with `main` green after each — still finish the takeable set before you stop.
4. **Implement** at the oneslice bar. Research foreign facts in the background per lane; reuse an existing research file instead of investigating the same primary source twice.
5. **Fault isolation.** One lane failing (tests, merge, human gate) does not cancel the others. Mark it blocked on the map. Keep A and C moving.
6. **Merge + measure.** Integration owner merges. Run tests. Walk as far as the destination currently allows. Write **Walk before → after this round** on the map. A round with no before/after is just typing.
7. **Rebalance.** Unlock Next whose waits landed. Split a lane that is the bottleneck. Merge lanes that turned out coupled. Check the clock and spend. Update the map. Next round, or exit.

Main stays green after every merge. A round that leaves the tree broken is a failed round: revert to last green, then investigate. Changes stay gradual and reversible (oneslice increments).

## Exit (only these)

- Destination Check passed (walk + proof + enforced), or
- Named human block on the map, or
- Time or spend budget hit — stop with meaningful landed lanes at full bar, map honest about Next.

Do not exit because a slice Check passed. Do not exit by lowering the implementation bar. Do not exit by rewriting the walk.

## Time and spend

- **Time budget given:** lock, note the time, check between rounds. When the remainder cannot fit another lane at full bar, do not start one. Finish in-flight well.
- **Don't degrade to hit the clock.** Don't take shortcuts. It is better to hit the limit with two lanes that would pass a hard review than with every slice roughly present and messy.
- **Spend:** don't spawn two agents on one lane; don't paste the repo into every brief; reuse research notes and committed seams (those are the cache; chat is not); prefer one round-trip of parallel Now lanes over a chatty "you wait, then you."
- If a spend ceiling is in sight, stop *starting* agents. Finish in-flight at full bar.

Do not pick models, prices, or dashboards this skill does not have. Track what you can see: rounds, agents in flight, idle waits, whether the walk advanced.

---

# Run

## One agent, one sitting

Lock, then loop the rounds above until an exit. If two independent Now lanes exist and you can only run one body: still finish both before you stop, unless a human gate or the clock says otherwise. Slice-one-and-stop is the wrong ending.

## Team / many agents

- The map is the assignment board. People claim an unclaimed **Now** lane, write their name, assign the ticket.
- Each claimed lane is an oneslice sitting for that owner, from a **thin brief**, not from the whole thread.
- They do not take a second lane until their Check passed and merged, unless the integration owner says the first is blocked on a seam they cannot write.
- Integration owner (named on the map) is the only one who changes Now/Next/Then, Owns, and Seams, and the only one who dispatches extra agents. If ownership is wrong, stop the colliding lanes, fix the map, restart. Do not "just resolve" a merge that means the map was a lie.
- Sync through the map and tickets. Chat lore that is not on the map does not exist.
- Do not wait for a meeting to unstick a seam. Stop the two lanes, write the contract, continue.
- New people: read the map, claim a Now lane, read the ADRs it cites. They should not need yesterday's thread.
- One failed owner does not rewind the team.

## Git (across lanes)

Oneslice git rules hold inside a lane. Across lanes:

- Default branch stays deployable.
- **No long-lived lane branches.** Short-lived `feature/<lane-spoken-slug>` (or per-slice), merge in days. A flag beats a long-lived branch.
- File ownership on the map is the lock. Touch another lane's files → you found a missing seam or a map bug. Stop. Fix the map.
- Worktrees when two agents must not share a working tree. One lane per worktree.
- Do not force-push shared branches. Never commit secrets. If a secret hits a remote: rotate first, then purge.
- After each lane merge, the integration owner runs the destination walk as far as it can go — not only at the end.

---

# Build loop (this sitting)

1. **Load.** Handoff, ADRs, Notes, How we'll know, tree layout. If any of those is missing, cuecards.
2. **Map.** Cut lanes and seams. Depth-review. Commit `docs/lanes/map-<slug>.md`. File/claim tickets.
3. **Lock.** Freeze the destination Check. Record time/spend constraints. Status = Locked.
4. **Round:** profile → brief → seams that blockers need → dispatch takeable lanes (oneslice bar, parallel if independent) → merge → walk before/after → rebalance.
5. Repeat 4 until an exit criterion.
6. **Destination Check** (if claiming shipped). Walk. Proof. Enforced. Break it on purpose and watch enforcement fail, then restore.
7. **Report** on the board issue and the map, in spoken English. What landed, what you didn't touch, which ADRs, where the proof lives, how many rounds, what the bottleneck was.
8. **Stop.** The destination shipped, or a named human block or budget is on the map. Not "slice 1 of n."

## Approval bar (do not call the destination shipped without this)

- Map exists, committed, and matches what actually ran (ownership, seams, Now/Next, round, profile)
- Target was locked before round 1 and not quietly moved
- Every landed lane passed the oneslice approval bar — not "good enough because we were looping"
- Destination **walk** works end to end without you in the room
- **Proof** is in the repo
- **Enforced:** CI (or the named pin) fails if that walk regresses
- Cited ADRs still true
- Nothing from **Not this effort** landed
- Seams that were shared were written before both sides cut
- Tickets and map are readable by a person who missed the sitting
- No long-lived lane branches left open as the real integration strategy
- No secrets in any lane's diff
- You did not reopen product decisions to make the map nicer
- Independent lanes were briefed thinly and dispatched together, not serialized for fear
- Each round recorded walk before → after; idle agents were not started
- A time/spend budget, if named, did not buy a messy finish

Any line above that fails is a blocker unless you can justify it in one sentence to the human. The classics: lanes parallel on the map but coupled in the tree, a seam invented twice, integration saved for the end, a lane shipped below the oneslice bar, the walk never run, or a map that is stale because chat knows more.

If the bar is not met, do not stop with "we'll tidy after launch." Fix it, or explicitly hand back a blocked destination with the map telling the truth.

## Tone

Be direct about collisions, fake parallelism, idle agents, and quality — including toward your own first map.
Do not be rude. Do not soften "these two lanes will fight in `src/db.ts`" into a note for later.
Do not confuse speed with shipping: six messy lanes is slower than two clean ones.
Do not confuse looping with thrashing: a round that does not move the walk is a failed round.

Useful self-talk (and useful things to tell the human):

- `no handoff. cuecards sitting, not a fake map.`
- `this is one ticket. oneslice, not lanes.`
- `these two lanes share a file. merging them / extracting a seam.`
- `honest map is one lane. writing that; still looping through the destination Check.`
- `target locked. round 1.`
- `this brief includes three other lanes. cutting it to Owns + seams + ADRs.`
- `agent would sit idle on an unwritten seam. not dispatching.`
- `contract isn't written. seam first, then the consumers.`
- `lane Check passed. merging; walking as far as the destination allows; not stopping.`
- `lane B died. map blocked; A and C still running.`
- `round had no before/after on the walk. measuring before dispatching more.`
- `time budget in 20 minutes. not starting a new lane; finishing in-flight at full bar.`
- `this contradicts ADR-NNNN. stopping. cuecards sitting.`
- `new auth/PII/upload/CORS. asking the human before that lane cuts.`
- `map was a slide. rewriting so a new teammate could claim a Now lane.`
- `destination walk still wouldn't catch a break. fixing proof before calling shipped.`

## It's working if

- The whole destination shipped this sitting, or the map honestly names the human block or budget and every takeable Now lane has landed at the oneslice bar — the exit was real, not a pile of slices with no loop.
- A teammate who missed the sitting can open `docs/lanes/map-<slug>.md`, claim an unclaimed Now lane, and not need yesterday's thread.
- You did not use this skill to decide product, match a picture, or leave the destination half-built.
