package graph

import (
	"errors"
	"testing"

	"github.com/asharipov/gpf/node"
	"github.com/asharipov/gpf/queue"
)

type connectState struct{}

func sourcePort[T any](t *testing.T, builder *Builder) OutPort[T] {
	t.Helper()
	ports, err := AddNode0(builder, node.Stateful[connectState, node.Node0[connectState, node.Outputs1[T], error]]{})
	if err != nil {
		t.Fatalf("AddNode0() error = %v", err)
	}
	return OutputPorts1(ports.Outputs()).Out1
}

func inputPort[T any](t *testing.T, builder *Builder) InPort[T] {
	t.Helper()
	ports, err := AddNode1(builder, node.Stateful[connectState, node.Node1[connectState, T, node.Outputs0, error]]{})
	if err != nil {
		t.Fatalf("AddNode1() error = %v", err)
	}
	return ports.Inputs().In1
}

func errorPort[T any](t *testing.T, builder *Builder) ErrorPort[T] {
	t.Helper()
	ports, err := AddNode0(builder, node.Stateful[connectState, node.Node0[connectState, node.Outputs0, T]]{})
	if err != nil {
		t.Fatalf("AddNode0() error = %v", err)
	}
	return ports.Errors()
}

func TestConnectRecordsTypedHeterogeneousEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		intPolicy    queue.Policy
		stringPolicy queue.Policy
		custom       bool
	}{
		{name: "default drop and custom wait", intPolicy: queue.Drop, stringPolicy: queue.Wait, custom: true},
		{name: "default wait and default drop", intPolicy: queue.Wait, stringPolicy: queue.Drop},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := NewBuilder()
			intOutput, intInput := sourcePort[int](t, builder), inputPort[int](t, builder)
			stringOutput, stringInput := sourcePort[string](t, builder), inputPort[string](t, builder)
			var factoryCalls int
			var stringFactory queue.Factory[string]
			if test.custom {
				stringFactory = func(queue.Config) (queue.QueueOps[string], error) {
					factoryCalls++
					return queue.QueueOps[string]{}, nil
				}
			}

			first, err := Connect(builder, intOutput, intInput, EdgeConfig[int]{Capacity: 4, BatchCapacity: 1, Policy: test.intPolicy})
			if err != nil {
				t.Fatalf("first Connect() error = %v", err)
			}
			second, err := Connect(builder, stringOutput, stringInput, EdgeConfig[string]{
				Capacity:      8,
				BatchCapacity: 1,
				Policy:        test.stringPolicy,
				Factory:       stringFactory,
			})
			if err != nil {
				t.Fatalf("second Connect() error = %v", err)
			}
			if first != 1 || second != 2 {
				t.Fatalf("edge IDs = %d, %d; want 1, 2", first, second)
			}
			if factoryCalls != 0 {
				t.Fatalf("Connect() invoked factory %d times", factoryCalls)
			}
			if len(builder.edges) != 2 || len(builder.edgeBindings) != 2 {
				t.Fatalf("stored edges/bindings = %d/%d, want 2/2", len(builder.edges), len(builder.edgeBindings))
			}
			if builder.edges[0].config != (queue.Config{Capacity: 4, BatchCapacity: 1, Policy: test.intPolicy}) ||
				builder.edges[1].config != (queue.Config{Capacity: 8, BatchCapacity: 1, Policy: test.stringPolicy}) {
				t.Fatalf("stored configs = %#v", builder.edges)
			}
			if builder.edgeBindings[0] == nil || builder.edgeBindings[1] == nil {
				t.Fatal("Connect() did not retain typed future bindings")
			}
		})
	}
}

func TestConnectRejectionsAreAtomic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    error
		prepare func(*Builder, *testing.T) (OutPort[int], InPort[int])
		sealed  bool
		reuse   bool
	}{
		{
			name: "sealed builder",
			want: ErrBuilderSealed,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				output, input := sourcePort[int](t, builder), inputPort[int](t, builder)
				builder.state = builderSealed
				return output, input
			},
			sealed: true,
		},
		{
			name: "zero output",
			want: ErrInvalidPort,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				return OutPort[int]{}, inputPort[int](t, builder)
			},
		},
		{
			name: "zero input",
			want: ErrInvalidPort,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				return sourcePort[int](t, builder), InPort[int]{}
			},
		},
		{
			name: "foreign output",
			want: ErrForeignPort,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				return sourcePort[int](t, NewBuilder()), inputPort[int](t, builder)
			},
		},
		{
			name: "foreign input",
			want: ErrForeignPort,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				return sourcePort[int](t, builder), inputPort[int](t, NewBuilder())
			},
		},
		{
			name:  "zero capacity",
			want:  ErrInvalidEdgeConfig,
			reuse: true,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				return sourcePort[int](t, builder), inputPort[int](t, builder)
			},
		},
		{
			name:  "unknown policy",
			want:  ErrInvalidEdgeConfig,
			reuse: true,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				return sourcePort[int](t, builder), inputPort[int](t, builder)
			},
		},
		{
			name: "duplicate input",
			want: ErrInputConnected,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				input := inputPort[int](t, builder)
				if _, err := Connect(builder, sourcePort[int](t, builder), input, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
					t.Fatalf("initial Connect() error = %v", err)
				}
				return sourcePort[int](t, builder), input
			},
		},
		{
			name: "fan-out",
			want: ErrFanOutUnsupported,
			prepare: func(builder *Builder, t *testing.T) (OutPort[int], InPort[int]) {
				output := sourcePort[int](t, builder)
				if _, err := Connect(builder, output, inputPort[int](t, builder), EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
					t.Fatalf("initial Connect() error = %v", err)
				}
				return output, inputPort[int](t, builder)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := NewBuilder()
			output, input := test.prepare(builder, t)
			beforeEdges, beforeBindings := len(builder.edges), len(builder.edgeBindings)
			var factoryCalls int
			config := EdgeConfig[int]{
				Capacity:      1,
				BatchCapacity: 1,
				Policy:        queue.Drop,
				Factory: func(queue.Config) (queue.QueueOps[int], error) {
					factoryCalls++
					return queue.QueueOps[int]{}, nil
				},
			}
			if test.name == "zero capacity" {
				config.Capacity = 0
			}
			if test.name == "unknown policy" {
				config.Policy = queue.Policy(99)
			}
			id, err := Connect(builder, output, input, config)
			if id != InvalidEdgeID || !errors.Is(err, test.want) {
				t.Fatalf("Connect() = (%d, %v), want (0, %v)", id, err, test.want)
			}
			if factoryCalls != 0 || len(builder.edges) != beforeEdges || len(builder.edgeBindings) != beforeBindings {
				t.Fatalf("rejected Connect() mutated builder or invoked factory: calls=%d edges=%d bindings=%d", factoryCalls, len(builder.edges), len(builder.edgeBindings))
			}
			if test.sealed {
				builder.state = builderOpen
			}
			nextOutput, nextInput := sourcePort[int](t, builder), inputPort[int](t, builder)
			if test.reuse {
				nextOutput, nextInput = output, input
			}
			next, err := Connect(builder, nextOutput, nextInput, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop})
			if err != nil || next != EdgeID(beforeEdges+1) {
				t.Fatalf("recovery Connect() = (%d, %v), want (%d, nil)", next, err, beforeEdges+1)
			}
		})
	}
}

func TestConnectValidationPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*Builder) (EdgeID, error)
		want error
	}{
		{
			name: "sealed before invalid port",
			call: func(builder *Builder) (EdgeID, error) {
				builder.state = builderSealed
				return Connect(builder, OutPort[int]{}, InPort[int]{}, EdgeConfig[int]{})
			},
			want: ErrBuilderSealed,
		},
		{
			name: "unknown local input before foreign output",
			call: func(builder *Builder) (EdgeID, error) {
				foreignOutput := sourcePort[int](t, NewBuilder())
				unknownInput := InPort[int]{owner: builder.owner, nodeID: 1, portID: 1}
				return Connect(builder, foreignOutput, unknownInput, EdgeConfig[int]{})
			},
			want: ErrInvalidPort,
		},
		{
			name: "unknown local output before foreign input",
			call: func(builder *Builder) (EdgeID, error) {
				unknownOutput := OutPort[int]{owner: builder.owner, nodeID: 1, portID: 1}
				foreignInput := inputPort[int](t, NewBuilder())
				return Connect(builder, unknownOutput, foreignInput, EdgeConfig[int]{})
			},
			want: ErrInvalidPort,
		},
		{
			name: "input before fan-out and config",
			call: func(builder *Builder) (EdgeID, error) {
				input := inputPort[int](t, builder)
				output := sourcePort[int](t, builder)
				if _, err := Connect(builder, output, input, EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
					t.Fatal(err)
				}
				return Connect(builder, output, input, EdgeConfig[int]{})
			},
			want: ErrInputConnected,
		},
		{
			name: "fan-out before config",
			call: func(builder *Builder) (EdgeID, error) {
				output := sourcePort[int](t, builder)
				if _, err := Connect(builder, output, inputPort[int](t, builder), EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
					t.Fatal(err)
				}
				return Connect(builder, output, inputPort[int](t, builder), EdgeConfig[int]{})
			},
			want: ErrFanOutUnsupported,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id, err := test.call(NewBuilder())
			if id != InvalidEdgeID || !errors.Is(err, test.want) {
				t.Fatalf("Connect() = (%d, %v), want (0, %v)", id, err, test.want)
			}
		})
	}
}

func TestValidateEdgeConfigAndDormantBindings(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		config queue.Config
		valid  bool
	}{
		{config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}, valid: true},
		{config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Wait}, valid: true},
		{config: queue.Config{Policy: queue.Drop}},
		{config: queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Policy(99)}},
	} {
		err := validateEdgeConfig(test.config)
		if (err == nil) != test.valid {
			t.Fatalf("validateEdgeConfig(%#v) error = %v, valid = %v", test.config, err, test.valid)
		}
		if !test.valid {
			var configError *EdgeConfigError
			if !errors.Is(err, ErrInvalidEdgeConfig) || !errors.As(err, &configError) || configError.Config != test.config {
				t.Fatalf("invalid config error = %#v", err)
			}
		}
	}

	builder := NewBuilder()
	var calls int
	factory := queue.Factory[int](func(queue.Config) (queue.QueueOps[int], error) {
		calls++
		return queue.QueueOps[int]{}, nil
	})
	if _, err := Connect(builder, sourcePort[int](t, builder), inputPort[int](t, builder), EdgeConfig[int]{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop, Factory: factory}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	for _, edge := range builder.edges {
		if err := validateEdgeConfig(edge.config); err != nil {
			t.Fatalf("stored edge config error = %v", err)
		}
	}
	if calls != 0 {
		t.Fatalf("connection or simulated validation invoked factory %d times", calls)
	}
}
