package graph

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/mmaveric/gpf/node"
	"github.com/loov/hrtime"
)

func TestTimingConfiguration(t *testing.T) {
	for _, config := range []TimingConfig{{Mode: TimingSampled}, {Mode: 99}, {Mode: TimingFull, Every: 3}, {Mode: TimingDisabled, Every: 2}, {ClockSource: TimingClockSource(99)}} {
		if err := SetTiming(NewBuilder(), config); !errors.Is(err, ErrInvalidTiming) {
			t.Fatalf("invalid config %+v: %v", config, err)
		}
	}
	b := NewBuilder()
	if err := SetTiming(b, TimingConfig{Mode: TimingFull}); err != nil {
		t.Fatal(err)
	}
	if err := SetTiming(b, TimingConfig{}); !errors.Is(err, ErrTimingConfigured) {
		t.Fatal(err)
	}
	if err := SetTiming(nil, TimingConfig{}); !errors.Is(err, ErrInvalidBuilder) {
		t.Fatal(err)
	}
}

func TestTimingClockSources(t *testing.T) {
	if !hrtime.TSCSupported() {
		if err := SetTiming(NewBuilder(), TimingConfig{Mode: TimingFull, ClockSource: TimingClockTSC}); !errors.Is(err, ErrUnsupportedTimingClock) {
			t.Fatalf("unsupported TSC config error = %v", err)
		}
		return
	}

	clock := timingClock(TimingClockTSC)
	previous := clock()
	for range 100 {
		current := clock()
		if current < previous {
			t.Fatalf("TSC clock moved backwards: previous=%d current=%d", previous, current)
		}
		previous = current
	}

	b := NewBuilder()
	if err := SetTiming(b, TimingConfig{Mode: TimingFull, ClockSource: TimingClockTSC}); err != nil {
		t.Fatal(err)
	}
	if b.timing.ClockSource != TimingClockTSC {
		t.Fatalf("configured clock source = %d, want %d", b.timing.ClockSource, TimingClockTSC)
	}

	if err := SetWorkerTiming(NewBuilder(), WorkerTimingConfig{Every: 1, ClockSource: TimingClockTSC}); err != nil {
		t.Fatalf("configure worker TSC clock: %v", err)
	}
}

func timedSource(t testing.TB, every uint64, clock func() uint64) (*normalizationTarget, *nodeCounters) {
	t.Helper()
	counts := &nodeCounters{timingEvery: every, clock: clock}
	target := &normalizationTarget{edges: &edgeBindingTarget{}, metrics: &metricStore{nodes: []*nodeCounters{counts}}}
	definition := node.Stateful[int, node.Node0[int, node.Outputs0, int]]{
		Factory: func() (int, error) { return 0, nil },
		Handlers: node.Node0[int, node.Outputs0, int]{Handle: func(calls *int, _ *node.Outputs0) node.Result[int] {
			*calls++
			return node.Result[int]{Failed: *calls%2 == 1}
		}},
	}
	if err := normalizeExplicit0(target, 1, definition); err != nil {
		t.Fatal(err)
	}
	return target, counts
}

func TestGeneratedTimingSamplingAndFailures(t *testing.T) {
	for _, mode := range []struct {
		name     string
		every    uint64
		measured uint64
	}{
		{"disabled", 0, 0}, {"full", 1, 7}, {"every-three", 3, 3},
	} {
		t.Run(mode.name, func(t *testing.T) {
			var reads uint64
			target, counts := timedSource(t, mode.every, func() uint64 { reads++; return reads * 10 })
			for range 7 {
				target.runners[0].run()
			}
			got := (&Executor{metrics: target.metrics}).Metrics().Nodes[0]
			if reads != mode.measured*2 || got.MeasuredCalls != mode.measured || got.MeasuredNanoseconds != mode.measured*10 || got.HandlerCalls != 7 || got.FailedResults != 4 {
				t.Fatalf("reads=%d metrics=%+v", reads, got)
			}
			if mode.measured != 0 && got.MaxMeasuredNanoseconds != 10 {
				t.Fatal("maximum measured duration incorrect")
			}
			before := reads
			_ = (&Executor{metrics: target.metrics}).Metrics()
			if reads != before || counts.calls.Load() != 7 {
				t.Fatal("snapshot called clock or changed counts")
			}
		})
	}
}

func TestTimingDefaultClockAndCopies(t *testing.T) {
	b := NewBuilder()
	_, err := AddNode0(b, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	config := TimingConfig{Mode: TimingSampled, Every: 3}
	if err := SetTiming(b, config); err != nil {
		t.Fatal(err)
	}
	config.Every = 99
	bindCPU(t, b, DefaultGroupID, 0)
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if d.timing.Every != 3 {
		t.Fatal("definition retained mutable timing config")
	}
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if len(e.Metrics().Nodes) != 1 {
		t.Fatal("timed snapshot missing node")
	}
}

func TestTimingTSCClockExecution(t *testing.T) {
	if !hrtime.TSCSupported() {
		t.Skip("invariant TSC is unavailable")
	}
	b := NewBuilder()
	if _, err := AddNode0(b, validIsolated()); err != nil {
		t.Fatal(err)
	}
	if err := SetTiming(b, TimingConfig{Mode: TimingFull, ClockSource: TimingClockTSC}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
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
	if got := e.Metrics().Nodes[0]; got.MeasuredCalls == 0 {
		t.Fatalf("TSC timing did not measure any calls: %+v", got)
	}
}

func TestTimingUpdatesAllocs(t *testing.T) {
	for _, every := range []uint64{0, 1, 3} {
		name := "disabled"
		if every == 1 {
			name = "full"
		}
		if every == 3 {
			name = "sampled"
		}
		t.Run(name, func(t *testing.T) {
			var ticks uint64
			target, _ := timedSource(t, every, func() uint64 { ticks++; return ticks })
			if allocations := testing.AllocsPerRun(100, func() { target.runners[0].run() }); allocations != 0 {
				t.Fatalf("allocations = %v", allocations)
			}
		})
	}
}

func BenchmarkTimingModes(b *testing.B) {
	for _, mode := range []struct {
		name  string
		every uint64
	}{{"off", 0}, {"full", 1}, {"sampled-64", 64}} {
		for _, source := range []struct {
			name  string
			clock TimingClockSource
		}{{"time", TimingClockStandard}, {"tsc", TimingClockTSC}} {
			if source.clock == TimingClockTSC && !hrtime.TSCSupported() {
				continue
			}
			b.Run(mode.name+"/"+source.name, func(b *testing.B) {
				target, _ := timedSource(b, mode.every, timingClock(source.clock))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					target.runners[0].run()
				}
			})
		}
	}
}

func TestTimingDoesNotMeasureSourceWaitRetries(t *testing.T) {
	e := waitingDrainFixture(t, nil)
	var reads atomic.Uint64
	if err := e.call(DefaultGroupID, func() error {
		m := e.metrics.nodes[0]
		m.timingEvery = 1
		m.clock = func() uint64 { return reads.Add(1) }
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.Drain(); err != nil {
		t.Fatal(err)
	}
	if reads.Load() != 0 || e.Metrics().Nodes[0].MeasuredCalls != 0 {
		t.Fatal("pending publication retries were timed as handler calls")
	}
}
