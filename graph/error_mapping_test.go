package graph

import (
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/node"
)

func TestMappingStaysDormantWithoutLogger(t *testing.T) {
	for _, kind := range []string{"explicit", "closure"} {
		t.Run(kind, func(t *testing.T) {
			b := NewBuilder()
			var mappings atomic.Uint64
			mapper := node.ErrorMapper[int](func(value int) node.ErrorEvent { mappings.Add(1); return node.ErrorEvent{Code: uint32(value)} })
			if kind == "explicit" {
				_, err := AddNode0(b, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, int]]{
					Factory:  func() (struct{}, error) { return struct{}{}, nil },
					Handlers: node.Node0[struct{}, node.Outputs0, int]{MapError: mapper, Handle: func(*struct{}, *node.Outputs0) node.Result[int] { return node.Result[int]{Failed: true, Value: 9} }},
				})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				_, err := AddClosureNode0(b, func() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, int]], error) {
					return node.ClosureReplica[node.ClosureNode0[node.Outputs0, int]]{Handlers: node.ClosureNode0[node.Outputs0, int]{MapError: mapper, Handle: func(*node.Outputs0) node.Result[int] { return node.Result[int]{Failed: true, Value: 9} }}}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			bindCPU(t, b, DefaultGroupID, 0)
			d, err := b.Build()
			if err != nil {
				t.Fatal(err)
			}
			if mappings.Load() != 0 {
				t.Fatal("Build called mapper")
			}
			e, err := d.Start()
			if err != nil {
				t.Fatal(err)
			}
			if err := e.Close(); err != nil {
				t.Fatal(err)
			}
			if mappings.Load() != 0 {
				t.Fatal("runtime without logger called mapper")
			}
		})
	}
}

func TestGeneratedMappingRunsOnceRegardlessOfErrorRoute(t *testing.T) {
	var mappings, events int
	var last node.ErrorEvent
	target := &normalizationTarget{edges: &edgeBindingTarget{}, currentGroup: 7, eventSink: func(group GroupID) func(node.ErrorEvent) bool {
		if group != 7 {
			t.Fatalf("sink group = %d", group)
		}
		return func(event node.ErrorEvent) bool { events++; last = event; return false }
	}}
	definition := node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, int]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, int]{
			MapError: func(value int) node.ErrorEvent {
				mappings++
				return node.ErrorEvent{NodeID: 999, GroupID: 999, Replica: 999, Worker: 999, Code: uint32(value), Attributes: [4]uint64{12}}
			},
			Handle: func(*struct{}, *node.Outputs0) node.Result[int] { return node.Result[int]{Failed: true, Value: 42} },
		},
	}
	if err := normalizeExplicit0(target, 3, definition); err != nil {
		t.Fatal(err)
	}
	if mappings != 0 {
		t.Fatal("normalization called mapper")
	}
	target.runners[0].run()
	if mappings != 1 || events != 1 || last.NodeID != 3 || last.GroupID != 7 || last.Replica != 0 || last.Worker != 0 || last.Code != 42 || last.Attributes[0] != 12 {
		t.Fatalf("mapping/event counts=%d/%d last=%+v", mappings, events, last)
	}
}

func TestBoundMapperUpdatesAllocateNothing(t *testing.T) {
	var last node.ErrorEvent
	target := &normalizationTarget{currentGroup: 9, eventSink: func(GroupID) func(node.ErrorEvent) bool {
		return func(event node.ErrorEvent) bool { last = event; return true }
	}}
	mapError := bindErrorMapper(target, 5, node.ErrorMapper[int](func(value int) node.ErrorEvent { return node.ErrorEvent{Code: uint32(value)} }))
	if allocations := testing.AllocsPerRun(100, func() { mapError(23) }); allocations != 0 {
		t.Fatalf("allocations = %v", allocations)
	}
	if last.Code != 23 || last.NodeID != 5 || last.GroupID != 9 {
		t.Fatalf("mapped identity = %+v", last)
	}
}
