package graph

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/mmaveric/gpf/node"
)

var (
	// ErrInvalidLogger reports a missing handler, zero capacity, or invalid size.
	ErrInvalidLogger = errors.New("invalid async logger configuration")
	// ErrLoggerConfigured reports a second logger configuration on one builder.
	ErrLoggerConfigured = errors.New("async logger already configured")
)

// LoggerConfig configures fixed numeric event transport. Capacity bounds each
// physical worker's ring. Handle runs on an unpinned logger goroutine; it may
// format or allocate but must return cooperatively for completed shutdown.
type LoggerConfig struct {
	Capacity uint32
	Handle   func(node.ErrorEvent)
}

// SetLogger records copied, dormant configuration on an open builder.
// Logger callback captures remain caller-owned immutable configuration.
func SetLogger(b *Builder, config LoggerConfig) error {
	if b == nil || b.owner == nil || b.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return ErrBuilderSealed
	}
	maxInt := uint64(^uint(0) >> 1)
	if config.Capacity == 0 || config.Handle == nil || uint64(config.Capacity) > maxInt/uint64(unsafe.Sizeof(node.ErrorEvent{})) {
		return ErrInvalidLogger
	}
	if b.logger != nil {
		return ErrLoggerConfigured
	}
	copy := config
	b.logger = &copy
	b.revision++
	return nil
}

// LoggerMetrics reports numeric event outcomes for one physical worker.
// AcceptedEvents = DeliveredEvents + ShutdownDiscardedEvents after stop.
type LoggerMetrics struct {
	Replica                                                                                       uint32
	GroupID                                                                                       GroupID
	AttemptedEvents, AcceptedEvents, QueueDroppedEvents, DeliveredEvents, ShutdownDiscardedEvents uint64
}

type loggerCounters struct {
	group                                              GroupID
	physicalGroup                                      GroupID
	replica                                            uint32
	attempted, accepted, dropped, delivered, discarded atomic.Uint64
}

type eventRing struct {
	values                []node.ErrorEvent
	head, tail            atomic.Uint64
	readIndex, writeIndex uint32
	counts                *loggerCounters
}

func (r *eventRing) push(event node.ErrorEvent) bool {
	r.counts.attempted.Add(1)
	tail := r.tail.Load()
	if tail-r.head.Load() == uint64(len(r.values)) {
		r.counts.dropped.Add(1)
		return false
	}
	r.values[r.writeIndex] = event
	r.writeIndex++
	if r.writeIndex == uint32(len(r.values)) {
		r.writeIndex = 0
	}
	r.tail.Store(tail + 1)
	r.counts.accepted.Add(1)
	return true
}

func (r *eventRing) pop(event *node.ErrorEvent) bool {
	head := r.head.Load()
	if head == r.tail.Load() {
		return false
	}
	*event = r.values[r.readIndex]
	r.readIndex++
	if r.readIndex == uint32(len(r.values)) {
		r.readIndex = 0
	}
	r.head.Store(head + 1)
	return true
}

type loggerCommand struct {
	drain    bool
	ctx      context.Context
	response chan error
}

type runtimeLogger struct {
	cursor   int
	rings    []*eventRing
	handle   func(node.ErrorEvent)
	commands chan loggerCommand
	done     chan struct{}
	stopped  bool // accessed only by sequential control-plane shutdown
	stopErr  error
}

func newRuntimeLogger(groups []groupPlan, config LoggerConfig, metrics *metricStore) *runtimeLogger {
	l := &runtimeLogger{handle: config.Handle, commands: make(chan loggerCommand, 1), done: make(chan struct{})}
	for _, group := range groups {
		logicalGroup := group.logical
		if logicalGroup == 0 {
			logicalGroup = group.id
		}
		counts := &loggerCounters{group: logicalGroup, physicalGroup: group.id, replica: group.replica}
		metrics.loggers = append(metrics.loggers, counts)
		l.rings = append(l.rings, &eventRing{values: make([]node.ErrorEvent, config.Capacity), counts: counts})
	}
	return l
}

func (l *runtimeLogger) sink(group GroupID) func(node.ErrorEvent) bool {
	for _, ring := range l.rings {
		if ring.counts.physicalGroup == group {
			return ring.push
		}
	}
	return nil
}

func (l *runtimeLogger) pass() bool {
	for range l.rings {
		ring := l.rings[l.cursor]
		l.cursor++
		if l.cursor == len(l.rings) {
			l.cursor = 0
		}
		var event node.ErrorEvent
		if ring.pop(&event) {
			l.handle(event)
			ring.counts.delivered.Add(1)
			return true
		}
	}
	return false
}

func (l *runtimeLogger) discardQueued() {
	for _, ring := range l.rings {
		var event node.ErrorEvent
		for ring.pop(&event) {
			ring.counts.discarded.Add(1)
		}
	}
}

func (l *runtimeLogger) loop() {
	defer close(l.done)
	for {
		select {
		case command := <-l.commands:
			var err error
			if command.drain {
				for {
					if err = drainCancellation(command.ctx); err != nil {
						l.discardQueued()
						break
					}
					if !l.pass() {
						break
					}
				}
			} else {
				l.discardQueued()
			}
			command.response <- err
			return
		default:
			if !l.pass() {
				runtime.Gosched()
			}
		}
	}
}

func (l *runtimeLogger) stop(ctx context.Context, drain bool) error {
	if l.stopped {
		return l.stopErr
	}
	response := make(chan error, 1)
	l.commands <- loggerCommand{drain: drain, ctx: ctx, response: response}
	l.stopErr = <-response
	l.stopped = true
	return l.stopErr
}

func (l *runtimeLogger) discard()                        { _ = l.stop(context.Background(), false) }
func (l *runtimeLogger) drain(ctx context.Context) error { return l.stop(ctx, true) }
func (l *runtimeLogger) join() error                     { <-l.done; return nil }
