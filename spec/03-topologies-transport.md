# Topologies and transport

This document consolidates the current normative requirements. Scenarios are retained where they add behavioral detail; shared concepts should be read together across sections.


## bounded-concurrent-queues

Specify bounded portable typed queues for single and multiple producer/consumer combinations.


### Bounded nonblocking queue models

The queue package SHALL provide validated preallocated MPSC, SPMC, and MPMC operations compatible with QueueOps[T]. TryPush and TryPop SHALL not block waiting for another caller. Failed push SHALL retain caller ownership; successful pop SHALL return a fully published owned batch exactly once. Concurrent Len SHALL be bounded by capacity and SHALL be documented as observational. No steady-state framework allocation SHALL occur.
#### Concurrent ownership transfer
- multiple valid producers and consumers exchange unique batches through an MPMC ring
- every successful publication is popped once without corrupted contents or capacity growth


### Validate capacities and counter boundaries

Construction SHALL reject zero or unrepresentable capacities. Producer and consumer cursors SHALL wrap modulo capacity without losing or duplicating slot ownership. Failed operations SHALL not consume storage.
#### Reject invalid storage
- construction receives an invalid capacity
- it returns an error without a usable queue or partially published state


## broadcast-fan-out

Specify typed copying broadcast with exclusive branch storage and independent bounded delivery outcomes.


### Explicit independent broadcast copies

Broadcast SHALL atomically register a fixed list of distinct compatible ordinary input destinations with matching batch capacities. Each delivered branch SHALL receive a distinct pooled storage slot containing the source's logical elements. Copying pointer-containing elements SHALL be documented as shallow. Ordinary Connect and error-route cardinality SHALL remain unchanged.
#### Deliver to two branches
- a valid three-element batch broadcasts to two available branches
- both observe the original elements in independent owned storage slots


### Independent bounded branch outcomes

Pool exhaustion SHALL drop only the affected branch copy. Queue Drop SHALL release that branch; Wait SHALL retry its retained copy without recopying or redelivering completed branches. Drain and Close SHALL account for all pending branch ownership exactly once. Broadcast batches SHALL respect the configured capacity precondition; a moved oversized batch SHALL be rejected on every branch with separate capacity-rejection counters.
#### Retry one waiting branch
- one branch accepts its copy while another branch's Wait queue is full
- only the waiting branch retries, and the accepted branch is neither recopied nor redelivered


### Fixed-topology endpoint compatibility

Broadcast sources and destinations SHALL NOT participate in whole-group replica or node-worker-pool expansion in this version. Build SHALL reject such a topology with `ErrBroadcastParallelExpansion`, leaving broadcast execution unavailable rather than assigning ambiguous delivery semantics.
#### Reject an expanded broadcast endpoint
- either endpoint of a registered broadcast is included in a replica or node-worker-pool expansion
- Build returns `ErrBroadcastParallelExpansion` before runtime resources are created
#### One branch waits
- one branch accepts and another branch waits on a full queue
- retries affect only the waiting branch and cannot duplicate the already accepted delivery


## element-sharding

Define fixed typed per-element distribution into bounded branch-specific pooled batches.


### Exactly one branch choice per element

An explicit sharding connection SHALL validate a fixed compatible destination list and typed selector before Build. Each processed element SHALL be assigned once to one valid branch, preserving element values without boxing. Invalid selector indices SHALL be documented as contract violations. Destination registration failure SHALL not partially connect topology.
#### Map selector indices to supplied input order
- destinations are supplied in a non-canonical order and the selector returns index `i`
- the element is sent to the destination at position `i` in the supplied list
#### Reject an invalid selector result
- the selector returns an index outside the fixed destination list
- execution panics with `ErrInvalidShardIndex` instead of silently remapping the element
#### Resume a waiting shard
- a selected branch waits for queue space
- resumption uses the retained branch choice without invoking the selector again for that element


### Bounded branch staging and cleanup

Sharding SHALL hold bounded per-branch staging and at most one active source batch, splitting full batches and flushing partial batches at input completion. Branch Wait/Drop and pool exhaustion SHALL have distinct element-loss accounting. Close SHALL release all source, partial, and pending batches exactly once. Global cross-branch order SHALL not be guaranteed.
#### Distribute beyond branch batch capacity
- an input contains more elements for one branch than its staging batch can hold
- the branch receives successive bounded batches or configured losses without allocating additional staging storage
#### Count branch pool exhaustion separately
- a selected branch has no staging slot available for an element
- only that element is dropped, its `PoolDroppedElements` increases, and queue-drop element counts remain unchanged
#### Reclaim pending sharding state on Close
- Close reaches a pending sharding operation with queued, staged, and unassigned elements
- every owned batch is released once and unassigned source elements are counted in `ShardShutdownElements`


### Fixed-topology endpoint compatibility

Sharding sources and destinations SHALL NOT participate in whole-group replica or node-worker-pool expansion in this version. Build SHALL reject such a topology with `ErrShardParallelExpansion` before runtime resources are created.
#### Reject an expanded sharding endpoint
- a sharding source or destination is included in a replica or node-worker-pool expansion
- Build rejects the topology with `ErrShardParallelExpansion`


## unordered-fan-in

Define explicit homogeneous multi-producer merging with exclusive batches and unordered downstream delivery.


### Atomic typed merge registration

Fan-in SHALL register compatible ordinary producers and one logical input atomically. Invalid, foreign, duplicate, already connected, or cyclic endpoints SHALL reject without partial topology mutation. Ordinary Connect SHALL retain its single-producer input contract.
#### Reject an invalid producer list
- one producer in a requested merge belongs to another builder
- no producer or input in that request becomes connected


### Unordered exclusive merged delivery

Each successfully accepted batch SHALL be delivered once through the selected shared bounded queue with its originating pool identity intact. No cross-producer ordering SHALL be guaranteed. Stop SHALL reclaim each shared queued batch and each producer's pending batch exactly once.
#### Drain multiple producer pools
- different typed-compatible source pools feed one merged sink
- accepted batches reach the sink without duplication and releases return to the corresponding origin pools

#### Apply independent branch queue policy
- a shared fan-in queue is full under Drop or Wait
- the publishing branch alone records Drop or retains its own pending output while other producers continue


## group-replication

Define fixed whole-group replicas with typed work distribution and independently owned runtime state.


### Static CPU-bound group replicas

A group SHALL accept a positive fixed replica count and explicit unique nonnegative CPUs per physical instance. Build SHALL validate and freeze expansion without invoking factories. Internal edges SHALL connect corresponding replica instances; shared boundaries SHALL distribute each accepted batch to one consumer, not broadcast it. Replica identities SHALL be deterministic and inspectable.
#### Expand a two-node group
- a group containing two connected nodes is configured with three replicas and complete CPU assignments
- Start creates three workers and three paired internal edge instances without multiplying accepted boundary batches


### Independent replica lifecycle

Each physical node replica SHALL receive one fresh factory result per Start and owner-worker lifecycle callbacks. Startup failure SHALL close each successfully constructed lifecycle once. Node-pool overlap and incomplete CPU bindings SHALL be rejected before execution.
#### Roll back a replica failure
- a later replica Init fails after earlier replicas constructed successfully
- every successfully owned lifecycle closes once and no worker or batch remains owned by the failed Start


### Explicit ingress and counter replica boundaries

Groups containing ingress adapters SHALL reject replica counts greater than one before runtime construction. Unreplicated ingress SHALL be allowed to feed replicated downstream groups. Generated counter binders SHALL install independent per-replica slots. CounterForReplica SHALL resolve one explicit zero-based replica index; Counter SHALL reject a node with multiple replicas as ambiguous without returning an arbitrary slot.

#### Reject replicated ingress
- a group containing an ingress adapter is configured with multiple replicas
- Build rejects the configuration without invoking factories or creating producer endpoints

#### Resolve replica-local counters
- a node with three replicas has a registered counter
- each replica has independent storage, explicit lookup selects only its requested replica, and logical-only lookup reports ambiguity


## node-worker-pools

Specify fixed node-level worker expansion with independent state and typed shared batch queues.


### Fixed node worker expansion

A logical node SHALL support a positive fixed worker count with explicit unique nonnegative CPU bindings. Every worker SHALL own a fresh factory result and per-input typed handlers/selectors. Shared input batches SHALL be processed by one worker, never broadcast implicitly. Worker identities SHALL be stable and observable.
#### Process pooled input
- multiple workers consume accepted unique input batches
- each batch is invoked once by one worker and state mutation remains local to that worker


### Validate pooled topology and lifecycle

Build SHALL reject invalid counts, missing CPUs, and overlap with whole-group replication without invoking factories. Drain and Close SHALL include every physical worker and return queued/pending work once. No global output ordering SHALL be promised.
#### Reject overlapping expansion
- a node pool is requested inside a replicated group
- validation fails before any runtime instance is created


### Per-port shared transport without implicit broadcast

Each logical input of a pooled node SHALL have one bounded shared queue with one consumer owner per physical worker, and each logical output SHALL use a queue selected from its physical producer/consumer cardinality. A batch SHALL be acquired from a producer-local pool and accepted by at most one downstream consumer. ReadyMask selection SHALL remain typed and local to the worker that invokes the selected handler. No ordering or fairness guarantee beyond accepted ownership SHALL be added.

#### Route heterogeneous ready inputs
- two differently typed logical inputs are ready across a pooled node's workers
- each worker's selector receives only its own ReadyMask and dispatches a matching typed handler exactly once


## automatic-queue-selection

Define deterministic built-in queue choice from physical endpoint concurrency after topology expansion.


### Select the least concurrent valid model

Physical edges SHALL use local transport only when all operations have one worker owner. Cross-worker 1:1, many:1, 1:many, and many:many endpoints SHALL select SPSC, MPSC, SPMC, and MPMC respectively. Invalid zero endpoint counts SHALL be rejected. Copied edge-plan metadata SHALL report model and cardinalities.
#### Resolve a shared consumer
- an expanded edge has three producer workers and one consumer worker
- its plan selects MPSC instead of SPSC or MPMC


### Retain factory dormancy

Build SHALL select models without creating queues or invoking callbacks. Start SHALL materialize fresh typed storage. A custom factory incompatible with physical multi-worker ownership SHALL be rejected before invocation.
#### Reject unsafe custom transport
- a custom local-only factory is configured on an expanded cross-worker edge
- validation reports the edge and leaves the factory uncalled


## remote-edge-minimization

Define deterministic constraint-preserving component coalescing that reduces unnecessary remote transport.


### Locally minimize legal remote boundaries

The coalescing algorithm SHALL preserve fixed labels and mandatory cuts, never increase remote-edge count, and terminate with no eligible adjacent improving merge. Results SHALL be deterministic for equivalent canonical inputs. It SHALL not claim globally optimal partitioning.
#### Coalesce an unconstrained DAG
- no manual or parallelism boundary constrains an input DAG
- the resulting plan places compatible nodes in one group with zero remote edges


### Preserve quotient acyclicity

Every accepted merge SHALL preserve an acyclic group quotient graph as well as original node topology. Reports SHALL include before/after remote-edge counts and constraint rejection reasons without invoking factories.
#### Reject a cyclic contraction
- merging an adjacent shortcut's endpoints would create a cycle through an intermediate component
- the merge is rejected even though the original node graph is a DAG
