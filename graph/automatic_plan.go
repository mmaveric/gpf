package graph

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrAutomaticPlanningDisabled = errors.New("automatic planning preview is not enabled")
	ErrStaleAutomaticPlan        = errors.New("automatic grouping preview is stale")
	ErrInvalidAutomaticCPUPlan   = errors.New("invalid automatic plan CPU bindings")
)

// ErrAutomaticPlanningDisabled reports use of PlanGroups without explicit opt-in.
// The Builder remains in manual mode unless EnableAutomaticPlanning is called.

// IsCurrent verifies that a preview belongs to this builder and its current
// topology revision. Preview mutation does not mutate the builder.
func (p AutomaticGroupPreview) IsCurrent(b *Builder) bool {
	return b != nil && b.owner == p.owner && b.revision == p.revision && b.planningMode
}

// ValidateCurrent rejects a foreign or invalidated automatic preview.
func (p AutomaticGroupPreview) ValidateCurrent(b *Builder) error {
	if !p.IsCurrent(b) {
		return ErrStaleAutomaticPlan
	}
	return nil
}

// PlanGroups computes an opt-in read-only preview. It accepts missing group CPU
// bindings as CPU-incomplete metadata; it does not make the preview executable.
func (b *Builder) PlanGroups() (AutomaticGroupPreview, error) {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return AutomaticGroupPreview{}, ErrInvalidBuilder
	}
	if !b.planningMode {
		return AutomaticGroupPreview{}, ErrAutomaticPlanningDisabled
	}
	report, validationErr := b.Validate()
	if validationErr != nil {
		for _, issue := range report.issues {
			if issue.Code != ValidationCodeMissingGroupCPU {
				return AutomaticGroupPreview{}, validationErr
			}
		}
	}
	placements, groups, remote := compileGroupPlan(b.nodes, report.order, b.edges, b.placements, b.cpuBindings, b.numaBindings)
	queuePlans, remote, err := compileEdgeQueuePlans(b.edges, placements)
	if err != nil {
		return AutomaticGroupPreview{}, err
	}
	logical := &Definition{planningMode: true, explicitPlacements: explicitPlacementSlice(b.nodes, b.placements), owner: b.owner, nodes: append([]nodeMetadata(nil), b.nodes...), edges: append([]edgeMetadata(nil), b.edges...), edgeBindings: append([]edgeBinding(nil), b.edgeBindings...), order: append([]NodeID(nil), report.order...), placements: placements, groups: groups, edgeRemote: remote, edgeQueuePlans: queuePlans, counters: compileCounters(b)}
	if b.logger != nil {
		config := *b.logger
		logical.logger = &config
	}
	if b.timing != nil {
		config := *b.timing
		logical.timing = &config
	}
	if b.workerTiming != nil {
		config := *b.workerTiming
		logical.workerTiming = &config
	}
	physical, err := expandReplicas(b, logical)
	if err != nil {
		return AutomaticGroupPreview{}, err
	}
	if physical == nil {
		physical = logical
	}
	physical, err = expandNodePools(b, physical)
	if err != nil {
		return AutomaticGroupPreview{}, err
	}
	physical.planningMode = true
	constraints := compilePlanningConstraints(physical, true)
	if err := validatePlanningConstraints(constraints); err != nil {
		return AutomaticGroupPreview{}, err
	}
	grouping, err := MinimizeRemoteEdges(constraints)
	if err != nil {
		return AutomaticGroupPreview{}, err
	}
	assignment := make(map[NodeID]GroupID, len(grouping.Assignments))
	for _, row := range grouping.Assignments {
		assignment[row.NodeID] = row.GroupID
	}
	groupCPU := make(map[GroupID]groupPlan, len(physical.groups))
	boundCPU := make(map[GroupID]bool, len(physical.groups))
	for _, group := range physical.groups {
		groupCPU[group.id] = group
		_, bound := b.cpuBindings[group.id]
		if group.replica > 0 || group.nodePoolWorker {
			bound = true
		}
		boundCPU[group.id] = bound
	}
	hardGroups := make(map[GroupID]bool)
	for _, row := range constraints.Nodes {
		if row.FixedGroup {
			hardGroups[row.PhysicalGroup] = true
		}
	}
	groupsByID := make(map[GroupID]PlannedGroup)
	for _, row := range grouping.Assignments {
		info, exists := groupsByID[row.GroupID]
		if !exists {
			info = PlannedGroup{GroupID: row.GroupID, LogicalGroupID: row.GroupID}
			if fixed, ok := groupCPU[row.GroupID]; ok && hardGroups[fixed.id] {
				info.LogicalGroupID = fixed.logical
				if info.LogicalGroupID == 0 {
					info.LogicalGroupID = fixed.id
				}
				info.Replica, info.NUMA = fixed.replica, fixed.numa
				if boundCPU[fixed.id] {
					info.CPU = fixed.cpu
					info.CPUBound = true
				}
			}
		}
		if row.GroupID != info.GroupID {
			return AutomaticGroupPreview{}, ErrInvalidConstraintPlan
		}
		groupsByID[row.GroupID] = info
	}
	preview := AutomaticGroupPreview{Constraints: clonePlanningConstraints(constraints), RemoteEdgesBefore: grouping.RemoteEdgesBefore, RemoteEdgesAfter: grouping.RemoteEdgesAfter, Rejections: append([]MergeRejection(nil), grouping.Rejections...), owner: b.owner, revision: b.revision, CPUComplete: true}
	for _, group := range groupsByID {
		if b.automaticCPURevision == b.revision {
			if cpu, ok := b.automaticCPUs[group.GroupID]; ok {
				group.CPU = cpu
				group.CPUBound = true
			}
		}
		preview.Groups = append(preview.Groups, group)
		if !group.CPUBound {
			preview.CPUComplete = false
		}
	}
	sort.Slice(preview.Groups, func(i, j int) bool { return preview.Groups[i].GroupID < preview.Groups[j].GroupID })
	preview.requiredGroups = append([]PlannedGroup(nil), preview.Groups...)
	preview.Nodes = make([]AutomaticNodeAssignment, len(grouping.Assignments))
	for i, row := range grouping.Assignments {
		logicalID, replica := row.NodeID, uint32(0)
		if len(physical.nodeInstances) != 0 {
			logicalID = physical.nodeInstances[row.NodeID-1].logical
			replica = physical.nodeInstances[row.NodeID-1].replica
		}
		preview.Nodes[i] = AutomaticNodeAssignment{NodeID: row.NodeID, LogicalNodeID: logicalID, Replica: replica, GroupID: row.GroupID}
	}
	preview.Edges = make([]EdgeQueuePlan, len(physical.edges))
	for index, edge := range physical.edges {
		old := physical.edgeQueuePlans[index]
		sources, targets := []endpointID{edge.source}, []endpointID{edge.target}
		if old.endpoints != nil {
			sources, targets = old.endpoints.sources, old.endpoints.targets
		}
		producerGroups, consumerGroups := make(map[GroupID]bool), make(map[GroupID]bool)
		for _, ep := range sources {
			producerGroups[assignment[ep.nodeID]] = true
		}
		for _, ep := range targets {
			consumerGroups[assignment[ep.nodeID]] = true
		}
		local := len(producerGroups) == 1 && len(consumerGroups) == 1
		if local {
			for group := range producerGroups {
				if !consumerGroups[group] {
					local = false
				}
			}
		}
		model, err := ResolveQueueModel(uint32(len(producerGroups)), uint32(len(consumerGroups)), local)
		if err != nil {
			return AutomaticGroupPreview{}, err
		}
		plan := old
		plan.Model = model
		plan.Remote = !local
		plan.endpoints = nil
		plan.bindingID = 0
		preview.Edges[index] = plan
	}
	execution := *physical
	execution.execution = nil
	execution.groups = make([]groupPlan, 0, len(preview.Groups))
	execution.groupInstances = make([]groupInstance, 0, len(preview.Groups))
	execution.placements = make([]GroupID, len(physical.nodes)+1)
	groupIndex := make(map[GroupID]int, len(preview.Groups))
	for _, group := range preview.Groups {
		groupIndex[group.GroupID] = len(execution.groups)
		logical := group.LogicalGroupID
		if logical == 0 {
			logical = group.GroupID
		}
		execution.groups = append(execution.groups, groupPlan{id: group.GroupID, logical: logical, replica: group.Replica, cpu: group.CPU, numa: group.NUMA})
		execution.groupInstances = append(execution.groupInstances, groupInstance{logical: logical, replica: group.Replica})
	}
	execution.placements = make([]GroupID, len(physical.nodes)+1)
	for _, node := range preview.Nodes {
		execution.placements[node.NodeID] = node.GroupID
		idx, ok := groupIndex[node.GroupID]
		if !ok {
			return AutomaticGroupPreview{}, ErrInvalidConstraintPlan
		}
		execution.groups[idx].nodes = append(execution.groups[idx].nodes, node.NodeID)
	}
	execution.edgeQueuePlans = append([]EdgeQueuePlan(nil), physical.edgeQueuePlans...)
	execution.edgeRemote = make([]bool, len(preview.Edges))
	for i, plan := range preview.Edges {
		execution.edgeQueuePlans[i].Model = plan.Model
		execution.edgeQueuePlans[i].Remote = plan.Remote
		execution.edgeRemote[i] = plan.Remote
	}
	execution.constraints = clonePlanningConstraints(constraints)
	preview.logical = logical
	preview.execution = &execution
	return preview, nil
}

// FinalizeAutomaticPlan binds one explicit unique CPU to every previewed group
// and seals the builder with an executable immutable definition. No discovery
// or default CPU policy is applied.
func FinalizeAutomaticPlan(b *Builder, preview AutomaticGroupPreview) (*Definition, error) {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return nil, ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return nil, ErrBuilderSealed
	}
	if err := preview.ValidateCurrent(b); err != nil {
		return nil, err
	}
	if preview.execution == nil || preview.logical == nil || b.automaticCPURevision != b.revision || len(b.automaticCPUs) != len(preview.requiredGroups) || len(preview.Groups) != len(preview.requiredGroups) {
		return nil, ErrInvalidAutomaticCPUPlan
	}
	byGroup := b.automaticCPUs
	for index, planned := range preview.requiredGroups {
		if preview.Groups[index].GroupID != planned.GroupID {
			return nil, ErrInvalidAutomaticCPUPlan
		}
		cpu, exists := byGroup[planned.GroupID]
		if !exists {
			return nil, ErrInvalidAutomaticCPUPlan
		}
		if planned.CPUBound && planned.CPU != cpu {
			return nil, ErrInvalidAutomaticCPUPlan
		}
	}
	execution := *preview.execution
	execution.groups = append([]groupPlan(nil), preview.execution.groups...)
	for i := range execution.groups {
		group := &execution.groups[i]
		cpu, ok := byGroup[group.id]
		if !ok {
			return nil, ErrInvalidAutomaticCPUPlan
		}
		group.cpu = cpu
		group.nodes = append([]NodeID(nil), group.nodes...)
	}
	execution.edgeQueuePlans = append([]EdgeQueuePlan(nil), preview.execution.edgeQueuePlans...)
	execution.edgeRemote = append([]bool(nil), preview.execution.edgeRemote...)
	logical := *preview.logical
	logical.execution = &execution
	logical.constraints = clonePlanningConstraints(preview.Constraints)
	b.definition = &logical
	b.state = builderSealed
	b.revision++
	return &logical, nil
}

// ExplainAutomaticPlan is a compact diagnostic for users inspecting an
// incomplete CPU plan without invoking Build or runtime factories.
func (p AutomaticGroupPreview) ExplainAutomaticPlan() string {
	return fmt.Sprintf("groups=%d remote_edges=%d→%d cpu_complete=%t", len(p.Groups), p.RemoteEdgesBefore, p.RemoteEdgesAfter, p.CPUComplete)
}
