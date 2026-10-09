## MODIFIED Requirements

### Requirement: Drain and close executor lifecycle
`Executor` SHALL expose `func (e *Executor) Drain() error`, `func (e *Executor) DrainContext(context.Context) error`, and `func (e *Executor) Close() error`. Drain SHALL delegate to context-aware draining without a deadline. Draining SHALL close runtime-local admission, pause workers, stop new zero-input source invocations, and process accepted local and remote work and pending Wait outputs until the fixed-plan executor has no runnable, queued, or pending batches. Already committed source outputs SHALL continue retrying without reinvoking their source handlers. DrainContext cancellation SHALL request forced discard at an acknowledged all-worker boundary and SHALL preserve both cancellation and explicit context cause in its result alongside cleanup errors. It SHALL not forcibly interrupt user callbacks that do not return. Close SHALL stop sources and release queued and pending local and remote batches without normal processing. All stop paths SHALL invoke owned node Close callbacks once on their still-locked owner workers in reverse successful-construction order, then stop and join every worker so each unlocks its OS thread. Runtime-local admission/logger participants SHALL receive the original drain context, discard unconsumed work when forced, and join before completed shutdown. Repeated lifecycle calls SHALL retain the final stop error without duplicate cleanup; concurrent lifecycle calls remain unsupported. Cleanup errors SHALL be surfaced with `errors.Join`.

#### Scenario: Gracefully drain default-group local work
- **WHEN** Drain is requested after a source has committed a batch to a local edge in a default-group definition
- **THEN** sources stop producing, downstream receives accepted work, and the locked worker runs Close only after queues and pending output are empty before unlocking and stopping

#### Scenario: Force close multi-group work
- **WHEN** Close is requested while a local or remote edge queue or Wait output holds batches
- **THEN** the executor releases those batches without invoking further handlers, closes each owned lifecycle once on its locked worker, and waits for every worker to unlock and stop

#### Scenario: Retry an existing source output while draining
- **WHEN** drain disables new source calls while an already committed source output waits for queue space
- **THEN** scheduler retries preserve that batch's ownership and do not call the source handler again after the pending output publishes

#### Scenario: Cancel at a safe barrier
- **WHEN** a supplied context cancels while queued or pending work remains and callbacks return
- **THEN** the next acknowledged barrier discards remaining framework-owned work once and returns an error matching the cancellation and any lifecycle errors
