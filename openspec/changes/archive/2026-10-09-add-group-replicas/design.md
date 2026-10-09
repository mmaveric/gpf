## Context
See proposal.md. Definitions contain logical nodes/groups; queue selection can resolve numeric physical cardinalities but does not expand runtime instances.
## Goals / Non-Goals
Goals: fixed whole-group replication. Non-goals: dynamic scaling, source broadcast, or node-pool overlap.
## Decisions
- Register a positive replica count and an explicit unique CPU for each physical group instance before Build. Assign stable (logical GroupID, replica index) identities without changing logical port handles.
- Pair same-group internal edges by replica index. Boundary edges use shared typed queues selected from physical cardinalities, giving each accepted batch to one destination replica; no automatic copying.
- Materialize each node factory once per physical replica per Start, preserving canonical logical-node then replica ordering for rollback and reverse cleanup. Source replicas execute independently with factory-local state.
- Extend metric and affinity rows with replica identity. Reject partial CPU assignment, overlapping node pools, or unsupported custom remote transport atomically.
- Ingress adapters cannot belong to a group with more than one replica in this version. An unreplicated ingress can feed a replicated downstream group through shared boundary transport.
- Generated BindCounter callbacks receive each replica's own registered slots. Add CounterForReplica(NodeID, replicaIndex, CounterID); existing Counter rejects ambiguous replicated-node lookup instead of selecting or aggregating a slot. Replica indices are zero-based, including zero for nonreplicated nodes.
## Risks / Trade-offs
- Factories share user mutable captures → Document existing author obligations and verify framework-local state isolation.
- Expanded topology changes queue ownership → Select models after expansion and test multiple producers/consumers.
## Migration Plan
Requires ownership, queue selection, and base metrics. Preserve count-one manual definitions; reconcile central replica deferrals against current specs when applying.
