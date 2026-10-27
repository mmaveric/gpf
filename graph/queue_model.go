package graph

import (
	"errors"
	"github.com/mmaveric/gpf/numa"
)

var (
	// ErrInvalidQueueCardinality reports a zero or incompatible endpoint-worker count.
	ErrInvalidQueueCardinality = errors.New("invalid queue endpoint worker cardinality")
	// ErrIncompatibleQueueFactory reports a custom local-only factory on shared transport.
	ErrIncompatibleQueueFactory = errors.New("custom queue factory is incompatible with selected model")
)

// QueueModel identifies the bounded transport selected for one physical edge.
type QueueModel uint8

const (
	QueueModelInvalid QueueModel = iota
	QueueModelLocal
	QueueModelSPSC
	QueueModelMPSC
	QueueModelSPMC
	QueueModelMPMC
)

func (m QueueModel) String() string {
	switch m {
	case QueueModelLocal:
		return "local"
	case QueueModelSPSC:
		return "SPSC"
	case QueueModelMPSC:
		return "MPSC"
	case QueueModelSPMC:
		return "SPMC"
	case QueueModelMPMC:
		return "MPMC"
	default:
		return "invalid"
	}
}

// EdgeQueuePlan is immutable copied transport metadata for a physical edge.
type EdgeQueuePlan struct {
	bindingID       EdgeID
	endpoints       *edgeEndpoints
	EdgeID          EdgeID
	LogicalEdgeID   EdgeID
	MergeID         EdgeID
	BroadcastID     EdgeID
	ShardID         EdgeID
	Replica         uint32
	Model           QueueModel
	ProducerWorkers uint32
	ConsumerWorkers uint32
	Remote          bool
	NUMA            numa.Placement
}

type edgeEndpoints struct{ sources, targets []endpointID }

// ResolveQueueModel chooses the least concurrent transport for physical
// endpoint-worker cardinalities. sameWorker is true only when every operation
// on the edge belongs to one worker; local transport requires exactly one
// producer and one consumer worker.
func ResolveQueueModel(producers, consumers uint32, sameWorker bool) (QueueModel, error) {
	if producers == 0 || consumers == 0 {
		return QueueModelInvalid, ErrInvalidQueueCardinality
	}
	if sameWorker {
		if producers != 1 || consumers != 1 {
			return QueueModelInvalid, ErrInvalidQueueCardinality
		}
		return QueueModelLocal, nil
	}
	switch {
	case producers == 1 && consumers == 1:
		return QueueModelSPSC, nil
	case producers > 1 && consumers == 1:
		return QueueModelMPSC, nil
	case producers == 1 && consumers > 1:
		return QueueModelSPMC, nil
	default:
		return QueueModelMPMC, nil
	}
}

func compileEdgeQueuePlans(edges []edgeMetadata, placements []GroupID) ([]EdgeQueuePlan, []bool, error) {
	plans := make([]EdgeQueuePlan, len(edges))
	remote := make([]bool, len(edges))
	for index, edge := range edges {
		producerGroups, consumerGroups := map[GroupID]bool{}, map[GroupID]bool{}
		var sources, targets []endpointID
		mergeID := EdgeID(0)
		if edge.fanInMerge != 0 {
			mergeID = edge.fanInMerge
			seenTargets := make(map[endpointID]bool)
			for _, branch := range edges {
				if branch.fanInMerge == mergeID {
					sources = append(sources, branch.source)
					if !seenTargets[branch.target] {
						targets = append(targets, branch.target)
						seenTargets[branch.target] = true
					}
					producerGroups[placements[branch.source.nodeID]] = true
					consumerGroups[placements[branch.target.nodeID]] = true
				}
			}
		} else {
			sources = []endpointID{edge.source}
			targets = []endpointID{edge.target}
			producerGroups[placements[edge.source.nodeID]] = true
			consumerGroups[placements[edge.target.nodeID]] = true
		}
		sameWorker := len(producerGroups) == 1 && len(consumerGroups) == 1
		if sameWorker {
			for group := range producerGroups {
				if !consumerGroups[group] {
					sameWorker = false
				}
			}
		}
		model, err := ResolveQueueModel(uint32(len(producerGroups)), uint32(len(consumerGroups)), sameWorker)
		if err != nil {
			return nil, nil, err
		}
		remote[index] = !sameWorker
		plans[index] = EdgeQueuePlan{EdgeID: edge.id, LogicalEdgeID: edge.id, MergeID: mergeID, BroadcastID: edge.broadcastGroup, ShardID: edge.shardGroup, Model: model, ProducerWorkers: uint32(len(producerGroups)), ConsumerWorkers: uint32(len(consumerGroups)), Remote: remote[index], NUMA: edge.config.NUMA}
		if mergeID != 0 {
			plans[index].endpoints = &edgeEndpoints{sources: append([]endpointID(nil), sources...), targets: append([]endpointID(nil), targets...)}
		}
	}
	return plans, remote, nil
}
