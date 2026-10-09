package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/asharipov/gpf/graph"
)

func TestBuildShowcaseQueuePlans(t *testing.T) {
	compiled, err := buildShowcase(&showcaseStats{})
	if err != nil {
		t.Fatal(err)
	}
	plans := compiled.definition.EdgeQueuePlans()
	byID := make(map[graph.EdgeID]graph.EdgeQueuePlan, len(plans))
	for _, plan := range plans {
		byID[plan.EdgeID] = plan
	}

	if len(compiled.broadcastEdges) != 2 {
		t.Fatalf("broadcast branches=%d, want 2", len(compiled.broadcastEdges))
	}
	broadcastModels := map[graph.QueueModel]bool{}
	for _, id := range compiled.broadcastEdges {
		plan, ok := byID[id]
		if !ok || plan.BroadcastID != compiled.broadcastEdges[0] {
			t.Fatalf("broadcast branch plan missing identity: %+v", plan)
		}
		broadcastModels[plan.Model] = true
	}
	if !broadcastModels[graph.QueueModelLocal] || !broadcastModels[graph.QueueModelSPSC] {
		t.Fatalf("broadcast models=%v, want local and SPSC", broadcastModels)
	}

	if len(compiled.shardEdges) != len(compiled.partitionNodes) {
		t.Fatalf("shard branches=%d, want %d", len(compiled.shardEdges), len(compiled.partitionNodes))
	}
	for index, id := range compiled.shardEdges {
		plan, ok := byID[id]
		if !ok || plan.ShardID != compiled.shardEdges[0] {
			t.Fatalf("shard branch %d has incorrect identity: %+v", index, plan)
		}
		if plan.Model != graph.QueueModelSPSC || !plan.Remote {
			t.Fatalf("shard branch %d model=%s remote=%t, want remote SPSC", index, plan.Model, plan.Remote)
		}
	}

	if len(compiled.fanInEdges) != 3 {
		t.Fatalf("fan-in branches=%d, want 3", len(compiled.fanInEdges))
	}
	for _, id := range compiled.fanInEdges {
		plan, ok := byID[id]
		if !ok || plan.MergeID != compiled.fanInEdges[0] || plan.Model != graph.QueueModelMPSC {
			t.Fatalf("fan-in branch %d has incorrect merge plan: %+v", id, plan)
		}
	}
}

func TestRunShowcaseProcessesValidOrdersAndReportsLifecycle(t *testing.T) {
	var output bytes.Buffer
	input := sampleOrders()
	if err := Run(&output, input); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{
		"orders submitted=5 processed=4 rejected=1 audited=4 units=7 gross_cents=1600 partitions=1,2,1",
		"queue_drop_elements=0 pool_drop_elements=0 logged_errors=1 pools_in_use=0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("showcase output missing %q:\n%s", want, text)
		}
	}
}
