## 1. Merge Topology
- [x] 1.1 Confirm queue selection, ownership, and base metrics are complete; verify shared-queue conformance tests.
- [x] 1.2 Add typed FanIn registration with atomic validation; verify foreign/duplicate/cyclic endpoint rejection preserves all prior connections.
- [x] 1.3 Materialize one shared queue and distinct origin-owned producer pools; verify selected model and per-producer Wait/Drop semantics.
## 2. Ownership and Completion
- [x] 2.1 Deduplicate shared queue drain/close ownership; verify each accepted/pending batch releases once to its original pool.
- [x] 2.2 Add concurrent uniqueness/property tests and serial allocation gates; verify unordered delivery has no hidden ordering reconstruction or framework allocations.
- [x] 2.3 Reconcile current merge/cardinality deltas/docs and run `go test -race ./...` and `openspec validate add-unordered-fan-in --strict`.
