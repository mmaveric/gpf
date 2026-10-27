package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mmaveric/gpf/examples/showcase/internal/showcase"
)

func main() {
	if err := runCommand(os.Args[1:], os.Stdout, os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, output, diagnostic io.Writer) error {
	flags := flag.NewFlagSet("showcase", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	data := flags.String("data", "", "orders JSON path (default: examples/data.json or ../data.json)")
	clock := flags.String("clock", "standard", "handler timing clock: standard or tsc (requires CPU support)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("showcase accepts flags only, not positional arguments")
	}
	config := showcase.Config{Clock: *clock}
	if err := config.Validate(); err != nil {
		return err
	}
	orders, err := loadOrdersFrom(*data)
	if err != nil {
		return err
	}
	return showcase.RunWithConfig(output, orders, config)
}

func loadOrders() ([]showcase.Order, error) {
	return loadOrdersFrom("")
}

func loadOrdersFrom(path string) ([]showcase.Order, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read order data %s: %w", path, err)
		}
		return decodeOrders(data, path)
	}
	data, path, err := readOrderData()
	if err != nil {
		return nil, err
	}
	return decodeOrders(data, path)
}

func decodeOrders(data []byte, path string) ([]showcase.Order, error) {
	var orders []showcase.Order
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("decode orders from %s: %w", path, err)
	}
	return orders, nil
}

func readOrderData() ([]byte, string, error) {
	for _, path := range []string{filepath.Join("examples", "data.json"), filepath.Join("..", "data.json")} {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, path, nil
		}
		if !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("read order data %s: %w", path, err)
		}
	}
	return nil, "", errors.New("read order data: examples/data.json not found; generate it or select -data")
}
