package graph

import (
	"errors"
	"sort"
)

var ErrPlanningConstraintConflict = errors.New("conflicting automatic grouping constraints")

// NodeConstraint describes immutable placement facts for one physical node.
// ExplicitGroup is zero when the user did not call Place for its logical node.
type NodeConstraint struct {
	NodeID        NodeID
	LogicalNodeID NodeID
	Replica       uint32
	PhysicalGroup GroupID
	ExplicitGroup GroupID
	FixedGroup    bool
}

// EdgeConstraint records mandatory grouping decisions for one physical edge.
type EdgeConstraint struct {
	EdgeID                     EdgeID
	LogicalEdgeID              EdgeID
	Replica                    uint32
	SourceNodeID, TargetNodeID NodeID
	MustLink, MustSeparate     bool
}

// PlanningConstraints is a copied control-plane preview. It performs no merge,
// queue creation, factory invocation, or runtime startup.
type PlanningConstraints struct {
	Nodes []NodeConstraint
	Edges []EdgeConstraint
}

// Constraints returns an independent physical constraint snapshot for later
// opt-in automatic planning. The current Definition's execution groups remain
// unchanged.
func (d *Definition) Constraints() PlanningConstraints {
	if d == nil || d.owner == nil {
		return PlanningConstraints{}
	}
	physical := d
	if d.execution != nil {
		physical = d.execution
	}
	return clonePlanningConstraints(physical.constraints)
}

func compilePlanningConstraints(physical *Definition, automatic bool) PlanningConstraints {
	result := PlanningConstraints{Nodes: make([]NodeConstraint, len(physical.nodes)), Edges: make([]EdgeConstraint, len(physical.edges))}
	for index, item := range physical.nodes {
		logical := item.nodeID
		replica := uint32(0)
		if len(physical.nodeInstances) != 0 {
			logical = physical.nodeInstances[index].logical
			replica = physical.nodeInstances[index].replica
		}
		explicit := GroupID(0)
		if int(logical) < len(physical.explicitPlacements) {
			explicit = physical.explicitPlacements[logical]
		}
		group := physical.placements[item.nodeID]
		fixed := !automatic || explicit != 0
		if len(physical.groupInstances) != 0 {
			groupIndex := sort.Search(len(physical.groups), func(i int) bool { return physical.groups[i].id >= group })
			if groupIndex < len(physical.groups) && physical.groups[groupIndex].id == group {
				if physical.groups[groupIndex].nodePoolWorker {
					fixed = true
				}
				logicalGroup := physical.groups[groupIndex].logical
				instances := 0
				for _, candidate := range physical.groups {
					if candidate.logical == logicalGroup && !candidate.nodePoolWorker {
						instances++
					}
				}
				if instances > 1 {
					fixed = true
				}
			}
		}
		if automatic && !fixed {
			group = InvalidGroupID
		}
		result.Nodes[index] = NodeConstraint{NodeID: item.nodeID, LogicalNodeID: logical, Replica: replica, PhysicalGroup: group, ExplicitGroup: explicit, FixedGroup: fixed}
	}
	for index, edge := range physical.edges {
		logical, replica := edge.id, uint32(0)
		if len(physical.edgeInstances) != 0 {
			logical = physical.edgeInstances[index].logical
			replica = physical.edgeInstances[index].replica
		}
		sourceGroup, targetGroup := physical.placements[edge.source.nodeID], physical.placements[edge.target.nodeID]
		sourceLogical, targetLogical := edge.source.nodeID, edge.target.nodeID
		if len(physical.nodeInstances) != 0 {
			sourceLogical = physical.nodeInstances[sourceLogical-1].logical
			targetLogical = physical.nodeInstances[targetLogical-1].logical
		}
		sourceExplicit, targetExplicit := GroupID(0), GroupID(0)
		if int(sourceLogical) < len(physical.explicitPlacements) {
			sourceExplicit = physical.explicitPlacements[sourceLogical]
		}
		if int(targetLogical) < len(physical.explicitPlacements) {
			targetExplicit = physical.explicitPlacements[targetLogical]
		}
		sourceGroupPlan, targetGroupPlan := groupPlan{}, groupPlan{}
		for _, group := range physical.groups {
			if group.id == sourceGroup {
				sourceGroupPlan = group
			}
			if group.id == targetGroup {
				targetGroupPlan = group
			}
		}
		physicalCut := !automatic && sourceGroup != targetGroup
		if automatic && sourceExplicit != 0 && targetExplicit != 0 && sourceExplicit != targetExplicit {
			physicalCut = true
		}
		if sourceGroup != targetGroup && (sourceGroupPlan.nodePoolWorker || targetGroupPlan.nodePoolWorker) {
			physicalCut = true
		}
		if sourceGroup != targetGroup && sourceGroupPlan.logical != 0 && targetGroupPlan.logical != 0 && sourceGroupPlan.logical != targetGroupPlan.logical {
			physicalCut = true
		}
		if sourceGroup != targetGroup && sourceGroupPlan.logical != 0 && sourceGroupPlan.logical == targetGroupPlan.logical && sourceGroupPlan.replica != targetGroupPlan.replica {
			physicalCut = true
		}
		if physical.nodes[edge.source.nodeID-1].kind == ingressRegistration || physical.nodes[edge.target.nodeID-1].kind == ingressRegistration {
			physicalCut = true
		}
		pairedReplica := sourceGroup == targetGroup && sourceGroupPlan.logical != 0 && !sourceGroupPlan.nodePoolWorker
		if pairedReplica {
			instances := 0
			for _, group := range physical.groups {
				if group.logical == sourceGroupPlan.logical && !group.nodePoolWorker {
					instances++
				}
			}
			pairedReplica = instances > 1
		}
		mustLink := !physicalCut && ((sourceExplicit != 0 && sourceExplicit == targetExplicit) || pairedReplica || (!automatic && sourceGroup == targetGroup))
		mustSeparate := physicalCut || (sourceExplicit != 0 && targetExplicit != 0 && sourceExplicit != targetExplicit)
		result.Edges[index] = EdgeConstraint{EdgeID: edge.id, LogicalEdgeID: logical, Replica: replica, SourceNodeID: edge.source.nodeID, TargetNodeID: edge.target.nodeID, MustLink: mustLink, MustSeparate: mustSeparate}
	}
	return result
}

func clonePlanningConstraints(in PlanningConstraints) PlanningConstraints {
	out := PlanningConstraints{Nodes: make([]NodeConstraint, len(in.Nodes)), Edges: make([]EdgeConstraint, len(in.Edges))}
	copy(out.Nodes, in.Nodes)
	copy(out.Edges, in.Edges)
	return out
}

func validatePlanningConstraints(plan PlanningConstraints) error {
	for _, edge := range plan.Edges {
		if edge.MustLink && edge.MustSeparate {
			return ErrPlanningConstraintConflict
		}
	}
	return nil
}
