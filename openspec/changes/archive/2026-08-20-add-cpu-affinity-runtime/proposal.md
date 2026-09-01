## Why

Manual groups currently create one worker each but have no CPU assignment or
thread-affinity behavior. The original runtime plan requires an explicit CPU
for every execution group and a portable locked-thread fallback, so the fixed
manual plan can become operational without adding automatic planning or
parallelism.

## What Changes

- Add a graph-construction CPU binding for each effective manual group, with no
  `Start` configuration. Build validates complete group coverage, nonnegative
  CPU IDs, and one CPU per effective group before publishing a definition.
- Carry a deeply copied immutable group-to-CPU binding plan in the definition
  and use it when materializing its one worker per group.
- Make every worker lock its OS thread before affinity setup, group `Init`, or
  handler processing. Add a build-tagged Linux affinity adapter and an
  unsupported-platform pure-Go fallback that retains the locked thread.
- Surface the immutable per-group outcome of affinity setup, including a
  fallback warning, through the executor without adding logging or metrics.
- Preserve deterministic startup, rollback, Drain, and Close by executing
  worker-owned Init and Close callbacks on their still-locked owner workers.
- Add platform-tagged and adapter-injected tests for binding validation,
  startup outcomes, fallback behavior, lifecycle ordering, and race safety.

## Capabilities

### New Capabilities
- `cpu-affinity-runtime`: Explicit fixed-group CPU binding, locked-worker
  execution, platform affinity adaptation, fallback outcomes, and lifecycle
  rules.

### Modified Capabilities
- `manual-multigroup-runtime`: Require CPU bindings for effective fixed groups
  and revise worker startup and lifecycle behavior for locked threads.
- `immutable-graph-definition`: Preserve the validated immutable CPU binding
  plan with the existing fixed group plan.
- `single-group-executor`: Define affinity-aware worker startup and the
  executor status API while retaining configuration-free `Start`.
- `typed-pipeline-contracts`: Document explicit CPU binding, the mandatory
  locked-thread fallback, and the newly available affinity outcome.
- `graph-dag-validation`: Identify CPU-binding validation failures with their
  affected group and CPU values in deterministic validation diagnostics.

## Impact

- Affects graph builder validation and immutable definition metadata,
  `Definition.Start`, executor worker control flow, graph and runtime package
  documentation, and platform-specific internal files.
- Adds a small graph API for group CPU binding, stable validation codes and
  binding errors, and a read-only executor affinity-status snapshot. Existing
  `Start`, `Drain`, and `Close` signatures remain unchanged.
- Adds no dependency, queue model, automatic placement, NUMA behavior,
  replica, ingress, observability pipeline, or platform fast path.
