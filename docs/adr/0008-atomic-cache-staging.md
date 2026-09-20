# ADR-0008: Atomic Cache Staging via Staging Directory and Rename

- **Status:** accepted
- **Date:** 2026-09-20
- **Author:** Vecto Engine Team
- **Supersedes:** (extends [ADR-0003](0003-hybrid-cache-storage-directory.md))

## Context
In Vecto, cached task outputs (stdout/stderr logs, generated file artifacts, and metadata) are stored in `.vecto/cache/<hash>/`.

Previously, `Store` wrote files directly into the destination directory. This introduced critical failure modes:
1. **Partial Writes on Cancellation:** If the user sends `SIGINT` (Ctrl+C) or a sibling task fails under fail-fast mode while artifacts or logs are being copied, the destination directory is left in a partially written, corrupt state.
2. **Cache Poisoning:** Because `Has(hash)` checks for the existence of `meta.json`, if an interrupted run manages to write `meta.json` before artifacts finish copying, all future runs treat the task as a cache hit and restore truncated or missing artifacts.
3. **Concurrent Run Collisions:** If two Vecto instances or concurrent runners compile the same task concurrently, both would write to the same directory simultaneously, racing on open file handles.

## Decision
We implement **Atomic Directory Staging** using POSIX `rename(2)` / NTFS directory rename semantics:

1. **Staging Directory Isolation:**
   When storing a task's output, Vecto creates a unique temporary staging directory inside the cache root:
   ```text
   .vecto/cache/tmp-<hash>-<pid>-<timestamp>
   ```
   Crucially, placing the staging directory within `.vecto/cache/` guarantees that both the staging directory and the final cache directory exist on the **same filesystem mount**, which is a prerequisite for atomic filesystem renames.

2. **Clean Rollback on Interruption:**
   The staging directory is wrapped in a cleanup guard (`defer func() { if !committed { os.RemoveAll(stagingDir) } }`). If artifact copying, log writing, or JSON serialization fails or the process is interrupted, the partial staging directory is deleted immediately without touching the cache index.

3. **Atomic Commit via `os.Rename`:**
   Once all logs, artifacts, and `meta.json` are completely written and flushed:
   ```go
   os.Rename(stagingDir, finalDir)
   ```
   Under POSIX filesystems, directory renames within the same mount point are atomic. A cache entry is either 100% complete and valid, or it does not exist at all.

4. **Concurrent Store Convergence:**
   If a concurrent process finished writing and committed the exact same hash before this process completed, `os.Rename` may fail (e.g. on Windows or if the destination directory exists). Because Vecto's hashing is deterministic and content-addressed, both writes produce identical outputs. If `Has(hash)` is already true, the current staging directory is discarded and `Store` returns success.

5. **Garbage Collection of Abandoned Staging Dirs:**
   `cache.Prune` scans for abandoned `tmp-*` directories left by ungraceful crashes (e.g. `SIGKILL` or power loss) and purges them.

## Consequences
- **Positive:** Cache corruption is impossible. `Has()` and `Restore()` will never encounter half-written or missing artifact directories.
- **Positive:** Multiple concurrent Vecto processes safely converge on the cache without file lock deadlocks.
- **Negative:** Requires sufficient disk space during the write phase to hold both the staging directory and the existing contents until rename commits.
