package main

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/graph"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/queue"
)

// Four single-element batches fit even if the worker has not run yet. This
// intentionally finite demo needs neither sleeps nor a producer retry loop.
const capacity = 4

type parseFailure struct {
	Input  string
	Reason string
}

type outcome struct {
	numbers  []int
	failures []parseFailure
	affinity []graph.AffinityStatus
	metrics  graph.MetricsSnapshot
}

func run(ctx context.Context, cpu int, inputs []string, dot io.Writer) (outcome, error) {
	if len(inputs) > capacity {
		return outcome{}, fmt.Errorf("quickstart accepts at most %d inputs", capacity)
	}
	if err := ctx.Err(); err != nil {
		return outcome{}, err
	}

	// Only the worker writes these slices. The caller reads them after DrainContext
	// joins the worker, so no mutex or timing-based completion check is needed.
	var result outcome
	builder := graph.NewBuilder()
	ingressDefinition, err := graph.AddIngress[string](builder, graph.IngressConfig{
		Capacity: capacity, BatchCapacity: 1, Policy: queue.Wait,
	})
	if err != nil {
		return outcome{}, err
	}
	parser, err := graph.AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[string, node.Outputs1[int], parseFailure]], error) {
		return node.ClosureReplica[node.ClosureNode1[string, node.Outputs1[int], parseFailure]]{
			Handlers: node.ClosureNode1[string, node.Outputs1[int], parseFailure]{Handle1: parse},
		}, nil
	})
	if err != nil {
		return outcome{}, err
	}
	sink, err := graph.AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode1[int, node.Outputs0, error]]{
			Handlers: node.ClosureNode1[int, node.Outputs0, error]{
				Handle1: func(input batch.Batch[int], _ *node.Outputs0) node.Result[error] {
					result.numbers = append(result.numbers, input.Items()...)
					return node.Result[error]{}
				},
			},
		}, nil
	})
	if err != nil {
		return outcome{}, err
	}
	errorSink, err := graph.AddClosureNode1(builder, func() (node.ClosureReplica[node.ClosureNode1[parseFailure, node.Outputs0, error]], error) {
		return node.ClosureReplica[node.ClosureNode1[parseFailure, node.Outputs0, error]]{
			Handlers: node.ClosureNode1[parseFailure, node.Outputs0, error]{
				Handle1: func(input batch.Batch[parseFailure], _ *node.Outputs0) node.Result[error] {
					result.failures = append(result.failures, input.Items()...)
					return node.Result[error]{}
				},
			},
		}, nil
	})
	if err != nil {
		return outcome{}, err
	}
	if _, err := graph.Connect(builder, ingressDefinition.Output(), parser.Inputs().In1,
		graph.EdgeConfig[string]{Capacity: capacity, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		return outcome{}, err
	}
	if _, err := graph.Connect(builder, graph.OutputPorts1(parser.Outputs()).Out1, sink.Inputs().In1,
		graph.EdgeConfig[int]{Capacity: capacity, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		return outcome{}, err
	}
	if _, err := graph.ConnectError(builder, parser.Errors(), errorSink.Inputs().In1,
		graph.EdgeConfig[parseFailure]{Capacity: capacity, BatchCapacity: 1, Policy: queue.Wait}); err != nil {
		return outcome{}, err
	}
	// All nodes use the default group: one worker, one explicit CPU request.
	if err := graph.BindCPU(builder, graph.DefaultGroupID, cpu); err != nil {
		return outcome{}, err
	}
	definition, err := builder.Build()
	if err != nil {
		return outcome{}, err
	}
	if dot != nil {
		if err := definition.WriteDOT(dot); err != nil {
			return outcome{}, fmt.Errorf("write DOT: %w", err)
		}
	}
	executor, err := definition.Start()
	if err != nil {
		return outcome{}, err
	}
	// Close is the discard-on-error safety net. A successful drain already stops
	// the executor; repeated sequential stop calls are safe.
	defer executor.Close()
	ingress, err := graph.BindIngress(executor, ingressDefinition)
	if err != nil {
		return outcome{}, err
	}
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return outcome{}, err
		}
		// Submit copies from caller memory and handles the acquired batch's cleanup,
		// including on rejection. Exactly one element goes into each input batch.
		if status := ingress.Submit([]string{input}); status != graph.IngressAccepted {
			return outcome{}, fmt.Errorf("ingress rejected input %q: status %d", input, status)
		}
	}
	if err := executor.DrainContext(ctx); err != nil {
		return outcome{}, err
	}
	result.affinity = executor.AffinityStatus()
	result.metrics = executor.Metrics()
	return result, nil
}

// parse converts one single-element input batch. Failed is explicit: a nonzero
// Value alone does not route a failure. The runtime releases input on return.
func parse(input batch.Batch[string], outputs *node.Outputs1[int]) node.Result[parseFailure] {
	text := input.Items()[0]
	value, err := strconv.Atoi(text)
	if err != nil {
		return node.Result[parseFailure]{Failed: true, Value: parseFailure{text, "not a decimal integer"}}
	}
	output, ok := outputs.Out1.Acquire()
	if !ok {
		return node.Result[parseFailure]{Failed: true, Value: parseFailure{text, "output pool exhausted"}}
	}
	output.Mutable()[0] = value
	output.SetLen(1)
	if !outputs.Out1.Commit(&output) {
		output.Release()
		return node.Result[parseFailure]{Failed: true, Value: parseFailure{text, "output commit rejected"}}
	}
	return node.Result[parseFailure]{}
}
