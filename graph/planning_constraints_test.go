package graph

import (
	"errors"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

func TestPlanningConstraintsCopyExplicitAndWorkerBoundaries(t *testing.T) {
	b := NewBuilder()
	var factories int
	ingress, err := AddIngress[int](b, IngressConfig{Capacity: 4, BatchCapacity: 1, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	first, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{Factory: func() (struct{}, error) { factories++; return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{Factory: func() (struct{}, error) { factories++; return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} }}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, first.NodeID(), 5); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, ingress.Output(), first.Inputs().In1, EdgeConfig[int]{Capacity: 4, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(first.Outputs()).Out1, second.Inputs().In1, EdgeConfig[int]{Capacity: 4, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	bindCPU(t, b, 5, 1)
	if err := SetNodeWorkerPool(b, first.NodeID(), 2); err != nil {
		t.Fatal(err)
	}
	if err := BindNodePoolCPU(b, first.NodeID(), 0, 2); err != nil {
		t.Fatal(err)
	}
	if err := BindNodePoolCPU(b, first.NodeID(), 1, 3); err != nil {
		t.Fatal(err)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if factories != 0 {
		t.Fatal("Build invoked factories")
	}
	constraints := d.Constraints()
	if factories != 0 || len(constraints.Nodes) != 4 || len(constraints.Edges) != 2 {
		t.Fatalf("preview side effects/rows=%d/%+v", factories, constraints)
	}
	if constraints.Nodes[0].ExplicitGroup != 0 || constraints.Nodes[1].ExplicitGroup != 5 || constraints.Nodes[2].ExplicitGroup != 5 || constraints.Nodes[3].ExplicitGroup != 0 {
		t.Fatalf("explicit labels=%+v", constraints.Nodes)
	}
	if !constraints.Edges[0].MustSeparate || constraints.Edges[0].MustLink || !constraints.Edges[1].MustSeparate {
		t.Fatalf("worker/external boundaries=%+v", constraints.Edges)
	}
	want := d.Constraints()
	constraints.Nodes[1].ExplicitGroup = 99
	constraints.Edges[0].MustSeparate = false
	if got := d.Constraints(); !reflect.DeepEqual(got, want) {
		t.Fatal("constraint slices alias definition")
	}
}

func TestGroupReplicaConstraintsPairAndCut(t *testing.T) {
	b := NewBuilder()
	source, _ := AddNode0(b, validSource())
	sink, _ := AddNode1(b, validSink())
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	if err := ReplicateGroup(b, DefaultGroupID, 2); err != nil {
		t.Fatal(err)
	}
	if err := BindReplicaCPU(b, DefaultGroupID, 1, 1); err != nil {
		t.Fatal(err)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	constraints := d.Constraints()
	if len(constraints.Edges) != 2 || !constraints.Edges[0].MustLink || !constraints.Edges[1].MustLink || constraints.Edges[0].Replica == constraints.Edges[1].Replica {
		t.Fatalf("replica pairing=%+v", constraints.Edges)
	}
}

func TestAutomaticConstraintsDistinguishUnplacedAndManualLabels(t *testing.T) {
	makeBuilder := func(t *testing.T, automatic, placeDifferent bool) *Builder {
		t.Helper()
		b := NewBuilder()
		source, err := AddNode0(b, validSource())
		if err != nil {
			t.Fatal(err)
		}
		middle, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{Factory: func() (struct{}, error) { return struct{}{}, nil }, Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }}})
		if err != nil {
			t.Fatal(err)
		}
		sink, err := AddNode1(b, validSink())
		if err != nil {
			t.Fatal(err)
		}
		config := EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}
		if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, middle.Inputs().In1, config); err != nil {
			t.Fatal(err)
		}
		if _, err := Connect(b, OutputPorts1(middle.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
			t.Fatal(err)
		}
		if placeDifferent {
			if err := Place(b, source.NodeID(), 8); err != nil {
				t.Fatal(err)
			}
			if err := Place(b, sink.NodeID(), 9); err != nil {
				t.Fatal(err)
			}
		}
		bindCPU(t, b, DefaultGroupID, 0)
		if placeDifferent {
			bindCPU(t, b, 8, 1)
			bindCPU(t, b, 9, 2)
		}
		if automatic {
			if err := EnableAutomaticPlanning(b); err != nil {
				t.Fatal(err)
			}
		}
		return b
	}
	t.Run("manual-defaults-remain-fixed", func(t *testing.T) {
		d, err := makeBuilder(t, false, false).Build()
		if err != nil {
			t.Fatal(err)
		}
		p := d.Constraints()
		for _, n := range p.Nodes {
			if n.PhysicalGroup != DefaultGroupID {
				t.Fatalf("manual effective group=%+v", n)
			}
		}
		for _, e := range p.Edges {
			if !e.MustLink || e.MustSeparate {
				t.Fatalf("manual internal link=%+v", e)
			}
		}
	})
	t.Run("automatic-unplaced-free", func(t *testing.T) {
		d, err := makeBuilder(t, true, false).Build()
		if err != nil {
			t.Fatal(err)
		}
		p := d.Constraints()
		for _, n := range p.Nodes {
			if n.PhysicalGroup != InvalidGroupID || n.ExplicitGroup != 0 {
				t.Fatalf("unplaced node was frozen=%+v", n)
			}
		}
		for _, e := range p.Edges {
			if e.MustLink || e.MustSeparate {
				t.Fatalf("unconstrained edge=%+v", e)
			}
		}
	})
	t.Run("distinct-explicit-labels-cut", func(t *testing.T) {
		b := NewBuilder()
		source, _ := AddNode0(b, validSource())
		sink, _ := AddNode1(b, validSink())
		if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
			t.Fatal(err)
		}
		if err := Place(b, source.NodeID(), 8); err != nil {
			t.Fatal(err)
		}
		if err := Place(b, sink.NodeID(), 9); err != nil {
			t.Fatal(err)
		}
		bindCPU(t, b, 8, 0)
		bindCPU(t, b, 9, 1)
		if err := EnableAutomaticPlanning(b); err != nil {
			t.Fatal(err)
		}
		d, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		p := d.Constraints()
		if len(p.Edges) != 1 || !p.Edges[0].MustSeparate || p.Edges[0].MustLink {
			t.Fatalf("manual cuts=%+v", p.Edges)
		}
	})
}

func TestConflictingPlanningConstraintsReject(t *testing.T) {
	plan := PlanningConstraints{Edges: []EdgeConstraint{{EdgeID: 1, MustLink: true, MustSeparate: true}}}
	if err := validatePlanningConstraints(plan); !errors.Is(err, ErrPlanningConstraintConflict) {
		t.Fatalf("conflict error=%v", err)
	}
	if err := validatePlanningConstraints(PlanningConstraints{}); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticGroupPreviewIsStableCPUIncompleteAndDormant(t *testing.T) {
	b := NewBuilder()
	var factories atomic.Uint64
	source, err := AddNode0(b, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs1[int], error]]{
		Factory:  func() (struct{}, error) { factories.Add(1); return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs1[int], error]{Handle: func(*struct{}, *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }},
	})
	if err != nil {
		t.Fatal(err)
	}
	middle, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs1[int], error]]{
		Factory:  func() (struct{}, error) { factories.Add(1); return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs1[int], error]{Handle1: func(*struct{}, batch.Batch[int], *node.Outputs1[int]) node.Result[error] { return node.Result[error]{} }},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	config := EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, middle.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(middle.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	first, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Groups, second.Groups) || !reflect.DeepEqual(first.Nodes, second.Nodes) || !reflect.DeepEqual(first.Edges, second.Edges) || !reflect.DeepEqual(first.Constraints, second.Constraints) || !first.IsCurrent(b) || first.CPUComplete || factories.Load() != 0 {
		t.Fatalf("preview stability/dormancy=%+v", first)
	}
	if len(first.Groups) == 0 || len(first.Edges) != 2 || first.RemoteEdgesAfter > first.RemoteEdgesBefore {
		t.Fatalf("preview costs/groups=%+v", first)
	}
	first.Groups[0].GroupID = 99
	first.Nodes[0].GroupID = 99
	first.Edges[0].Model = QueueModelMPMC
	first.Constraints.Nodes[0].PhysicalGroup = 99
	third, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(third.Groups, second.Groups) || !reflect.DeepEqual(third.Nodes, second.Nodes) || !reflect.DeepEqual(third.Edges, second.Edges) || !reflect.DeepEqual(third.Constraints, second.Constraints) {
		t.Fatal("preview rows alias builder or later previews")
	}
	if err := Place(b, source.NodeID(), 8); err != nil {
		t.Fatal(err)
	}
	if err := second.ValidateCurrent(b); !errors.Is(err, ErrStaleAutomaticPlan) {
		t.Fatalf("placement did not stale preview: %v", err)
	}
}

func TestPlanGroupsOptInAndTopologyRevision(t *testing.T) {
	b := NewBuilder()
	source, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.PlanGroups(); !errors.Is(err, ErrAutomaticPlanningDisabled) {
		t.Fatal(err)
	}
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	plan, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if err := BindCPU(b, DefaultGroupID, 0); err != nil {
		t.Fatal(err)
	}
	if err := plan.ValidateCurrent(b); !errors.Is(err, ErrStaleAutomaticPlan) {
		t.Fatalf("binding change did not stale plan: %v", err)
	}
}

func TestPlanGroupsPreservesExplicitIDsAndReportsMissingCPUs(t *testing.T) {
	b := NewBuilder()
	source, err := AddNode0(b, validSource())
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, validSink())
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, source.NodeID(), 8); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	first, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Groups, second.Groups) || !reflect.DeepEqual(first.Nodes, second.Nodes) || !reflect.DeepEqual(first.Edges, second.Edges) || first.CPUComplete || len(first.Groups) != 2 || first.Groups[0].GroupID != 8 || first.Groups[1].GroupID != 9 {
		t.Fatalf("manual labels/CPU status=%+v", first)
	}
	if first.RemoteEdgesBefore != 1 || first.RemoteEdgesAfter != 1 || first.Edges[0].Model != QueueModelSPSC {
		t.Fatalf("edge preview=%+v", first.Edges)
	}
	if b.state != builderOpen {
		t.Fatal("preview sealed the builder")
	}
	if err := BindCPU(b, 8, 0); err != nil {
		t.Fatal(err)
	}
	if err := first.ValidateCurrent(b); !errors.Is(err, ErrStaleAutomaticPlan) {
		t.Fatalf("CPU binding did not invalidate preview: %v", err)
	}
}

func TestPlanGroupsRegistrationInvalidatesPreview(t *testing.T) {
	b := NewBuilder()
	connectValidPair(t, b)
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	plan, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddNode0(b, validIsolated()); err != nil {
		t.Fatal(err)
	}
	if err := plan.ValidateCurrent(b); !errors.Is(err, ErrStaleAutomaticPlan) {
		t.Fatalf("registration did not invalidate preview: %v", err)
	}
}

func TestFinalizeAutomaticPlanRequiresFreshCompleteUniqueCPUs(t *testing.T) {
	b := NewBuilder()
	source, _ := AddNode0(b, validSource())
	sink, _ := AddNode1(b, validSink())
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	preview, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if preview.CPUComplete || len(preview.Groups) != 1 {
		t.Fatalf("plan completeness=%+v", preview)
	}
	if _, err := FinalizeAutomaticPlan(b, preview); !errors.Is(err, ErrInvalidAutomaticCPUPlan) {
		t.Fatalf("missing CPUs error=%v", err)
	}
	group := preview.Groups[0].GroupID
	if err := BindAutomaticPlanCPU(b, &preview, group, -1); !errors.Is(err, ErrInvalidCPU) {
		t.Fatalf("negative CPU error=%v", err)
	}
	if err := BindAutomaticPlanCPU(b, &preview, group, 0); err != nil {
		t.Fatal(err)
	}
	if !preview.CPUComplete || !preview.Groups[0].CPUBound || preview.Groups[0].CPU != 0 {
		t.Fatalf("CPU binding status=%+v", preview.Groups)
	}
	if err := BindAutomaticPlanCPU(b, &preview, group, 1); !errors.Is(err, ErrInvalidAutomaticCPUPlan) {
		t.Fatalf("duplicate group CPU error=%v", err)
	}
	if err := BindAutomaticPlanCPU(b, &preview, 999, 2); !errors.Is(err, ErrInvalidAutomaticCPUPlan) {
		t.Fatalf("unknown group CPU error=%v", err)
	}
	frozen, err := FinalizeAutomaticPlan(b, preview)
	if err != nil {
		t.Fatal(err)
	}
	if !frozen.executionCPUComplete() {
		t.Fatal("finalized plan is not CPU complete")
	}
	if got, err := b.Build(); err != nil || got != frozen {
		t.Fatalf("Build did not return frozen plan: %p/%p %v", got, frozen, err)
	}
	if err := preview.ValidateCurrent(b); !errors.Is(err, ErrStaleAutomaticPlan) {
		t.Fatal("finalized preview remains current")
	}
}

func TestFinalizeAutomaticPlanRejectsForeignAndTopologyStalePreviews(t *testing.T) {
	makeBuilder := func(t *testing.T) (*Builder, AutomaticGroupPreview) {
		t.Helper()
		b := NewBuilder()
		connectValidPair(t, b)
		if err := EnableAutomaticPlanning(b); err != nil {
			t.Fatal(err)
		}
		p, err := b.PlanGroups()
		if err != nil {
			t.Fatal(err)
		}
		return b, p
	}
	b, p := makeBuilder(t)
	foreign, _ := makeBuilder(t)
	if _, err := FinalizeAutomaticPlan(foreign, p); !errors.Is(err, ErrStaleAutomaticPlan) {
		t.Fatalf("foreign preview=%v", err)
	}
	if err := Place(b, 1, 7); err != nil {
		t.Fatal(err)
	}
	if err := BindAutomaticPlanCPU(b, &p, p.Groups[0].GroupID, 0); !errors.Is(err, ErrStaleAutomaticPlan) {
		t.Fatalf("stale preview bind=%v", err)
	}
}

func TestAutomaticFinalizedPlanStartsWithChosenCPU(t *testing.T) {
	b := NewBuilder()
	source, _ := AddNode0(b, validSource())
	sink, _ := AddNode1(b, validSink())
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	p, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Groups) != 1 {
		t.Fatalf("groups=%+v", p.Groups)
	}
	setAffinityAdapterForTest(t, func() affinityAdapter {
		return testAffinity(func(int) error { return errors.New("automatic affinity rejection") })
	})
	if err := BindAutomaticPlanCPU(b, &p, p.Groups[0].GroupID, 3); err != nil {
		t.Fatal(err)
	}
	d, err := FinalizeAutomaticPlan(b, p)
	if err != nil {
		t.Fatal(err)
	}
	if d.NodeCount() != 2 || d.EdgeCount() != 1 {
		t.Fatalf("logical definition counts=%d/%d", d.NodeCount(), d.EdgeCount())
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	status := e.AffinityStatus()
	if len(status) != 1 || status[0].CPU != 3 || status[0].Outcome != AffinityOutcomeLockedThreadFallback {
		t.Fatalf("automatic affinity=%+v", status)
	}
}

func TestAutomaticFinalizedPlanSequentialLifecycleAndMovedInput(t *testing.T) {
	b := NewBuilder()
	var factories, closes, delivered atomic.Uint64
	type state struct{ emitted bool }
	source, err := AddNode0(b, node.Stateful[state, node.Node0[state, node.Outputs1[int], error]]{
		Factory: func() (state, error) { factories.Add(1); return state{}, nil },
		Handlers: node.Node0[state, node.Outputs1[int], error]{Handle: func(s *state, out *node.Outputs1[int]) node.Result[error] {
			if s.emitted {
				return node.Result[error]{}
			}
			v, ok := out.Out1.Acquire()
			if !ok {
				return node.Result[error]{}
			}
			v.Mutable()[0], v.Mutable()[1] = 17, 19
			v.SetLen(2)
			out.Out1.Commit(&v)
			s.emitted = true
			return node.Result[error]{}
		}},
		Close: func(*state) error { closes.Add(1); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { factories.Add(1); return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			if input.Len() != 2 || input.Items()[0] != 17 || input.Items()[1] != 19 {
				t.Error("automatic execution lost moved input")
			}
			delivered.Add(1)
			return node.Result[error]{}
		}},
		Close: func(*struct{}) error { closes.Add(1); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 2, BatchCapacity: 2, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	p, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Groups) != 1 {
		t.Fatalf("auto groups=%+v", p.Groups)
	}
	if err := BindAutomaticPlanCPU(b, &p, p.Groups[0].GroupID, 0); err != nil {
		t.Fatal(err)
	}
	d, err := FinalizeAutomaticPlan(b, p)
	if err != nil {
		t.Fatal(err)
	}
	if factories.Load() != 0 {
		t.Fatal("finalization invoked factories")
	}
	if got, err := b.Build(); err != nil || got != d {
		t.Fatalf("sealed Build=%p/%p %v", got, d, err)
	}
	for iteration := uint64(1); iteration <= 2; iteration++ {
		e, err := d.Start()
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for delivered.Load() < iteration {
			if time.Now().After(deadline) {
				_ = e.Close()
				t.Fatal("automatic handler delivery timed out")
			}
			runtime.Gosched()
		}
		if err := e.Drain(); err != nil {
			t.Fatal(err)
		}
	}
	if factories.Load() != 4 || closes.Load() != 4 {
		t.Fatalf("factory/close counts=%d/%d", factories.Load(), closes.Load())
	}
}

func TestFinalizeAutomaticPlanRejectsDuplicateCPUs(t *testing.T) {
	b := NewBuilder()
	source, _ := AddNode0(b, validSource())
	sink, _ := AddNode1(b, validSink())
	if _, err := Connect(b, OutputPorts1(source.Outputs()).Out1, sink.Inputs().In1, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, source.NodeID(), 8); err != nil {
		t.Fatal(err)
	}
	if err := Place(b, sink.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	if err := EnableAutomaticPlanning(b); err != nil {
		t.Fatal(err)
	}
	p, err := b.PlanGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Groups) != 2 {
		t.Fatalf("groups=%+v", p.Groups)
	}
	if err := BindAutomaticPlanCPU(b, &p, p.Groups[0].GroupID, 0); err != nil {
		t.Fatal(err)
	}
	if err := BindAutomaticPlanCPU(b, &p, p.Groups[1].GroupID, 0); !errors.Is(err, ErrCPUAlreadyBound) {
		t.Fatalf("duplicate CPU=%v", err)
	}
	if err := BindAutomaticPlanCPU(b, &p, p.Groups[1].GroupID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAutomaticPlan(b, p); err != nil {
		t.Fatal(err)
	}
}

func (d *Definition) executionCPUComplete() bool { return d != nil && d.execution != nil }
