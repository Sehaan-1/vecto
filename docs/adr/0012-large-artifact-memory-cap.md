# ADR-0012: What happens to huge files when saving

- **Status:** accepted
- **Date:** 2026-09-20
- **Card:** [How do we stop huge files blowing up memory?](https://github.com/Sehaan-1/vecto/issues/15)
- **Board:** [Week 1: Stop losing points](https://github.com/Sehaan-1/vecto/issues/12)
- **Supersedes:** none
- **Superseded by:** none

## Context
`Store()` kept every saved file fully in memory (`artifactBytesMap`) for async remote upload, and the remote bundle encodes bytes as base64 JSON (+33%). One 100MB output spikes the run. We chose between keeping big files on disk and skipping them from memory upload (A) and keeping everything in memory (B). Full streaming rewrite is explicitly out for this week.

## Decision
Big files stay on disk and do not blow up memory. Files over 50MB (`MaxRemoteArtifactBytes = 50 << 20`) are still copied to local cache on disk with hash verification, but excluded from the in-memory remote-upload map. `copyAndHashFile` streams (no full-file buffer for large files). A 100MB regression test pins this.

## Consequences
What this means from now on:
- **People notice:** big builds finish with flat memory; small files still share to team cache.
- **Later cards/ADRs must:** a future streaming-upload ADR must handle the skipped `>50MB` remote case; the 50MB constant lives in one place.
- **We give up:** sharing huge files remotely this week (local cache still works).
- **Look/CI/proof:** `TestCache_LargeArtifactSkippedFromMemory` with 100MB artifact, green under `-race`.

## History
- 2026-09-20 accepted
