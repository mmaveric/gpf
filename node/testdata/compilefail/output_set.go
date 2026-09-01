package compilefail

import "github.com/asharipov/gpf/node"

type arbitraryOutput struct{}

func (arbitraryOutput) OutputCount() uint8 { return 0 }

var _ = node.Node1[struct{}, int, arbitraryOutput, error]{}
