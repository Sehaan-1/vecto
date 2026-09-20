# ADR-0009: Graph Inspection and Dry-Run Execution Mode

- **Status:** accepted
- **Date:** 2026-09-20
- **Author:** Vecto Engine Team

## Context
Developers working in complex multi-task or monorepo setups need visibility into the build topology and execution planning before running computationally expensive or side-effecting commands:
1. **Visual Documentation:** Developers often want to document their build graph in GitHub PRs, READMEs, or architecture documents without relying on external diagram tools.
2. **Predictive Execution Planning:** Before running a full build, developers or CI pipelines want to preview which tasks will hit the cache (`[⚡ CACHED]`) versus which tasks have been invalidated and will execute (`[WILL EXECUTE]`), without actually running shell commands.
3. **CLI Argument Flexibility:** Real-world workflows require passing dynamic flags directly to underlying commands (e.g. `vecto run test -- -v -run TestFoo`).

## Decision
We implement graph visualization, dry-run simulation, and argument passthrough:

1. **`vecto graph [--format=mermaid|dot] [targets...]` Subcommand:**
   - **Mermaid.js Format (default):** Emits standard GitHub-flavored Mermaid `graph TD` syntax. Dependencies point to dependents (`dep --> task`), allowing direct inclusion in markdown files and pull requests.
   - **Graphviz DOT Format:** Emits `digraph G { ... }` compatible with standard UNIX visualization utilities (`dot`, Graphviz).
   - If targets are specified, only the target tasks and their transitive ancestors are visualized.

2. **`vecto run --dry-run` Flag:**
   - Resolves the complete DAG and identifies the required subgraph.
   - Computes cryptographic SHA-256 fingerprints in topological order (incorporating upstream dependency fingerprints).
   - Queries the cache store (`Has(hash)`) for every task.
   - Prints a formatted execution plan showing cache hits vs. cache misses without spawning subprocesses or mutating the cache.

3. **Dynamic Argument Passthrough (`--`):**
   - The CLI parser recognizes the standard POSIX `--` delimiter in `vecto run`.
   - Arguments trailing `--` are captured and appended to the underlying command string for the target task.

## Consequences
- **Positive:** Instant diagram generation with zero external dependencies.
- **Positive:** Enables CI pipelines to check cache status and plan builds before dispatching remote runners.
- **Positive:** Enhances CLI developer ergonomics to match modern tools like Turborepo and Cargo.
