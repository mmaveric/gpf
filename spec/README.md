# GPF specifications

The five topic contracts describe the current system, not an implementation-stage
sequence. **Must/must not** are requirements; **may** is permission. Examples and
dated research explain the contracts without adding hidden behavior.

| Topic | Read for |
| --- | --- |
| [Typed data and graph](01-typed-graph.md) | Batch ownership, node/factory types, handles, registration, connection errors, DAG diagnostics, immutable Build. |
| [Execution and lifecycle](02-execution-lifecycle.md) | Manual groups/CPUs, startup/rollback, locked workers/affinity, dispatch, ingress, drain/deadline/Close. |
| [Topologies and transport](03-topologies-transport.md) | Queue selection, bounded concurrent queues, broadcast, shard, fan-in, replicas, node pools. |
| [Errors and observability](04-errors-observability.md) | Typed failure delivery, mapping/logging, numeric snapshots/counters, handler/worker timing, clock evidence. |
| [Planning and performance](05-planning-performance.md) | Automatic constraints/preview/finalization, NUMA preparation only, allocation/benchmark gates, optional fast paths. |

GPF builds a typed DAG in a mutable control plane, then starts fresh fixed workers,
pools, and bounded queues from immutable plans. Ownership is exclusive; overload
and cleanup are explicit. Manual execution is the default; automatic execution
requires preview, complete CPU assignment, and finalization. Parallel paths add
no global ordering guarantee. Shutdown is cooperative, so deadlines cannot
interrupt nonreturning user callbacks. NUMA intent is metadata, not placement.
