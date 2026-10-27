package node_test

import (
	"errors"
	"testing"

	"github.com/mmaveric/gpf/node"
)

type factoryState struct {
	values []int
}

type factoryError struct{}

func TestFactoryDeclarationSignatures(t *testing.T) {
	t.Parallel()

	var stateFactory node.StateFactory[factoryState] = newFactoryState
	var initFunc node.InitFunc[factoryState] = initFactoryState
	var closeFunc node.CloseFunc[factoryState] = closeFactoryState
	var closureFactory node.ClosureFactory[node.ClosureNode0[node.Outputs0, factoryError]] = newClosureReplica

	declaration := node.Stateful[
		factoryState,
		node.Node0[factoryState, node.Outputs0, factoryError],
	]{
		Factory: stateFactory,
		Handlers: node.Node0[factoryState, node.Outputs0, factoryError]{
			Handle: factorySourceHandler,
		},
		Init:  initFunc,
		Close: closeFunc,
	}
	if declaration.Factory == nil || declaration.Init == nil || declaration.Close == nil {
		t.Fatal("Stateful did not preserve callbacks")
	}

	if _, err := closureFactory(); err != nil {
		t.Fatalf("ClosureFactory() error = %v", err)
	}
}

func TestFactoryDataContracts(t *testing.T) {
	t.Parallel()

	explicit := node.Stateful[
		factoryState,
		node.Node0[factoryState, node.Outputs0, factoryError],
	]{
		Factory: newFactoryState,
		Handlers: node.Node0[factoryState, node.Outputs0, factoryError]{
			Handle: factorySourceHandler,
		},
	}
	if explicit.Init != nil || explicit.Close != nil {
		t.Fatal("optional explicit lifecycle hooks are not nil")
	}

	closure := node.ClosureReplica[node.ClosureNode0[node.Outputs0, factoryError]]{
		Handlers: node.ClosureNode0[node.Outputs0, factoryError]{
			Handle: factoryClosureHandler,
		},
	}
	if closure.Init != nil || closure.Close != nil {
		t.Fatal("optional closure lifecycle hooks are not nil")
	}
}

func TestFactoryResultsAreIndependent(t *testing.T) {
	t.Parallel()

	first, err := newFactoryState()
	if err != nil {
		t.Fatalf("first StateFactory() error = %v", err)
	}
	second, err := newFactoryState()
	if err != nil {
		t.Fatalf("second StateFactory() error = %v", err)
	}
	first.values[0] = 1
	if second.values[0] != 0 {
		t.Fatal("StateFactory reused mutable state")
	}

	firstClosure, firstCount, err := newClosureReplicaWithCounter()
	if err != nil {
		t.Fatalf("first ClosureFactory() error = %v", err)
	}
	secondClosure, secondCount, err := newClosureReplicaWithCounter()
	if err != nil {
		t.Fatalf("second ClosureFactory() error = %v", err)
	}
	firstClosure.Handlers.Handle(&node.Outputs0{})
	if *firstCount != 1 || *secondCount != 0 {
		t.Fatal("ClosureFactory reused mutable closure state")
	}
	_ = secondClosure
}

func TestLifecycleOwnershipContract(t *testing.T) {
	t.Parallel()

	state, err := newFactoryState()
	if err != nil {
		t.Fatalf("StateFactory() error = %v", err)
	}
	if err := initFactoryState(&state); err != nil {
		t.Fatalf("InitFunc() error = %v", err)
	}
	if err := closeFactoryState(&state); err != nil {
		t.Fatalf("CloseFunc() error = %v", err)
	}
	if state.values != nil {
		t.Fatal("CloseFunc did not release state")
	}

	failedFactory := node.StateFactory[factoryState](func() (factoryState, error) {
		return factoryState{}, errFactory
	})
	if _, err := failedFactory(); !errors.Is(err, errFactory) {
		t.Fatalf("failed StateFactory() error = %v, want errFactory", err)
	}

	closeCalls := 0
	partial := node.ClosureReplica[node.ClosureNode0[node.Outputs0, factoryError]]{
		Handlers: node.ClosureNode0[node.Outputs0, factoryError]{
			Handle: factoryClosureHandler,
		},
		Init: func() error { return errFactory },
		Close: func() error {
			closeCalls++
			return nil
		},
	}
	if err := partial.Init(); !errors.Is(err, errFactory) {
		t.Fatalf("partial Init() error = %v, want errFactory", err)
	}
	if err := partial.Close(); err != nil {
		t.Fatalf("partial Close() error = %v", err)
	}
	if closeCalls != 1 {
		t.Fatalf("partial Close() calls = %d, want 1", closeCalls)
	}
}

func TestFactoryDeclarationsAreInert(t *testing.T) {
	t.Parallel()

	called := 0
	declaration := node.Stateful[int, node.Node0[int, node.Outputs0, factoryError]]{
		Factory: func() (int, error) {
			called++
			return 0, nil
		},
	}
	copy := declaration
	_ = copy.Handlers
	if called != 0 {
		t.Fatalf("constructing and copying declaration invoked factory %d times", called)
	}
}

func newFactoryState() (factoryState, error) {
	return factoryState{values: []int{0}}, nil
}

func initFactoryState(*factoryState) error {
	return nil
}

func closeFactoryState(state *factoryState) error {
	state.values = nil
	return nil
}

func factorySourceHandler(*factoryState, *node.Outputs0) node.Result[factoryError] {
	return node.Result[factoryError]{}
}

func factoryClosureHandler(*node.Outputs0) node.Result[factoryError] {
	return node.Result[factoryError]{}
}

var errFactory = errors.New("factory failure")

func newClosureReplica() (node.ClosureReplica[node.ClosureNode0[node.Outputs0, factoryError]], error) {
	replica, _, err := newClosureReplicaWithCounter()
	return replica, err
}

func newClosureReplicaWithCounter() (
	node.ClosureReplica[node.ClosureNode0[node.Outputs0, factoryError]],
	*int,
	error,
) {
	count := 0
	countPointer := &count
	return node.ClosureReplica[node.ClosureNode0[node.Outputs0, factoryError]]{
		Handlers: node.ClosureNode0[node.Outputs0, factoryError]{
			Handle: func(*node.Outputs0) node.Result[factoryError] {
				count++
				return node.Result[factoryError]{}
			},
		},
	}, countPointer, nil
}
