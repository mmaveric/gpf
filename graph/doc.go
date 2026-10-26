// Package graph defines typed graph-construction contracts.
//
// Registration and typed connections mutate an open Builder without
// invoking callbacks or queue factories. Connect validates port provenance,
// one producer per input, and one consumer per output. ConnectBroadcast makes
// shallow copies into branch-owned slots; ConnectShard routes each element to
// one branch with bounded staging. Both require fixed, non-expanded endpoints.
// It returns dense one-based EdgeID values and deterministically reports local
// errors. ConnectError explicitly routes one ErrorPort to one matching ordinary
// input. Error outputs are optional; queue configuration validation is pure and factories construct fresh
// operations only for a future runtime.
//
// Validate has signature func (b *Builder) Validate() (*ValidationReport,
// error). It returns a point-in-time ValidationReport and an aggregate
// ValidationError. Nil and zero-value builders report an invalid-builder issue;
// initialized empty builders report an empty-graph issue. Open and already
// sealed non-empty builders are analyzed without changing their state. Reports
// own deep-copy issues, canonical DAG order, and ordinary and error forward and reverse
// adjacency snapshots.
//
// Registration records only static required and present masks: factories,
// typed handlers, and multi-input selectors. Validation reads those masks and
// never invokes factories, callbacks, or queue factories. It requires every
// ordinary input and output to be connected and reuses Connect's pure edge
// configuration validation. Connected error outputs participate in analysis,
// while unconnected error outputs remain optional.
//
// Build has signature func (b *Builder) Build() (*Definition, error). Nil and
// zero-value builders return ErrInvalidBuilder without mutation. An open
// builder that fails validation remains open so its topology can be corrected.
// A successful Build copies the validated topology, retains the builder's
// private non-zero owner identity, caches the resulting Definition, and seals
// registration and Connect with ErrBuilderSealed. Later Build calls return the
// same cached definition. Definition exposes NodeCount, EdgeCount, a cloned
// TopologicalOrder, and Start; it does not expose adjacency or owner identity.
//
// Place assigns a registered node once to a fixed nonzero GroupID. Unplaced
// nodes use DefaultGroupID. BindCPU assigns one explicit nonnegative unique CPU
// to every effective group before Build. Build preserves dormant state, closure,
// and eligible local queue factory bindings, including explicit lifecycle
// callbacks, but invokes none of them. It compiles a private deterministic CPU
// group plan and selects edge queue models from producer and consumer counts.
// Failed results discard ordinary output and publish one typed error batch when
// connected.
//
// Start materializes locked OS-thread workers for the fixed execution groups,
// replicas, and node pools, with fresh pools, queues, and node state. Linux
// workers attempt CPU affinity with a pure-Go syscall; unsupported or rejected
// affinity retains the locked-thread fallback. AffinityStatus returns the
// immutable physical-worker ordered result for each logical group and replica.
// Queue models are selected from producer and consumer cardinalities; remote
// edges use the corresponding bounded typed queue. Start also wires ingress,
// metrics, logging, error mapping, retries, broadcast, sharding, and fan-in.
// Drain pauses workers at runner boundaries, disables sources, and processes
// accepted work to a stable global snapshot subject to its context deadline;
// Close discards queued and pending batches from that barrier. Both dispatch
// owned lifecycle callbacks to their locked owner workers in reverse canonical
// construction order. Panics are not recovered automatically.
//
// PlanningConstraints and MinimizeRemoteEdges are pure copied control-plane
// previews. The minimizer is deterministic and checks local legality and
// quotient acyclicity; it does not claim global optimality or change Build/Start
// placement. Builder.PlanGroups composes constraint normalization and
// minimization into an opt-in CPU-completeness preview; it does not seal or
// execute the builder. NUMA annotations and typed allocator descriptors are
// copied inactive metadata; there is no host topology discovery or NUMA-specific
// allocation. Platform-specific fast paths were assessed and declined; the
// portable Go implementation remains in use.
package graph
