package graph

import (
	"errors"
	"sync/atomic"
)

// ErrAffinityUnavailable reports that a worker continues on its locked OS
// thread because CPU affinity could not be applied.
var ErrAffinityUnavailable = errors.New("graph CPU affinity unavailable")

type affinityAdapter interface {
	bind(cpu int) error
}

type affinityFactory struct{ new func() affinityAdapter }

var affinityAdapters atomic.Pointer[affinityFactory]

func init() {
	affinityAdapters.Store(&affinityFactory{new: newPlatformAffinity})
}

func newAffinityAdapter() affinityAdapter { return affinityAdapters.Load().new() }
