## 1. Definition Snapshot Model

- [x] 1.1 Add an opaque `Definition` representation that retains the builder's exact non-zero private owner identity and stores dense node and edge metadata without referencing mutable builder topology or lifecycle state.
- [x] 1.2 Deep-copy the `ValidationReport` canonical forward and reverse adjacency plus canonical topological order into definition-owned storage, remapping internal references to copied data where required.
- [x] 1.3 Preserve dormant typed registration and edge-binding closures carrying `Stateful.Factory`, `ClosureFactory`, optional explicit lifecycle callbacks, and `queue.Factory` without invoking them or storing factory-produced state, `ClosureReplica`, or live `QueueOps`.
- [x] 1.4 Implement exactly `NodeCount`, `EdgeCount`, and `TopologicalOrder`, including nil and zero-value behavior, a new topological-order allocation on every valid call, and no public adjacency accessor.

## 2. Transactional Build

- [x] 2.1 Implement `func (b *Builder) Build() (*Definition, error)` so nil and zero-value receivers return `(nil, ErrInvalidBuilder)` without panic, allocation, or mutation.
- [x] 2.2 Wire open-builder Build to consume one `ValidationReport`, including sorted issues and canonical adjacency and order, without duplicating validation, adjacency construction, or sorting algorithms.
- [x] 2.3 Keep failed Build side-effect-free: return `(nil, *ValidationError)` without sealing, caching a partial definition, invoking a factory or callback, or preventing subsequent valid builder corrections.
- [x] 2.4 Construct the complete copied definition in local state after issue-free validation, then cache its pointer and seal registration and connection mutation; those mutation paths shall reuse the builder prerequisite's `ErrBuilderSealed`.
- [x] 2.5 Return the cached identical definition pointer and nil immediately on every Build after success without returning `ErrBuilderSealed`, revalidating, or reconstructing.

## 3. Definition Contract Tests

- [x] 3.1 Add tests that nil and zero-value builders return `ErrInvalidBuilder`, then correct topology on an initialized builder after failed Build and verify it remains open.
- [x] 3.2 Add tests that successful Build seals node registration and connection with the shared `ErrBuilderSealed` sentinel while Build itself continues returning the cache.
- [x] 3.3 Add tests that repeated successful Build calls return the same pointer without another validation pass and preserve the report's canonical adjacency and topological order.
- [x] 3.4 Add aliasing tests for source metadata, nested report adjacency, stored order, and repeated `TopologicalOrder` results, confirming all definition storage remains unchanged and no adjacency accessor is public.
- [x] 3.5 Add exact accessor tests for valid, valid-empty, nil-receiver, and zero-value definitions, including non-nil empty order for a valid zero-node definition.
- [x] 3.6 Add provenance tests showing every successful definition retains its builder's non-zero owner identity and rejects equality with zero or foreign-builder owners.
- [x] 3.7 Add observable tests proving state factories, closure factories, queue factories, and explicit lifecycle callbacks remain dormant, closure lifecycle callbacks remain uncreated, and no produced state or live `QueueOps` is stored.
- [x] 3.8 Add sequential-reuse contract tests at the definition boundary, confirming repeated inspection and cached Build reuse do not mutate identity, topology, or dormant factory bindings, without starting a runtime or executing a handler.

## 4. Documentation and Verification

- [x] 4.1 Document exact Build receiver-state behavior, correction after failure, `ErrBuilderSealed` mutation behavior, the three inspection methods, persistent non-zero identity, and captured factory configuration immutability in graph package documentation and examples.
- [x] 4.2 Update queue, runtime, and README documentation to describe dormant factories and sequential future reuse while clearly marking runtime starts, execution, scheduling, CPU affinity, concurrent starts, and live resources as unavailable.
- [x] 4.3 Run `gofmt` on changed Go files, `go test ./...`, `go test -race ./...`, and `go vet ./...`.
- [x] 4.4 Run `git diff --check` and `openspec validate --all --strict`, resolving all artifact and repository validation failures attributable to this change.
