# GPF specifications

The detailed contracts are consolidated into five documents. Read top to
bottom for the system model, or choose a topic:

1. [Typed data and graph](01-typed-graph.md) — batches, ownership, node and port
   contracts, graph construction and validation, immutable definitions.
2. [Execution and lifecycle](02-execution-lifecycle.md) — executors, placement,
   ingress, CPU affinity, draining, and shutdown.
3. [Topologies and transport](03-topologies-transport.md) — bounded queues,
   fan-out, sharding, fan-in, replicas, worker pools, and queue planning.
4. [Errors and observability](04-errors-observability.md) — typed error
   delivery, logging, metrics, and processing-time measurement.
5. [Planning and performance](05-planning-performance.md) — automatic
   grouping, CPU assignment, NUMA metadata, benchmarks, and optimization policy.

The five documents retain the current normative requirements and behavioral
scenarios. Earlier capability files have been merged into these topics rather
than maintained as a second source of truth.

## Architectural baseline

GPF is a batch-oriented typed pipeline framework: a mutable control plane builds
and validates a DAG, then an immutable definition starts fresh execution
resources. Data-plane values remain statically typed; bounded pools and queues
make ownership and overload behavior explicit. Work is grouped into fixed
workers; manual placement is the baseline, while automatic plans require an
explicit preview and finalization. Parallel topology is explicit (replicas,
worker pools, sharding, fan-out, and fan-in); ordering is not guaranteed across
parallel paths. Graceful drain is bounded by a deadline, and forced cleanup
returns owned resources exactly once.

These consolidated contracts supersede the earlier staged implementation
plan. Its API sketches and phase sequence are historical, not additional
requirements or a current roadmap. When details differ, the five capability
documents above are authoritative.
