## MODIFIED Requirements

### Requirement: Typed per-runtime queue factory
The queue contract SHALL declare the exact API `type Factory[T any] func(Config) (QueueOps[T], error)`. Each future runtime instance SHALL invoke a configured factory separately for each local edge it binds, and a factory invocation SHALL return queue operations for the same element type `T` or an error. A nil local-edge factory SHALL select the future built-in local queue. A cross-group edge classified by Build SHALL use the framework-owned remote SPSC queue and SHALL reject a non-nil configured factory before runtime creation.

#### Scenario: Preserve typed local custom queue construction
- **WHEN** an `EdgeConfig[int]` contains a custom `Factory[int]` and its endpoints have the same effective group
- **THEN** the edge retains a constructor that can produce only `QueueOps[int]` for that future local runtime edge

#### Scenario: Create fresh local operations for each future runtime
- **WHEN** two future runtime instances bind the same local edge with a custom factory
- **THEN** each runtime invokes the factory independently and does not reuse queue operations created for the other runtime

#### Scenario: Reject a custom remote queue
- **WHEN** a configured custom factory edge is classified cross-group by Build
- **THEN** Build rejects the graph without invoking the factory and the future runtime uses no custom remote operations
