// Package showcase implements the bounded order-processing demonstration.
package showcase

import (
	"fmt"
	"sync/atomic"

	"github.com/mmaveric/gpf/graph"
)

// Config selects the handler timing clock. Its zero value uses the standard
// monotonic clock; TSC timing must be requested explicitly and requires CPU support.
type Config struct {
	// Clock accepts "standard" (also the empty default) or the explicit "tsc" option.
	Clock string
}

// Validate checks the clock selection without building or starting a graph.
func (c Config) Validate() error {
	_, err := c.clockSource()
	return err
}

func (c Config) clockSource() (graph.TimingClockSource, error) {
	switch c.Clock {
	case "", "standard":
		return graph.TimingClockStandard, nil
	case "tsc":
		return graph.TimingClockTSC, nil
	default:
		return 0, fmt.Errorf("unknown showcase clock %q: want standard or tsc", c.Clock)
	}
}

// Order is one input order for the showcase pipeline.
type Order struct {
	ID         uint64 `json:"id"`
	CustomerID uint64 `json:"customerId"`
	Units      uint32 `json:"units"`
	UnitCents  uint32 `json:"unitCents"`
}

type order struct {
	ID         uint64
	CustomerID uint64
	Units      uint32
	UnitCents  uint32
}

type validationError struct {
	OrderID uint64
	Code    uint32
}

type receipt struct {
	OrderID    uint64
	CustomerID uint64
	Partition  uint32
	Units      uint32
	TotalCents uint64
}

type showcaseStats struct {
	audited    atomic.Uint64
	rejected   atomic.Uint64
	processed  atomic.Uint64
	units      atomic.Uint64
	totalCents atomic.Uint64
	partitions [3]atomic.Uint64
}

type showcaseGraph struct {
	definition     *graph.Definition
	ingress        graph.IngressDefinition[Order]
	broadcastEdges []graph.EdgeID
	shardEdges     []graph.EdgeID
	fanInEdges     []graph.EdgeID
	partitionNodes [3]graph.NodeID
}
