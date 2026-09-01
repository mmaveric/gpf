## Context

The completed manual-multigroup runtime compiles an immutable, ascending
`GroupID` plan and starts one goroutine per group. Its workers currently run
`Init`, handlers, and lifecycle cleanup without a locked OS thread or CPU
binding. The original plan requires a CPU ID for every group and asks the
runtime to retain a locked-thread fallback when platform affinity is not
available. The existing `Start`, `Drain`, and `Close` APIs must remain
configuration-free and deterministic.

## Goals / Non-Goals

**Goals:**

- Require exactly one nonnegative, unique CPU ID for every effective manual
  group before Build can publish a definition.
- Copy that binding plan into the immutable definition and create workers in
  group order that lock their own OS threads before affinity, `Init`, or
  processing.
- Use a Linux-specific adapter when compiled for Linux, with a pure-Go
  unsupported-platform adapter and a fallback after adapter failure.
- Make the successful affinity or locked-thread-fallback result observable
  without logging or metrics.
- Keep global canonical Init and Close order, rollback, Drain, and Close
  deterministic while every owned lifecycle callback runs on its locked worker.

**Non-Goals:**

- Automatic grouping, inferred placement, CPU discovery, CPU-set validation,
  NUMA placement, replicas, worker pools, sharding, fan-out, new queue models,
  ingress, error routing, metrics, logging, or concurrent starts/lifecycle
  calls.
- A Start-time configuration object, public affinity-adapter injection, a
  platform fast path, cgo, or a new dependency.
- A fatal Start failure when affinity is unavailable or rejected by the OS;
  locked-thread execution remains mandatory and sufficient.

## Decisions

### Bind CPUs to effective groups on the open builder

The graph package will add exactly:

```go
func BindCPU(builder *Builder, group GroupID, cpu int) error

var ErrInvalidCPU
var ErrUnknownGroup
var ErrGroupCPUAssigned
var ErrCPUAlreadyBound
```

`BindCPU` records one CPU mapping for an already effective group: a group that
is `DefaultGroupID` for at least one currently registered unplaced node or has
at least one explicit placement. It accepts only an open initialized builder,
a nonzero group, and `cpu >= 0`. Its error order is invalid builder, sealed
builder, invalid group ID, negative CPU, unknown effective group, duplicate
group binding, then duplicate CPU binding. It neither changes placement nor
creates a worker. Requiring a currently effective group rejects dead
configuration at its source and makes the binding API a minimal complement to
`Place`.

Build and Validate independently verify complete effective-group coverage,
nonnegative CPUs, unique CPU IDs, and no binding for an ineffective group so
same-package corruption remains a deterministic `ValidationError` rather than
a malformed definition. New CPU binding issue codes use `ValidationIssue`'s
new `GroupID` and `CPU` fields; unused fields remain zero. The CPU upper bound
is deliberately not validated: Go has no portable process CPU-set query, and a
host may change allowed CPUs between Build and Start. Linux affinity rejection
therefore becomes a fallback warning, not an invalid graph.

An API accepting `GroupConfig`, modifying `Place`, or passing bindings to
`Start` was rejected because it would either couple opaque placement to a
runtime configuration type or make an immutable definition depend on mutable
startup input.

### Compile one immutable binding plan with the group plan

After ordinary validation succeeds, Build resolves effective groups exactly as
today, sorts them by `GroupID`, and copies a parallel CPU binding for every
group into the definition. The definition retains no adapter, thread, worker,
or mutable status. Builder map mutation after Build cannot alter the copied
plan. `Start` continues to have no arguments and uses only that plan.

Keeping CPU IDs only in the builder was rejected because sequential Start
would no longer be reproducible. Exposing the plan was rejected because
callers cannot safely mutate it and existing plans remain private.

### Lock and bind inside the worker goroutine

Each worker begins by calling `runtime.LockOSThread`, then invokes its internal
adapter for the assigned CPU, records one outcome, and signals startup
readiness. It defers `runtime.UnlockOSThread` until after it receives its final
stop command. No coordinator goroutine locks, unlocks, or applies affinity on
behalf of a worker. Workers are spawned and made ready in ascending `GroupID`
order; `Start` returns only after every worker has reported an outcome.

The internal adapter has one operation, conceptually `bind(cpu int) error`.
The Linux implementation is isolated behind a `linux` build tag and performs
the OS affinity call. An unsupported build-tagged implementation returns
`ErrAffinityUnavailable`. A test-only internal factory can inject successful,
failing, and blocking adapters without exposing an application API. Injection
is serialized and restored by the test helper, so parallel tests never replace
the adapter concurrently.

Any adapter error records a fallback outcome and warning, then the locked
worker proceeds. The fallback uses only Go's locked thread and performs no
second OS-specific attempt. This handles unsupported OS builds, unsupported
Linux environments, permission errors, and an invalid host CPU set uniformly
without hiding the outcome.

### Preserve lifecycle ordering through worker commands

Factories and edge resources remain materialized by the coordinator in their
existing deterministic orders. After all workers report ready, the coordinator
sends each `Init` callback, one at a time in canonical topological node order,
to that node's owning worker and waits for its response. Thus Init is globally
ordered while always running after that worker has locked its thread and
attempted affinity. No worker begins scheduler passes until all Init callbacks
succeed.

On Init failure, the coordinator sends Close commands in reverse successful
canonical order to the owning locked workers, waits for each result, drains
framework queues, then stops and joins workers. `Drain` and `Close` retain the
existing pause barrier and queue semantics, but dispatch their reverse-order
Close callbacks before final worker stop. This preserves the existing visible
lifecycle order and ensures a lifecycle implementation can rely on its group
thread remaining locked through Close. Commands and acknowledgements remain
control-plane synchronization, outside the typed batch data path.

Running Init or Close in the coordinator was rejected because `LockOSThread`
there would not constrain the worker that processes the group. Initializing a
whole group independently was rejected because it would change the existing
global canonical ordering.

### Expose immutable startup outcomes, not a warning stream

The graph package will add:

```go
type AffinityOutcome uint8

const (
    AffinityOutcomeApplied AffinityOutcome = iota + 1
    AffinityOutcomeLockedThreadFallback
)

type AffinityStatus struct {
    GroupID GroupID
    CPU     int
    Outcome AffinityOutcome
    Warning error
}

func (e *Executor) AffinityStatus() []AffinityStatus
```

The method returns a new slice in ascending group order. Applied results have a
nil warning. Fallback results have a non-nil warning that wraps the stable
`ErrAffinityUnavailable` sentinel and retains the adapter cause when present.
The executor writes statuses before publishing successful Start completion and
never changes them after, so reads while processing do not race with worker
state. A nil or zero executor returns nil.

A callback, logger, metric, or asynchronous event was rejected because this
stage needs only an inspectable outcome and must not introduce observability
infrastructure.

## Risks / Trade-offs

- [A requested CPU is not permitted or does not exist when Start runs] -> The
  adapter records a warning and retains locked-thread execution rather than
  failing or silently pretending affinity was applied.
- [Build has no portable CPU-count or process-CPU-set limit] -> It enforces
  only nonnegative and unique IDs; the OS adapter is the authoritative runtime
  check.
- [Lifecycle callbacks need group thread affinity] -> Init and Close are sent
  to, acknowledged by, and executed on the locked owner worker in global
  canonical order.
- [An adapter test seam could cause global test races] -> Keep it unexported,
  serialize installation/restoration, and prohibit parallel use by its helper.
- [Platform compilation drifts] -> Compile and test Linux and unsupported-tag
  adapter variants; every variant shares the same pure-Go fallback contract.

## Migration Plan

1. Add group CPU binding metadata, sentinels, validation diagnostics, and the
   immutable copied CPU plan. Existing callers must bind every effective group
   before Build; no existing built definition changes at runtime.
2. Add the internal tagged adapters, locked-worker startup handshake, immutable
   affinity status, and worker-command lifecycle dispatch.
3. Update package documentation and focused validation, lifecycle, tagged
   adapter, and race tests; run strict OpenSpec validation and the Go test
   matrix.

There is no persisted definition format or deployed affinity configuration.
Rollback removes the new builder binding requirement and locked worker path;
the prior fixed-group behavior resumes.

## Open Questions

None. Actual CPU availability intentionally remains an OS-time fallback
condition rather than a portable Build-time policy.
