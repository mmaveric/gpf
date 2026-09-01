## 1. Validation API, Report, and Diagnostics

- [x] 1.1 Add the invalid zero `ValidationCode` value, `ValidationCodeInvalidBuilder`, `ValidationCodeEmptyGraph`, and the remaining fixed exported emitted-code set in specified sort order, plus exact exported `ValidationIssue` code/entity fields and structured closed cycle witnesses.
- [x] 1.2 Add opaque `ValidationReport` storage plus `ValidationAdjacency` and `ValidationAdjacentEdge`, with deep-copy accessors for sorted issues, canonical topological order, and forward and reverse adjacency snapshots.
- [x] 1.3 Implement `ValidationError`, defensive deep-copy issue access, `errors.As`-compatible error behavior, exact report/error issue agreement, and stable entity-aware formatting.
- [x] 1.4 Implement deterministic issue comparison by code, node, port, edge, and lexicographic witness, with unit tests covering every ordering key and nested defensive copies.
- [x] 1.5 Implement `func (b *Builder) Validate() (*ValidationReport, error)` and table-driven tests for nil receiver, zero value, initialized empty, non-empty open, and already sealed builders, including exact report/error combinations and no `ErrBuilderSealed` result.

## 2. Registration Static Descriptors

- [x] 2.1 Add the non-generic registration validation descriptor with `uint16` required and present masks for factory bit 0, selector bit 1, and handler-position bits 2 through 9.
- [x] 2.2 Update every generated explicit-state registration path to set required and present bits directly from typed factory, source/input handler, and multi-input selector fields.
- [x] 2.3 Update every generated closure registration path to require and record only its typed closure factory, leaving factory-produced callbacks for runtime startup.
- [x] 2.4 Implement missing static issue generation from `required &^ present` without reflection, runtime type assertions, `any` callback storage, or callback invocation.
- [x] 2.5 Add table-driven tests across source, single-input, multi-input, explicit-state, and closure registrations, including mask mapping, zero/one-input selector behavior, and proof that no user factory or callback executes.

## 3. Connectivity and Shared Edge Validation

- [x] 3.1 Validate exactly one connection for each ordinary input and output while permitting zero ordinary arity and excluding non-connectable typed error outputs from all graph analysis.
- [x] 3.2 Reuse or extract the unexported non-generic pure edge-configuration validator called by `Connect`, and map its capacity and policy violations to edge-specific validation codes without duplicating predicates.
- [x] 3.3 Add tests proving `Connect` and whole-graph validation share capacity and policy decisions, typed queue factories remain dormant, and typed error outputs create no connectivity or adjacency data.

## 4. Deterministic Graph Analysis

- [x] 4.1 Snapshot report-owned forward and reverse adjacency with one ascending row per node and adjacent entries ordered by adjacent node then edge identity.
- [x] 4.2 Implement smallest-ready-`NodeID` Kahn sorting over the snapshots, retaining a full canonical order for every acyclic topology and no partial order for cyclic topology.
- [x] 4.3 Implement ascending residual DFS that returns the first deterministic closed back-edge witness, including `[N, N]` for a self-loop.
- [x] 4.4 Implement forward traversal from declared zero-input sources and reverse traversal from declared zero-output sinks over report snapshots, emitting per-node issues for each failed direction.
- [x] 4.5 Add focused tests for adjacency row/edge ordering, edgeless rows, tie-broken topological order, acyclic graphs with other issues, multiple possible cycles, cycle witness edge validity, self-loops, components without sources or sinks, and valid isolated zero-input/zero-output nodes.

## 5. Non-Mutation and Property Coverage

- [x] 5.1 Add tests proving successful and failed validation do not alter topology, consume identities, cache results, open or seal builders, or alter preexisting sealed state, and that failed open topology can be corrected in place.
- [x] 5.2 Add tests proving a report is an owned point-in-time snapshot unaffected by later open-builder mutation and suitable for direct package-private copying by final `Build`.
- [x] 5.3 Add fuzz tests over nil, zero, empty, open, sealed, and constructible builder operation sequences asserting termination without panic and identical reports and diagnostics across repeated unchanged validation.
- [x] 5.4 Add property checks that every issue-free report contains every node exactly once, places each ordinary edge's source before its target, and has mutually consistent forward and reverse adjacency.

## 6. Documentation and Verification

- [x] 6.1 Document the exact `Validate` signature and state matrix, report snapshot semantics, registration masks, shared pure edge checks, dormant callbacks, required ordinary connectivity, non-connectable error outputs, and the absence of new sealing or immutable definition construction in this stage.
- [x] 6.2 Run `gofmt` on touched Go files and run focused graph unit and fuzz smoke tests.
- [x] 6.3 Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and `git diff --check`.
- [x] 6.4 Run `openspec validate add-graph-dag-validation --strict` and confirm no `Build`, immutable `Definition`, or builder-sealing behavior was added.
