## MODIFIED Requirements

### Requirement: Edge transport and loss counts
`EdgeMetrics` SHALL add `PoolDroppedElements`, counting individual sharding elements lost because no branch staging-pool slot was available, distinct from `PoolExhaustions` attempts and `QueueDroppedElements`. It SHALL add `ShardID` to identify sharding branches and `ShardShutdownElements` for unassigned source elements released by forced shutdown; this shutdown count SHALL be recorded only on the first branch row for the sharding group to prevent duplicate accounting. Pool exhaustion SHALL not fabricate a dropped batch, and all counters SHALL wrap modulo 2^64.

#### Scenario: Distinguish sharding pool loss from queue Drop
- **WHEN** a sharding branch cannot acquire a staging slot for one element and a different branch successfully accepts its elements
- **THEN** the exhausted branch increments `PoolExhaustions` and `PoolDroppedElements` for the element, while the other branch's metrics remain unaffected

#### Scenario: Account for unassigned elements at forced shutdown
- **WHEN** Close discards a pending sharding operation before its cursor reaches the source batch end
- **THEN** the first branch's `ShardShutdownElements` increases by the remaining unassigned source-element count

#### Scenario: Retry a retained output
- **WHEN** an initial Wait push fails, two subsequent retries fail, and the third retry succeeds for a batch of length 4
- **THEN** WaitRetries increases by three, AcceptedBatches by one, AcceptedElements by four, and queue-drop counts remain unchanged

#### Scenario: Distinguish connected failure losses
- **WHEN** one connected error publication exhausts its pool and another acquired error batch encounters a full Drop queue
- **THEN** PoolExhaustions increases by one for the first outcome and QueueDroppedBatches and QueueDroppedElements increase by one for the second, without incrementing UnconnectedErrors

#### Scenario: Close pending and queued work
- **WHEN** Close discards one queued batch and one Wait-pending batch on an edge
- **THEN** both are counted once as shutdown discards, their lengths are counted, and cleanup does not increase normal dequeue counts
