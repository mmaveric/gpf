## 1. Snapshot Contracts and Runtime Storage

- [x] 1.1 Capture pre-instrumentation local, remote, and error-routing benchmark results with `go test -run '^$' -bench . -benchmem ./...`; retain the command, toolchain, machine, and results for the final comparison.
- [x] 1.2 Add the public numeric snapshot types and `Executor.Metrics()` contract; verify nil/zero behavior, complete identity rows, sparse-group ordering, and defensive copies with API tests.
- [x] 1.3 Allocate runtime-local atomic metric records before edge/node binding and keep immutable row metadata after teardown; verify Build factory dormancy, independent executors, and fresh sequential Start counters.
- [x] 1.4 Separate producer and consumer update storage and implement atomic snapshot reads without worker commands or mutable runner access; verify concurrent readers under `go test -race ./graph`.

## 2. Generated Handler and Scheduler Accounting

- [x] 2.1 Extend explicit-state and closure templates for HandlerCalls, input batches/elements captured before moves, FailedResults, and UnconnectedErrors across arities zero through eight; verify representative source, sink, heterogeneous selector, moved-input, success, and failure tests.
- [x] 2.2 Regenerate committed arity outputs and verify `go test ./internal/genarity ./node ./graph` catches generated drift and compile errors.
- [x] 2.3 Record group Passes and IdlePasses at completed scheduler passes; verify idle polling, disabled sources, and pending Wait retry do not create handler or input counts.

## 3. Typed Edge and Queue Accounting

- [x] 3.1 Instrument source-pool acquisition wrappers for genuine exhaustion and exclude duplicate Acquire rejection; verify ordinary and error acquisition tests show exhaustion counts without fabricated batch losses.
- [x] 3.2 Record accepted/dequeued batches and elements at typed ownership boundaries for local built-in, custom local, and remote SPSC edges; verify exact totals after stop, including moved batches and one-element errors.
- [x] 3.3 Add distinct queue Drop, failed committed ordinary output, and Wait retry attempt accounting; verify an initial rejection followed by two failed retries and a successful third retry counts three retries and one accepted batch.
- [x] 3.4 Publish bounded QueueDepthSample and QueueHighWaterSample from owner transport callbacks, with race-safe maximum updates; verify sampled bounds, retained maxima, and absence of custom queue callbacks on snapshot reader goroutines.

## 4. Lifecycle and Concurrency Integration

- [x] 4.1 Count forced-close discards of queued and pending committed ordinary/error batches exactly once, capturing lengths before release; verify cleanup does not increment normal dequeues or double-count policy drops.
- [x] 4.2 Retain final records and clear depth samples at the existing stop barrier; verify Drain, repeated Close/Drain, sequential restart, and final immutable metadata snapshots.
- [x] 4.3 Exercise snapshot readers during local/remote processing and a single Drain or Close, plus startup rollback; verify `go test -race ./...` passes without data races, worker leaks, or changed lifecycle order.

## 5. Performance, Documentation, and Validation

- [x] 5.1 Extend serial `testing.AllocsPerRun` gates for instrumented local/remote ordinary/error delivery, Drop, Wait retry, and pool exhaustion; verify zero framework allocations with snapshots and setup outside measured operations.
- [x] 5.2 Repeat matching benchmark cases with metrics enabled and retain a before/after throughput and allocation comparison; verify ReportAllocs and batch/group dimensions remain present and review contention overhead.
- [x] 5.3 Document the snapshot API, counter boundaries, modulo overflow, sampled occupancy, live consistency, post-stop exactness, and deferred timing/export/logging; verify README and graph/node/queue documentation match the delta specs.
- [x] 5.4 Run generation stability checks, formatting, `go test ./...`, `go test -race ./...`, `go vet ./...`, and `git diff --check`; resolve regressions attributable to instrumentation.
- [x] 5.5 Run `openspec validate add-runtime-metrics --strict` and `openspec validate --all --strict`; verify the implemented behavior satisfies every new and modified requirement before completing the checklist.
