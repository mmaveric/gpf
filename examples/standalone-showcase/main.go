package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type order struct {
	ID         uint64 `json:"id"`
	CustomerID uint64 `json:"customerId"`
	Units      uint32 `json:"units"`
	UnitCents  uint32 `json:"unitCents"`
}

type receipt struct {
	units      uint32
	totalCents uint64
}

type totals struct {
	processed  uint64
	rejected   uint64
	audited    uint64
	units      uint64
	totalCents uint64
	partitions [3]uint64
}

type partitionResult struct {
	partition int
	count     uint64
}

type aggregateResult struct {
	processed  uint64
	units      uint64
	totalCents uint64
}

const channelCapacity = 256

func main() {
	if err := runCommand(os.Args[1:], os.Stdout, os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, output, diagnostic io.Writer) error {
	flags := flag.NewFlagSet("standalone-showcase", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	data := flags.String("data", "", "orders JSON path (default: examples/data.json or ../data.json)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("standalone-showcase accepts flags only, not positional arguments")
	}
	orders, err := loadOrdersFrom(*data)
	if err != nil {
		return err
	}
	return run(output, orders)
}

func loadOrders() ([]order, error) {
	return loadOrdersFrom("")
}

func loadOrdersFrom(path string) ([]order, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read order data %s: %w", path, err)
		}
		return decodeOrders(data, path)
	}
	var data []byte
	var source string
	var err error
	for _, path := range []string{filepath.Join("examples", "data.json"), filepath.Join("..", "data.json")} {
		data, err = os.ReadFile(path)
		if err == nil {
			source = path
			break
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read order data %s: %w", path, err)
		}
	}
	if source == "" {
		return nil, errors.New("read order data: examples/data.json not found; generate it or select -data")
	}
	return decodeOrders(data, source)
}

func decodeOrders(data []byte, source string) ([]order, error) {
	var orders []order
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("decode orders from %s: %w", source, err)
	}
	return orders, nil
}

func run(output io.Writer, orders []order) error {
	if output == nil {
		return errors.New("showcase output writer is nil")
	}
	ingress := make(chan order, channelCapacity)
	auditInput := make(chan order, channelCapacity)
	partitionInputs := [3]chan order{
		make(chan order, channelCapacity),
		make(chan order, channelCapacity),
		make(chan order, channelCapacity),
	}
	fanIn := make(chan receipt, channelCapacity)
	auditDone := make(chan uint64, 1)
	validatorDone := make(chan uint64, 1)
	partitionDone := make(chan partitionResult, len(partitionInputs))
	aggregateDone := make(chan aggregateResult, 1)

	go func() {
		var audited uint64
		for range auditInput {
			audited++
		}
		auditDone <- audited
	}()

	var partitions sync.WaitGroup
	for index, input := range partitionInputs {
		partitions.Add(1)
		go func(partition int, orders <-chan order) {
			defer partitions.Done()
			var count uint64
			for value := range orders {
				fanIn <- receipt{
					units:      value.Units,
					totalCents: uint64(value.Units) * uint64(value.UnitCents),
				}
				count++
			}
			partitionDone <- partitionResult{partition: partition, count: count}
		}(index, input)
	}

	go func() {
		var aggregate aggregateResult
		for value := range fanIn {
			aggregate.processed++
			aggregate.units += uint64(value.units)
			aggregate.totalCents += value.totalCents
		}
		aggregateDone <- aggregate
	}()

	go func() {
		var rejected uint64
		for value := range ingress {
			if value.ID == 0 || value.Units == 0 || value.UnitCents == 0 {
				rejected++
				continue
			}
			auditInput <- value
			partition := value.CustomerID % uint64(len(partitionInputs))
			partitionInputs[partition] <- value
		}
		close(auditInput)
		for _, input := range partitionInputs {
			close(input)
		}
		validatorDone <- rejected
	}()

	for _, value := range orders {
		ingress <- value
	}
	close(ingress)
	rejected := <-validatorDone
	audited := <-auditDone
	partitions.Wait()
	close(fanIn)
	aggregate := <-aggregateDone
	var result totals
	result.rejected = rejected
	result.audited = audited
	result.processed = aggregate.processed
	result.units = aggregate.units
	result.totalCents = aggregate.totalCents
	for range partitionInputs {
		partition := <-partitionDone
		result.partitions[partition.partition] = partition.count
	}
	_, err := fmt.Fprintf(output,
		"orders submitted=%d processed=%d rejected=%d audited=%d units=%d gross_cents=%d partitions=%d,%d,%d\n",
		len(orders), result.processed, result.rejected, result.audited, result.units,
		result.totalCents, result.partitions[0], result.partitions[1], result.partitions[2])
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "runtime mode=standalone framework=none")
	return err
}
