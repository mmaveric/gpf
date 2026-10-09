package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Run(os.Stdout, sampleOrders()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
