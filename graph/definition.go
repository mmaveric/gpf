package graph

import (
	"github.com/asharipov/gpf/numa"
	"sort"
)

type groupPlan struct {
	id             GroupID
	logical        GroupID
	replica        uint32
	nodePoolWorker bool
	cpu            int
	numa           numa.Placement
	nodes          []NodeID
}

// GroupNUMAMetadata is a copied inactive placement annotation for one planned group.
type GroupNUMAMetadata struct {
	GroupID   GroupID
	Replica   uint32
	Placement numa.Placement
}

// Build validates an open builder and atomically publishes its immutable
// definition. A failed validation leaves the builder open for correction.
func (b *Builder) Build() (*Definition, error) {
	if b == nil || b.state == builderInvalid || b.owner == nil {
		return nil, ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return b.definition, nil
	}

	report, err := b.Validate()
	if err != nil {
		return nil, err
	}

	placements, groups, edgeRemote := compileGroupPlan(b.nodes, report.order, b.edges, b.placements, b.cpuBindings, b.numaBindings)
	edgeQueuePlans, edgeRemote, err := compileEdgeQueuePlans(b.edges, placements)
	if err != nil {
		return nil, err
	}
	definition := &Definition{
		planningMode:       b.planningMode,
		explicitPlacements: explicitPlacementSlice(b.nodes, b.placements),
		counters:           compileCounters(b),
		owner:              b.owner,
		nodes:              append([]nodeMetadata(nil), b.nodes...),
		edges:              append([]edgeMetadata(nil), b.edges...),
		edgeBindings:       append([]edgeBinding(nil), b.edgeBindings...),
		forward:            copyValidationAdjacency(report.forward),
		reverse:            copyValidationAdjacency(report.reverse),
		order:              append([]NodeID(nil), report.order...),
		placements:         placements,
		groups:             groups,
		edgeRemote:         edgeRemote,
		edgeQueuePlans:     edgeQueuePlans,
	}
	definition.constraints = compilePlanningConstraints(definition, b.planningMode)
	if err := validatePlanningConstraints(definition.constraints); err != nil {
		return nil, err
	}
	if b.logger != nil {
		config := *b.logger
		definition.logger = &config
	}
	if b.timing != nil {
		config := *b.timing
		definition.timing = &config
	}
	if b.workerTiming != nil {
		config := *b.workerTiming
		definition.workerTiming = &config
	}
	execution, err := expandReplicas(b, definition)
	if err != nil {
		return nil, err
	}
	baseExecution := definition
	if execution != nil {
		baseExecution = execution
	}
	pooled, err := expandNodePools(b, baseExecution)
	if err != nil {
		return nil, err
	}
	if pooled != baseExecution {
		execution = pooled
	}
	if execution != nil {
		execution.planningMode = b.planningMode
		execution.constraints = compilePlanningConstraints(execution, b.planningMode)
		if err := validatePlanningConstraints(execution.constraints); err != nil {
			return nil, err
		}
	}
	definition.execution = execution
	b.definition = definition
	b.state = builderSealed
	return definition, nil
}

func explicitPlacementSlice(nodes []nodeMetadata, placements map[NodeID]GroupID) []GroupID {
	result := make([]GroupID, len(nodes)+1)
	for _, item := range nodes {
		if group, ok := placements[item.nodeID]; ok {
			result[item.nodeID] = group
		}
	}
	return result
}

// EdgeQueuePlans returns an independent edge-ordered copy of physical transport
// model and endpoint-worker cardinalities selected during Build.
func (d *Definition) EdgeQueuePlans() []EdgeQueuePlan {
	if d == nil || d.owner == nil {
		return nil
	}
	plans := make([]EdgeQueuePlan, len(d.edgeQueuePlans))
	if d.execution != nil {
		plans = make([]EdgeQueuePlan, len(d.execution.edgeQueuePlans))
		copy(plans, d.execution.edgeQueuePlans)
	} else {
		copy(plans, d.edgeQueuePlans)
	}
	for i := range plans {
		if plans[i].LogicalEdgeID == 0 {
			plans[i].LogicalEdgeID = plans[i].EdgeID
		}
		plans[i].endpoints = nil
		plans[i].bindingID = 0
	}
	return plans
}

// GroupNUMA returns an independent ordered copy of inactive NUMA annotations.
func (d *Definition) GroupNUMA() []GroupNUMAMetadata {
	if d == nil || d.owner == nil {
		return nil
	}
	physical := d
	if d.execution != nil {
		physical = d.execution
	}
	rows := make([]GroupNUMAMetadata, len(physical.groups))
	for i, g := range physical.groups {
		rows[i] = GroupNUMAMetadata{GroupID: g.id, Replica: g.replica, Placement: g.numa}
	}
	return rows
}

func compileGroupPlan(nodes []nodeMetadata, order []NodeID, edges []edgeMetadata, explicit map[NodeID]GroupID, cpuBindings map[GroupID]int, numaBindings map[GroupID]numa.Placement) ([]GroupID, []groupPlan, []bool) {
	placements := make([]GroupID, len(nodes)+1)
	byGroup := make(map[GroupID][]NodeID, len(nodes))
	for _, nodeID := range order {
		group := explicit[nodeID]
		if group == InvalidGroupID {
			group = DefaultGroupID
		}
		placements[nodeID] = group
		byGroup[group] = append(byGroup[group], nodeID)
	}
	ids := make([]GroupID, 0, len(byGroup))
	for group := range byGroup {
		ids = append(ids, group)
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	groups := make([]groupPlan, len(ids))
	for index, group := range ids {
		groups[index] = groupPlan{id: group, cpu: cpuBindings[group], numa: numaBindings[group], nodes: append([]NodeID(nil), byGroup[group]...)}
	}
	remote := make([]bool, len(edges))
	for index, edge := range edges {
		remote[index] = placements[edge.source.nodeID] != placements[edge.target.nodeID]
	}
	return placements, groups, remote
}

// NodeCount returns the number of nodes in the definition.
func (d *Definition) NodeCount() int {
	if d == nil || d.owner == nil {
		return 0
	}
	return len(d.nodes)
}

// EdgeCount returns the number of edges in the definition.
func (d *Definition) EdgeCount() int {
	if d == nil || d.owner == nil {
		return 0
	}
	return len(d.edges)
}

// TopologicalOrder returns an independent copy of the canonical node order.
func (d *Definition) TopologicalOrder() []NodeID {
	if d == nil || d.owner == nil {
		return nil
	}
	order := make([]NodeID, len(d.order))
	copy(order, d.order)
	return order
}
