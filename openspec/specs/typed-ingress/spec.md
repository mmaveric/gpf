# typed-ingress Specification

## Purpose
Define single-producer typed admission and caller/runtime ownership for external pipeline batches.

## Requirements

### Requirement: Typed bounded producer operations
A registered ingress SHALL bind only to its owning executor Start and SHALL expose typed Acquire, Commit, and copying Submit. Accepted Commit SHALL clear the caller handle; a Wait-full or closed rejection SHALL leave caller ownership intact. Configured Drop SHALL consume and release a rejected full-queue batch. Oversized Submit SHALL reject without partial delivery, and empty Submit SHALL be a no-op. Built-in operations SHALL allocate no framework memory after setup.
#### Scenario: Retry a full ingress
- **WHEN** a producer commits to a full Wait ingress
- **THEN** it retains the unchanged batch and can retry or release it without partial publication

### Requirement: Close admission before draining
Drain and Close SHALL reject new commits, finish admitted nonblocking commits, and account for all accepted batches. Uncommitted producer-owned batches SHALL remain releasable after stop. Restart SHALL produce fresh endpoint resources; stale endpoints SHALL reject new publication.
#### Scenario: Commit overlaps stop
- **WHEN** Commit races a single supported stop operation
- **THEN** it either publishes one accepted batch included in stop processing or rejects without transferring caller ownership
