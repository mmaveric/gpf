## Context

The repository currently contains only the implementation plan. That plan targets a Go 1.22+ batch-oriented DAG framework with generic heterogeneous ports, zero steady-state allocations, interface-free scheduling, and a later hybrid multi-group runtime. The first implementation stage must make its API and behavioral contracts executable before storage, graph execution, or concurrency mechanisms exist.

## Goals / Non-Goals

**Goals:**

- Create a Go module and public package skeleton that compiles on Go 1.22.
- Represent the planned public contract in typed generic declarations for batches and pools, graph ports and `Connect[T]`, arity-specific node and output forms, `Result[E]`, queue operations, and runtime configuration.
- Document ownership and lifecycle rules next to their owning API, including move, automatic release, pool exhaustion, queue policies, error routing, and restart semantics.
- Establish sub-benchmarks and `testing.AllocsPerRun` assertions as a stable performance test interface for later stages.
- Make valid heterogeneous graphs compile in examples without using `any`, runtime type assertions, or reflection in the declared hot-path contracts.

**Non-Goals:**

- Implement batch allocation, pools, graph validation, node scheduling, queues, CPU affinity, shutdown, metrics, logging, or generated arity source.
- Promise zero allocations for paths that do not yet execute; this change creates the gate and test shape only.
- Add optimized, unsafe, assembler, cgo, or NUMA-specific paths.
- Provide a compatibility guarantee before the initial framework release.

## Decisions

### Use a compileable API skeleton as the contract source of truth

Public declarations and their package documentation will be implemented in Go rather than maintained only in prose. This gives later stages compiler-checked signatures and allows consumers to validate generic edge compatibility. The declarations do not perform pipeline work until the owning implementation stage.

Alternatives considered:

- A markdown-only API contract: rejected because signatures and type relationships could silently diverge from code.
- Implementing the batch subsystem immediately: rejected because the documented roadmap places contract and measurement work first.

### Keep package ownership aligned with the planned module layout

The initial layout will introduce `batch`, `graph`, `node`, `queue`, `runtime`, and internal benchmark support as required for the public contracts. `edge`, `affinity`, `metrics`, and `eventlog` remain absent until they own executable behavior. This keeps imports directional and avoids placeholder packages with no contract to expose.

Alternatives considered:

- A single root package: rejected because it makes the eventual boundaries described in the plan harder to preserve.
- Scaffold every planned package: rejected because empty packages obscure which behavior is actually supported.

### Model typed data-plane boundaries with generics and function fields

`Batch[T]`, typed input/output handles, `Connect[T]`, `Result[E]`, and `QueueOps[T]` will be generic declarations. Scheduler-facing work remains represented by concrete function types and numeric metadata rather than interface contracts. The design permits function pointers and closures, as the plan does, but forbids `interface{}`, reflection, and runtime type assertions in future steady-state paths.

Alternatives considered:

- An interface-based queue or node abstraction: rejected because it violates the planned data-plane constraint.
- A runtime type registry: rejected because `Connect[T]` must enforce compatible edges at compile time.

### Capture semantics in API documentation and tests before enforcement is possible

Package docs and focused tests will specify that a handler owns its input, may move only a compatible batch, and otherwise has it released by the framework; exhausted pools drop rather than allocate; `Wait` spins and `Drop` releases; definitions become immutable after build; every start creates a fresh runtime. Tests validate declarations and documented contract fixtures, not unimplemented execution.

Alternatives considered:

- Deferring ownership semantics until pool implementation: rejected because those semantics determine every later API.

### Make benchmark registration and allocation assertions independent of implementation

The harness will define named benchmark cases for node calls, pool operations, local edges, remote queues, and an end-to-end DAG. Each case will report allocations and expose an `AllocsPerRun` assertion that implementation stages enable once their path is executable. Benchmarks will include batch size and topology dimensions in subtest names to preserve comparability.

Alternatives considered:

- Adding benchmarks after implementation: rejected because there would be no agreed measurement surface or regression gate.
- Enforcing throughput or latency thresholds: rejected because the plan explicitly limits the CI gate to allocations and keeps performance reports for manual comparison.

## Risks / Trade-offs

- [Premature API shapes constrain later implementations] → Keep contracts limited to signatures and semantics explicitly established in the plan; defer convenience APIs and configuration detail.
- [Stub declarations could be mistaken for working runtime features] → Document unsupported operations clearly and do not provide constructors that imply execution is available.
- [Allocation gates can pass while CPU efficiency regresses] → Report benchmark results and retain the documented limitation that allocation checks are not throughput or latency gates.
- [Go 1.22 generic limitations require repetitive arity APIs] → Define the arity-generation boundary and defer generated `Node0` through `Node8` bodies to the graph-definition stage.

## Migration Plan

1. Add the module, package contracts, documentation, examples, and benchmark harness.
2. Run formatting, unit tests, compile examples, and allocation-harness tests in CI.
3. Build the batch memory subsystem against these contracts in the next change; update contracts only through a new OpenSpec change if implementation exposes a conflict.

There is no deployed system or persisted data. Rollback consists of reverting this initial module change.

## Open Questions

- What canonical module import path should be used before the repository is published?
- Which benchmark result format and storage location should CI use for non-blocking throughput and latency reports?
