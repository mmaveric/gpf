package graph

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func TestMinimizeUnconstrainedDAGAndDeterminism(t *testing.T) {
	plan := PlanningConstraints{Nodes: []NodeConstraint{{NodeID: 3}, {NodeID: 1}, {NodeID: 2}}, Edges: []EdgeConstraint{{EdgeID: 2, SourceNodeID: 2, TargetNodeID: 3}, {EdgeID: 1, SourceNodeID: 1, TargetNodeID: 2}}}
	first, err := MinimizeRemoteEdges(plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MinimizeRemoteEdges(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || first.RemoteEdgesBefore != 2 || first.RemoteEdgesAfter != 0 {
		t.Fatalf("deterministic result=%+v second=%+v", first, second)
	}
	if first.Assignments[0].NodeID != 1 || first.Assignments[0].GroupID != first.Assignments[1].GroupID || first.Assignments[1].GroupID != first.Assignments[2].GroupID {
		t.Fatalf("assignments=%+v", first.Assignments)
	}
}

func TestMinimizePreservesManualLabelsAndMandatoryCuts(t *testing.T) {
	plan := PlanningConstraints{Nodes: []NodeConstraint{{NodeID: 1, ExplicitGroup: 8}, {NodeID: 2}, {NodeID: 3, ExplicitGroup: 9}}, Edges: []EdgeConstraint{{EdgeID: 1, SourceNodeID: 1, TargetNodeID: 2}, {EdgeID: 2, SourceNodeID: 2, TargetNodeID: 3, MustSeparate: true}}}
	result, err := MinimizeRemoteEdges(plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Assignments[0].GroupID != 8 || result.Assignments[2].GroupID != 9 || result.Assignments[0].GroupID == result.Assignments[2].GroupID || result.RemoteEdgesAfter != 1 {
		t.Fatalf("manual labels/cut changed: %+v", result)
	}
	conflict := PlanningConstraints{Nodes: []NodeConstraint{{NodeID: 1}, {NodeID: 2}}, Edges: []EdgeConstraint{{EdgeID: 1, SourceNodeID: 1, TargetNodeID: 2, MustLink: true, MustSeparate: true}}}
	if _, err := MinimizeRemoteEdges(conflict); !errors.Is(err, ErrPlanningConstraintConflict) {
		t.Fatalf("conflict error=%v", err)
	}
}

func TestMinimizeRejectsCyclicQuotientContraction(t *testing.T) {
	plan := PlanningConstraints{Nodes: []NodeConstraint{{NodeID: 1}, {NodeID: 2}, {NodeID: 3}}, Edges: []EdgeConstraint{{EdgeID: 1, SourceNodeID: 1, TargetNodeID: 3}, {EdgeID: 2, SourceNodeID: 1, TargetNodeID: 2}, {EdgeID: 3, SourceNodeID: 2, TargetNodeID: 3}}}
	result, err := MinimizeRemoteEdges(plan)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rejection := range result.Rejections {
		if rejection.EdgeID == 1 && rejection.Reason == "merge creates a cyclic quotient" {
			found = true
		}
	}
	if !found || result.RemoteEdgesAfter != 0 {
		t.Fatalf("shortcut contraction result=%+v", result)
	}
}

func TestMinimizeGeneratedDAGProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(19))
	for trial := 0; trial < 100; trial++ {
		const n = 12
		plan := PlanningConstraints{}
		for id := 1; id <= n; id++ {
			plan.Nodes = append(plan.Nodes, NodeConstraint{NodeID: NodeID(id)})
		}
		edgeID := EdgeID(1)
		for source := 1; source < n; source++ {
			for target := source + 1; target <= n; target++ {
				if rng.Intn(4) == 0 {
					plan.Edges = append(plan.Edges, EdgeConstraint{EdgeID: edgeID, SourceNodeID: NodeID(source), TargetNodeID: NodeID(target)})
					edgeID++
				}
			}
		}
		first, err := MinimizeRemoteEdges(plan)
		if err != nil {
			t.Fatal(err)
		}
		second, err := MinimizeRemoteEdges(plan)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, second) || first.RemoteEdgesAfter > first.RemoteEdgesBefore {
			t.Fatalf("trial %d non-deterministic or increased cost", trial)
		}
		if len(first.Assignments) != n {
			t.Fatalf("trial %d assignment count=%d", trial, len(first.Assignments))
		}
		if len(plan.Edges) > 0 && first.RemoteEdgesAfter != 0 {
			t.Fatalf("trial %d unconstrained DAG retained %d remote edges: edges=%+v result=%+v", trial, first.RemoteEdgesAfter, plan.Edges, first)
		}
	}
}
