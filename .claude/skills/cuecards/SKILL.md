---
name: cuecards
description: "Use when you need choice cards a non-technical person can read and answer, when an idea is too big or too fuzzy for one sitting, when work items would otherwise come out as engineering tasks or jargon, when a board of questions is needed before anyone builds, or when the user asks for cuecards, note cards, or better questions."
disable-model-invocation: true
---

# Cuecards

Write cards so good that a smart person with no technical background can open one, understand why it exists, and decide.

Cards and the board live as **GitHub issues** on the current repo: one parent issue for the board, one child issue per card, with native blocking so the frontier is visible where the code and CI are.

**Caveat — easy words, hard thinking.** Simple language is how you *speak*. It is not permission to *think* simply. Questions and recommended answers are shown in everyday terms only after you have thoroughly reviewed them: would this choice, if taken, help build something people would actually find impressive? If the recommendation is merely the cheapest, safest, or easiest-to-explain default, you are not ready to present. Think offstage. Present onstage.

This skill lays out a fuzzy effort as a **board of choice cards** and works the frontier until the destination is *checkable*, not merely agreed in words. Durable choices become **named ADRs** other cards and later agents can cite. When the board is done, write a **handoff** — a decomposition a building agent can follow. This skill still does **not** ship the destination. Look-cards may leave real, kept evidence. If you feel the pull to build everything, you have left the skill.

Announce at start: `Using cuecards to [sort | lay out | write cards | work the board | record ADR | hand off].`

## Hard gates

1. **Do not ship the destination from this skill.** Look-cards may add a real working slice, fixtures, screenshots, or a CI check as *evidence for a choice*. Chores may unblock. Find-cards write findings. Write ADRs and, when the board is done, a handoff. Do not start the rest of the product.
2. **A card is a choice, not a sprint job.** If the title could be a to-do ("build login", "add Stripe"), it is mis-typed — unless it is a **look** card (produce real evidence so a choice can be made) or a **chore** (unblock a choice). Rewrite anything else as a question, or it is still fuzzy.
3. **The body is for a non-technical reader.** YAML, labels, and `## For the agent` / `## Tracking` are for you. If a tired founder cannot answer the card in a few minutes, rewrite it before you file it. Jargon in the human body is a defect.
4. **Think, then simplify, then present.** Never show a question or a recommendation you have not run through the depth review. Plain language is pass two. Pass one is: is this the real fork, and does the recommendation aim at a product that would actually be impressive?
5. **Review before create or present.** Depth review, then quality bar. Do not dump thin cards. Do not think out loud at them in jargon.
6. **If it needs them, they speak.** Never answer a for-you question yourself. Never pick the look-card. Facts are your job.
7. **File the whole sharp frontier.** Do not starve the board. Unattended cards run in parallel this sitting. For-you cards may be batched when they are independent and the human agrees; otherwise one for-you thread at a time so they are not flooded.
8. **Where we're headed must be testable.** If you cannot name the walk, the proof, and how it is enforced, you do not have a destination yet. A sentence they can say is how a *card* locks. It is not how the *board* is done.
9. **The live board lives on the repo's issue tracker.** By default GitHub, via `gh`. If `gh` cannot see this repo (not GitHub, or auth fails), say so out loud, then use the local fallback under `.cuecards/boards/<slug>/`. Never leave the live board only in chat, and never pretend issues exist.
10. **Durable choices are ADRs.** A closed-card gist on the parent is an index line, not the record. Write `docs/adr/NNNN-slug.md`, cite it as **ADR-NNNN** by name, and never reopen a closed card to rewrite history. Supersede with a new ADR.
11. **A decided board is not a build plan.** After the board is done, write the handoff. Do not hand a building agent only a pile of closed issues.

## Sort

Say the path out loud so they can override.

- **One sitting** — the whole question fits a conversation. Do not create a board. Ask card-quality questions in chat until you share an understanding. Then stop.
- **Still fuzzy** — they can name what "done" looks like, but not the route, and it will not fit one sitting. Lay out a board on GitHub. Stay here.
- **Already decided** — they want you to build the destination. This skill still does not ship it. Say the deciding looks done only if **How we'll know we're there** already passes **and** accepted ADRs plus a handoff exist. Otherwise write those first.

When in doubt, it is still fuzzy.

---

# Tracker (GitHub by default)

Live state is issues on the current repo. The parent issue is the board. Child issues are cards. Blocking uses GitHub's native "blocked by" so the UI shows what is takeable. Claim = assignee.

The human may work cards in parallel across sessions, so expect other sessions to be editing the tracker concurrently. Re-read the parent issue before you write to it.

Run **Board operations** at the bottom of this file. Create labels if missing. Every title is spoken English. Refer to issues **by that name**, wrapping the link; never a bare `#42` in anything the human reads.

If `gh` is missing, the repo is not GitHub, or auth fails: say so, then use the local fallback under `.cuecards/boards/<slug>/` with the same bodies. Do not pretend issues exist.

---

# Card craft (the point of this skill)

Assume the reader is intelligent, busy, and does not know Git, APIs, databases, "auth", "schema", or your tools. They *do* know their customer, their money, their time, and what they are trying to get done.

You write for that person. Labels, YAML, and Tracking are for the agent and CI.

## A card must do eight jobs

1. **Name** itself in spoken English.
2. **Place** itself: where we are, what is already decided, why this question now.
3. **Ask exactly one question** in everyday words.
4. **Fence** what it is not asking.
5. **Offer 2–4 options** in outcome language (what a person would notice, what it costs, what you give up) — not library names.
6. **Recommend** one option, with a short reason they can reject. The recommendation is the one you'd defend to a great product person, not the one that is easiest to build or easiest to explain.
7. **Define done** as a sentence they could say out loud. Look-cards *also* attach real evidence (see types).
8. **Show the chain:** answering this unlocks X; also list **what this blocks**.

If any job is missing, it is not filed yet.

## Shape

Every card issue uses this body. Do not skip sections. Shorten them; do not omit them.

```markdown
# <spoken-English name>

> **For you** — please decide.
> (or: **We'll handle this** — unattended. You do not need to answer.)
> (or: **For you** — look, then pick. A real slice will exist to react to.)
> (or: **A chore** — then we can decide.)

## In one sentence
We need to decide <X> so that <Y>.

## Why this, why now
<2–5 sentences. The chain from where we're headed to this question.
 What goes wrong if we skip it or guess.>

## What we already know
- **Where we're headed:** <one line>
- **Already decided:** [ADR-NNNN Title](docs/adr/NNNN-slug.md) ([card](url)): <gist>
- **Waiting on:** nothing / [<card name>](url)

## The question
<exactly one question. everyday words.>

## What this is not asking
- <nearby question that belongs elsewhere, or is not this effort>

## Options
### A — <name a person would use>
- **You'd notice:**
- **It costs:**
- **You give up:**

### B — ...
### C — ...   (2–4. never one. never seven.)

## Recommendation
**A**, because <one short reason>. <what would change your mind.>

## How you'll know it's answered
You can say: "<a sentence in their voice that locks the choice>."

## Proof (required for look-cards; optional elsewhere)
- **Walk:** <what someone does>
- **Artifact:** <path, screenshot, fixture, or CI job that will exist>
- **Keep:** yes. Link it here when it exists.

## After you answer
- **Unlocks / this blocks:** <names>
- **Still later:** <what this does not settle>
- **ADR:** ADR-NNNN (accepted) / none — find and chore usually none

## For the agent
<!-- required before present. human can ignore. -->

## Tracking
- Board: [<board name>](url)
- Type: ask | look | find | chore
- Mode: for-you | unattended
- Priority: now | next | later
- Blocked by: [<name>](url)
- Blocks: [<name>](url)
```

YAML in local fallback (agent only):

```yaml
id: "001"
title: <same spoken-English name>
type: ask               # ask | look | find | chore
status: open            # open | claimed | closed | not-this-effort
needs_you: true         # for-you. false = unattended
priority: now           # now | next | later
who_decides: you
claimed_by: ""
blocked_by: []
blocks: []
created: YYYY-MM-DD
```

On GitHub, the same facts are **labels + assignee + native blocked-by + Tracking**. Keep them in sync.

## Voice

- Titles you'd say to a friend: "Who pays?" "Where does the invoice go?"
- Not: `billing-entity-model`, `auth-provider-selection`.
- Refer to every board, card, and ADR **by name** (ADR-NNNN plus title), wrapping the link. Never a bare `#42` or a bare `0001`.
- Prefer "you" and "we".
- Options named after outcomes, not libraries.
- Gloss the rare unavoidable term once.
- Cut. If you would not read it on a phone, it is too long.
- Facts vs choices: look up prices and what exists. Ask them only for preferences, taste, risk, and who it is for.

### Ban-list in the human body

Do not use unless immediately glossed: auth, OAuth, schema, API, endpoint, repo, PR, webhook, idempotent, eventual consistency, "the stack", class names, file paths.

File paths belong in Proof, Tracking, or `## For the agent`.

## Types

| Type | Mode | Banner | When | Done when |
| --- | --- | --- | --- | --- |
| ask | for-you | **For you** — please decide. | Talking can settle it. Default. | They pick (or rewrite) an option. |
| look | for-you | **For you** — look, then pick. | Talking cannot settle how it looks / feels / goes. | A **real kept artifact** exists, they pick using it, Proof is filled and linked. |
| find | unattended | **We'll handle this.** | A fact you can look up is blocking a choice. | A one-page finding in plain language, linked on the issue. They do not do homework. |
| chore | unattended if you can, else for-you | **A chore** — then we can decide. | Signup, access, moving data so its shape can be seen. | Checklist complete. Never "build the destination." |

**Look-cards are real.** They exist to produce something you can run or inspect: a working module or slice in the repo, captured screenshots, DOM fixtures, a canary run, a CI check that fails when the slice regresses. Label nothing "throwaway you do not keep." Cheap and rough is fine; fake is not. You still do not pick the option — they do, facing the artifact.

**Find-cards** stay unattended. Findings are rewritten so the next ask-card can use them without opening a white paper.

**Chore** unblocks a question. "Implement accounts" is still a build job in disguise: delete it.

---

# Depth review (pass one — before they ever see it)

Do this **privately**. Fill `## For the agent`. Do not show this block as the conversation. Then quality bar. Then present.

1. **Think** — real fork, what would *great* look like.
2. **Draft** — options include the ambitious-right one.
3. **Depth review** — this list.
4. **Simplify** — same thinking, easier words.
5. **Present or file.**

If you cannot defend the question and recommendation against "would this help us build something impressive?", you are not ready.

### The product bar

Impressive is **taste on the right axis**, not more features and not more cards.

- **Real fork.** Experience-level choice, not a vendor/tool proxy.
- **Name great.** What would people tell someone else if we got this right? If no option points at that, the options are wrong.
- **Recommendation honesty.** If you picked easy/cheap/agent-convenient, change it or say the trade in the open.
- **Hidden option.** If you dropped a better option because it was hard to say simply, that is a writing failure.
- **Six-month regret.** Do not recommend the pick that makes this feel like every other product.
- **Cut extra knobs, not ambition.**
- **Cost is honest.**

### `## For the agent` (required)

```markdown
## For the agent

### What would impressive look like here
### Real fork
### Why this recommendation isn't the lazy one
### Option I almost hid
### Six-month generic
```

Empty or a shrug: do not present.

## Quality bar (pass two)

Read the card as someone who cannot code. Thinking stays; words get simpler.

1. Title you'd say out loud
2. Exactly one question
3. Not a sprint job (unless look/chore as defined)
4. Self-contained with named gists
5. So-that chain is obvious
6. Fence is real
7. 2–4 outcome-named options with notice / cost / give-up
8. Recommendation rejectable, not a hedge
9. Done sentence they could utter
10. Unlocks **and** blocks named
11. No ungossed jargon in the human body
12. One short scroll (or it is two cards / still fuzzy)
13. Banner matches for-you vs unattended
14. Assumptions labeled
15. Depth block filled
16. Ambition survived simplifying
17. **Look-cards:** Proof names a walk, an artifact that will be kept, and (when the destination is UI) how a screenshot / DOM fixture / CI job will pin it
18. **Priority** is set: now / next / later
19. **Closing an ask or look:** an ADR is drafted or accepted in the same sitting (see ADRs). The board gist cites **ADR-NNNN**, not only the issue.

Fail any one: rewrite. Do not file. Do not present.

### Red flags

| Thought | Reality |
| --- | --- |
| "They'll know what I mean" | The card has to carry it. |
| "I'll put the real detail only in Tracking" | Then they decide blind. |
| "Build the X" | Find the question — unless this is a look-card producing evidence. |
| "Just list the technical options" | Name outcomes. Recommend. |
| "Keep the board tiny so we look careful" | Starving the frontier is a defect. File every sharp question. |
| "Simple words means a simple recommendation" | Simple words, hard call. |
| "I'll recommend A because we'll have to build it later" | Recommend the impressive product. |
| "Throwaway mock is enough for a look-card" | Produce something real and keep it. |
| "They said yes, so the board is done" | Done is the walk + proof + enforcement, not a vibe. |
| "I'll figure out the real question while they answer" | Depth review is before present. |
| "The gist on the board is enough" | The gist is an index. The ADR is the record. |
| "I'll edit the closed issue to change the decision" | Write ADR-NNNN+1 that supersedes. Append History on the old ADR. Leave the card closed. |
| "The closed board is the build plan" | Write the handoff. Closed issues are context, not sequence. |

### Bad vs good

Bad title: `Select auth provider`  
Good title: `How do people sign in?`

Bad question: `Should we use Stripe or Braintree for PCI-DSS SAQ-A?`  
Good question: `When someone pays, do they type their card on our site, or on a checkout page that belongs to a payments company?`

Bad option: `A — Postgres. B — Mongo.`  
Good option: `A — One list of invoices we can all trust, even if that takes longer to set up.`

Bad look-card: a sketch you delete.  
Good look-card: a running slice in the repo, screenshot (or fixture) linked, CI pin if it is a UI walk, they pick A/B/C facing that.

Bad "done": "we agreed in chat."  
Good "done" for the board: a named walk works, proof is in the repo, CI fails if it regresses.

---

# The board

The board is the **parent GitHub issue**. It is an index, not a store. A choice lives on its card issue. The board gists, links, and lists what is takeable.

A non-technical person should open the parent issue and know what to do.

```markdown
# <spoken name>

## Where we're headed
<one or two lines. outcome language.>

## How we'll know we're there
- **Walk:** <the person can do X, end to end, without us in the room>
- **Proof:** <screenshot trace of that walk, and/or DOM fixture path, and/or test name>
- **Enforced:** <CI job that fails if this regresses. If none yet, the look-card that will add it.>

If any of the three is missing, this heading is not ready. Keep asking.

## How to read this
One card = one choice. Open a **Ready now** card. Ignore agent notes.
**We'll handle this** runs without you.

## Notes
- **Domain:** <who it's for, what already exists>
- **Standing rules:** <constraints every sitting must honor — examples: stdlib first; tests stay green; dry-run enforced three ways>
- **Consult:** <files or skills every sitting should read, including `docs/adr/`>
- **ADRs in force:** [ADR-0001 Title](docs/adr/0001-….md), …
Do not leave this empty. Cards should not have to repeat these rules. Cite ADRs by name, never by a bare number.

## Ready now — for you
- [<name>](url) · **now** · blocks [<name>](url): <question in one line>

## Ready now — unattended
- [<name>](url) · **now**: <what we'll handle>

## Waiting
- [<name>](url) · waiting on [<name>](url) · blocks [<name>](url)

## Decided
- [ADR-NNNN Title](docs/adr/NNNN-slug.md) · [<card name>](url) · accepted: <one-line gist>
- (deprecated / superseded ADRs stay in `docs/adr/README.md`, not here, unless Notes still need the warning)

## Not sharp enough to ask yet
- <suspected question, why it isn't a card yet>

## Not this effort
- <gist + why it sits past where we're headed>
```

**Fuzzy or card?** Can you state the question precisely *now* — not can you answer it now.

- Sharp → card, even if waiting, and even if several are open at once. Cover the edge.
- Not sharp → **Not sharp enough**. Do not pre-slice into fake cards.

**Not this effort** is scope, not fuzz. Close those issues as `not-this-effort`; they do not belong under Decided.

Where you're headed, **How we'll know we're there**, and **Notes** are agreed **before** any card exists.

---

# Lay out the board

One sitting. Create the parent issue and every sharp card. Resolve none except **start all unattended find-cards in parallel**.

1. **Name where you're headed** and fill **How we'll know we're there** (walk, proof, enforcement). Think first about impressive-and-checkable, then ask rounds in plain language. Get an explicit yes. No cards before this. A timid heading with no proof is not done.
2. **Fill Notes** (domain, standing rules, consult). Get a nod.
3. **Fan out, breadth-first.** What is already askable across the whole space? What is still fuzzy? Do not deep-dive one thread. Do not stop at two cards if eight sharp questions exist.
4. **Nothing fuzzy, and it fits one sitting?** Then Sort said so already — do not create a board, say so.
5. **Create the parent issue.** Then create every sharp card as a child issue. Wire `blocked_by` / `blocks` (native + Tracking) in a **second pass**. Set priority `now` / `next` / `later`. Set `for-you` vs `unattended`.
6. **Both reviews pass on every card** before you file it. No thin stubs. No timid defaults. Filing many excellent cards is the point.
7. **Start every ready unattended card now** (find, and chores you can do). Parallel.
8. **Stop the for-you side.** Show: destination, how we'll know, Notes, Ready now (both lists). Next sitting works the frontier. Do not answer for-you cards while laying out.

## Work the board

1. Load the **parent issue** (low-res). Refresh Ready now from GitHub (open, unassigned, blockers closed).
2. **Run all Ready now — unattended** in parallel this sitting.
3. **For-you:** take the highest `now` card. If several are independent and they want speed, batch them in one sitting as separate numbered questions — still wait, still do not answer for them. If they want one at a time, honor that.
4. **Claim** (assign) before work.
5. Re-run depth review if the world changed. Present the card almost as written. Wait. If they ask what a word means, fix the card. If options feel small, go back to the product bar.
6. **Look-cards:** build the real slice / capture the walk / add the fixture or CI pin; link Proof; then they pick. Keep the artifact.
7. Record **Answer** on the issue (comment + close). For every closed **ask** or **look**, **write or accept the ADR in the same sitting**. Gist the parent **Decided** line as `ADR-NNNN Title — gist`, linking both the ADR file and the card. Update Ready now / Waiting / blocks. Add the ADR to Notes → ADRs in force and to `docs/adr/README.md`.
8. Graduate newly sharp questions into quality-bar cards (create issues, wire edges). Rule out of this effort if a card now sits past the destination. Durable exclusions get an ADR too (`Status: accepted`, decision = out of this effort) so later cards can cite them.
9. Unattended work does not wait for a for-you card to finish. Do not end the sitting while ready unattended cards remain.

Wrong closed choice: do **not** reopen the card to rewrite it. Write a new ADR that **supersedes** the old one. On the old ADR, append History and set Status to superseded (append-only). Comment on the closed card: "Superseded by ADR-NNNN" as a pointer only. Update Notes → ADRs in force.

### The board is done when

- Where we're headed still matches.
- **Walk** works end to end.
- **Proof** exists in the repo (screenshot trace and/or DOM fixture and/or named test).
- **Enforced:** CI fails if that walk regresses — or a closed look-card added that pin and it is green.
- No open cards.
- Nothing left under Not sharp enough.
- Every accepted choice has an **ADR-NNNN** file; the parent Decided list cites those names.
- `docs/adr/README.md` matches reality (status, supersession).

Then **write the handoff**. Stop. Do not ship extra product. The handoff is the last Cuecards artifact; code beyond look-card evidence is a different sitting and a different skill.

## Ask rounds

Think first, then ask only what you cannot look up. Independent questions in one round, each with a recommendation. Wait.

```text
❓ Q1 - <spoken title>
<the question>
A / B / C in outcome language

➡️ I recommend A because <one reason>
```

Do not ask a question that depends on another still open in this round.

User instructions (`AGENTS.md`, a direct request) outrank this skill.

---

# ADRs

A closed card is a conversation that ended. An **ADR** is the durable, citable record. Other cards, Notes, CI comments, and the handoff cite **ADR-NNNN Title**, never a gist alone.

## When to write one

| Closed card | ADR? |
| --- | --- |
| ask | **Always.** |
| look | **Always** (the choice + pointer to Proof). |
| find | Only if the finding *locks a constraint* others must honor. Otherwise leave it as a linked finding. |
| chore | No. |
| not-this-effort | **Yes** if the exclusion should stay citable. Decision = out of this effort, with consequences. |

Write it in the **same sitting** as the close. Do not batch "ADRs later."

## Where

```text
docs/adr/NNNN-spoken-slug.md     # never reuse or renumber NNNN
docs/adr/README.md               # replaceable index: number, title, status, supersedes
```

NNNN is four digits, next unused. The file is the record. Git versions it next to the code. The GitHub card stays closed; it *points* at the ADR.

## Format (required)

Spoken-English title. Same voice as the card for Context and Decision. Consequences must say what this means *downstream* — for people and for later work — not only what was picked.

```markdown
# ADR-NNNN: <spoken title>

- **Status:** proposed | accepted | deprecated | superseded
- **Date:** YYYY-MM-DD
- **Card:** [<card name>](issue url)
- **Board:** [<board name>](issue url)
- **Supersedes:** ADR-MMMM | none
- **Superseded by:** none | ADR-PPPP

## Context
Why this came up. What we already knew. What we were choosing between (A/B/C in outcome language). Link the card.

## Decision
The locked choice, in a sentence they could say. Same as the card's Answer.

## Consequences
What this means from now on:
- **People notice:** …
- **Later cards/ADRs must:** …
- **We give up:** …
- **Look/CI/proof:** paths or jobs this commits us to, if any

## History
Append-only. Do not edit Context or Decision as if the past changed.
- YYYY-MM-DD accepted
- YYYY-MM-DD superseded by ADR-PPPP — <one line why>
```

## Status lifecycle

```text
proposed → accepted → deprecated
                     ↘ superseded → (new ADR is accepted)
```

- **proposed** — optional draft while the card is still open (big forks only). Default is to write on accept.
- **accepted** — in force. Listed under Notes → ADRs in force and on the parent Decided list.
- **deprecated** — historically true, no longer in force, no replacement yet. Remove from ADRs in force. Keep the file. Append History. Do **not** delete. Do **not** reopen the card to rewrite it.
- **superseded** — a newer ADR replaces it. Old file: set Status, set Superseded by, append History. New file: Status accepted, Supersedes ADR-NNNN. ADRs in force lists only the new one.

Never rewrite Decision on an accepted ADR. Correction = new ADR.

## Index (`docs/adr/README.md`)

Replace this table whenever status changes. Do not copy Decision text here.

```markdown
# Decisions in force

| ID | Title | Status | Supersedes |
| --- | --- | --- | --- |
| [ADR-0001](0001-who-owes-us.md) | Who owes us? | accepted | |
| [ADR-0002](0002-where-the-invoice-goes.md) | Where does the invoice go? | accepted | |
```

## Citing

In cards, Notes, handoff, and chat:

> Already decided: [ADR-0001 Who owes us?](docs/adr/0001-who-owes-us.md)

Not: "see #12" or "as we said last week."

---

# Handoff

Cuecards output is excellent **context**. It is not a build plan. After the board is done, write a decomposition a building agent can follow. Then stop.

`docs/cuecards/handoff-<board-slug>.md`

```markdown
# Handoff: <board spoken name>

**For a building agent.** Do not start this from Cuecards. Cuecards sitting ends when this file is written.

## Where we're headed
<from the board>

## How we'll know we're there
- Walk:
- Proof:
- Enforced:

## Constraints (from Notes)
- …

## ADRs this implements (in force)
- [ADR-0001 Who owes us?](../adr/0001-who-owes-us.md)
- …

## Not this effort
- [ADR-00NN …](…) — <gist>

## Sequence
Ordered slices. Each slice is independently checkable. Cite ADRs. Name the walk/proof this slice advances. Do not paste product code.

### Slice 1: <spoken name>
- **ADRs:** ADR-0001, ADR-0002
- **Does:** <what exists when this slice is done>
- **Check:** <walk or test>
- **Depends on:** none

### Slice 2: …
- **Depends on:** Slice 1
```

No placeholders. If you cannot sequence it, an ADR is missing or still fuzzy — go back to the board, do not invent a fake plan.

Show the human the file. Stop. The next sitting (different skill or a human "go build") executes it.

---

# Board operations (GitHub)

Use `gh` in the current repo. `{owner}/{repo}` from `gh repo view --json nameWithOwner -q .nameWithOwner`.

## Labels (create if missing)

`cuecards`, `board`, `card`, `ask`, `look`, `find`, `chore`, `for-you`, `unattended`, `now`, `next`, `later`

```bash
for l in cuecards board card ask look find chore for-you unattended now next later; do
  gh label create "$l" -c "0E8A16" -d "$l" 2>/dev/null || true
done
```

## Create the board (parent)

```bash
gh issue create --title "<spoken board name>" --label cuecards,board --body-file /tmp/board.md
```

## Create a card (child)

```bash
gh issue create --title "<spoken card name>" --label cuecards,card,<type>,<for-you|unattended>,<now|next|later> --body-file /tmp/card.md
```

Make it a sub-issue of the board (`ISSUE_ID` is the child's numeric `id`, not the number):

```bash
CHILD_ID=$(gh api repos/{owner}/{repo}/issues/<child-number> --jq .id)
gh api -X POST repos/{owner}/{repo}/issues/<board-number>/sub_issues -f sub_issue_id="$CHILD_ID"
```

## Native blocked-by

```bash
BLOCKER_ID=$(gh api repos/{owner}/{repo}/issues/<blocker-number> --jq .id)
gh api -X POST repos/{owner}/{repo}/issues/<blocked-number>/dependencies/blocked_by -f issue_id="$BLOCKER_ID"
```

If that API is unavailable, still write Blocked by / Blocks in **Tracking** and on the parent lists. Never leave edges only in your head.

## Claim / close / comment

- Claim: `gh issue edit <n> --add-assignee @me`
- Answer + close: comment the **Answer** in their voice plus `ADR-NNNN` path, then `gh issue close <n>`
- After every close or new card: **edit the parent body** so Ready now / Waiting / Decided / blocks still tell the truth
- Commit ADR files and `docs/adr/README.md` in the same sitting as the close

## Query the frontier

Ready now: open, label `card`, no assignee, not blocked (or all blockers closed). Split by `for-you` vs `unattended`. Sort `now` before `next` before `later`.

## Local fallback (only if GitHub is impossible)

```text
.cuecards/boards/<slug>/
  BOARD.md
  cards/001-<slug>.md
  assets/
```

Same bodies. Say out loud that issues were not created. ADRs still go in `docs/adr/` — they are repo files, not issues.

## It's working if

- The board is a GitHub issue; every card is a child issue with labels, edges, and priority.
- A non-technical reader can pick a Ready now — for you issue and decide.
- Unattended cards actually run without them, in parallel.
- Notes hold standing rules **and ADRs in force** so cards don't repeat them.
- Look-cards left a kept artifact and (when UI) a pin CI can fail on.
- How we'll know we're there has walk + proof + enforcement, and the board is not called done until those pass.
- Every accepted ask/look has `docs/adr/NNNN-….md` with Context, Decision, Consequences, and a status. Later cards cite **ADR-NNNN** by name.
- Supersession is a new ADR plus History on the old file — closed issues are not rewritten.
- When the board is done, `docs/cuecards/handoff-<slug>.md` sequences slices that cite ADRs. That file exists before anyone writes destination code.
- Every open card still reads as one question, with options and a recommendation, in easy words, after hard thinking.
- You did not ship the destination as a side effect.
