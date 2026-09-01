//go:build !linux || unsupported

package graph

type unsupportedAffinity struct{}

func newPlatformAffinity() affinityAdapter { return unsupportedAffinity{} }

func (unsupportedAffinity) bind(int) error { return ErrAffinityUnavailable }
