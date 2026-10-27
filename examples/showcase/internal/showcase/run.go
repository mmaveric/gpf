package showcase

import (
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/mmaveric/gpf/graph"
)

// Run executes the order-processing showcase and writes its report to output.
// Handler timing uses the standard monotonic clock.
func Run(output io.Writer, input []Order) error {
	return RunWithConfig(output, input, Config{})
}

// RunWithConfig executes the same pipeline with an explicit timing clock.
// A TSC request fails rather than falling back when the CPU does not support it.
func RunWithConfig(output io.Writer, input []Order, config Config) (runErr error) {
	if output == nil {
		return errors.New("showcase output writer is nil")
	}
	if config.Clock == "" {
		config.Clock = "standard"
	}
	stats := &showcaseStats{}
	compiled, err := buildShowcase(stats, config)
	if err != nil {
		return err
	}
	executor, err := compiled.definition.Start()
	if err != nil {
		return fmt.Errorf("start showcase: %w", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			runErr = errors.Join(runErr, executor.Close())
		}
	}()
	ingress, err := graph.BindIngress(executor, compiled.ingress)
	if err != nil {
		return fmt.Errorf("bind showcase ingress: %w", err)
	}
	for offset := 0; offset < len(input); offset += int(ingressBatchCapacity) {
		end := min(offset+int(ingressBatchCapacity), len(input))
		if err := submitOrders(ingress, input[offset:end]); err != nil {
			return fmt.Errorf("submit orders: %w", err)
		}
	}
	if err := executor.Drain(); err != nil {
		return fmt.Errorf("drain showcase: %w", err)
	}
	stopped = true
	return writeReport(output, len(input), stats, executor, config.Clock)
}

func submitOrders(ingress *graph.Ingress[Order], values []Order) error {
	for {
		switch status := ingress.Submit(values); status {
		case graph.IngressAccepted:
			return nil
		case graph.IngressFull, graph.IngressExhausted:
			runtime.Gosched()
		default:
			return fmt.Errorf("ingress status %d", status)
		}
	}
}
