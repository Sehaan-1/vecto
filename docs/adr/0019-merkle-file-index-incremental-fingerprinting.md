# ADR-0019: Merkle File Index — O(changed) Incremental Fingerprinting

- **Status:** accepted
- **Date:** 2026-09-20
- **Supersedes:** none
- **Superseded by:** none
- **Research:** [Finding 003 — Incremental detection at 100k files](../findings/003-incremental-detection-at-100k-files.md)

## Context

`vecto run` computes every task fingerprint by walking the tree and content-hashing **every**
matched input file, **per task, per run** (`internal/hash/hash.go`). On a 100,000-file repository
with tasks globbing `packages/**`, a run where nothing changed still pays T×100k full file reads
for T such tasks. Finding 003 shows the incumbents each solve part of this (Turborepo: git-index
stat fast path, git required; Watchman: daemon, event stream, recrawl recovery; ccache:
invocation-scoped manifest; rustc: try-mark-green invalidation) but none ships the full solution
in a git-independent, daemon-free, zero-dependency one-shot runner.

ADR-0004 fixed the *key model*: cache keys are deterministic SHA-256 over content. This ADR does
not weaken that — the fingerprint is still a content-addressed digest of exactly
(command, env, dep fingerprints, matched file paths+contents). It only changes *how the runner
discovers that content*, by persisting a Merkle file index between runs.

## Decision

Introduce a **Merkle File Index (VFI)** in `internal/fileindex`, on by default, opt-out via
`vecto run --no-file-index`. Four rules define the algorithm:

### R1 — Persistent Merkle forest (what is stored)

Under `.vecto/fileindex.json` (git-ignored) the runner stores, for every non-ignored file and
directory in the base directory:

- **file node**: `SHA256("VFI/1 file\0" + relPath + "\0" + size + "\0" + SHA256(content))`
- **dir node**: `SHA256("VFI/1 dir\0" + relPath + "\0" + concat(sorted name + "\0" + childNodeHash))`
- per entry: the `lstat(2)` tuple `(dev, ino, ctime, mtime, size)` plus the node hash
- per task: the previous run's fingerprint + the def hash that produced it
- root node hash (the dir hash of the base directory) and the snapshot timestamp

File nodes hash the **relative path**, so renames change keys (same rule as today's fingerprint).
Directory nodes hash sorted child (name, hash) pairs — the same Merkle construction as git tree
objects (Finding 003, S8/S11), so any descendant change changes every ancestor hash.

### R2 — Stat cascade with racy re-hash (how content hashes are kept current)

On each run, `Index.Sync` walks the tree (bounded-parallel, one worker per directory) and, for
each path, compares the current `lstat` tuple with the stored one:

- **match** ⇒ the stored content hash is trusted. Zero file reads. This is git's
  `ce_match_stat` fast path (Finding 003, S9).
- **racy** ⇒ the stored `ctime` or `mtime` is within the same timestamp resolution as the index
  snapshot time ⇒ re-hash the content instead of trusting (git's racy-git fix, S10).
- **mismatch / new / racy** ⇒ open, stream, SHA-256, update node + stat tuple.
- **deleted** ⇒ node removed.
- **symlinks** ⇒ never trusted, always re-hashed (a symlink's own metadata does not change when
  its target is edited in place).

`ctime` is trusted as an unforgeable "any inode change" marker on Linux (it cannot be set by
user-space); the residual risk of a forged `mtime/size` pair is the same documented residual risk
git accepts (S9/S10) and is stated in the `vecto index` output and this ADR.

### R3 — Dirty-cutoff propagation (how directory hashes are recomputed)

After the stat cascade, directory nodes are recomputed bottom-up. A directory whose recomputed
hash **equals** its stored hash is marked *settled* and invalidation stops propagating through it
(rustc try-mark-green, S12; git cache-tree "skip sections where a tree comparison demonstrates
equality", S8). A 1-file edit therefore touches O(depth) directory hashes, not O(n).

### R4 — Glob-coverage fingerprints (what a task fingerprints)

A task fingerprint v2 is `SHA256("scheme:fileindex-v2\n" + cmd + env + sorted deps + coverage)`
where **coverage** is the digest over the *covering nodes* of the task's input globs:

- a wildcard glob that matches an entire subtree (e.g. `packages/**`) collapses to the single
  covering **dir node** — O(1) per such glob, not O(matched files);
- a partial glob (e.g. `packages/*/src/**`) collapses to the maximal set of dir/file nodes that
  exactly covers the matched file set (a dir node covers its files only when the glob matched
  **every** file under it, which preserves set equivalence);
- literal paths fingerprint as their single node; a missing literal input is still an error, as
  today.

Coverage is a deterministic function of the matched (path, content) multiset: set-equal matched
sets ⇒ set-equal coverings ⇒ identical digest. The scheme marker guarantees v2 keys can never
collide with v1 (legacy) keys, so an upgrade costs at most one cold run.

### Runner integration

- `run` builds (or syncs) the index **once** before dispatch; every task then fingerprints from
  the index. The per-task full walk + per-task full content hashing is gone from the hot path.
- Task-level memo: if root hash, task def, and dep fingerprints are all unchanged since the last
  run, the stored fingerprint is reused without recomputing coverage.
- The legacy per-file path remains as `--no-file-index` (and as the fallback if index load fails
  with a corrupted file — the index is never trusted to *skip* a task on error; it degrades to
  the old behavior, which is the safe direction).
- `vecto index` prints index stats; `vecto index --rebuild` resets it.

## Complexity

| Scenario | Vecto v1 (legacy) | Vecto VFI |
|---|---|---|
| Cold (first run) | O(n) reads | O(n) reads (identical; unavoidable) |
| Warm, 0 changes, 100k files | T walks + T×100k reads | 1 parallel stat cascade (≈100k `lstat`, 0 reads) + O(1)/task |
| Warm, k files changed | T walks + T×100k reads | stat cascade + k reads + O(k·depth) dir recompute + O(changed tasks) fingerprints |

## Consequences

**Positive:**
- Warm replay on large trees becomes stat-bound, not IO-bound; the 100k-file benchmark
  (`benchmarks/hundredk/`, `docs/benchmarks.md` §7) measures this against Turborepo 2.x
  (git and non-git modes) and against Vecto's own legacy path, per ADR-0015's honesty rules.
- A warm 0-change run performs **zero index writes**: the index is persisted only when the
  root hash changed or a task record was added. Retaining the older stat tuples is sound —
  a real change still shows up as a tuple mismatch, and a same-second rewrite is caught by
  the racy rule against the retained (older) snapshot second.
- The directory-hash pass is O(N) per sync (a precomputed direct-child index) rather than
  O(N × dirs) — at 100k files / 500 dirs that difference is ~3s vs ~0.3s for one warm sync.
- Fingerprints remain content-addressed (ADR-0004 intact); cache entries stay portable; the
  remote-cache protocol is unchanged.
- Zero new dependencies; the index is plain JSON under `.vecto/` (ADR-0003 territory, git-ignored).

**Neutral / Trade-offs:**
- One-time cold run after upgrade (new fingerprint namespace) — documented, one-shot.
- The index is a second state file to keep correct; corruption degrades to legacy hashing,
  never to a wrong cache decision (R2 only *trusts* a stored hash when the stat tuple matches;
  any doubt re-hashes).
- On filesystems without reliable `ino`/`dev` (some network mounts), the fast path degrades to
  `mtime/size/ctime` comparison — weaker but still sound (git documents the same caveat for
  `st_dev`, S9).

**Look/CI/proof:**
- `internal/fileindex` unit tests: no-op sync rehashes nothing; 1-file edit rehashes exactly one
  file and O(depth) dirs; dirty cutoff observed; incremental history converges to the same root
  hash as a from-scratch index; `touch -d` (ctime bump) detected; symlink edit detected.
- `test/e2e`: cached → edit inside glob → re-run; edit outside glob → still cached.
- `benchmarks/hundredk`: generator + measured matrix (legacy, VFI, turbo git, turbo non-git);
  skips cleanly when `turbo` is absent (ADR-0015).

## Alternatives Considered

- **eBPF scheduler (kernel-side task dispatch):** flashy, but Linux-only, needs `CAP_BPF`/
  privileged containers, cannot be reproduced in CI, and attacks the *scheduling* cost — which is
  not the bottleneck; at 100k files the cost is *detection*, and a kernel module adds an
  unevidencable dependency to a lab project. Rejected.
- **Watchman dependency / built-in inotify daemon:** solves the sweep but requires a long-lived
  process + per-directory kernel watches + overflow recovery (Finding 003, S6/S7), breaks the
  one-shot CLI contract, and is a third-party binary. The stat cascade is the cost of not running
  a daemon, and it is parallel. Rejected for v1; the index format is daemon-compatible (a future
  `vecto watch` could feed R2 from events instead of a sweep).
- **Anchor on git like Turborepo:** requires git, couples fingerprints to `.git/index` state, and
  still folds per-package (not per-glob). Our index is git-independent and per-glob. Rejected.
- **mtime-only (no content hash in the key):** violates ADR-0004 (false hits on `touch`-only or
  restored mtime). The stat tuple is a *cache for the hash*, never a substitute. Rejected.

## History

- 2026-09-20 accepted — implements Finding 003; proof lands in `docs/benchmarks.md` §7.
