## Context

See proposal.md for motivation. `batch.Batch[T]` currently contains a pool-slot pointer; Move clears only its receiver, and Pool.Release clears the slot's logical length and makes it available. Generated explicit and closure handlers receive batches by value and unconditionally release the runner's retained handle after finalization. Thus clearing the handler's handle does not inform runner cleanup. The failing metrics delivery regression observes an accepted three-element forwarded batch dequeued with length zero.

## Goals / Non-Goals

**Goals:**
- Keep the public by-value handler and output APIs compatible.
- Distinguish input ownership relinquishment from normal automatic cleanup without comparing heterogeneous output types.
- Preserve ownership and length while batches travel asynchronously or remain Wait-pending.
- Provide regression coverage independent of the partial metrics implementation.

**Non-Goals:**
- Detect arbitrary copied-handle misuse, double release, stale handles, or wrong-pool release.
- Change batch contents, queue policies, failure publication, CPU placement, or error reliability.
- Complete metrics implementation or mark its outstanding tasks complete as a side effect.

## Decisions

### Use runner-local input scopes, not pool-slot transfer flags

Add a reusable typed `batch.InputScope[T]` runtime bridge with Begin and End operations. A generated runner owns one stable scope per typed input, allocated during normalization and reused across invocations. Begin associates the popped batch with that scope and clears its consumed flag. The handler receives a copied handle carrying a private scope pointer. End releases the runner's input only if the scope was not consumed, then clears the runner handle and scope association.

Batch.Move marks the attached scope consumed, clears the caller handle, and returns an owned handle with no scope pointer. Batch.Release and direct Pool.Release mark an attached scope consumed before returning the slot. An accepted Output.Commit already calls Move, so it needs no type-erased identity comparisons; rejection must never consume the scope. Standalone pool batches with no scope retain their existing behavior.

The scope records a legitimate transfer event; it is not a general ownership validator or runtime pool lookup. It uses a non-atomic flag because only the invoking worker accesses it. Any batch transferred downstream has the scope pointer detached before queue publication, so consumers cannot access a producer runner's mutable scope.

Changing handlers to accept batch pointers was rejected as a breaking API migration. Placing flags in pool slots was rejected because slots can be reused independently of a producer's invocation and remote consumers would contend on shared mutable state. Scanning ordinary outputs was rejected because heterogeneous output types and user-owned Move results would require an expanded identity protocol.

### End the scope after result and output finalization

After pop, Begin wraps the batch before dispatch. Existing failed-result, ordinary-output, and error-output finalization order remains intact. End replaces unconditional input Release in every explicit-state and closure input branch, across arities one through eight. Sources have no input scopes.

If a handler moves its input and returns success, downstream or the pending output owns the slot. If it commits the input and returns failure, output finalization releases it and scope cleanup skips the already-consumed input. If it explicitly releases input, cleanup skips it. If it does neither, cleanup returns it to its origin pool. Rejected commits leave the argument available for another valid action or automatic cleanup.

Scopes are reset only after the preceding invocation has completed. An independently moved batch may outlive that invocation because its handle has no reference to the reusable scope. Retaining the original scoped copy after return remains invalid usage under the existing contract.

### Test exclusive ownership, not just values at a fast sink

Standalone tests must demonstrate that a moved slot cannot be reacquired while its output is queued or Wait-pending, that downstream observes nonzero length and original items, and that release eventually restores capacity. Include local/custom-local and cross-group paths, multi-hop moves, error-batch forwarding to ordinary outputs, failed results, rejected Commit, explicit release, and forced cleanup.

Use deterministic typed binding/runner tests for saturation and cleanup, and actual fixed-group execution for remote handoff and race checks. Avoid relying solely on a sink that happens to run before the faulty Release.

### Preserve the metrics worktree and use the fix as a prerequisite

The current worktree already contains partial metrics changes in generated bindings, node finalization, executor code, and tests. Apply ownership changes against that state without overwriting it; standalone ownership tests must not import or assert Metrics. Run the previously failing `TestMetricsDeliveryAndRestart` as an additional compatibility check, then resume the separate metrics change explicitly.

## Risks / Trade-offs

- [An added handle pointer affects cache footprint and escape behavior] → Preallocate scopes once per runner input; measure serial AllocsPerRun for forwarding and compare existing transport benchmarks.
- [A scope pointer escapes to another worker] → Detach it in Move and verify remote forwarding under the race detector.
- [Consumed flags survive into another invocation] → Reset in Begin and test repeated delivery, pool reuse, and sequential restarts.
- [Cleanup mistakes become hidden by idempotent-looking bookkeeping] → Keep exactly-once ownership assertions for both transferred and untouched inputs; do not promise general misuse recovery.
- [Generated edits overwrite partial metrics work] → Change templates first, regenerate, and review the combined diff while retaining all existing instrumentation.

## Migration Plan

Add a failing standalone reproduction, implement the typed scope bridge and Move/Release bookkeeping, update generator input branches, regenerate bindings, and run correctness/race/allocation checks. Update ownership documentation and validate this change. Existing application handlers require no source changes. Rollback removes the scope integration but reintroduces the ownership bug, so it is not a correctness-preserving fallback.
