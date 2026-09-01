package graph

import "sort"

type groupPlan struct {
	id    GroupID
	cpu   int
	nodes []NodeID
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

	placements, groups, edgeRemote := compileGroupPlan(b.nodes, report.order, b.edges, b.placements, b.cpuBindings)
	definition := &Definition{
		owner:        b.owner,
		nodes:        append([]nodeMetadata(nil), b.nodes...),
		edges:        append([]edgeMetadata(nil), b.edges...),
		edgeBindings: append([]edgeBinding(nil), b.edgeBindings...),
		forward:      copyValidationAdjacency(report.forward),
		reverse:      copyValidationAdjacency(report.reverse),
		order:        append([]NodeID(nil), report.order...),
		placements:   placements,
		groups:       groups,
		edgeRemote:   edgeRemote,
	}
	b.definition = definition
	b.state = builderSealed
	return definition, nil
}

func compileGroupPlan(nodes []nodeMetadata, order []NodeID, edges []edgeMetadata, explicit map[NodeID]GroupID, cpuBindings map[GroupID]int) ([]GroupID, []groupPlan, []bool) {
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
		groups[index] = groupPlan{id: group, cpu: cpuBindings[group], nodes: append([]NodeID(nil), byGroup[group]...)}
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
