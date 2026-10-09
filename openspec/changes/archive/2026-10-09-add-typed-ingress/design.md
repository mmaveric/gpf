## Context
See proposal.md. Current sources run on workers; external callers need a typed binding to a fresh runtime without boxed endpoint lookup.
## Goals / Non-Goals
Goals: single-producer ingress and coordinated admission. Non-goals: multi-producer submission or producer thread affinity.
## Decisions
- Register an IngressDefinition[T] with a typed output and dormant runtime binder; BindIngress[T](executor, definition) validates owner identity and returns a runtime-local Ingress[T]. Never keep live endpoints in Definition.
- Ingress owns a preallocated pool and SPSC queue into an adapter runner. Acquire returns ownership; Commit(*Batch[T]) returns accepted/full/closed status, consuming only on accepted publication or configured Drop. Wait-full retains caller ownership for retry, never blocks inside Commit.
- Submit copies at most one configured-capacity batch, rejects oversized slices atomically, and releases its private acquisition on any nonaccepted result. Empty Submit is a successful no-op.
- An admission flag plus in-flight commit counter closes admission before drain and waits for admitted nonblocking operations. Previously acquired uncommitted handles remain caller-owned and releasable after stop; Stop never frees their backing storage. No forced reclamation of producer-owned slots.
- Expose AddIngress[T], IngressDefinition.Output/NodeID, BindIngress, and explicit IngressStatus values. The source adapter is framework polling, not a user source handler, so its registered node's HandlerCalls stays zero. Numeric Ingresses rows track producer outcomes separately.
- The SPSC transport keeps separate producer/consumer physical indices and uint64 publication counters so arbitrary capacities retain FIFO order across counter wrap. The corrected wrap behavior is exercised by a regression before ingress adoption.
## Risks / Trade-offs
- Commit races stop → Double-check admission inside the in-flight protocol and test all acceptance/closure outcomes.
- Unreturned caller handles → Document release obligations; preserve storage instead of invalidating handles.
## Migration Plan
Requires ownership and base metrics. Reconcile the ingress deferral in current contracts when implementing; integrate a deadline-drain hook when that capability is present.
