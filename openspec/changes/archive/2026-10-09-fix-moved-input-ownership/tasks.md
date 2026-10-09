## 1. Standalone Reproduction and Transfer Bookkeeping

- [x] 1.1 Add a standalone generated-handler regression that forwards a populated input by Commit and demonstrates premature pool reuse or zero downstream length before the fix; verify it fails without depending on Metrics.
- [x] 1.2 Add the reusable typed input-scope bridge and private Batch scope association; verify Begin/End return untouched inputs once and can be reused without altering a previously transferred batch.
- [x] 1.3 Update Batch.Move to consume and detach input scope state while preserving standalone Move behavior; verify copied-handler moves retain contents, clear their caller handle, and leave no producing-scope reference in the owned result.
- [x] 1.4 Update Batch.Release and direct Pool.Release to consume attached scope state before releasing storage; verify explicit releases are not repeated by End and unscoped pool lifecycle tests still pass.

## 2. Generated Handler Integration

- [x] 2.1 Add one stable scope per input in explicit-state and closure normalization templates, call Begin after pop, and replace unconditional cleanup with End after result finalization; verify all input branches at arities one through eight use scoped cleanup and sources remain unaffected.
- [x] 2.2 Regenerate committed files with `go generate ./...` while preserving partial metrics instrumentation; verify `go test ./internal/genarity ./node` and generated stability checks pass.
- [x] 2.3 Verify accepted and rejected compatible Commit behavior using populated inputs; test that failed-result and queue Drop finalization reclaim committed inputs once, while a rejected duplicate Commit leaves the input eligible for normal cleanup.

## 3. Transport, Reuse, and Lifecycle Regressions

- [x] 3.1 Add local and custom-local multi-hop forwarding regressions for explicit and closure handlers; verify nonzero logical lengths, original items, and source-slot exclusion until downstream release.
- [x] 3.2 Add cross-group forwarding and typed-error-to-ordinary forwarding regressions; verify repeated transfers and sequential restarts preserve contents and origin-pool reclamation under `go test -race ./graph`.
- [x] 3.3 Add deterministic Wait-pending and forced-Close regressions; verify retained slots cannot be reacquired before transfer or discard and cleanup restores capacity exactly once.
- [x] 3.4 Run `go test ./graph -run TestMetricsDeliveryAndRestart -count=1`; verify the original metrics blocker passes without changing metrics task completion or weakening its assertions.

## 4. Performance, Documentation, and Completion

- [x] 4.1 Add serial allocation gates for scoped input invocation, forwarding, explicit release, Wait retry, and cleanup after setup; verify `testing.AllocsPerRun` reports zero framework allocations and run the existing transport benchmarks with toolchain/machine context recorded.
- [x] 4.2 Document scoped input transfer, by-value cleanup behavior, detached moved handles, and unchanged misuse limitations in batch/node/graph documentation; verify examples and API comments match the ownership specs.
- [x] 4.3 Run formatting, generation stability, `go test ./...`, `go test -race ./...`, `go vet ./...`, and `git diff --check`; verify ownership changes and partial metrics code coexist without regressions.
- [x] 4.4 Run `openspec validate fix-moved-input-ownership --strict` and `openspec validate --all --strict`; verify every ownership scenario is covered and report that `add-runtime-metrics` can resume after this prerequisite completes.
