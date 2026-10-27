// Command quickstart runs a bounded ingress, transform, and sink graph.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"
)

func main() {
	cpu := flag.Int("cpu", 0, "requested CPU ID (affinity failure uses the locked-thread fallback)")
	dotPath := flag.String("dot", "", "optional output path for the logical graph in Graphviz DOT format")
	flag.Parse()
	if err := execute(*cpu, *dotPath, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(cpu int, dotPath string, stdout io.Writer) (err error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var dot io.Writer
	if dotPath != "" {
		file, createErr := os.Create(dotPath)
		if createErr != nil {
			return fmt.Errorf("create DOT file: %w", createErr)
		}
		dot = file
		defer func() { err = errors.Join(err, file.Close()) }()
	}

	result, err := run(ctx, cpu, []string{"10", "not-a-number", "30"}, dot)
	if err != nil {
		return err
	}
	for _, status := range result.affinity {
		if status.Warning != nil {
			fmt.Fprintf(os.Stderr, "CPU %d: %v; continuing on a locked OS thread\n", status.CPU, status.Warning)
		}
	}
	if _, err := fmt.Fprintf(stdout, "numbers: %v\n", result.numbers); err != nil {
		return fmt.Errorf("write numbers: %w", err)
	}
	for _, failure := range result.failures {
		if _, err := fmt.Fprintf(stdout, "rejected %q: %s\n", failure.Input, failure.Reason); err != nil {
			return fmt.Errorf("write rejection: %w", err)
		}
	}
	return nil
}
