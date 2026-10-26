package graph

import (
	"errors"
	"time"

	"github.com/loov/hrtime"
)

// TimingMode controls handler measurement independently of numeric counters.
type TimingMode uint8

const (
	TimingDisabled TimingMode = iota
	TimingFull
	TimingSampled
)

// TimingClockSource selects the clock implementation used for timing metrics.
type TimingClockSource uint8

const (
	// TimingClockStandard uses Go's monotonic time clock and is the default.
	TimingClockStandard TimingClockSource = iota
	// TimingClockTSC uses the processor time-stamp counter and approximate
	// calibrated nanoseconds. It is only available on supported processors.
	TimingClockTSC
)

var (
	// ErrInvalidTiming reports an unsupported mode or invalid sample interval.
	ErrInvalidTiming = errors.New("invalid processing timing configuration")
	// ErrTimingConfigured reports a second timing configuration on one builder.
	ErrTimingConfigured = errors.New("processing timing already configured")
	// ErrUnsupportedTimingClock reports a clock source unavailable on this CPU.
	ErrUnsupportedTimingClock = errors.New("unsupported processing timing clock")
)

// TimingConfig is immutable handler timing configuration. Every is required
// only for sampled mode; calls one, 1+Every, and so forth are measured.
type TimingConfig struct {
	// Clock optionally supplies monotonic modular nanoseconds. It must be safe
	// across workers and must return without blocking or allocating. When set,
	// it overrides ClockSource.
	Clock       func() uint64
	ClockSource TimingClockSource
	Mode        TimingMode
	Every       uint64
}

// SetTiming records optional timing on an open builder. Timing is off by default.
func SetTiming(b *Builder, config TimingConfig) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	if (config.Mode == TimingSampled && config.Every == 0) || config.Mode > TimingSampled || (config.Mode != TimingSampled && config.Every != 0) {
		return ErrInvalidTiming
	}
	if err := validateTimingClock(config.ClockSource, config.Clock != nil); err != nil {
		return err
	}
	if b.timing != nil {
		return ErrTimingConfigured
	}
	copy := config
	b.timing = &copy
	b.revision++
	return nil
}

func monotonicClock() func() uint64 {
	epoch := time.Now()
	return func() uint64 { return uint64(time.Since(epoch).Nanoseconds()) }
}

func timingClock(source TimingClockSource) func() uint64 {
	if source != TimingClockTSC {
		return monotonicClock()
	}

	// ApproxDuration lazily calibrates the counter on its first call. Warm it
	// here so calibration does not run on a worker's first measured handler.
	_ = hrtime.Count(0).ApproxDuration()
	epoch := hrtime.TSC()
	return func() uint64 {
		return uint64(hrtime.TSCSince(epoch).ApproxDuration().Nanoseconds())
	}
}

func validateTimingClock(source TimingClockSource, custom bool) error {
	if source > TimingClockTSC {
		return ErrInvalidTiming
	}
	if source == TimingClockTSC && !custom && !hrtime.TSCSupported() {
		return ErrUnsupportedTimingClock
	}
	return nil
}

func (m *nodeCounters) beginTiming() (uint64, bool) {
	if m.clock == nil || m.timingEvery == 0 || m.calls.Load()%m.timingEvery != 0 {
		return 0, false
	}
	return m.clock(), true
}

func (m *nodeCounters) endTiming(start uint64, measured bool) {
	if !measured {
		return
	}
	elapsed := m.clock() - start
	m.measured.Add(1)
	m.nanoseconds.Add(elapsed)
	for old := m.maximum.Load(); elapsed > old; old = m.maximum.Load() {
		if m.maximum.CompareAndSwap(old, elapsed) {
			break
		}
	}
}
