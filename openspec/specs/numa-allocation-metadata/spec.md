# numa-allocation-metadata Specification

## Purpose
Specify dormant NUMA annotations and typed allocation descriptors without activating locality behavior.

## Requirements

### Requirement: Immutable optional NUMA metadata
Pool, physical group, edge, and ingress-pool configuration SHALL accept validated optional NUMA annotations with an unspecified zero default. Definitions and copied plan introspection SHALL preserve annotations without changing CPU assignment, queue selection, or existing portable allocation behavior.
#### Scenario: Configure locality intent
- **WHEN** a caller builds and starts a valid graph carrying NUMA annotations
- **THEN** copied metadata retains the intent while processing and allocation follow the existing portable behavior

### Requirement: Dormant typed allocator descriptors
Typed batch-pool, graph-edge pool/queue, and ingress-pool allocator hook descriptors SHALL remain uninvoked during registration, Build, and Start in this capability. They SHALL not box batch storage or imply active NUMA allocation. Activation SHALL require a separately approved behavioral change.
#### Scenario: Keep a configured hook inactive
- **WHEN** a definition with an observable allocator descriptor starts sequentially twice
- **THEN** the descriptor's call count remains zero and fresh ordinary pools are created each time
