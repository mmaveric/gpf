## Purpose
Define context-triggered transition from graceful draining to ownership-safe forced shutdown.
## ADDED Requirements
### Requirement: Cooperative deadline drain
Executor SHALL expose DrainContext(context.Context) error while existing Drain remains unbounded. Cancellation SHALL close admission and request forced discard at the next safe all-worker boundary. Accepted batches SHALL be processed or reclaimed exactly once, and returned errors SHALL preserve the cancellation cause and lifecycle errors. Deadlines SHALL not promise interruption of nonreturning user callbacks.
#### Scenario: Cancel pending drain
- **WHEN** the context cancels while returning callbacks leave queued or Wait-pending work
- **THEN** the next acknowledged stop boundary discards remaining owned work and returns an error matching the context cause
### Requirement: Share one lifecycle budget
Ingress closure, worker draining, and any configured logger draining SHALL use the same cancellation budget. Repeated stop calls SHALL preserve the established final outcome without duplicate cleanup.
#### Scenario: Repeat a canceled stop
- **WHEN** DrainContext has completed forced cleanup and Close is called again
- **THEN** no batch or lifecycle is released twice and the recorded stop error remains available
