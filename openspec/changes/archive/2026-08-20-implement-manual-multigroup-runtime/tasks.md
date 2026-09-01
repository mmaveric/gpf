## 1. Manual Placement and Immutable Planning

- [x] 1.1 Add `InvalidGroupID`, `DefaultGroupID`, placement sentinels, and the exact `Place(*Builder, NodeID, GroupID) error` API with documented precedence and no topology or callback side effects.
- [x] 1.2 Add builder-owned placement metadata that accepts one nonzero assignment for each registered node, defaults unplaced nodes to group 1, and remains sealed after successful Build.
- [x] 1.3 Extend validation reporting with deterministic invalid-placement and remote-custom-factory issue codes; preserve factory dormancy and Build’s transactional correction behavior.
- [x] 1.4 Compile and deep-copy a private immutable group plan at Build: ascending sparse group IDs, canonical per-group node order, and local-or-remote classification for every edge.
- [x] 1.5 Add construction and Build tests for placement precedence, sparse IDs, defaulting, duplicate rejection, plan determinism, immutability, and remote factory rejection without invocation.

## 2. Typed Local and Remote Edge Binding

- [x] 2.1 Keep existing local-edge pool, nil-factory ring, and custom `QueueOps` binding behavior only for same-group classified edges.
- [x] 2.2 Implement a preallocated typed bounded remote SPSC ring with correct atomic acquire/release cursor handoff, FIFO behavior, and no locks, channels, type erasure, or steady-state allocations.
- [x] 2.3 Bind each remote edge to the built-in SPSC ring, preserve typed output acquisition and origin-aware release, and prevent remote custom factory invocation.
- [x] 2.4 Apply existing Drop and one-pending-output Wait semantics to remote SPSC publication while allowing the independently scheduled consumer group to progress.
- [x] 2.5 Add focused local-versus-remote binding tests for queue selection, capacity, FIFO, ownership transfer, Drop, Wait retry, and exact release behavior.

## 3. Fixed-Plan Startup and Scheduling

- [x] 3.1 Partition generated monomorphized runners by the immutable group plan while retaining typed input polling, output finalization, selector behavior, and canonical runner order within each group.
- [x] 3.2 Generalize Start to materialize all edge resources in EdgeID order, all node factories and Init callbacks in global canonical order, then create exactly one worker goroutine per group in ascending GroupID order.
- [x] 3.3 Preserve one factory result per logical node per start, fresh sequential starts, and existing one-group local behavior without adding replicas, pools, affinity, or Start configuration.
- [x] 3.4 Extend startup rollback to release local and remote framework-managed batches, avoid worker publication on failure, and close all acquired node lifecycles in reverse global canonical order with joined errors.
- [x] 3.5 Add end-to-end explicit-state and closure tests for two or more groups, heterogeneous remote inputs, exact worker count, factory freshness, local custom-factory eligibility, and rollback order.

## 4. Coordinated Lifecycle

- [x] 4.1 Add lifecycle control that pauses every group worker at runner boundaries in deterministic group order and disables sources without placing channels in the batch data path.
- [x] 4.2 Implement Drain as repeated pause, inspect, and resume rounds until all local queues, remote queues, and pending Wait outputs are empty in one stable global paused snapshot.
- [x] 4.3 Implement Close from the pause barrier by releasing queued and pending local and remote work without further handlers, stopping every worker, and closing node lifecycles in reverse global canonical order.
- [x] 4.4 Retain stopped-call idempotence and cached lifecycle errors while explicitly leaving concurrent Start and lifecycle calls unsupported.
- [x] 4.5 Add lifecycle tests for cross-group drain, forced close, source suppression, late remote delivery prevention, exact once-only release and Close, joined Close errors, and default-group equivalence.

## 5. Performance, Race, and Documentation Gates

- [x] 5.1 Add post-Start `testing.AllocsPerRun` coverage for built-in SPSC push/pop and fixed two-group source-to-sink delivery, preserving zero allocations for existing local paths.
- [x] 5.2 Enable the remote-queue and two-group end-to-end benchmark matrix entries with `ReportAllocs`, setup exclusion, batch and group dimensions, and documented non-blocking throughput/latency interpretation.
- [x] 5.3 Run focused concurrent handoff and lifecycle coverage with `go test -race ./...`; investigate and eliminate races and goroutine leaks in the supported SPSC and shutdown paths.
- [x] 5.4 Update README and graph, queue, node, batch, and runtime documentation to describe fixed placement, default group behavior, local versus remote queue policy, deterministic lifecycle, and all deferred features.
- [x] 5.5 Run gofmt, generation verification, `go test ./...`, `go test -race ./...`, `go vet ./...`, allocation tests, benchmarks, and `git diff --check`.
- [x] 5.6 Run `openspec validate implement-manual-multigroup-runtime --strict` and `openspec validate --all --strict`, resolving validation failures attributable to this change.
