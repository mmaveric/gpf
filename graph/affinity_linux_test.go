//go:build linux && !unsupported

package graph

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/mmaveric/gpf/node"
)

func TestLinuxAffinityAdapterSelected(t *testing.T) {
	if _, ok := newPlatformAffinity().(linuxAffinity); !ok {
		t.Fatalf("platform adapter = %T, want linuxAffinity", newPlatformAffinity())
	}
}

func TestLifecycleCallbacksRunOnLockedOwnerWorker(t *testing.T) {
	workerThread := 0
	setAffinityAdapterForTest(t, func() affinityAdapter {
		return testAffinity(func(int) error {
			workerThread = syscall.Gettid()
			return nil
		})
	})
	builder := NewBuilder()
	if _, err := AddNode0(builder, node.Stateful[struct{}, node.Node0[struct{}, node.Outputs0, error]]{
		Factory: func() (struct{}, error) { return struct{}{}, nil },
		Handlers: node.Node0[struct{}, node.Outputs0, error]{
			Handle: func(*struct{}, *node.Outputs0) node.Result[error] { return node.Result[error]{} },
		},
		Init: func(*struct{}) error {
			if got, want := syscall.Gettid(), workerThread; got != want {
				return fmt.Errorf("Init thread = %d, worker thread = %d", got, want)
			}
			return nil
		},
		Close: func(*struct{}) error {
			if got, want := syscall.Gettid(), workerThread; got != want {
				return fmt.Errorf("Close thread = %d, worker thread = %d", got, want)
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.Drain(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-executor.workers[0].done:
	default:
		t.Fatal("worker did not exit after Drain")
	}
}
