# group-replication Specification

## Purpose
Define fixed whole-group replicas with typed work distribution and independently owned runtime state.

## Requirements

### Requirement: Static CPU-bound group replicas
A group SHALL accept a positive fixed replica count and explicit unique nonnegative CPUs per physical instance. Build SHALL validate and freeze expansion without invoking factories. Internal edges SHALL connect corresponding replica instances; shared boundaries SHALL distribute each accepted batch to one consumer, not broadcast it. Replica identities SHALL be deterministic and inspectable.
#### Scenario: Expand a two-node group
- **WHEN** a group containing two connected nodes is configured with three replicas and complete CPU assignments
- **THEN** Start creates three workers and three paired internal edge instances without multiplying accepted boundary batches

### Requirement: Independent replica lifecycle
Each physical node replica SHALL receive one fresh factory result per Start and owner-worker lifecycle callbacks. Startup failure SHALL close each successfully constructed lifecycle once. Node-pool overlap and incomplete CPU bindings SHALL be rejected before execution.
#### Scenario: Roll back a replica failure
- **WHEN** a later replica Init fails after earlier replicas constructed successfully
- **THEN** every successfully owned lifecycle closes once and no worker or batch remains owned by the failed Start

### Requirement: Explicit ingress and counter replica boundaries
Groups containing ingress adapters SHALL reject replica counts greater than one before runtime construction. Unreplicated ingress SHALL be allowed to feed replicated downstream groups. Generated counter binders SHALL install independent per-replica slots. CounterForReplica SHALL resolve one explicit zero-based replica index; Counter SHALL reject a node with multiple replicas as ambiguous without returning an arbitrary slot.

#### Scenario: Reject replicated ingress
- **WHEN** a group containing an ingress adapter is configured with multiple replicas
- **THEN** Build rejects the configuration without invoking factories or creating producer endpoints

#### Scenario: Resolve replica-local counters
- **WHEN** a node with three replicas has a registered counter
- **THEN** each replica has independent storage, explicit lookup selects only its requested replica, and logical-only lookup reports ambiguity
