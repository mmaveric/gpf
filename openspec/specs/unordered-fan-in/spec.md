# unordered-fan-in Specification

## Purpose
Define explicit homogeneous multi-producer merging with exclusive batches and unordered downstream delivery.

## Requirements

### Requirement: Atomic typed merge registration
Fan-in SHALL register compatible ordinary producers and one logical input atomically. Invalid, foreign, duplicate, already connected, or cyclic endpoints SHALL reject without partial topology mutation. Ordinary Connect SHALL retain its single-producer input contract.
#### Scenario: Reject an invalid producer list
- **WHEN** one producer in a requested merge belongs to another builder
- **THEN** no producer or input in that request becomes connected

### Requirement: Unordered exclusive merged delivery
Each successfully accepted batch SHALL be delivered once through the selected shared bounded queue with its originating pool identity intact. No cross-producer ordering SHALL be guaranteed. Stop SHALL reclaim each shared queued batch and each producer's pending batch exactly once.
#### Scenario: Drain multiple producer pools
- **WHEN** different typed-compatible source pools feed one merged sink
- **THEN** accepted batches reach the sink without duplication and releases return to the corresponding origin pools

#### Scenario: Apply independent branch queue policy
- **WHEN** a shared fan-in queue is full under Drop or Wait
- **THEN** the publishing branch alone records Drop or retains its own pending output while other producers continue
