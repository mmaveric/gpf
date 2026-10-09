package graph

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

// The selector continuously prefers input two, leaving one queued and one
// Wait-pending batch on input one's source until admission stops.
func waitingDrainFixture(t *testing.T, closeErr error) *Executor {
	return waitingDrainFixtureWithHook(t, closeErr, nil)
}

func waitingDrainFixtureWithHook(t *testing.T, closeErr error, onFirstInput func()) *Executor {
	t.Helper()
	b := NewBuilder()
	source := func() NodePorts[InPorts0, node.Outputs1[int], error] {
		p, err := AddNode0(b, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs1[int], error]]{
			Factory: func() (struct{}, error) { return struct{}{}, nil },
			Handlers: node.Node0[struct{}, node.Outputs1[int], error]{Handle: func(_ *struct{}, out *node.Outputs1[int]) node.Result[error] {
				v, ok := out.Out1.Acquire()
				if ok {
					v.SetLen(3)
					out.Out1.Commit(&v)
				}
				return node.Result[error]{}
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	first, second := source(), source()
	sink, err := AddNode2(b, node.Stateful[struct{}, node.Node2[struct{}, int, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node2[struct{}, int, int, node.Outputs0, error]{
			Handle1: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] {
				if onFirstInput != nil {
					onFirstInput()
				}
				return node.Result[error]{}
			},
			Handle2: func(*struct{}, batch.Batch[int], *node.Outputs0) node.Result[error] { return node.Result[error]{} },
			Select:  func(*struct{}, node.ReadyMask) node.PortIndex { return node.Port2 },
		},
		Close: func(*struct{}) error { return closeErr },
	})
	if err != nil {
		t.Fatal(err)
	}
	config := EdgeConfig[int]{Capacity: 1, BatchCapacity: 3, Policy: queue.Wait}
	if _, err := Connect(b, OutputPorts1(first.Outputs()).Out1, sink.Inputs().In1, config); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(b, OutputPorts1(second.Outputs()).Out1, sink.Inputs().In2, config); err != nil {
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
	t.Cleanup(func() { _ = e.Close() })
	deadline := time.Now().Add(5 * time.Second)
	for e.Metrics().Edges[0].WaitRetries == 0 {
		if time.Now().After(deadline) {
			t.Fatal("source did not reach Wait pending state")
		}
		runtime.Gosched()
	}
	e.pauseWorkers()
	return e
}

func TestDrainContextCanceledDiscardsQueuedAndPending(t *testing.T) {
	closeErr := errors.New("close failed")
	cause := errors.New("caller shutdown reason")
	e := waitingDrainFixture(t, closeErr)
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	err := e.DrainContext(ctx)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) || !errors.Is(err, closeErr) {
		t.Fatalf("joined stop error = %v", err)
	}
	final := e.Metrics()
	got := final.Edges[0]
	if got.AcceptedBatches != 1 || got.DequeuedBatches != 0 || got.ShutdownDiscardedBatches != 2 || got.ShutdownDiscardedElements != 6 || got.QueueDepthSample != 0 {
		t.Fatalf("forced-discard metrics: %+v", got)
	}
	if err := e.Close(); !errors.Is(err, cause) || !errors.Is(err, closeErr) {
		t.Fatalf("repeat stop lost errors: %v", err)
	}
	if e.Metrics().Edges[0] != got {
		t.Fatal("repeat stop changed final counters")
	}
}

func TestDrainContextPublishesPendingSourceWithoutReinvocation(t *testing.T) {
	e := waitingDrainFixture(t, nil)
	before := e.Metrics().Nodes[0].HandlerCalls
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.DrainContext(ctx); err != nil {
		t.Fatal(err)
	}
	got := e.Metrics()
	if got.Nodes[0].HandlerCalls != before {
		t.Fatalf("source reinvoked during drain: before=%d after=%d", before, got.Nodes[0].HandlerCalls)
	}
	if got.Edges[0].AcceptedBatches != 2 || got.Edges[0].DequeuedBatches != 2 || got.Edges[0].ShutdownDiscardedBatches != 0 {
		t.Fatalf("pending source was not drained: %+v", got.Edges[0])
	}
}

func TestDrainContextCancelsBetweenPasses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := waitingDrainFixtureWithHook(t, nil, cancel)
	if err := e.DrainContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	got := e.Metrics().Edges[0]
	if got.DequeuedBatches+got.ShutdownDiscardedBatches != 2 {
		t.Fatalf("accepted/pending work was lost: %+v", got)
	}
}

func TestDrainContextExpiredDeadline(t *testing.T) {
	e := waitingDrainFixture(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	if err := e.DrainContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
	if e.Metrics().Edges[0].ShutdownDiscardedBatches != 2 {
		t.Fatal("expired deadline did not force cleanup")
	}
}

func TestStopParticipantsShareBudgetAndRunOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var admission, drained, joined int
	e := &Executor{participants: []stopParticipant{{
		closeAdmission: func(got context.Context) error {
			admission++
			if got != ctx {
				t.Error("admission budget replaced")
			}
			return nil
		},
		drain: func(got context.Context) error {
			drained++
			if got != ctx {
				t.Error("drain budget replaced")
			}
			return nil
		},
		discard: func() { t.Error("unexpected forced discard") },
		join:    func() error { joined++; return nil },
	}}}
	if err := e.DrainContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if admission != 1 || drained != 1 || joined != 1 {
		t.Fatalf("callback counts = %d/%d/%d", admission, drained, joined)
	}
}

func TestStopParticipantCancellationDiscardsAndJoins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var discarded, joined int
	e := &Executor{participants: []stopParticipant{{
		drain: func(got context.Context) error {
			if got != ctx {
				t.Error("participant budget replaced")
			}
			cancel()
			return got.Err()
		},
		discard: func() { discarded++ },
		join:    func() error { joined++; return nil },
	}}}
	if err := e.DrainContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("participant stop error = %v", err)
	}
	if err := e.Close(); !errors.Is(err, context.Canceled) {
		t.Fatalf("repeated error = %v", err)
	}
	if discarded != 1 || joined != 1 {
		t.Fatalf("discard/join counts = %d/%d", discarded, joined)
	}
}
