## Context
See proposal.md. NUMA behavior is intentionally inactive; this change only makes placement/allocation intent representable.
## Goals / Non-Goals
Goals: immutable metadata and typed dormant hook descriptors. Non-goals: OS topology discovery, local memory allocation, or changing CPU policy.
## Decisions
- Add optional NUMA IDs with zero-value unspecified and explicit presence semantics; validate representable IDs without pretending to discover host topology.
- Add typed allocator hook descriptors for pool storage and attach numeric annotations to group/queue plans. Preserve hook function values dormant; registration, Build, and Start continue using existing pure-Go allocation.
- Copy annotations into definitions and introspection. Document that a configured dormant hook is not a promise it has been called; later activation must use a separate versioned change.
- Preserve source compatibility for keyed configurations and zero defaults; document that adding exported fields can affect callers using unkeyed literals.
## Risks / Trade-offs
- Metadata looks like activated NUMA behavior → Explicit inactive status and callback-count tests across Start.
- Untyped allocator would box batches → Keep typed descriptors and no runtime data conversion.
## Migration Plan
Requires CPU-complete auto planning. Add metadata without allocator dispatch; future-placement design consumes it separately.
