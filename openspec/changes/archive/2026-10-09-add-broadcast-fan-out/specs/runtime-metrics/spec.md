## MODIFIED Requirements

### Requirement: Edge transport and loss counts
EdgeMetrics SHALL expose all existing accepted/dequeued, policy-drop, failed-output, forced-shutdown, pool-exhaustion, and Wait retry counts and SHALL additionally expose CapacityRejectedBatches and CapacityRejectedElements for explicit broadcast branches that reject an oversized moved batch before copying. Capacity rejection SHALL be distinct from queue Drop and pool exhaustion. Existing semantics SHALL remain unchanged: accepted counts require successful queue transfer, dequeue counts require normal handler delivery, pool exhaustion is an attempt count without fabricated batch loss, and WaitRetries counts subsequent attempts but not the initial push. Batch lengths SHALL be captured before ownership clearing and counters SHALL wrap modulo 2^64.

#### Scenario: Retry a retained output
- **WHEN** an initial Wait push fails, two subsequent retries fail, and a third retry succeeds for a batch of length four
- **THEN** WaitRetries increases by three, accepted counts by one/four, and queue-drop/capacity-rejection counts remain unchanged

#### Scenario: Distinguish connected failure losses
- **WHEN** a connected error publication exhausts its pool and another acquired error batch encounters a full Drop queue
- **THEN** PoolExhaustions records one acquisition attempt and QueueDropped counts only the dropped committed batch

#### Scenario: Close pending and queued work
- **WHEN** Close discards one queued batch and one Wait-pending batch
- **THEN** shutdown discard counts each committed batch once and normal dequeue counts do not increase

#### Scenario: Distinguish sharding pool loss from queue Drop
- **WHEN** a sharding branch cannot acquire a staging slot for one element and a different branch successfully accepts its elements
- **THEN** the exhausted branch increments `PoolExhaustions` and `PoolDroppedElements` for the element, while the other branch's metrics remain unaffected

#### Scenario: Account for unassigned elements at forced shutdown
- **WHEN** Close discards a pending sharding operation before its cursor reaches the source batch end
- **THEN** the first branch's `ShardShutdownElements` increases by the remaining unassigned source-element count
