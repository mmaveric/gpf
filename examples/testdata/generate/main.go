// Command generate reproduces the optional million-order benchmark fixtures.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const orderCount = 1_000_000

type outputFile struct {
	path  string
	mixed bool
}

func main() {
	if err := runCommand(os.Args[1:], os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, diagnostic io.Writer) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	valid := flags.String("valid-out", "", "new output path for 1,000,000 valid orders (never overwritten)")
	mixed := flags.String("mixed-out", "", "new output path for 1,000,000 orders, 5% invalid (never overwritten)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("generate accepts flags only, not positional arguments")
	}
	outputs := make([]outputFile, 0, 2)
	if *valid != "" {
		outputs = append(outputs, outputFile{path: *valid})
	}
	if *mixed != "" {
		outputs = append(outputs, outputFile{path: *mixed, mixed: true})
	}
	if len(outputs) == 0 {
		return errors.New("select at least one explicit output: -valid-out or -mixed-out")
	}
	// Check every destination before creating any files. O_EXCL below also closes
	// the race with another process creating a destination after this check.
	seen := make(map[string]bool, len(outputs))
	for _, output := range outputs {
		absolute, err := filepath.Abs(output.path)
		if err != nil {
			return err
		}
		if seen[absolute] {
			return fmt.Errorf("output paths must differ: %s", output.path)
		}
		seen[absolute] = true
		_, err = os.Lstat(output.path)
		if err == nil {
			return fmt.Errorf("refuse to overwrite existing output %s", output.path)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check output %s: %w", output.path, err)
		}
	}
	for _, output := range outputs {
		if err := writeFile(output); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(output outputFile) error {
	file, err := os.OpenFile(output.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create output %s: %w", output.path, err)
	}
	generateErr := generate(file, orderCount, output.mixed)
	closeErr := file.Close()
	if err := errors.Join(generateErr, closeErr); err != nil {
		return fmt.Errorf("write output %s (file may be partial): %w", output.path, err)
	}
	return nil
}

func generate(output io.Writer, count int, mixed bool) error {
	writer := bufio.NewWriterSize(output, 64*1024)
	if _, err := writer.WriteString("[\n"); err != nil {
		return err
	}
	for i := range count {
		id := uint64(i + 1)
		customer := uint64(i%10_000 + 1)
		units := uint32(i%10 + 1)
		cents := uint32(50 + (uint64(i)*37)%9951)
		block := i / 20
		if mixed && i%20 == block%20 {
			switch block % 3 {
			case 0:
				id = 0
			case 1:
				units = 0
			case 2:
				cents = 0
			}
		}
		if i != 0 {
			if _, err := writer.WriteString(",\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(
			writer,
			`{"id":%d,"customerId":%d,"units":%d,"unitCents":%d}`,
			id,
			customer,
			units,
			cents,
		); err != nil {
			return err
		}
	}
	if _, err := writer.WriteString("\n]\n"); err != nil {
		return err
	}
	return writer.Flush()
}
