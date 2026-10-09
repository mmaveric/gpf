## ADDED Requirements

### Requirement: Runtime-owned typed error transport batch
A connected `ErrorPort[E]` SHALL be backed at runtime by a framework-owned typed source pool. Only the generated executor binding may acquire its `Batch[E]`; user handlers receive it only through the target `InPort[E]`. For each failed result, the binding SHALL acquire at most one batch, populate exactly one element, and either transfer it to the edge queue, retain it as the runner's one Wait pending output, or release it. It SHALL not expose an error `node.Output[E]` to a handler.

#### Scenario: Preserve origin ownership through an error consumer
- **WHEN** a connected error batch crosses a local or remote edge and the consumer leaves it un-moved
- **THEN** executor cleanup returns it to the error edge's original typed pool without a runtime pool lookup or type erasure

#### Scenario: Discard a pending error batch on close
- **WHEN** Close observes a runner holding a Wait-pending typed error batch
- **THEN** the executor releases that batch exactly once and clears the pending handle without invoking another handler
