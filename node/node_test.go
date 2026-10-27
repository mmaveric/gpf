package node_test

import (
	"testing"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
)

type (
	testState struct{}
	testError struct{}
	input1    struct{}
	input2    struct{}
	input3    struct{}
	input4    struct{}
	input5    struct{}
	input6    struct{}
	input7    struct{}
	input8    struct{}
)

func TestOutputSets(t *testing.T) {
	t.Parallel()

	assertOutputCount(t, node.Outputs0{}, 0)
	assertOutputCount(t, node.Outputs1[int]{}, 1)
	assertOutputCount(t, node.Outputs2[int, string]{}, 2)
	assertOutputCount(t, node.Outputs3[int, string, bool]{}, 3)
	assertOutputCount(t, node.Outputs4[int, string, bool, byte]{}, 4)
	assertOutputCount(t, node.Outputs5[int, string, bool, byte, rune]{}, 5)
	assertOutputCount(t, node.Outputs6[int, string, bool, byte, rune, float32]{}, 6)
	assertOutputCount(t, node.Outputs7[int, string, bool, byte, rune, float32, float64]{}, 7)
	assertOutputCount(
		t,
		node.Outputs8[int, string, bool, byte, rune, float32, float64, complex64]{},
		8,
	)
}

func TestExplicitHandlerSets(t *testing.T) {
	t.Parallel()

	_ = node.Node0[testState, node.Outputs0, testError]{
		Handle: sourceHandler,
	}
	_ = node.Node1[testState, input1, node.Outputs0, testError]{
		Handle1: handleInput1,
	}
	_ = node.Node2[testState, input1, input2, node.Outputs0, testError]{
		Handle1: handleInput1,
		Handle2: handleInput2,
		Select:  selectExplicit,
	}
	_ = node.Node3[testState, input1, input2, input3, node.Outputs0, testError]{
		Handle1: handleInput1,
		Handle2: handleInput2,
		Handle3: handleInput3,
		Select:  selectExplicit,
	}
	_ = node.Node4[testState, input1, input2, input3, input4, node.Outputs0, testError]{
		Handle1: handleInput1,
		Handle2: handleInput2,
		Handle3: handleInput3,
		Handle4: handleInput4,
		Select:  selectExplicit,
	}
	_ = node.Node5[testState, input1, input2, input3, input4, input5, node.Outputs0, testError]{
		Handle1: handleInput1,
		Handle2: handleInput2,
		Handle3: handleInput3,
		Handle4: handleInput4,
		Handle5: handleInput5,
		Select:  selectExplicit,
	}
	_ = node.Node6[
		testState,
		input1,
		input2,
		input3,
		input4,
		input5,
		input6,
		node.Outputs0,
		testError,
	]{
		Handle1: handleInput1,
		Handle2: handleInput2,
		Handle3: handleInput3,
		Handle4: handleInput4,
		Handle5: handleInput5,
		Handle6: handleInput6,
		Select:  selectExplicit,
	}
	_ = node.Node7[
		testState,
		input1,
		input2,
		input3,
		input4,
		input5,
		input6,
		input7,
		node.Outputs0,
		testError,
	]{
		Handle1: handleInput1,
		Handle2: handleInput2,
		Handle3: handleInput3,
		Handle4: handleInput4,
		Handle5: handleInput5,
		Handle6: handleInput6,
		Handle7: handleInput7,
		Select:  selectExplicit,
	}
	_ = node.Node8[
		testState,
		input1,
		input2,
		input3,
		input4,
		input5,
		input6,
		input7,
		input8,
		node.Outputs0,
		testError,
	]{
		Handle1: handleInput1,
		Handle2: handleInput2,
		Handle3: handleInput3,
		Handle4: handleInput4,
		Handle5: handleInput5,
		Handle6: handleInput6,
		Handle7: handleInput7,
		Handle8: handleInput8,
		Select:  selectExplicit,
	}
}

func TestClosureHandlerSets(t *testing.T) {
	t.Parallel()

	_ = node.ClosureNode0[node.Outputs0, testError]{
		Handle: closureSourceHandler,
	}
	_ = node.ClosureNode1[input1, node.Outputs0, testError]{
		Handle1: closureInput1,
	}
	_ = node.ClosureNode2[input1, input2, node.Outputs0, testError]{
		Handle1: closureInput1,
		Handle2: closureInput2,
		Select:  selectClosure,
	}
	_ = node.ClosureNode3[input1, input2, input3, node.Outputs0, testError]{
		Handle1: closureInput1,
		Handle2: closureInput2,
		Handle3: closureInput3,
		Select:  selectClosure,
	}
	_ = node.ClosureNode4[input1, input2, input3, input4, node.Outputs0, testError]{
		Handle1: closureInput1,
		Handle2: closureInput2,
		Handle3: closureInput3,
		Handle4: closureInput4,
		Select:  selectClosure,
	}
	_ = node.ClosureNode5[input1, input2, input3, input4, input5, node.Outputs0, testError]{
		Handle1: closureInput1,
		Handle2: closureInput2,
		Handle3: closureInput3,
		Handle4: closureInput4,
		Handle5: closureInput5,
		Select:  selectClosure,
	}
	_ = node.ClosureNode6[
		input1,
		input2,
		input3,
		input4,
		input5,
		input6,
		node.Outputs0,
		testError,
	]{
		Handle1: closureInput1,
		Handle2: closureInput2,
		Handle3: closureInput3,
		Handle4: closureInput4,
		Handle5: closureInput5,
		Handle6: closureInput6,
		Select:  selectClosure,
	}
	_ = node.ClosureNode7[
		input1,
		input2,
		input3,
		input4,
		input5,
		input6,
		input7,
		node.Outputs0,
		testError,
	]{
		Handle1: closureInput1,
		Handle2: closureInput2,
		Handle3: closureInput3,
		Handle4: closureInput4,
		Handle5: closureInput5,
		Handle6: closureInput6,
		Handle7: closureInput7,
		Select:  selectClosure,
	}
	_ = node.ClosureNode8[
		input1,
		input2,
		input3,
		input4,
		input5,
		input6,
		input7,
		input8,
		node.Outputs0,
		testError,
	]{
		Handle1: closureInput1,
		Handle2: closureInput2,
		Handle3: closureInput3,
		Handle4: closureInput4,
		Handle5: closureInput5,
		Handle6: closureInput6,
		Handle7: closureInput7,
		Handle8: closureInput8,
		Select:  selectClosure,
	}
}

func TestPortIndexesAndSelectors(t *testing.T) {
	tests := []struct {
		name string
		port node.PortIndex
		want node.PortIndex
	}{
		{name: "port 1", port: node.Port1, want: 0},
		{name: "port 2", port: node.Port2, want: 1},
		{name: "port 3", port: node.Port3, want: 2},
		{name: "port 4", port: node.Port4, want: 3},
		{name: "port 5", port: node.Port5, want: 4},
		{name: "port 6", port: node.Port6, want: 5},
		{name: "port 7", port: node.Port7, want: 6},
		{name: "port 8", port: node.Port8, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.port != tt.want {
				t.Fatalf("port index = %d, want %d", tt.port, tt.want)
			}
			mask := node.ReadyMask(1 << tt.port)
			if mask != node.ReadyMask(1<<tt.want) {
				t.Fatalf("ready mask = %08b, want bit %d", mask, tt.want)
			}
		})
	}

	ready := node.ReadyMask(1<<node.Port1 | 1<<node.Port2)
	var state testState
	if selected := selectExplicit(&state, ready); selected != node.Port1 {
		t.Fatalf("explicit selector = %d, want Port1", selected)
	}
	if selected := selectClosure(ready); selected != node.Port1 {
		t.Fatalf("closure selector = %d, want Port1", selected)
	}
}

func TestOutputOwnership(t *testing.T) {
	t.Parallel()

	pool, err := batch.NewPool[int](batch.PoolConfig{Capacity: 2, BatchCapacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	var output node.Output[int]
	node.InstallOutput(&output, pool.Acquire)
	value, ok := output.Acquire()
	if !ok || !output.Commit(&value) || output.Commit(&value) {
		t.Fatal("Output acquisition or duplicate commit handling failed")
	}
	if value.Cap() != 0 {
		t.Fatal("Commit() did not move caller batch")
	}
	published := 0
	if !node.FinalizeOutput(&output, false, func(value batch.Batch[int]) bool {
		published++
		value.Release()
		return true
	}) || published != 1 {
		t.Fatal("FinalizeOutput() did not publish committed batch")
	}
	abandoned, ok := output.Acquire()
	if !ok || abandoned.Cap() == 0 {
		t.Fatal("Acquire() after publish = unavailable")
	}
	if !node.FinalizeOutput(&output, false, func(batch.Batch[int]) bool { return true }) {
		t.Fatal("FinalizeOutput() did not clean abandoned acquisition")
	}
	if _, ok := pool.Acquire(); !ok {
		t.Fatal("abandoned acquisition was not released")
	}
}

func TestGeneratedOutputFields(t *testing.T) {
	var outputs node.Outputs8[int, string, bool, byte, rune, float32, float64, complex64]
	var _ node.Output[int] = outputs.Out1
	var _ node.Output[string] = outputs.Out2
	var _ node.Output[bool] = outputs.Out3
	var _ node.Output[byte] = outputs.Out4
	var _ node.Output[rune] = outputs.Out5
	var _ node.Output[float32] = outputs.Out6
	var _ node.Output[float64] = outputs.Out7
	var _ node.Output[complex64] = outputs.Out8
}

func assertOutputCount[O node.OutputSet](t *testing.T, output O, want uint8) {
	t.Helper()
	if got := output.OutputCount(); got != want {
		t.Fatalf("OutputCount() = %d, want %d", got, want)
	}
}

func sourceHandler(*testState, *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput1(*testState, batch.Batch[input1], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput2(*testState, batch.Batch[input2], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput3(*testState, batch.Batch[input3], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput4(*testState, batch.Batch[input4], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput5(*testState, batch.Batch[input5], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput6(*testState, batch.Batch[input6], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput7(*testState, batch.Batch[input7], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func handleInput8(*testState, batch.Batch[input8], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureSourceHandler(*node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput1(batch.Batch[input1], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput2(batch.Batch[input2], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput3(batch.Batch[input3], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput4(batch.Batch[input4], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput5(batch.Batch[input5], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput6(batch.Batch[input6], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput7(batch.Batch[input7], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func closureInput8(batch.Batch[input8], *node.Outputs0) node.Result[testError] {
	return node.Result[testError]{}
}

func selectExplicit(*testState, node.ReadyMask) node.PortIndex {
	return node.Port1
}

func selectClosure(node.ReadyMask) node.PortIndex {
	return node.Port1
}
