package graph

import (
	"errors"
	"fmt"
	"github.com/asharipov/gpf/numa"
	"math"
	"sort"
)

var (
	ErrInvalidReplicas   = errors.New("invalid group replica configuration")
	ErrInvalidWorkerPool = errors.New("invalid node worker-pool configuration")
	ErrReplicatedIngress = errors.New("ingress cannot belong to a replicated group")
	ErrAmbiguousCounter  = errors.New("counter lookup requires an explicit replica")
)

type replicaConfig struct {
	count uint32
	cpus  map[uint32]int
}
type nodePoolConfig struct {
	count uint32
	cpus  map[uint32]int
}
type nodeInstance struct {
	logical NodeID
	replica uint32
}
type groupInstance struct {
	logical GroupID
	replica uint32
}
type edgeInstance struct {
	logical EdgeID
	replica uint32
}

func logicalGroupFor(d *Definition, physical GroupID) GroupID {
	for i, g := range d.groups {
		if g.id == physical && len(d.groupInstances) != 0 {
			return d.groupInstances[i].logical
		}
	}
	return physical
}

// ReplicateGroup configures a fixed count before Build. Replica zero uses the
// group's existing BindCPU assignment; other replicas require BindReplicaCPU.
func ReplicateGroup(b *Builder, group GroupID, count uint32) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	if group == 0 || count == 0 {
		return ErrInvalidReplicas
	}
	if b.replicas == nil {
		b.replicas = make(map[GroupID]replicaConfig)
	}
	if _, exists := b.replicas[group]; exists {
		return ErrInvalidReplicas
	}
	b.replicas[group] = replicaConfig{count: count, cpus: make(map[uint32]int)}
	b.revision++
	return nil
}

// BindReplicaCPU assigns one explicit CPU to a zero-based group replica.
func BindReplicaCPU(b *Builder, group GroupID, replica uint32, cpu int) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	config, exists := b.replicas[group]
	if !exists || replica >= config.count || cpu < 0 {
		return ErrInvalidReplicas
	}
	if replica == 0 {
		return BindCPU(b, group, cpu)
	}
	if _, exists := config.cpus[replica]; exists {
		return ErrInvalidReplicas
	}
	config.cpus[replica] = cpu
	b.revision++
	return nil
}

// BindAutomaticPlanCPU validates an explicit CPU choice for one current preview
// row without assigning CPU affinity to the builder's manual plan.
func BindAutomaticPlanCPU(b *Builder, preview *AutomaticGroupPreview, group GroupID, cpu int) error {
	if preview == nil {
		return ErrInvalidAutomaticCPUPlan
	}
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	if err := preview.ValidateCurrent(b); err != nil {
		return err
	}
	if cpu < 0 {
		return ErrInvalidCPU
	}
	if b.automaticCPURevision != b.revision {
		b.automaticCPUs = make(map[GroupID]int)
		b.automaticCPURevision = b.revision
	}
	if len(preview.requiredGroups) != len(preview.Groups) || preview.execution == nil {
		return ErrInvalidAutomaticCPUPlan
	}
	for index, planned := range preview.requiredGroups {
		if planned.GroupID == group {
			if planned.CPUBound && planned.CPU != cpu {
				return ErrInvalidAutomaticCPUPlan
			}
			if _, exists := b.automaticCPUs[group]; exists {
				return ErrInvalidAutomaticCPUPlan
			}
			for _, bound := range b.automaticCPUs {
				if bound == cpu {
					return ErrCPUAlreadyBound
				}
			}
			b.automaticCPUs[group] = cpu
			preview.Groups[index].CPU = cpu
			preview.Groups[index].CPUBound = true
			preview.requiredGroups[index] = preview.Groups[index]
			preview.CPUComplete = true
			for _, row := range preview.Groups {
				if !row.CPUBound {
					preview.CPUComplete = false
					break
				}
			}
			return nil
		}
	}
	return ErrInvalidAutomaticCPUPlan
}

// ReplicateNode configures fixed node-local parallelism without replicating its
// surrounding logical group. Every worker requires an explicit CPU binding.
func ReplicateNode(b *Builder, nodeID NodeID, count uint32) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	if nodeID == 0 || int(nodeID) > len(b.nodes) || count == 0 {
		return ErrInvalidWorkerPool
	}
	if b.nodePools == nil {
		b.nodePools = make(map[NodeID]nodePoolConfig)
	}
	if _, exists := b.nodePools[nodeID]; exists {
		return ErrInvalidWorkerPool
	}
	b.nodePools[nodeID] = nodePoolConfig{count: count, cpus: make(map[uint32]int)}
	b.revision++
	return nil
}

// SetNodeWorkerPool configures fixed node-local worker parallelism.
func SetNodeWorkerPool(b *Builder, nodeID NodeID, count uint32) error {
	return ReplicateNode(b, nodeID, count)
}

// BindNodePoolCPU assigns one explicit unique CPU to a zero-based node worker.
func BindNodePoolCPU(b *Builder, nodeID NodeID, worker uint32, cpu int) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	config, exists := b.nodePools[nodeID]
	if !exists || worker >= config.count || cpu < 0 {
		return ErrInvalidWorkerPool
	}
	if _, exists := config.cpus[worker]; exists {
		return ErrInvalidWorkerPool
	}
	config.cpus[worker] = cpu
	b.revision++
	return nil
}

func expandReplicas(b *Builder, logical *Definition) (*Definition, error) {
	if len(b.replicas) == 0 {
		return nil, nil
	}
	groups := make(map[GroupID]groupPlan)
	usedCPUs := make(map[int]bool)
	var maxGroup GroupID
	for _, g := range logical.groups {
		groups[g.id] = g
		usedCPUs[g.cpu] = true
		if g.id > maxGroup {
			maxGroup = g.id
		}
	}
	replicated := make([]GroupID, 0, len(b.replicas))
	for group := range b.replicas {
		replicated = append(replicated, group)
	}
	sort.Slice(replicated, func(i, j int) bool { return replicated[i] < replicated[j] })
	for _, group := range replicated {
		config := b.replicas[group]
		g, exists := groups[group]
		if !exists {
			return nil, fmt.Errorf("group %d: %w", group, ErrInvalidReplicas)
		}
		if uint64(config.count) > uint64(math.MaxUint32)-uint64(maxGroup) {
			return nil, ErrInvalidReplicas
		}
		for _, id := range g.nodes {
			if config.count > 1 && logical.nodes[id-1].kind == ingressRegistration {
				return nil, ErrReplicatedIngress
			}
			if config.count > 1 && b.nodePools[id].count > 1 {
				return nil, errors.Join(ErrInvalidReplicas, ErrInvalidWorkerPool)
			}
			if config.count > 1 {
				for _, edge := range b.edges {
					if edge.broadcastGroup != 0 && (edge.source.nodeID == id || edge.target.nodeID == id) {
						return nil, ErrBroadcastParallelExpansion
					}
					if edge.shardGroup != 0 && (edge.source.nodeID == id || edge.target.nodeID == id) {
						return nil, ErrShardParallelExpansion
					}
				}
			}
		}
		for replica := uint32(1); replica < config.count; replica++ {
			cpu, exists := config.cpus[replica]
			if !exists || cpu < 0 || usedCPUs[cpu] {
				return nil, fmt.Errorf("group %d replica %d: %w", group, replica, ErrInvalidReplicas)
			}
			usedCPUs[cpu] = true
		}
	}
	physical := &Definition{owner: logical.owner, planningMode: logical.planningMode, explicitPlacements: append([]GroupID(nil), logical.explicitPlacements...), logger: logical.logger, timing: logical.timing, workerTiming: logical.workerTiming}
	groupIDs := make(map[GroupID][]GroupID)
	for _, g := range logical.groups {
		count := uint32(1)
		if config, exists := b.replicas[g.id]; exists {
			count = config.count
		}
		physicalIDs := make([]GroupID, count)
		physicalIDs[0] = g.id
		groupIDs[g.id] = physicalIDs
		if uint64(count-1) > uint64(math.MaxUint32)-uint64(maxGroup) {
			return nil, ErrInvalidReplicas
		}
		maxGroup += GroupID(count - 1)
	}
	baseGroup := maxGroup
	for _, g := range logical.groups {
		count := uint32(1)
		if config, exists := b.replicas[g.id]; exists {
			count = config.count
		}
		for replica := uint32(0); replica < count; replica++ {
			id, cpu := g.id, g.cpu
			if replica != 0 {
				id = baseGroup + 1
				for _, prior := range logical.groups {
					if prior.id >= g.id {
						break
					}
					if cfg, ok := b.replicas[prior.id]; ok {
						id += GroupID(cfg.count - 1)
					}
				}
				id += GroupID(replica - 1)
				cpu = b.replicas[g.id].cpus[replica]
			}
			groupIDs[g.id][replica] = id
			physical.groups = append(physical.groups, groupPlan{id: id, logical: g.id, replica: replica, cpu: cpu, numa: g.numa})
			physical.groupInstances = append(physical.groupInstances, groupInstance{g.id, replica})
		}
	}
	indices := make([]int, len(physical.groups))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool { return physical.groups[indices[i]].id < physical.groups[indices[j]].id })
	sortedGroups := make([]groupPlan, len(indices))
	sortedIDs := make([]groupInstance, len(indices))
	for i, j := range indices {
		sortedGroups[i] = physical.groups[j]
		sortedIDs[i] = physical.groupInstances[j]
	}
	physical.groups, physical.groupInstances = sortedGroups, sortedIDs
	nodeIDs := make(map[NodeID][]NodeID)
	physical.placements = append(physical.placements, 0)
	for _, n := range logical.nodes {
		logicalGroup := logical.placements[n.nodeID]
		for replica, group := range groupIDs[logicalGroup] {
			if uint64(len(physical.nodes)) >= uint64(math.MaxUint32) {
				return nil, ErrInvalidReplicas
			}
			id := NodeID(len(physical.nodes) + 1)
			copy := n
			copy.nodeID = id
			physical.nodes = append(physical.nodes, copy)
			physical.nodeInstances = append(physical.nodeInstances, nodeInstance{n.nodeID, uint32(replica)})
			physical.placements = append(physical.placements, group)
			nodeIDs[n.nodeID] = append(nodeIDs[n.nodeID], id)
		}
	}
	for _, logicalID := range logical.order {
		physical.order = append(physical.order, nodeIDs[logicalID]...)
	}
	for i := range physical.groups {
		for _, id := range physical.order {
			if physical.placements[id] == physical.groups[i].id {
				physical.groups[i].nodes = append(physical.groups[i].nodes, id)
			}
		}
	}
	for _, counter := range logical.counters {
		for _, id := range nodeIDs[counter.NodeID] {
			physical.counters = append(physical.counters, counterDefinition{id, counter.CounterID})
		}
	}
	sort.Slice(physical.counters, func(i, j int) bool {
		if physical.counters[i].NodeID != physical.counters[j].NodeID {
			return physical.counters[i].NodeID < physical.counters[j].NodeID
		}
		return physical.counters[i].CounterID < physical.counters[j].CounterID
	})
	for index, edge := range logical.edges {
		sources, targets := nodeIDs[edge.source.nodeID], nodeIDs[edge.target.nodeID]
		local := logical.placements[edge.source.nodeID] == logical.placements[edge.target.nodeID]
		copies := 1
		if local {
			copies = len(sources)
		}
		for replica := 0; replica < copies; replica++ {
			id := EdgeID(len(physical.edges) + 1)
			copy := edge
			copy.id = id
			plan := EdgeQueuePlan{EdgeID: id, bindingID: edge.id, LogicalEdgeID: edge.id, Replica: uint32(replica), Remote: !local, endpoints: &edgeEndpoints{}}
			if local {
				copy.source.nodeID = sources[replica]
				copy.target.nodeID = targets[replica]
				plan.endpoints.sources = []endpointID{copy.source}
				plan.endpoints.targets = []endpointID{copy.target}
			} else {
				for _, source := range sources {
					ep := edge.source
					ep.nodeID = source
					plan.endpoints.sources = append(plan.endpoints.sources, ep)
				}
				for _, target := range targets {
					ep := edge.target
					ep.nodeID = target
					plan.endpoints.targets = append(plan.endpoints.targets, ep)
				}
				copy.source = plan.endpoints.sources[0]
				copy.target = plan.endpoints.targets[0]
			}
			plan.ProducerWorkers = uint32(len(plan.endpoints.sources))
			plan.ConsumerWorkers = uint32(len(plan.endpoints.targets))
			model, err := ResolveQueueModel(plan.ProducerWorkers, plan.ConsumerWorkers, local)
			if err != nil {
				return nil, err
			}
			plan.Model = model
			physical.edges = append(physical.edges, copy)
			physical.edgeBindings = append(physical.edgeBindings, logical.edgeBindings[index])
			physical.edgeRemote = append(physical.edgeRemote, !local)
			physical.edgeQueuePlans = append(physical.edgeQueuePlans, plan)
			physical.edgeInstances = append(physical.edgeInstances, edgeInstance{edge.id, uint32(replica)})
		}
	}
	return physical, nil
}

func expandNodePools(b *Builder, input *Definition) (*Definition, error) {
	if len(b.nodePools) == 0 {
		return input, nil
	}
	baseCPU := make(map[int]bool)
	maxGroup := GroupID(0)
	for _, g := range input.groups {
		baseCPU[g.cpu] = true
		if g.id > maxGroup {
			maxGroup = g.id
		}
	}
	for logicalID, config := range b.nodePools {
		logicalGroup := b.placements[logicalID]
		if logicalGroup == InvalidGroupID {
			logicalGroup = DefaultGroupID
		}
		if rc, ok := b.replicas[logicalGroup]; ok && rc.count > 1 {
			return nil, ErrInvalidWorkerPool
		}
		for index := uint32(0); index < config.count; index++ {
			cpu, ok := config.cpus[index]
			if !ok || cpu < 0 || baseCPU[cpu] {
				return nil, ErrInvalidWorkerPool
			}
			baseCPU[cpu] = true
		}
		if uint64(config.count) > uint64(math.MaxUint32)-uint64(maxGroup) {
			return nil, ErrInvalidWorkerPool
		}
		maxGroup += GroupID(config.count)
		if b.nodes[logicalID-1].kind == ingressRegistration {
			return nil, ErrInvalidWorkerPool
		}
		for _, edge := range b.edges {
			if edge.broadcastGroup != 0 && (edge.source.nodeID == logicalID || edge.target.nodeID == logicalID) {
				return nil, ErrBroadcastParallelExpansion
			}
			if edge.shardGroup != 0 && (edge.source.nodeID == logicalID || edge.target.nodeID == logicalID) {
				return nil, ErrShardParallelExpansion
			}
		}
	}
	physical := &Definition{owner: input.owner, planningMode: input.planningMode, explicitPlacements: append([]GroupID(nil), input.explicitPlacements...), logger: input.logger, timing: input.timing, workerTiming: input.workerTiming, placements: []GroupID{InvalidGroupID}}
	physicalNodeIDs := make(map[NodeID][]NodeID)
	physicalGroups := make(map[GroupID]groupPlan)
	physicalGroupInstances := make(map[GroupID]groupInstance)
	for i, g := range input.groups {
		physicalGroups[g.id] = groupPlan{id: g.id, logical: g.id, cpu: g.cpu, numa: g.numa}
		gi := groupInstance{logical: g.id}
		if len(input.groupInstances) > 0 {
			gi = input.groupInstances[i]
		}
		physicalGroupInstances[g.id] = gi
	}
	nodePools := make(map[NodeID]nodePoolConfig)
	for id, cfg := range b.nodePools {
		nodePools[id] = cfg
	}
	for _, old := range input.order {
		logicalID := old
		replica := uint32(0)
		if len(input.nodeInstances) > 0 {
			logicalID = input.nodeInstances[old-1].logical
			replica = input.nodeInstances[old-1].replica
		}
		cfg, pooled := nodePools[logicalID]
		count := uint32(1)
		if pooled {
			count = cfg.count
		}
		for worker := uint32(0); worker < count; worker++ {
			if uint64(len(physical.nodes)) >= uint64(math.MaxUint32) {
				return nil, ErrInvalidWorkerPool
			}
			id := NodeID(len(physical.nodes) + 1)
			meta := input.nodes[old-1]
			meta.nodeID = id
			group := input.placements[old]
			gi := groupInstance{logical: logicalGroupFor(input, group), replica: replica}
			if pooled {
				maxGroup++
				group = maxGroup
				gi = groupInstance{logical: logicalGroupFor(input, input.placements[old]), replica: worker}
				numaPlacement := numa.Placement{}
				for _, candidate := range input.groups {
					if candidate.id == input.placements[old] {
						numaPlacement = candidate.numa
						break
					}
				}
				physicalGroups[group] = groupPlan{id: group, logical: gi.logical, replica: worker, nodePoolWorker: true, cpu: cfg.cpus[worker], numa: numaPlacement}
				physicalGroupInstances[group] = gi
			}
			physical.nodes = append(physical.nodes, meta)
			physical.nodeInstances = append(physical.nodeInstances, nodeInstance{logical: logicalID, replica: func() uint32 {
				if pooled {
					return worker
				}
				return replica
			}()})
			physical.placements = append(physical.placements, group)
			physical.order = append(physical.order, id)
			physicalNodeIDs[old] = append(physicalNodeIDs[old], id)
			physicalGroups[group] = func() groupPlan { g := physicalGroups[group]; g.nodes = append(g.nodes, id); return g }()
		}
	}
	physical.groups = make([]groupPlan, 0, len(physicalGroups))
	for _, g := range physicalGroups {
		physical.groups = append(physical.groups, g)
	}
	sort.Slice(physical.groups, func(i, j int) bool { return physical.groups[i].id < physical.groups[j].id })
	physical.groupInstances = make([]groupInstance, len(physical.groups))
	for i, g := range physical.groups {
		physical.groupInstances[i] = physicalGroupInstances[g.id]
	}
	for _, counter := range input.counters {
		for _, id := range physicalNodeIDs[counter.NodeID] {
			rep := physical.nodeInstances[id-1].replica
			physical.counters = append(physical.counters, counterDefinition{id, counter.CounterID})
			_ = rep
		}
	}
	sort.Slice(physical.counters, func(i, j int) bool {
		if physical.counters[i].NodeID != physical.counters[j].NodeID {
			return physical.counters[i].NodeID < physical.counters[j].NodeID
		}
		return physical.counters[i].CounterID < physical.counters[j].CounterID
	})
	for index, edge := range input.edges {
		logicalSources, logicalTargets := []NodeID{edge.source.nodeID}, []NodeID{edge.target.nodeID}
		if input.edgeQueuePlans[index].endpoints != nil {
			logicalSources = nil
			logicalTargets = nil
			for _, ep := range input.edgeQueuePlans[index].endpoints.sources {
				logicalSources = append(logicalSources, ep.nodeID)
			}
			for _, ep := range input.edgeQueuePlans[index].endpoints.targets {
				logicalTargets = append(logicalTargets, ep.nodeID)
			}
		}
		var sources, targets []NodeID
		for _, id := range logicalSources {
			sources = append(sources, physicalNodeIDs[id]...)
		}
		for _, id := range logicalTargets {
			targets = append(targets, physicalNodeIDs[id]...)
		}
		local := len(sources) == 1 && len(targets) == 1 && physical.placements[sources[0]] == physical.placements[targets[0]]
		bindingID := input.edgeQueuePlans[index].bindingID
		if bindingID == 0 {
			bindingID = input.edgeQueuePlans[index].LogicalEdgeID
			if bindingID == 0 {
				bindingID = edge.id
			}
		}
		logicalEdgeID := input.edgeQueuePlans[index].LogicalEdgeID
		if logicalEdgeID == 0 {
			logicalEdgeID = edge.id
		}
		edgeReplica := uint32(0)
		if len(input.edgeInstances) != 0 {
			edgeReplica = input.edgeInstances[index].replica
			logicalEdgeID = input.edgeInstances[index].logical
		}
		plan := EdgeQueuePlan{EdgeID: EdgeID(len(physical.edges) + 1), LogicalEdgeID: logicalEdgeID, bindingID: bindingID, Replica: edgeReplica, Remote: !local, endpoints: &edgeEndpoints{}}
		if !local {
			for _, id := range sources {
				ep := edge.source
				ep.nodeID = id
				plan.endpoints.sources = append(plan.endpoints.sources, ep)
			}
			for _, id := range targets {
				ep := edge.target
				ep.nodeID = id
				plan.endpoints.targets = append(plan.endpoints.targets, ep)
			}
		} else {
			copy := edge
			copy.source.nodeID = sources[0]
			copy.target.nodeID = targets[0]
			edge = copy
			plan.endpoints.sources = []endpointID{edge.source}
			plan.endpoints.targets = []endpointID{edge.target}
		}
		plan.ProducerWorkers, plan.ConsumerWorkers = uint32(len(plan.endpoints.sources)), uint32(len(plan.endpoints.targets))
		model, err := ResolveQueueModel(plan.ProducerWorkers, plan.ConsumerWorkers, local)
		if err != nil {
			return nil, err
		}
		plan.Model = model
		if edge.hasFactory && model != QueueModelLocal {
			return nil, fmt.Errorf("edge %d: %w", edge.id, ErrIncompatibleQueueFactory)
		}
		edge.id = plan.EdgeID
		if !local {
			edge.source = plan.endpoints.sources[0]
			edge.target = plan.endpoints.targets[0]
		}
		physical.edges = append(physical.edges, edge)
		physical.edgeBindings = append(physical.edgeBindings, input.edgeBindings[index])
		physical.edgeRemote = append(physical.edgeRemote, !local)
		physical.edgeQueuePlans = append(physical.edgeQueuePlans, plan)
		physical.edgeInstances = append(physical.edgeInstances, edgeInstance{logical: plan.LogicalEdgeID, replica: plan.Replica})
	}
	return physical, nil
}
