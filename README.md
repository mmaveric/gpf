# Go Pipeline Framework

GPF is a typed, batch-oriented Go pipeline framework. Build and validate a DAG,
then start fixed workers with bounded pools and queues. Concrete generic batches
and generated handlers keep data-plane values statically typed, without
reflection, runtime type assertions, or boxed payloads.

**Status:** unreleased; the target is **v0.1.0 experimental**, not a completed
release or an API-stability promise. Licensing is deferred and blocks publication. See
[release preparation and support evidence](RELEASE.md).

## Getting started

The module declares **Go 1.22** as its minimum. Fresh full-suite runs passed on
**Go 1.22.12 and Go 1.26.8, linux/amd64**, against the same completed source snapshot.
This is not a tested promise for Go 1.22.0, every newer Go version, or every OS.
See the [verification status](RELEASE.md#compatibility-evidence).

From a checkout of the repository, run the small, finite example:

```sh
go run ./examples/quickstart
```

Expected standard output:

```text
numbers: [10 30]
rejected "not-a-number": not a decimal integer
```

The example submits `10`, `not-a-number`, and `30` through a string ingress,
parses valid integers, and sends failures to a separate typed error sink. A
rejected input is an expected data result, not a command failure.

Read the [quickstart guide](examples/quickstart/README.md) for the complete code,
tests, `-cpu` selection, optional DOT export, batch ownership, and shutdown.
It needs no input files or external services. Its default CPU request is 0,
which may not be allowed by your host/container. An affinity warning on standard
error means the worker continues on a locked OS thread, **not** that CPU pinning
succeeded.

For a larger order-processing pipeline, see the
[showcase](examples/showcase/README.md). For the comparison workload, see the
[standalone showcase](examples/standalone-showcase/README.md). Their own guides
describe current inputs, timing options, and measurement commands. Their ordinary
tests do not need large JSON files; optional application/file-benchmark inputs
come from the [deterministic fixture generator](examples/testdata/README.md).

The import path is `github.com/mmaveric/gpf`. Until a release policy is settled,
review and pin the commit you use rather than assuming a stable versioned API.

## Features and contracts

| Topic | Capabilities | Detailed contract |
| --- | --- | --- |
| Typed data and graph | Preallocated batches, explicit ownership, generated `Node0`–`Node8` and closure handlers, typed ports, DAG validation, immutable definitions | [Typed data and graph](spec/01-typed-graph.md) |
| Execution and lifecycle | Manual fixed groups, explicit CPU bindings, affinity fallback, typed external ingress, transactional startup, drain and close | [Execution and lifecycle](spec/02-execution-lifecycle.md) |
| Parallel topology and transport | Local/SPSC/MPSC/SPMC/MPMC queue planning, explicit fan-in, shallow-copy broadcast, element sharding, fixed group replicas and node worker pools | [Topologies and transport](spec/03-topologies-transport.md) |
| Errors and inspection | Separate `ErrorPort[E]` / `ConnectError`, numeric metrics and user counters, optional handler/worker timing, bounded asynchronous error logging | [Errors and observability](spec/04-errors-observability.md) |
| Planning and performance | Opt-in automatic previews and explicit CPU-complete finalization, inactive NUMA metadata, allocation gates and benchmark policy | [Planning and performance](spec/05-planning-performance.md) |

The [specification guide](spec/README.md) is the reading map. Package
documentation describes the Go APIs; the quickstart shows how to compose them.
Source review also found [open conformance gates](RELEASE.md#conformance-review-gates);
the specs and passing tests do not establish that every intended guarantee is met.

### Ownership and overload essentials

- Queue `Capacity` counts **batches**; `BatchCapacity` counts **elements per
  batch**. Pools are separately bounded. Exhaustion does not allocate replacement
  storage and is not proof that a queue is full.
- Acquired batches are owned until moved or released. A successful output
  `Commit` moves the batch for publication after the handler returns. Inputs
  that handlers neither move nor explicitly release are reclaimed by the runtime.
- Graph-edge `Drop` releases a rejected output. `Wait` retains pending output
  for scheduler retry and pauses that runner, not the consumer worker. External
  ingress is nonblocking: callers must check statuses and retry or release
  retained acquisitions themselves. Accepted/Dropped ingress commits consume
  ownership; Full/Closed commits retain it.
- A failed `Result[E]` discards ordinary outputs and routes at most one
  one-element typed error batch. Unconnected failures, error-pool exhaustion,
  and configured Drop may lose the failure value. Error logging is a separate,
  optional bounded path, not a delivery guarantee.
- Ordinary connections do not imply fan-out or fan-in. Broadcast copies are
  shallow; sharding selects one destination per element. Parallel paths do not
  promise a global order or fairness. See the topology contracts before composing
  these features with replicas or node pools.

## Limits and operational expectations

- Every physical worker needs an explicit unique nonnegative CPU binding in its
  fixed plan. Linux attempts per-thread affinity; unavailable/rejected affinity
  keeps the thread locked and reports a fallback through `AffinityStatus`.
  There is no automatic host CPU discovery or CPU-set selection policy.
- Manual placement is the default. Automatic planning requires an explicit
  preview, CPU assignment, and finalization; a preview alone cannot execute.
  Replica and node-pool sizes are fixed, not dynamically resized.
- Ingress adapters cannot be replicated or node-pooled. A node pool cannot
  overlap whole-group replication. Broadcast and sharding endpoints cannot
  participate in either expansion. Custom queue factories require local
  single-worker ownership.
- NUMA annotations and typed allocator descriptors are **inactive metadata**.
  They do not discover topology, bind memory, activate allocators, or prove memory
  locality. Descriptors are not invoked during registration, Build, or Start.
- `Drain()` has no deadline. `DrainContext(ctx)` closes admission and drains
  accepted work, switching to forced discard at an acknowledged worker barrier
  on cancellation. This is cooperative: a handler, queue callback, logger
  callback, or lifecycle hook that never returns can prevent shutdown. `Close()`
  discards outstanding work rather than completing normal delivery.
- There is no handler retry or panic recovery. Concurrent starts from one
  definition and concurrent lifecycle calls are unsupported. Definitions may be
  reused after an executor stops; callers must not share mutable factory state
  between physical instances accidentally.
- Metrics snapshots are copied, but live fields are read independently rather
  than transactionally. Queue/pool gauges are sampled observations. Timing is
  optional and reports measured wall durations, not CPU usage or extrapolated
  unsampled work. Export backends are not provided.
- Zero-framework-allocation guarantees apply to the supported built-in
  steady-state paths after setup, not arbitrary user handlers, callbacks,
  custom queues, snapshots, startup, or shutdown. Zero allocations alone do not
  establish throughput or tail-latency performance.

## Development

```sh
go test ./...
go test -run '^$' -bench . -benchmem ./internal/benchtest ./graph ./queue
```

Generated contracts are committed. See [CONTRIBUTING.md](CONTRIBUTING.md) for
regeneration, fresh/race checks, and benchmark expectations. Throughput and
latency are comparison signals, not merge-blocking thresholds.
The benchmark command above targets framework packages; example file-loading
benchmarks require optional fixtures. Follow the
[Showcase](examples/showcase/README.md#reproducible-benchmark-boundaries) and
[standalone](examples/standalone-showcase/README.md#benchmarks-without-overlays)
guides for generation and package-specific `-showcase-data` selection; do not pass
example-only flags to the framework command.

## Release preparation and licensing

See the [Unreleased changelog](CHANGELOG.md) and [release checklist](RELEASE.md).
The project is licensed under [MIT](LICENSE), with the copyright notice
`Copyright (c) 2026 Altynbek Sharipov`. Repository access alone is not a license
grant; verify rights to all included materials before redistribution.
