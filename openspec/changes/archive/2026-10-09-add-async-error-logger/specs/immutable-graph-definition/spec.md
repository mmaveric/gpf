## ADDED Requirements

### Requirement: Dormant copied logger configuration
Build SHALL retain copied optional logger configuration and its callback without invoking mappers or logging handlers, creating event rings, or starting a logger goroutine. Each successful sequential Start SHALL materialize fresh per-physical-worker rings and counters, and SHALL start one unpinned logger only after every data-node Init succeeds. Startup rollback before activation SHALL leave no logger goroutine running. These dormant callback/configuration fields extend the runtime-neutral definition contract without exposing mutable runtime resources.

#### Scenario: Fail initialization before logger activation
- **WHEN** a configured definition's node Init fails during Start
- **THEN** its owned lifecycle rolls back and no logger callback or running logger is created

#### Scenario: Restart a configured definition
- **WHEN** one executor stops and the same configured definition starts again
- **THEN** logging uses fresh empty event transport and counters without changing the stored configuration
