## 1. Example Pipeline

- [x] 1.1 Add typed order, validation-error, and result models plus named bounded-capacity/group configuration; verify the showcase package builds without dependencies beyond the standard library.
- [x] 1.2 Implement the fixed-group graph with ingress, validation, typed error routing, audit broadcast, customer-key sharding, partition processors, and unordered fan-in; verify tests inspect queue plans and distinct branch identities (end-to-end routing is verified in 1.3).
- [x] 1.3 Add the finite sample workload, checked ingress submissions, application-owned atomic summaries, graceful Drain, affinity reporting, and runtime metrics output; verify accepted/rejected totals and resource cleanup in an end-to-end test.

## 2. Documentation and Integration

- [x] 2.1 Add `examples/showcase/README.md` with the topology, feature notes, run command, and affinity-fallback behavior; verify every documented command works.
- [x] 2.2 Link the showcase from the root README and verify the example with `go run ./examples/showcase`, `go test ./...`, and `go vet ./...`.
