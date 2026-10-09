# async-error-logging Specification

## Purpose
Define bounded asynchronous logging outside pinned pipeline workers and its lifecycle loss accounting.

## Requirements

### Requirement: Nonblocking bounded event delivery
Configured logging SHALL map one event for each failed result and publish once to a bounded worker-owned ring. Full rings SHALL drop the event and increment a separate loss counter without blocking data workers. Logger callbacks SHALL execute outside the data-plane workers. Disabled logging SHALL create no logger worker or invoke mapper callbacks.
#### Scenario: Saturate logging transport
- **WHEN** a returning but slow logger leaves its ring full
- **THEN** subsequent failures increment logging-drop counters while ordinary processing and typed error routing continue

### Requirement: Join logger lifecycle cooperatively
Drain SHALL finish accepted logger events within the shared drain budget; cancellation SHALL discard unconsumed events. Stop SHALL join the logger after any in-flight callback returns, preserving exactly-once accepted-event delivery or discard. Nonreturning callbacks SHALL be documented as preventing completed shutdown.
#### Scenario: Cancel logger flushing
- **WHEN** the drain deadline expires with queued events and returning callbacks
- **THEN** queued leftovers are discarded, counters retain the loss, and the logger goroutine exits before stop completes
