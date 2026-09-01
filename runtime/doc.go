// Package runtime defines lifecycle and execution-placement contracts.
//
// A graph Definition is immutable after Build and retains dormant factory
// configuration for sequential reuse. The executable one-worker API belongs to
// graph: Definition.Start creates it, and Executor.Drain and Executor.Close
// stop it. The graph API supports fixed manual placement with an explicit CPU
// binding for every effective group, one locked worker per group, typed local
// and SPSC cross-group ordinary/error edges, and deterministic pause-barrier shutdown. Linux attempts
// per-worker CPU affinity; unsupported or rejected affinity retains the pure-Go
// locked-thread fallback, visible through Executor.AffinityStatus. This package
// has no separate Start API. Automatic grouping, CPU discovery or CPU-set
// policy, NUMA placement, hybrid scheduling, replicas, metrics, logging, error
// mapping/retries/observability, and concurrent starts remain unavailable.
package runtime
