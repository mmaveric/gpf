package graph

import (
	"sync/atomic"
	"testing"

	"github.com/asharipov/gpf/node"
)

func TestCustomClockConfigDormantAtBuildAndSnapshot(t *testing.T) {
	for _, mode := range []TimingMode{TimingDisabled, TimingFull, TimingSampled} {
		name := "disabled"
		if mode == TimingFull {
			name = "full"
		}
		if mode == TimingSampled {
			name = "sampled"
		}
		t.Run(name, func(t *testing.T) {
			var reads atomic.Uint64
			b := NewBuilder()
			_, err := AddNode0(b, validIsolated())
			if err != nil {
				t.Fatal(err)
			}
			config := TimingConfig{Mode: mode, Clock: func() uint64 { return reads.Add(10) }}
			if mode == TimingSampled {
				config.Every = 3
			}
			if err := SetTiming(b, config); err != nil {
				t.Fatal(err)
			}
			bindCPU(t, b, DefaultGroupID, 0)
			d, err := b.Build()
			if err != nil {
				t.Fatal(err)
			}
			if reads.Load() != 0 {
				t.Fatal("Build called custom clock")
			}
			e, err := d.Start()
			if err != nil {
				t.Fatal(err)
			}
			if err := e.Drain(); err != nil {
				t.Fatal(err)
			}
			before := reads.Load()
			got := e.Metrics().Nodes[0]
			if reads.Load() != before || before/10 != got.MeasuredCalls*2 {
				t.Fatalf("clock reads/duration counts = %d/%+v", before/10, got)
			}
			if mode == TimingDisabled && before != 0 {
				t.Fatal("disabled clock was invoked")
			}
		})
	}
}

func TestCustomClockFortyNanosecondsAndWrap(t *testing.T) {
	for _, values := range []struct {
		name       string
		start, end uint64
		want       uint64
	}{
		{"forty", 100, 140, 40}, {"modular-wrap", ^uint64(0) - 9, 15, 25},
	} {
		t.Run(values.name, func(t *testing.T) {
			var reads int
			target, _ := timedSource(t, 1, func() uint64 {
				reads++
				if reads%2 == 1 {
					return values.start
				}
				return values.end
			})
			target.runners[0].run()
			got := (&Executor{metrics: target.metrics}).Metrics().Nodes[0]
			if got.MeasuredCalls != 1 || got.MeasuredNanoseconds != values.want || got.MaxMeasuredNanoseconds != values.want || reads != 2 {
				t.Fatalf("clock outcome = %+v reads=%d", got, reads)
			}
		})
	}
}

func TestCustomClockSharedAcrossGroups(t *testing.T) {
	b := NewBuilder()
	_, err := AddNode0(b, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddNode0(b, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, error]]{
		Factory:  func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, error]{Handle: func(*struct{}, *node.Outputs0) node.Result[error] { return node.Result[error]{} }},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(b, second.NodeID(), 7); err != nil {
		t.Fatal(err)
	}
	var reads atomic.Uint64
	if err := SetTiming(b, TimingConfig{Mode: TimingFull, Clock: func() uint64 { return reads.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	bindCPU(t, b, 7, 1)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	var measured uint64
	for _, row := range e.Metrics().Nodes {
		measured += row.MeasuredCalls
	}
	if reads.Load() != measured*2 {
		t.Fatalf("shared clock reads = %d, measured=%d", reads.Load(), measured)
	}
}
