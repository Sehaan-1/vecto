# ADR-0006: Reactive Event-Driven Task Scheduling

**Status:** Accepted  
**Date:** 2026-09-20

## Context

The original runner grouped tasks into execution layers using `ExecutionLayers()` (Kahn's BFS) and executed each layer with `wg.Wait()` before advancing to the next.

**Problem — head-of-line blocking:** If Layer 0 contains Task A (60 seconds) and Task B (1 second), and Layer 1 contains Task C (which only depends on B), the original scheduler forces C to wait 59 idle seconds for A, even though C's dependency is already satisfied.

This is the same failure mode that makes naive Makefiles slower than `ninja` or Bazel on large graphs with asymmetric task durations.

## Decision

Replace layer-based execution with a **reactive event-driven worker pool**:

1. Compute `pendingDeps[task]` — the count of needed dependencies not yet resolved — once at startup.
2. Push all zero-dep tasks into a buffered `ready` channel.
3. The main goroutine drains exactly `len(needed)` tasks from the channel. For each task, it spawns a goroutine that:
   - Checks skip/cancel status (no semaphore consumed for skips).
   - Acquires a bounded concurrency semaphore.
   - Executes the task.
   - Decrements `pendingDeps` for each dependent; pushes any that reach zero to `ready`.
4. The channel stays live until all tasks are dispatched. `wg.Wait()` collects stragglers.

## Consequences

**Positive:**
- Tasks are dispatched the instant their last dependency resolves, regardless of sibling task durations.
- `TestRunner_NoHeadOfLineBlocking` documents and enforces this property: a 1.5s task and an instant task run concurrently; their shared dependent fires within the ~1.5s window.
- Skipped tasks (failed dependency chain) do not consume semaphore slots — they clear their counter and push downstream tasks to propagate the skip cascade.

**Neutral:**
- `ExecutionLayers()` is preserved as a public Graph API (used for documentation, debugging, and dry-run display). It is no longer on the hot execution path.

**Negative / Trade-offs:**
- The dependency-counter approach requires the runner to track `pendingDeps` state that was previously implicit in the layer structure.
- Scheduling order within a wave of simultaneously-ready tasks is non-deterministic (goroutine dispatch order). Determinism within a layer is preserved by the sorted `seeds` slice at startup but not guaranteed for tasks that become ready mid-run.

## Alternatives Considered

- **Channel-per-task futures:** Each task writes its result to a dedicated channel; dependents `select` on their deps' channels. Cleaner semantics but O(E) channel allocations and complex fan-in.
- **Keep layer-based + add intra-layer parallelism:** Does not fix head-of-line blocking between layers; merely shuffles which tasks are blocked.
