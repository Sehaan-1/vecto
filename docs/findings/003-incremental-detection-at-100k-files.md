# Finding 003: Incremental Change Detection at 100k Files — How the Incumbents Do It, and Where the Gap Is

- **Status:** Complete
- **Date:** 2026-09-20
- **Author:** Vecto Engine Team

## Question

Vecto today re-walks the tree and re-hashes **every matched input file, for every task, on every run**
(`internal/hash/hash.go`: `filepath.Walk` + open/read/SHA-256 per file, called from
`internal/runner/runner.go` inside `runTask`). On a 100,000-file repository with a handful of
`packages/**`-style tasks, that is O(n) directory walks + O(n) full content reads *per task* —
seconds of pure detection work on every "nothing changed" replay.

The novelty question: **how do the leading caching runners decide what changed, what does it cost,
and is there a defensible, provably-sound algorithm that does better at 100k files in a
zero-dependency, one-shot (no daemon) CLI?** All claims below are traced to primary sources
(official docs or source code), listed at the end.

---

## 1. Turborepo (measured rival, verified in source)

From the Turborepo documentation [S1] and the current source at
`crates/turborepo-scm` / `crates/turborepo-task-hash` in `vercel/turborepo` (master, 2026-09) [S2]:

- **Two-level fingerprint.** "Under the hood, Turborepo creates two hashes: a global hash and a
  task hash. If either of the hashes change, the task will miss cache." [S1]
- **Default file inputs are git-tracked.** Package hash inputs include "File changes — Defaults to
  all source-controlled files in the package directory. Configurable with `inputs`." [S1]
- **Git mode reads `.git/index` directly.** `RepoGitIndex::new_from_gix_index` parses the git index
  with `gix-index` ("skip_hash: don't verify the index checksum (2x faster)"), then stat-compares
  each entry against the filesystem (`trust_ctime: true`, `use_nsec: true`). Clean entries **reuse
  the committed blob OID** — no content read. Dirty entries are content-hashed as git blob objects
  (SHA-1, CRLF-normalized per `.gitattributes`) [S2].
- **Racy entries are deferred, not trusted.** "Racy-git entries (where mtime >= index timestamp, so
  we can't trust the stat comparison) are deferred to per-package hashing rather than
  content-hashed inline." [S2]
- **Non-git ("manual") mode has no memoization.** `SCM::get_package_file_hashes` in `manual.rs`
  walks the package with `ignore::WalkBuilder` and content-hashes every file, **on every run** [S2].
- **Per-package hash is a fold over all package files, every run.** `FileHashes` (a sorted
  (path, OID) pair list) is serialized via Cap'n Proto and hashed
  (`crates/turborepo-hash/src/lib.rs`) [S2]. A 100k-file package pays a 100k-pair
  sort/serialize/hash on every run even with zero changes.
- **Daemon is optional precomputation, not the hash store.** The per-repo gRPC daemon
  "watches files and pre-computes data to speed up turbo's execution" (package discovery,
  file-watch state, cookie files for event sync) [S2]. The authoritative file list for hashing is
  still the git index read on the run path.

**Cost profile at 100k files, warm run, zero changes:**
- *With git:* parse git index (100k entries) + stat-compare 100k entries + per-package OID folds.
  No content reads. Bounded below by the git index parse and the package folds.
- *Without git:* full tree walk + **full content re-hash of every file**. This is the worst case.

## 2. Vecto (this repository, current state)

- `hash.ComputeTaskFingerprint` [S3]: hashes command, sorted dep fingerprints, env, then
  `resolveInputFiles` — a `filepath.Walk(baseDir)` **per wildcard glob** — followed by open/read/
  SHA-256 of **every matched file**. No cross-run state.
- Called once **per task per run** from `runner.runTask` [S3]. With T tasks globbing the same 100k
  files: T walks + T×100k content reads per run, including runs where nothing changed.

## 3. Bazel

- Build = actions; "Each action has inputs, output names, a command line, and environment
  variables" and the cache is keyed by the **action hash** ("the action cache, which is a map of
  action hashes to action result metadata" + CAS) [S4].
- Incrementality is carried by Bazel's internal action/analysis graph (C++, its own package/rule
  model); nothing in the public surface offers a file-index API that a foreign task graph could
  reuse. Relevant pattern: cache keys are content-hash-based; changed inputs change the key,
  invalidating the action and its dependents [S4].

## 4. ccache (per-invocation compiler cache)

- "ccache uses two ways of doing the detection: the **direct mode**, where ccache hashes the source
  code and include files directly … the **preprocessor mode**." In direct mode it stores "hash sums
  of the include files at the time the compilation results were stored" in a **manifest**, re-reads
  current hashes and compares, falling back to the preprocessor on mismatch [S5].
- ccache's detection is **invocation-scoped** (the compiler tells it exactly which files are
  inputs), so it never faces "scan 100k files to find my inputs". The manifest+compare pattern is
  the same family as git's stat-cache: cheap metadata compare first, content verify second [S5].

## 5. Watchman (Meta's file-watching daemon)

- "Watchman exists to watch files and record when they change" — a **persistent server** with
  clocks/triggers; "You can query a root for file changes since you last checked" [S6].
- On Linux the event stream is inotify, and the kernel's per-user watch limit is small by default:
  when the event queue overflows Watchman must **recrawl** — "a potentially expensive full tree
  crawl" [S6].
- `inotify(7)` [S7]: monitoring "is **not recursive**: to monitor subdirectories … additional
  watches must be created. This can take a significant amount of time for large directory trees";
  "the event queue can overflow. In this case, events are lost. Robust applications should handle
  the possibility of lost events gracefully … it may be necessary to rebuild part or all of the
  application cache"; events for `mmap` writes are not delivered at all [S7].

**Implication:** event-driven (daemon) change detection is the incumbent answer to "avoid rescanning
100k files" — but it requires a long-lived process, per-directory kernel watches, and a full-rescan
recovery path. A one-shot CLI task runner cannot ride it portably (macOS FSEvents / Windows
ReadDirectoryChanges have their own limits and semantics), which is why every mainstream runner
still performs a **detection sweep** on the run path and relies on metadata to make the sweep
cheap.

## 6. Git — the proven soundness model for "trust the stat, verify the content"

Primary source: git source + its documentation [S8][S9][S10].

- **Stat-cached entries.** "The index entries record the information obtained from the filesystem
  via `lstat(2)` system call when they were last updated … If some of these 'cached stat
  information' fields do not match, Git can tell that the files are modified **without even looking
  at their contents**." Compared fields: type, exec bit, `st_mtime`, `st_ctime`, `st_uid`,
  `st_gid`, `st_ino` (and `st_dev` behind a compile option; nsec fields when `USE_NSEC`) [S9].
- **Content verification on mismatch.** `ce_match_stat_basic` → `ce_modified_check_fs` → for
  regular files `ce_compare_data` **opens the file, hashes the content, and compares OIDs**
  (`read-cache.c`) [S9].
- **The racy-window fix.** "Racy-git" documents the one case the stat fast path cannot trust:
  a file modified within the same timestamp resolution as the index update. Git re-hashes
  racy entries instead of trusting them [S10].
- **Merkle directory hashing with dirty invalidation (cache-tree).** "The cache tree extension
  stores a recursive tree structure that describes the trees that already exist and completely
  match sections of the cache entries. This speeds up … by **only computing the trees that are
  'new'** … sections of the index can be skipped when a tree comparison demonstrates equality. When
  a path is updated in index, Git invalidates all nodes … corresponding to the parent directories
  of that path" [S8].
- **Tree objects are a Merkle forest**: a directory is the hash of its sorted (mode, name, child
  hash) entries, so any descendant change changes every ancestor hash [S8][S11].

## 7. rustc incremental compilation — the "dirty cutoff" invalidation rule

From the Rust Compiler Development Guide [S12]:

- Queries are colored: "red … its result has **changed** … green … **the same** as the previous
  compilation."
- Two rules: (1) all-green inputs ⇒ result unchanged, skip; (2) "even if some inputs to a query
  changes, it may be that it **still** produces the same result … after executing a query, we
  always check whether it produced the same result as the previous time. **If it did, we can still
  mark the query as green**, and hence avoid re-executing dependent queries." (try-mark-green)
- Salsa (the query-incremental library used by `rust-analyzer`) generalizes the same model: track
  accessed inputs, recompute a derived value, compare with the stored one, and propagate
  invalidation only past values that actually changed [S12].

## 8. The gap at 100k files

| Property | Vecto (today) [S3] | Turborepo git mode [S2] | Turborepo non-git [S2] | Watchman-style daemon [S6] |
|---|---|---|---|---|
| Warm run, 0 changes, 100k files | T walks + T×100k **content reads** (per task T) | index parse + 100k stat-compare + package OID folds | **full walk + full content re-hash** | O(1) query (daemon required) |
| Warm run, 1 file changed | same as above (plus the edit) | + 1 re-hash | full re-hash | O(1) query (daemon required) |
| Requires | nothing | **git repository** | nothing | long-lived daemon + per-dir watches |
| Hash unit | per-file, per-task, per-run | per-file OID, per-**package** fold | per-file, per-package | n/a (events) |
| Fingerprint cost for `packages/**` glob | O(matched files) | O(package files) | O(package files) | n/a |

Three gaps a Vecto algorithm can close, each with a primary-source precedent for soundness:

1. **Cross-run content-hash memoization with a stat fast path** (git's `ce_match_stat` + racy
   re-hash [S9][S10]; ccache manifest compare [S5]) — turns "read 100k files" into "stat 100k
   files, read 0".
2. **Merkle directory nodes so a glob's fingerprint is a *coverage* digest** (git tree objects
   [S8][S11]; git cache-tree dirty invalidation [S8]) — turns "fold 100k file hashes per task"
   into "one directory hash per covering directory", i.e. O(1) per directory-covering glob.
3. **Dirty-cutoff propagation** (rustc try-mark-green [S12]; git cache-tree [S8]) — recompute a
   directory hash; if it equals the stored one, stop propagating upward.

None of the incumbents ships all three in a **git-independent, daemon-free, zero-dependency
one-shot runner**. That combination — plus the fingerprint equivalence argument (below) — is the
novel algorithm. It is specified in [ADR-0019](../adr/0019-merkle-file-index-incremental-fingerprinting.md).

### Fingerprint equivalence (what "sound" must mean)

The task fingerprint must be a deterministic function of exactly:
(command, env, sorted dep fingerprints, multiset of (relative path, content) over the matched
input set). Any of: file content edit, add, delete, or rename ⇒ a file node hash changes ⇒ every
covering directory node changes ⇒ the coverage digest changes ⇒ the task fingerprint changes.
Conversely, if none of those change, the fingerprint is byte-identical. The stat fast path never
enters the fingerprint — it only decides whether the *stored* content hash is still valid (same
model and same documented residual risks as git's index [S9][S10]: a change that forges
identical `ctime/mtime/size/ino` metadata is undetectable without a content sentinel; `ctime`
cannot be set by user-space on Linux, which is why git and this design trust it).

---

## Conclusion & Recommendation

Adopt **Merkle File Index fingerprinting (VFI)** as specified in ADR-0019:

- Persistent Merkle index under `.vecto/` (already git-ignored), stat-cached content hashes,
  racy-window re-hash, symlinks always re-hashed.
- Task fingerprints computed from **glob-coverage digests** over Merkle nodes.
- Dirty-cutoff propagation so a 1-file edit touches O(depth) directory hashes, not O(n).
- Proof target: on a generated 100k-file tree, warm-run detection+replay faster than
  (a) Vecto's current per-task full-scan hashing and (b) Turborepo 2.x (git and non-git modes),
  measured per the method in ADR-0015 (`docs/benchmarks.md` §7).

**Status (2026-09-20):** implemented. `internal/fileindex` (VFI), ADR-0019 accepted,
proof in `docs/benchmarks.md` §7 — at 100k files: VFI 0.59s warm vs Vecto legacy 5.69s
(9.6×), Turbo non-git 1.02s (1.7×), Turbo git+daemon 0.58s (parity, no git/daemon/Node
required).

## Sources

- [S1] Turborepo docs, "Caching" (official): https://turborepo.dev/docs/crafting-your-repository/caching
- [S2] Turborepo source (primary), https://github.com/vercel/turborepo — `crates/turborepo-scm/src/repo_index.rs` (`new_from_gix_index`, stat options, racy deferral), `crates/turborepo-scm/src/manual.rs` (non-git walk + hash), `crates/turborepo-hash/src/lib.rs` (`FileHashes` Cap'n Proto fold), `crates/turborepo-daemon/src/lib.rs` (daemon role), `crates/turborepo-lib/src/task_graph/visitor/mod.rs` (`precompute_task_hashes`).
- [S3] Vecto source, this repo: `internal/hash/hash.go`, `internal/hash/glob.go`, `internal/runner/runner.go`.
- [S4] Bazel docs, "Remote Caching" (official): https://bazel.build/remote/caching
- [S5] ccache manual, "How ccache works" / direct mode manifest (official): https://manpages.ubuntu.com/manpages/xenial/man1/ccache.1.html
- [S6] Watchman docs (Meta, official): https://facebook.github.io/watchman/ and https://facebook.github.io/watchman/docs/troubleshooting (recrawl on inotify limits)
- [S7] `inotify(7)`, Linux man-pages 6.19 (official): https://man7.org/linux/man-pages/man7/inotify.7.html (non-recursive watches, per-user watch limit, queue overflow, mmap not reported)
- [S8] Git documentation, index format incl. Cache-tree extension (primary): https://github.com/git/blob/master/Documentation/gitformat-index.adoc
- [S9] Git source, `read-cache.c` (`ce_match_stat_basic`, `ce_modified_check_fs`, `ce_compare_data`) and `Documentation/technical/racy-git.adoc` (primary): https://github.com/git/git
- [S10] Git documentation, "Use of index and Racy Git problem" (primary): https://github.com/git/blob/master/Documentation/technical/racy-git.adoc
- [S11] Git tree object format (mode/name/OID entries; Merkle property) (primary): https://github.com/git/git — see `write-tree` / `read-tree` documentation and `cache-tree` extension.
- [S12] Rust Compiler Development Guide, "Incremental compilation" (red-green / try-mark-green) and "How Salsa works" (primary): https://github.com/rust-lang/rustc-dev-guide/tree/master/src/queries
