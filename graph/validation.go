package graph

import (
	"container/heap"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ValidationCode identifies a graph validation failure.
type ValidationCode uint8

const (
	// ValidationCodeInvalid is the zero value and does not identify a validation issue.
	ValidationCodeInvalid ValidationCode = iota
	ValidationCodeInvalidBuilder
	ValidationCodeEmptyGraph
	ValidationCodeMissingFactory
	ValidationCodeMissingHandler
	ValidationCodeMissingSelector
	ValidationCodeUnconnectedInput
	ValidationCodeUnconnectedOutput
	ValidationCodeInvalidEdgeCapacity
	ValidationCodeInvalidEdgePolicy
	ValidationCodeCycle
	ValidationCodeUnreachableFromSource
	ValidationCodeCannotReachSink
	ValidationCodeInvalidGroupPlacement
	ValidationCodeRemoteQueueFactory
	ValidationCodeMissingGroupCPU
	ValidationCodeInvalidGroupCPU
	ValidationCodeDuplicateGroupCPU
	ValidationCodeIneffectiveGroupCPU
)

// ValidationIssue identifies one independently discoverable graph defect.
type ValidationIssue struct {
	Code    ValidationCode
	NodeID  NodeID
	PortID  PortID
	EdgeID  EdgeID
	GroupID GroupID
	CPU     int
	Cycle   []NodeID
}

// ValidationAdjacentEdge identifies one edge incident to an adjacency row.
type ValidationAdjacentEdge struct {
	EdgeID         EdgeID
	AdjacentNodeID NodeID
}

// ValidationAdjacency is one node's deterministic adjacency snapshot.
type ValidationAdjacency struct {
	NodeID NodeID
	Edges  []ValidationAdjacentEdge
}

// ValidationReport is an owned point-in-time validation snapshot.
type ValidationReport struct {
	issues  []ValidationIssue
	order   []NodeID
	forward []ValidationAdjacency
	reverse []ValidationAdjacency
}

// Issues returns a deep copy of the report's sorted issues.
func (r *ValidationReport) Issues() []ValidationIssue {
	if r == nil {
		return nil
	}
	return copyValidationIssues(r.issues)
}

// TopologicalOrder returns a copy of the report's canonical DAG order.
func (r *ValidationReport) TopologicalOrder() []NodeID {
	if r == nil {
		return nil
	}
	return append([]NodeID(nil), r.order...)
}

// ForwardAdjacency returns a deep copy of ordinary outgoing edge snapshots.
func (r *ValidationReport) ForwardAdjacency() []ValidationAdjacency {
	if r == nil {
		return nil
	}
	return copyValidationAdjacency(r.forward)
}

// ReverseAdjacency returns a deep copy of ordinary incoming edge snapshots.
func (r *ValidationReport) ReverseAdjacency() []ValidationAdjacency {
	if r == nil {
		return nil
	}
	return copyValidationAdjacency(r.reverse)
}

// ValidationError aggregates every issue found during validation.
type ValidationError struct {
	issues []ValidationIssue
}

// Error formats sorted diagnostics deterministically.
func (e *ValidationError) Error() string {
	if e == nil || len(e.issues) == 0 {
		return "graph validation failed"
	}
	parts := make([]string, len(e.issues))
	for index, issue := range e.issues {
		parts[index] = formatValidationIssue(issue)
	}
	return "graph validation failed: " + strings.Join(parts, "; ")
}

// Issues returns a deep copy of the error's sorted issues.
func (e *ValidationError) Issues() []ValidationIssue {
	if e == nil {
		return nil
	}
	return copyValidationIssues(e.issues)
}

// String returns the diagnostic label for c, or "invalid validation code" when unknown.
func (c ValidationCode) String() string {
	switch c {
	case ValidationCodeInvalidBuilder:
		return "invalid builder"
	case ValidationCodeEmptyGraph:
		return "empty graph"
	case ValidationCodeMissingFactory:
		return "missing factory"
	case ValidationCodeMissingHandler:
		return "missing handler"
	case ValidationCodeMissingSelector:
		return "missing selector"
	case ValidationCodeUnconnectedInput:
		return "unconnected input"
	case ValidationCodeUnconnectedOutput:
		return "unconnected output"
	case ValidationCodeInvalidEdgeCapacity:
		return "invalid edge capacity"
	case ValidationCodeInvalidEdgePolicy:
		return "invalid edge policy"
	case ValidationCodeCycle:
		return "cycle"
	case ValidationCodeUnreachableFromSource:
		return "unreachable from source"
	case ValidationCodeCannotReachSink:
		return "cannot reach sink"
	case ValidationCodeInvalidGroupPlacement:
		return "invalid group placement"
	case ValidationCodeRemoteQueueFactory:
		return "remote queue factory"
	case ValidationCodeMissingGroupCPU:
		return "missing group CPU"
	case ValidationCodeInvalidGroupCPU:
		return "invalid group CPU"
	case ValidationCodeDuplicateGroupCPU:
		return "duplicate group CPU"
	case ValidationCodeIneffectiveGroupCPU:
		return "ineffective group CPU"
	default:
		return "invalid validation code"
	}
}

func formatValidationIssue(issue ValidationIssue) string {
	var text strings.Builder
	text.WriteString(issue.Code.String())
	if issue.NodeID != InvalidNodeID {
		fmt.Fprintf(&text, " node=%d", issue.NodeID)
	}
	if issue.PortID != InvalidPortID {
		fmt.Fprintf(&text, " port=%d", issue.PortID)
	}
	if issue.EdgeID != InvalidEdgeID {
		fmt.Fprintf(&text, " edge=%d", issue.EdgeID)
	}
	if issue.GroupID != InvalidGroupID {
		fmt.Fprintf(&text, " group=%d", issue.GroupID)
	}
	if issue.Code == ValidationCodeInvalidGroupCPU || issue.Code == ValidationCodeDuplicateGroupCPU || issue.Code == ValidationCodeIneffectiveGroupCPU {
		fmt.Fprintf(&text, " cpu=%d", issue.CPU)
	}
	if len(issue.Cycle) != 0 {
		fmt.Fprintf(&text, " cycle=%v", issue.Cycle)
	}
	return text.String()
}

// Validate analyzes the current builder without invoking callbacks or mutating it.
func (b *Builder) Validate() (*ValidationReport, error) {
	if b == nil || b.state == builderInvalid || b.owner == nil {
		return nil, newValidationError([]ValidationIssue{{Code: ValidationCodeInvalidBuilder}})
	}

	report := &ValidationReport{}
	if len(b.nodes) == 0 {
		report.issues = []ValidationIssue{{Code: ValidationCodeEmptyGraph}}
		return report, newValidationError(report.issues)
	}

	nodes, metadata := validationNodes(b.nodes)
	forward, reverse, inputEdges, outputEdges := validationAdjacency(nodes, b.edges, b.inputs, b.outputs)
	report.forward = forward
	report.reverse = reverse

	issues := make([]ValidationIssue, 0)
	fanInCounts := make(map[endpointID]int)
	broadcastOutputCounts := make(map[endpointID]int)
	shardOutputCounts := make(map[endpointID]int)
	for _, edge := range b.edges {
		if edge.fanInMerge != 0 {
			fanInCounts[edge.target]++
		}
		if edge.broadcastGroup != 0 {
			broadcastOutputCounts[edge.source]++
		}
		if edge.shardGroup != 0 {
			shardOutputCounts[edge.source]++
		}
	}
	for nodeID, group := range b.placements {
		if group == InvalidGroupID || nodeID == InvalidNodeID || int(nodeID) > len(b.nodes) || b.nodes[nodeID-1].nodeID != nodeID {
			issues = append(issues, ValidationIssue{Code: ValidationCodeInvalidGroupPlacement, NodeID: nodeID})
		}
	}
	for _, nodeID := range nodes {
		metadata := metadata[nodeID]
		issues = append(issues, staticValidationIssues(nodeID, metadata.validation, metadata.inputCount)...)
		for portID := PortID(1); portID <= PortID(metadata.inputCount); portID++ {
			ep := endpointID{nodeID: nodeID, portID: portID}
			expected := 1
			if fanInCounts[ep] > 0 {
				expected = fanInCounts[ep]
			}
			if inputEdges[ep] != expected {
				issues = append(issues, ValidationIssue{Code: ValidationCodeUnconnectedInput, NodeID: nodeID, PortID: portID})
			}
		}
		for portID := PortID(1); portID <= PortID(metadata.outputCount); portID++ {
			ep := endpointID{nodeID: nodeID, portID: portID}
			expected := 1
			if broadcastOutputCounts[ep]+shardOutputCounts[ep] > 0 {
				expected = broadcastOutputCounts[ep] + shardOutputCounts[ep]
			}
			if outputEdges[ep] != expected {
				issues = append(issues, ValidationIssue{Code: ValidationCodeUnconnectedOutput, NodeID: nodeID, PortID: portID})
			}
		}
	}
	for _, edge := range b.edges {
		output := b.outputs[edge.source]
		if output == nil {
			continue
		}
		if err := validateEdgeConfig(edge.config); err != nil {
			var configError *EdgeConfigError
			if !errors.As(err, &configError) {
				continue
			}
			if configError.capacityInvalid || configError.batchCapacityInvalid {
				issues = append(issues, ValidationIssue{Code: ValidationCodeInvalidEdgeCapacity, EdgeID: edge.id})
			}
			if configError.policyInvalid {
				issues = append(issues, ValidationIssue{Code: ValidationCodeInvalidEdgePolicy, EdgeID: edge.id})
			}
		}
	}
	for _, edge := range b.edges {
		if !edge.hasFactory {
			continue
		}
		sourceGroup := b.placements[edge.source.nodeID]
		if sourceGroup == InvalidGroupID {
			sourceGroup = DefaultGroupID
		}
		targetGroup := b.placements[edge.target.nodeID]
		if targetGroup == InvalidGroupID {
			targetGroup = DefaultGroupID
		}
		if sourceGroup != targetGroup {
			issues = append(issues, ValidationIssue{Code: ValidationCodeRemoteQueueFactory, EdgeID: edge.id})
		}
	}
	effective := effectiveGroups(b.nodes, b.placements)
	issues = append(issues, cpuBindingIssues(effective, b.cpuBindings)...)

	order, residual := validationKahn(nodes, forward)
	if len(residual) == 0 {
		report.order = order
	} else {
		issues = append(issues, ValidationIssue{Code: ValidationCodeCycle, Cycle: validationCycle(nodes, forward, residual)})
	}
	fromSource := validationReachable(nodes, forward, metadata, true)
	toSink := validationReachable(nodes, reverse, metadata, false)
	for _, nodeID := range nodes {
		if !fromSource[nodeID] {
			issues = append(issues, ValidationIssue{Code: ValidationCodeUnreachableFromSource, NodeID: nodeID})
		}
		if !toSink[nodeID] {
			issues = append(issues, ValidationIssue{Code: ValidationCodeCannotReachSink, NodeID: nodeID})
		}
	}

	sortValidationIssues(issues)
	report.issues = copyValidationIssues(issues)
	if len(issues) == 0 {
		return report, nil
	}
	return report, newValidationError(issues)
}

func effectiveGroups(nodes []nodeMetadata, placements map[NodeID]GroupID) map[GroupID]bool {
	groups := make(map[GroupID]bool, len(nodes))
	for _, metadata := range nodes {
		group := placements[metadata.nodeID]
		if group == InvalidGroupID {
			group = DefaultGroupID
		}
		groups[group] = true
	}
	return groups
}

func cpuBindingIssues(effective map[GroupID]bool, bindings map[GroupID]int) []ValidationIssue {
	groups := make([]GroupID, 0, len(effective))
	for group := range effective {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(left, right int) bool { return groups[left] < groups[right] })
	issues := make([]ValidationIssue, 0)
	for _, group := range groups {
		if _, exists := bindings[group]; !exists {
			issues = append(issues, ValidationIssue{Code: ValidationCodeMissingGroupCPU, GroupID: group})
		}
	}

	boundGroups := make([]GroupID, 0, len(bindings))
	for group := range bindings {
		boundGroups = append(boundGroups, group)
	}
	sort.Slice(boundGroups, func(left, right int) bool { return boundGroups[left] < boundGroups[right] })
	seen := make(map[int]GroupID, len(boundGroups))
	for _, group := range boundGroups {
		cpu := bindings[group]
		if !effective[group] {
			issues = append(issues, ValidationIssue{Code: ValidationCodeIneffectiveGroupCPU, GroupID: group, CPU: cpu})
			continue
		}
		if cpu < 0 {
			issues = append(issues, ValidationIssue{Code: ValidationCodeInvalidGroupCPU, GroupID: group, CPU: cpu})
			continue
		}
		if _, exists := seen[cpu]; exists {
			issues = append(issues, ValidationIssue{Code: ValidationCodeDuplicateGroupCPU, GroupID: group, CPU: cpu})
			continue
		}
		seen[cpu] = group
	}
	return issues
}

func newValidationError(issues []ValidationIssue) *ValidationError {
	return &ValidationError{issues: copyValidationIssues(issues)}
}

func validationNodes(nodes []nodeMetadata) ([]NodeID, map[NodeID]nodeMetadata) {
	metadata := make(map[NodeID]nodeMetadata, len(nodes))
	for _, node := range nodes {
		metadata[node.nodeID] = node
	}
	ids := make([]NodeID, 0, len(metadata))
	for nodeID := range metadata {
		ids = append(ids, nodeID)
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	return ids, metadata
}

func validationAdjacency(nodes []NodeID, edges []edgeMetadata, inputs map[endpointID]*inputMetadata, outputs map[endpointID]*outputMetadata) ([]ValidationAdjacency, []ValidationAdjacency, map[endpointID]int, map[endpointID]int) {
	forward := make([]ValidationAdjacency, len(nodes))
	reverse := make([]ValidationAdjacency, len(nodes))
	known := make(map[NodeID]bool, len(nodes))
	for index, nodeID := range nodes {
		known[nodeID] = true
		forward[index].NodeID = nodeID
		reverse[index].NodeID = nodeID
	}
	forwardByNode := make(map[NodeID]int, len(nodes))
	reverseByNode := make(map[NodeID]int, len(nodes))
	for index, nodeID := range nodes {
		forwardByNode[nodeID] = index
		reverseByNode[nodeID] = index
	}
	inputEdges := make(map[endpointID]int)
	outputEdges := make(map[endpointID]int)
	for _, edge := range edges {
		output := outputs[edge.source]
		if output == nil || inputs[edge.target] == nil || !known[edge.source.nodeID] || !known[edge.target.nodeID] {
			continue
		}
		if !output.isError {
			outputEdges[edge.source]++
		}
		inputEdges[edge.target]++
		forward[forwardByNode[edge.source.nodeID]].Edges = append(forward[forwardByNode[edge.source.nodeID]].Edges, ValidationAdjacentEdge{EdgeID: edge.id, AdjacentNodeID: edge.target.nodeID})
		reverse[reverseByNode[edge.target.nodeID]].Edges = append(reverse[reverseByNode[edge.target.nodeID]].Edges, ValidationAdjacentEdge{EdgeID: edge.id, AdjacentNodeID: edge.source.nodeID})
	}
	for index := range forward {
		sort.Slice(forward[index].Edges, func(left, right int) bool {
			return adjacentEdgeLess(forward[index].Edges[left], forward[index].Edges[right])
		})
		sort.Slice(reverse[index].Edges, func(left, right int) bool {
			return adjacentEdgeLess(reverse[index].Edges[left], reverse[index].Edges[right])
		})
	}
	return forward, reverse, inputEdges, outputEdges
}

func adjacentEdgeLess(left, right ValidationAdjacentEdge) bool {
	if left.AdjacentNodeID != right.AdjacentNodeID {
		return left.AdjacentNodeID < right.AdjacentNodeID
	}
	return left.EdgeID < right.EdgeID
}

func staticValidationIssues(nodeID NodeID, validation staticValidationDescriptor, inputCount uint8) []ValidationIssue {
	missing := validation.required &^ validation.present
	issues := make([]ValidationIssue, 0, 10)
	if missing&factoryMask != 0 {
		issues = append(issues, ValidationIssue{Code: ValidationCodeMissingFactory, NodeID: nodeID})
	}
	if missing&selectorMask != 0 {
		issues = append(issues, ValidationIssue{Code: ValidationCodeMissingSelector, NodeID: nodeID})
	}
	for position, mask := range []uint16{handler0Mask, handler1Mask, handler2Mask, handler3Mask, handler4Mask, handler5Mask, handler6Mask, handler7Mask} {
		if missing&mask == 0 {
			continue
		}
		portID := InvalidPortID
		if inputCount != 0 {
			portID = PortID(position + 1)
		}
		issues = append(issues, ValidationIssue{Code: ValidationCodeMissingHandler, NodeID: nodeID, PortID: portID})
	}
	return issues
}

type nodeIDHeap []NodeID

func (h nodeIDHeap) Len() int                  { return len(h) }
func (h nodeIDHeap) Less(left, right int) bool { return h[left] < h[right] }
func (h nodeIDHeap) Swap(left, right int)      { h[left], h[right] = h[right], h[left] }
func (h *nodeIDHeap) Push(value any)           { *h = append(*h, value.(NodeID)) }
func (h *nodeIDHeap) Pop() any {
	old := *h
	last := len(old) - 1
	value := old[last]
	*h = old[:last]
	return value
}

func validationKahn(nodes []NodeID, forward []ValidationAdjacency) ([]NodeID, map[NodeID]bool) {
	indegree := make(map[NodeID]int, len(nodes))
	rows := make(map[NodeID][]ValidationAdjacentEdge, len(forward))
	for _, row := range forward {
		rows[row.NodeID] = row.Edges
		for _, edge := range row.Edges {
			indegree[edge.AdjacentNodeID]++
		}
	}
	ready := nodeIDHeap{}
	for _, nodeID := range nodes {
		if indegree[nodeID] == 0 {
			heap.Push(&ready, nodeID)
		}
	}
	order := make([]NodeID, 0, len(nodes))
	for ready.Len() != 0 {
		nodeID := heap.Pop(&ready).(NodeID)
		order = append(order, nodeID)
		for _, edge := range rows[nodeID] {
			indegree[edge.AdjacentNodeID]--
			if indegree[edge.AdjacentNodeID] == 0 {
				heap.Push(&ready, edge.AdjacentNodeID)
			}
		}
	}
	if len(order) == len(nodes) {
		return order, nil
	}
	residual := make(map[NodeID]bool, len(nodes)-len(order))
	for _, nodeID := range nodes {
		if indegree[nodeID] != 0 {
			residual[nodeID] = true
		}
	}
	return nil, residual
}

func validationCycle(nodes []NodeID, forward []ValidationAdjacency, residual map[NodeID]bool) []NodeID {
	rows := make(map[NodeID][]ValidationAdjacentEdge, len(forward))
	for _, row := range forward {
		rows[row.NodeID] = row.Edges
	}
	colors := make(map[NodeID]uint8, len(residual))
	stack := make([]NodeID, 0, len(residual))
	positions := make(map[NodeID]int, len(residual))
	var visit func(NodeID) []NodeID
	visit = func(nodeID NodeID) []NodeID {
		colors[nodeID] = 1
		positions[nodeID] = len(stack)
		stack = append(stack, nodeID)
		for _, edge := range rows[nodeID] {
			next := edge.AdjacentNodeID
			if !residual[next] {
				continue
			}
			switch colors[next] {
			case 0:
				if cycle := visit(next); cycle != nil {
					return cycle
				}
			case 1:
				cycle := append([]NodeID(nil), stack[positions[next]:]...)
				return append(cycle, next)
			}
		}
		stack = stack[:len(stack)-1]
		colors[nodeID] = 2
		return nil
	}
	for _, nodeID := range nodes {
		if residual[nodeID] && colors[nodeID] == 0 {
			if cycle := visit(nodeID); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

func validationReachable(nodes []NodeID, adjacency []ValidationAdjacency, metadata map[NodeID]nodeMetadata, source bool) map[NodeID]bool {
	rows := make(map[NodeID][]ValidationAdjacentEdge, len(adjacency))
	for _, row := range adjacency {
		rows[row.NodeID] = row.Edges
	}
	reachable := make(map[NodeID]bool, len(nodes))
	queue := make([]NodeID, 0, len(nodes))
	for _, nodeID := range nodes {
		count := metadata[nodeID].inputCount
		if !source {
			count = metadata[nodeID].outputCount
		}
		if count == 0 {
			reachable[nodeID] = true
			queue = append(queue, nodeID)
		}
	}
	for index := 0; index < len(queue); index++ {
		for _, edge := range rows[queue[index]] {
			if !reachable[edge.AdjacentNodeID] {
				reachable[edge.AdjacentNodeID] = true
				queue = append(queue, edge.AdjacentNodeID)
			}
		}
	}
	return reachable
}

func sortValidationIssues(issues []ValidationIssue) {
	sort.Slice(issues, func(left, right int) bool {
		return compareValidationIssues(issues[left], issues[right]) < 0
	})
}

func compareValidationIssues(left, right ValidationIssue) int {
	if left.Code != right.Code {
		return compareUint(uint64(left.Code), uint64(right.Code))
	}
	if left.NodeID != right.NodeID {
		return compareUint(uint64(left.NodeID), uint64(right.NodeID))
	}
	if left.PortID != right.PortID {
		return compareUint(uint64(left.PortID), uint64(right.PortID))
	}
	if left.EdgeID != right.EdgeID {
		return compareUint(uint64(left.EdgeID), uint64(right.EdgeID))
	}
	if left.GroupID != right.GroupID {
		return compareUint(uint64(left.GroupID), uint64(right.GroupID))
	}
	if left.CPU != right.CPU {
		if left.CPU < right.CPU {
			return -1
		}
		return 1
	}
	for index := 0; index < len(left.Cycle) && index < len(right.Cycle); index++ {
		if left.Cycle[index] != right.Cycle[index] {
			return compareUint(uint64(left.Cycle[index]), uint64(right.Cycle[index]))
		}
	}
	return compareUint(uint64(len(left.Cycle)), uint64(len(right.Cycle)))
}

func compareUint(left, right uint64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func copyValidationIssues(issues []ValidationIssue) []ValidationIssue {
	copyIssues := append([]ValidationIssue(nil), issues...)
	for index := range copyIssues {
		copyIssues[index].Cycle = append([]NodeID(nil), copyIssues[index].Cycle...)
	}
	return copyIssues
}

func copyValidationAdjacency(adjacency []ValidationAdjacency) []ValidationAdjacency {
	copyAdjacency := append([]ValidationAdjacency(nil), adjacency...)
	for index := range copyAdjacency {
		copyAdjacency[index].Edges = append([]ValidationAdjacentEdge(nil), copyAdjacency[index].Edges...)
	}
	return copyAdjacency
}
