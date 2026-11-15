# Vecto docs

This directory is the design record behind Vecto: why the runner is shaped the
way it is, what was measured, and how each number was produced. Start with the
ADRs, then the findings that fed them, then the benchmarks, and treat the
process files as history rather than guidance.

## Reading order

1. **[ADRs](adr/README.md)** — one decision per file, each with context, consequences, and a history line. The index lists all of them.
2. **[Findings](findings/)** — longer investigations that produced those decisions, beginning with [custom DAG evaluation](findings/001-custom-dag-evaluation.md) and the [systems hardening roadmap](findings/002-systems-hardening-roadmap.md).
3. **[Benchmarks](benchmarks.md)** — measured tables, environment blocks, and the exact rerun command behind every number in the README.
4. **Process log** — [`lanes/`](lanes/) and [`cuecards/`](cuecards/) are the week-by-week working history kept for provenance; they are archived context, not current guidance.

`assets/` holds the demo GIF referenced by the README. Handoff notes are session
scratch, not part of the reading order.
