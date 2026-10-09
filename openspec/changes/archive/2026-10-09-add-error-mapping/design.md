## Context
See proposal.md. Typed failure bindings already retain concrete E; mapper contracts can be established before logger transport is enabled.
## Goals / Non-Goals
Goals: fixed event representation and typed configuration. Non-goals: formatting strings, transporting logs, or guaranteeing user callback allocation behavior.
## Decisions
- Define ErrorEvent with numeric Code/Severity and fixed [4]uint64 attributes; no strings, slices, pointers, or interfaces. Node/replica identity is populated by the runtime at publication.
- Define ErrorMapper[E] func(E) ErrorEvent plus validated typed mapper configuration, captured immutably with the node registration. Build stores it without invocation.
- Supply a concrete typed conversion helper and generated publication hook; without a logger destination the mapper remains dormant. The async logger change activates one mapping per failed return, independent of error-port pool/drop outcomes.
- Treat mapper callbacks as nonblocking/nonallocating user contracts. Framework cannot certify them and does not recover panic.
## Risks / Trade-offs
- Fixed payload limits expressiveness → Put rich formatting in the control-plane logger.
- Mapping accidentally depends on error delivery → Invoke from failed return, not from accepted error-edge publication.
## Migration Plan
Requires ownership. Add mapping contracts and dormant hooks, then let async logging integrate them; no standalone mapping execution is falsely advertised as logging.
