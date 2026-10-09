## Context
See proposal.md. Drain uses worker pause barriers and repeated quiescence checks; user callbacks are synchronous and cannot be forcibly interrupted safely.
## Goals / Non-Goals
Goals: cancel further draining and reclaim framework-owned work. Non-goals: kill a goroutine running a blocking handler, selector, queue callback, or Close hook.
## Decisions
- Add DrainContext(ctx) error; Drain delegates with an uncancelled context. Check cancellation before admission closure and between acknowledged worker passes.
- Reuse the all-worker barrier before forced discard, retain accepted-work and cleanup counts, and return errors.Join(context cause, lifecycle failures).
- Cancellation requests forced shutdown but does not promise wall-clock completion if user callbacks do not return. Document cooperative callback obligations explicitly.
- Add lifecycle participants for future ingress and logger shutdown. All participants receive the same deadline; no component restarts a fresh timeout.
- At the pause barrier, disable new source invocations while continuing existing source-output retries. Generated source runners consult an invocation gate after completing their pending finalizers; retry success does not call the source handler again during drain.
## Risks / Trade-offs
- Worker acknowledgment hangs in user code → State the cooperative limit and test cancellation with returning callbacks, not forced goroutine abandonment.
- Cancellation after quiescence → Decide completion at the barrier; do not discard successfully drained work.
## Migration Plan
Requires ownership and base metrics. Preserve sequential lifecycle calls and repeat-stop results; adapt ingress/logger participants as those changes land.
