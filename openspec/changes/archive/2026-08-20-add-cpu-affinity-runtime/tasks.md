## 1. CPU Binding and Immutable Planning

- [x] 1.1 Add builder-owned group-to-CPU metadata and the exact `BindCPU` API, sentinels, effective-group predicate, documented error precedence, and no-side-effect behavior.
- [x] 1.2 Extend `ValidationCode`, `ValidationIssue`, diagnostic formatting, copying, and deterministic sorting to report missing, invalid, duplicate, and ineffective CPU bindings with GroupID and CPU.
- [x] 1.3 Extend Validate and Build to derive effective groups once, enforce complete nonnegative unique CPU coverage without a portable upper bound, retain correction behavior on failure, and reject malformed same-package metadata.
- [x] 1.4 Compile and deep-copy one CPU binding into each ascending private group-plan entry without exposing mutable plan data or creating a runtime resource.
- [x] 1.5 Add builder and definition tests for default and sparse groups, BindCPU precedence, incomplete coverage, negative and duplicate CPU data, ineffective groups, immutable plan copying, factory dormancy, and deterministic diagnostics.

## 2. Affinity Adapters and Status

- [x] 2.1 Define the internal affinity-adapter contract, stable `ErrAffinityUnavailable`, and a serialized unexported test injection helper with restoration.
- [x] 2.2 Add build-tagged Linux adapter files that attempt only per-worker CPU affinity and complementary unsupported-platform files that return the unavailable sentinel, without a new dependency, cgo, or fast path.
- [x] 2.3 Add `AffinityOutcome`, `AffinityStatus`, and the defensive-copy `Executor.AffinityStatus` API; retain one immutable status per ascending planned group.
- [x] 2.4 Add adapter unit tests for Linux-tagged selection, unsupported fallback selection, injected success and failure, warning wrapping, status order, and status-slice immutability without depending on host CPU availability.

## 3. Locked Worker Startup and Lifecycle

- [x] 3.1 Extend worker runtime state and control commands so a worker locks its own OS thread, attempts affinity, publishes readiness and status, and unlocks only from its final worker exit path.
- [x] 3.2 Change Start to construct resources and factories in current deterministic order, create and ready locked workers in ascending GroupID order, and publish an executor only after all statuses are recorded.
- [x] 3.3 Dispatch Init callbacks to locked owner workers one at a time in global canonical order, block scheduler passes until every Init succeeds, and preserve factory ownership rules.
- [x] 3.4 Implement startup rollback that drains framework work, dispatches each owned Close callback to its locked owner worker in reverse canonical order, joins lifecycle errors, and then stops and joins every worker.
- [x] 3.5 Update Drain and Close pause-barrier paths to preserve accepted-work and discard semantics while closing on locked owner workers in reverse canonical order before workers unlock and stop.
- [x] 3.6 Add focused one- and multi-group tests proving lock-before-Init, owner-worker Init and Close, affinity fallback does not fail Start, global rollback order, Drain and Close behavior, sequential Start freshness, and worker exit after unlock.

## 4. Documentation and Verification

- [x] 4.1 Update graph, runtime, and README documentation to require graph-construction CPU bindings, explain Linux affinity and pure-Go locked-thread fallback, describe `AffinityStatus`, and retain the explicit deferred-feature list.
- [x] 4.2 Run gofmt and generation verification; run focused unit tests plus `go test ./...`, Linux and unsupported-tag compilation/tests, `go test -race ./...`, `go vet ./...`, and `git diff --check`.
- [x] 4.3 Run `openspec validate add-cpu-affinity-runtime --strict` and `openspec validate --all --strict`, resolving validation failures attributable to this change.
