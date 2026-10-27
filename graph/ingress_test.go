package graph

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

func ingressFixture(t *testing.T, remote bool) (*Definition, IngressDefinition[int], *atomic.Uint64) {
	t.Helper()
	b := NewBuilder()
	def, err := AddIngress[int](b, IngressConfig{Capacity: 4, BatchCapacity: 3, Policy: queue.Wait})
	if err != nil {
		t.Fatal(err)
	}
	delivered := &atomic.Uint64{}
	sink, err := AddNode1(b, node.Stateful[struct{}, node.Node1[struct{}, int, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node1[struct{}, int, node.Outputs0, error]{Handle1: func(_ *struct{}, input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
			if input.Len() != 3 || input.Items()[0] != 7 || input.Items()[2] != 19 {
				t.Error("ingress corrupted payload")
			}
			delivered.Add(1)
			return node.Result[error]{}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if remote {
		if err := Place(b, sink.NodeID(), 7); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Connect(b, def.Output(), sink.Inputs().In1, EdgeConfig[int]{Capacity: 4, BatchCapacity: 3, Policy: queue.Wait}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, b, DefaultGroupID, 0)
	if remote {
		bindCPU(t, b, 7, 1)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return d, def, delivered
}

func TestIngressBindingAndSequentialStarts(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "remote"}[remote], func(t *testing.T) {
			d, handle, delivered := ingressFixture(t, remote)
			if _, err := BindIngress[int](nil, handle); !errors.Is(err, ErrInvalidIngress) {
				t.Fatal(err)
			}
			var old *Ingress[int]
			for n := uint64(1); n <= 2; n++ {
				e, err := d.Start()
				if err != nil {
					t.Fatal(err)
				}
				in, err := BindIngress(e, handle)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := BindIngress(e, IngressDefinition[int]{}); !errors.Is(err, ErrInvalidIngress) {
					t.Fatal(err)
				}
				foreign, err := AddIngress[int](NewBuilder(), IngressConfig{Capacity: 1, BatchCapacity: 3, Policy: queue.Wait})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := BindIngress(e, foreign); !errors.Is(err, ErrInvalidIngress) {
					t.Fatal("foreign endpoint was bound")
				}
				if old != nil && old.Submit([]int{7, 11, 19}) != IngressClosed {
					t.Fatal("old endpoint reopened")
				}
				if status := in.Submit([]int{7, 11, 19}); status != IngressAccepted {
					t.Fatalf("Submit = %d", status)
				}
				waitForLogger(t, func() bool { return delivered.Load() == n })
				held, ok := in.Acquire()
				if !ok {
					t.Fatal("held acquisition failed")
				}
				if err := e.Drain(); err != nil {
					t.Fatal(err)
				}
				if in.Commit(&held) != IngressClosed || held.Cap() != 3 {
					t.Fatal("closed commit consumed caller ownership")
				}
				held.Release()
				if in.pool.Metrics().InUseSample != 0 {
					t.Fatal("ingress source pool leaked")
				}
				if _, err := BindIngress(e, handle); !errors.Is(err, ErrInvalidIngress) {
					t.Fatal("stopped executor rebound ingress")
				}
				old = in
			}
		})
	}
}

func TestIngressFullAndDropOwnership(t *testing.T) {
	for _, policy := range []queue.Policy{queue.Wait, queue.Drop} {
		t.Run(map[queue.Policy]string{queue.Wait: "wait", queue.Drop: "drop"}[policy], func(t *testing.T) {
			pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 2, BatchCapacity: 3})
			if err != nil {
				t.Fatal(err)
			}
			in := &Ingress[int]{pool: pool, queue: newRemoteRing[int](1), config: IngressConfig{Capacity: 1, BatchCapacity: 3, Policy: policy}, counts: &ingressCounters{}}
			in.open.Store(true)
			if in.Submit(nil) != IngressAccepted || in.Submit([]int{1, 2, 3, 4}) != IngressTooLarge {
				t.Fatal("empty or oversize submission incorrect")
			}
			first, _ := in.Acquire()
			first.SetLen(3)
			if in.Commit(&first) != IngressAccepted || first.Cap() != 0 {
				t.Fatal("accepted commit did not transfer")
			}
			second, _ := in.Acquire()
			second.SetLen(2)
			status := in.Commit(&second)
			if policy == queue.Wait {
				if status != IngressFull || second.Cap() == 0 {
					t.Fatal("Wait rejection consumed ownership")
				}
				second.Release()
			} else if status != IngressDropped || second.Cap() != 0 {
				t.Fatal("Drop failed to release")
			}
			var received batch.Batch[int]
			if !in.queue.tryPop(&received) {
				t.Fatal("accepted batch missing")
			}
			received.Release()
			if in.pool.Metrics().InUseSample != 0 {
				t.Fatal("full-queue operation leaked")
			}
		})
	}
}

func TestIngressProducerRacesDrain(t *testing.T) {
	d, handle, _ := ingressFixture(t, true)
	e, err := d.Start()
	if err != nil {
		t.Fatal(err)
	}
	in, err := BindIngress(e, handle)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			status := in.Submit([]int{7, 11, 19})
			if status == IngressClosed {
				return
			}
			runtime.Gosched()
		}
	}()
	if err := e.DrainContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-done
	if in.queue.len() != 0 || in.pool.Metrics().InUseSample != 0 {
		t.Fatal("admitted work remained after drain")
	}
}

func TestIngressPublicationAllocs(t *testing.T) {
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 3})
	if err != nil {
		t.Fatal(err)
	}
	in := &Ingress[int]{pool: pool, queue: newRemoteRing[int](1), config: IngressConfig{Capacity: 1, BatchCapacity: 3, Policy: queue.Wait}, counts: &ingressCounters{}}
	in.open.Store(true)
	var received batch.Batch[int]
	items := []int{7, 11, 19}
	if allocations := testing.AllocsPerRun(100, func() {
		if in.Submit(items) != IngressAccepted || !in.queue.tryPop(&received) {
			panic("ingress round trip failed")
		}
		received.Release()
	}); allocations != 0 {
		t.Fatalf("allocations = %v", allocations)
	}
}

func TestIngressSubmissionExhaustionAndValidation(t *testing.T) {
	for _, config := range []IngressConfig{{}, {Capacity: 1}, {Capacity: 1, BatchCapacity: 1, Policy: 99}} {
		if _, err := AddIngress[int](NewBuilder(), config); !errors.Is(err, ErrInvalidIngress) {
			t.Fatalf("invalid configuration %+v: %v", config, err)
		}
	}
	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 1, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	in := &Ingress[int]{pool: pool, queue: newRemoteRing[int](1), config: IngressConfig{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}, counts: &ingressCounters{}}
	in.open.Store(true)
	held, ok := in.Acquire()
	if !ok {
		t.Fatal("initial acquisition failed")
	}
	if in.Submit([]int{1}) != IngressExhausted || in.queue.len() != 0 || in.counts.exhausted.Load() != 1 {
		t.Fatal("exhaustion caused partial publication or incorrect accounting")
	}
	held.Release()
}
