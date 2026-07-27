package runtime

import "time"

// GroupConfig describes placement for one future execution group.
type GroupConfig struct {
	ID       uint32
	CPU      int
	NUMA     int
	Replicas uint32
}

// Config controls lifecycle behavior for a future graph runtime.
type Config struct {
	Groups       []GroupConfig
	DrainTimeout time.Duration
}

// Lifecycle documents the future runtime states.
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
