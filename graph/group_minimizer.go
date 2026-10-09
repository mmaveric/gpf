package graph

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrInvalidConstraintPlan = errors.New("invalid automatic grouping constraint plan")
	ErrCyclicConstraintPlan  = errors.New("constraint plan quotient is cyclic")
)

// GroupAssignment maps one physical node identity to a proposed plan group.
type GroupAssignment struct {
	NodeID  NodeID
	GroupID GroupID
}

// MergeRejection explains why one canonical adjacent merge was skipped.
type MergeRejection struct {
	EdgeID EdgeID
	Reason string
}

// GroupingResult is a deterministic local-minimum grouping preview.
type GroupingResult struct {
	Assignments       []GroupAssignment
	RemoteEdgesBefore uint32
	RemoteEdgesAfter  uint32
	Rejections        []MergeRejection
}

type constraintSet struct {
	parent  []int
	key     []NodeID
	labels  []GroupID
	members [][]int
}

func (s *constraintSet) root(index int) int {
	for s.parent[index] != index {
		s.parent[index] = s.parent[s.parent[index]]
		index = s.parent[index]
	}
	return index
}

func (s *constraintSet) join(left, right int) int {
	a, b := s.root(left), s.root(right)
	if a == b {
		return a
	}
	if s.key[b] < s.key[a] {
		a, b = b, a
	}
	s.parent[b] = a
	if s.labels[a] == 0 {
		s.labels[a] = s.labels[b]
	}
	s.members[a] = append(s.members[a], s.members[b]...)
	return a
}

// MinimizeRemoteEdges computes a deterministic constrained local minimum. It
// performs no Build, factory, queue, affinity, or runtime operation and does not
// promise a globally optimal graph partition.
func MinimizeRemoteEdges(input PlanningConstraints) (GroupingResult, error) {
	if len(input.Nodes) == 0 && len(input.Edges) != 0 {
		return GroupingResult{}, ErrInvalidConstraintPlan
	}
	nodes := append([]NodeConstraint(nil), input.Nodes...)
	edges := append([]EdgeConstraint(nil), input.Edges...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].NodeID < nodes[j].NodeID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].EdgeID < edges[j].EdgeID })
	index := make(map[NodeID]int, len(nodes))
	set := constraintSet{parent: make([]int, len(nodes)), key: make([]NodeID, len(nodes)), labels: make([]GroupID, len(nodes)), members: make([][]int, len(nodes))}
	labelOwner := make(map[GroupID]int)
	for i, n := range nodes {
		if n.NodeID == 0 {
			return GroupingResult{}, ErrInvalidConstraintPlan
		}
		if _, exists := index[n.NodeID]; exists {
			return GroupingResult{}, ErrInvalidConstraintPlan
		}
		index[n.NodeID] = i
		set.parent[i] = i
		set.key[i] = n.NodeID
		set.members[i] = []int{i}
		hardGroup := n.ExplicitGroup
		if n.FixedGroup && n.PhysicalGroup != InvalidGroupID {
			hardGroup = n.PhysicalGroup
		}
		set.labels[i] = hardGroup
		if hardGroup != 0 {
			if prior, ok := labelOwner[hardGroup]; ok {
				set.join(prior, i)
			} else {
				labelOwner[hardGroup] = i
			}
		}
	}
	for i, e := range edges {
		if i > 0 && edges[i-1].EdgeID == e.EdgeID {
			return GroupingResult{}, ErrInvalidConstraintPlan
		}
		if _, ok := index[e.SourceNodeID]; !ok {
			return GroupingResult{}, ErrInvalidConstraintPlan
		}
		if _, ok := index[e.TargetNodeID]; !ok {
			return GroupingResult{}, ErrInvalidConstraintPlan
		}
		if e.MustLink && e.MustSeparate {
			return GroupingResult{}, ErrPlanningConstraintConflict
		}
	}
	separate := make([][2]int, 0)
	for _, e := range edges {
		a, b := index[e.SourceNodeID], index[e.TargetNodeID]
		if e.MustSeparate {
			separate = append(separate, [2]int{a, b})
			continue
		}
		if e.MustLink {
			la, lb := nodes[a].ExplicitGroup, nodes[b].ExplicitGroup
			if la != 0 && lb != 0 && la != lb {
				return GroupingResult{}, ErrPlanningConstraintConflict
			}
			set.join(a, b)
		}
	}
	for _, pair := range separate {
		if set.root(pair[0]) == set.root(pair[1]) {
			return GroupingResult{}, ErrPlanningConstraintConflict
		}
	}
	canJoin := func(left, right int) bool {
		a, b := set.root(left), set.root(right)
		if a == b {
			return true
		}
		if set.labels[a] != 0 && set.labels[b] != 0 && set.labels[a] != set.labels[b] {
			return false
		}
		for _, pair := range separate {
			x, y := set.root(pair[0]), set.root(pair[1])
			if (x == a && y == b) || (x == b && y == a) {
				return false
			}
		}
		return true
	}
	acyclicAfterJoin := func(left, right int) bool {
		a, b := set.root(left), set.root(right)
		find := func(value int) int {
			root := set.root(value)
			if root == b {
				return a
			}
			return root
		}
		adj := make([][]int, len(nodes))
		indegree := make([]int, len(nodes))
		for _, edge := range edges {
			x, y := find(index[edge.SourceNodeID]), find(index[edge.TargetNodeID])
			if x == y {
				continue
			}
			// Count a quotient arc once even if several original edges connect it.
			duplicate := false
			for _, prior := range adj[x] {
				if prior == y {
					duplicate = true
					break
				}
			}
			if !duplicate {
				adj[x] = append(adj[x], y)
				indegree[y]++
			}
		}
		queue := make([]int, 0, len(nodes))
		active := make(map[int]bool, len(nodes))
		for i := range nodes {
			root := find(i)
			if root == i {
				active[root] = true
				if indegree[root] == 0 {
					queue = append(queue, root)
				}
			}
		}
		visited := 0
		for len(queue) > 0 {
			x := queue[0]
			queue = queue[1:]
			visited++
			for _, y := range adj[x] {
				indegree[y]--
				if indegree[y] == 0 {
					queue = append(queue, y)
				}
			}
		}
		return visited == len(active)
	}
	if len(nodes) > 0 && !acyclicAfterJoin(0, 0) {
		return GroupingResult{}, ErrCyclicConstraintPlan
	}
	remoteCount := func() uint32 {
		var count uint32
		for _, edge := range edges {
			if set.root(index[edge.SourceNodeID]) != set.root(index[edge.TargetNodeID]) {
				count++
			}
		}
		return count
	}
	result := GroupingResult{RemoteEdgesBefore: remoteCount()}
	rejected := make(map[EdgeID]string)
	for {
		changed := false
		for _, edge := range edges {
			a, b := index[edge.SourceNodeID], index[edge.TargetNodeID]
			if set.root(a) == set.root(b) || edge.MustSeparate {
				continue
			}
			if !canJoin(a, b) {
				rejected[edge.EdgeID] = "fixed label or mandatory cut"
				continue
			}
			if !acyclicAfterJoin(a, b) {
				rejected[edge.EdgeID] = "merge creates a cyclic quotient"
				continue
			}
			set.join(a, b)
			changed = true
		}
		if !changed {
			break
		}
	}
	result.RemoteEdgesAfter = remoteCount()
	for edgeID, reason := range rejected {
		result.Rejections = append(result.Rejections, MergeRejection{edgeID, reason})
	}
	sort.Slice(result.Rejections, func(i, j int) bool { return result.Rejections[i].EdgeID < result.Rejections[j].EdgeID })
	roots := make(map[int]GroupID)
	used := make(map[GroupID]bool)
	for i := range nodes {
		root := set.root(i)
		if set.labels[root] != 0 {
			used[set.labels[root]] = true
			roots[root] = set.labels[root]
		}
	}
	ordered := make([]int, 0)
	for i := range nodes {
		root := set.root(i)
		if root == i {
			ordered = append(ordered, root)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return set.key[ordered[i]] < set.key[ordered[j]] })
	candidate := GroupID(1)
	for _, root := range ordered {
		if roots[root] != 0 {
			continue
		}
		for used[candidate] || candidate == InvalidGroupID {
			candidate++
		}
		roots[root] = candidate
		used[candidate] = true
		candidate++
	}
	result.Assignments = make([]GroupAssignment, len(nodes))
	for i, n := range nodes {
		result.Assignments[i] = GroupAssignment{NodeID: n.NodeID, GroupID: roots[set.root(i)]}
	}
	if result.RemoteEdgesAfter > result.RemoteEdgesBefore {
		return GroupingResult{}, fmt.Errorf("remote edge count increased: %w", ErrInvalidConstraintPlan)
	}
	return result, nil
}
