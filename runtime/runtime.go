package runtime

import "time"

// GroupConfig describes placement and replication for an execution group.
// CPU and NUMA are placement identifiers; this package does not discover or
// validate available CPUs or NUMA nodes. Replicas is reserved configuration
// and does not currently create additional workers.
type GroupConfig struct {
	ID       uint32
	CPU      int
	NUMA     int
	Replicas uint32
}

// Config contains runtime group placement and shutdown timing configuration.
// Groups describes execution groups. DrainTimeout is the configured duration
// for draining accepted work; this package does not currently expose a
// separate runtime start API.
type Config struct {
	Groups       []GroupConfig
	DrainTimeout time.Duration
}

// Lifecycle identifies a runtime's lifecycle state.
// The constants describe the intended state sequence; this package currently
// does not implement a separate runtime lifecycle API.
type Lifecycle uint8

const (
	// Created is a runtime allocated from an immutable graph definition.
	Created Lifecycle = iota
	// Running is a runtime accepting committed ingress batches.
	Running
	// Draining is a runtime processing work accepted before ingress closure.
	Draining
	// Stopped is a runtime whose workers have exited.
	Stopped
)
