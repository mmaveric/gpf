## Purpose
Specify bounded portable typed queues for single and multiple producer/consumer combinations.
## ADDED Requirements
### Requirement: Bounded nonblocking queue models
The queue package SHALL provide validated preallocated MPSC, SPMC, and MPMC operations compatible with QueueOps[T]. TryPush and TryPop SHALL not block waiting for another caller. Failed push SHALL retain caller ownership; successful pop SHALL return a fully published owned batch exactly once. Concurrent Len SHALL be bounded by capacity and SHALL be documented as observational. No steady-state framework allocation SHALL occur.
#### Scenario: Concurrent ownership transfer
- **WHEN** multiple valid producers and consumers exchange unique batches through an MPMC ring
- **THEN** every successful publication is popped once without corrupted contents or capacity growth
### Requirement: Validate capacities and counter boundaries
Construction SHALL reject zero or unrepresentable capacities. Producer and consumer cursors SHALL wrap modulo capacity without losing or duplicating slot ownership. Failed operations SHALL not consume storage.
#### Scenario: Reject invalid storage
- **WHEN** construction receives an invalid capacity
- **THEN** it returns an error without a usable queue or partially published state
