package graph

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

var affinityAdapterMu sync.Mutex

// setAffinityAdapterForTest serializes adapter replacement and restores the
// platform adapter when the test finishes. Callers must not use t.Parallel.
func setAffinityAdapterForTest(t *testing.T, factory func() affinityAdapter) {
	t.Helper()
	affinityAdapterMu.Lock()
	previous := affinityAdapters.Load()
	affinityAdapters.Store(&affinityFactory{new: factory})
	t.Cleanup(func() {
		affinityAdapters.Store(previous)
		affinityAdapterMu.Unlock()
	})
}

type testAffinity func(int) error

func (bind testAffinity) bind(cpu int) error { return bind(cpu) }

func TestBindCPUPrecedenceAndValidation(t *testing.T) {
	var zero Builder
	if err := BindCPU(&zero, InvalidGroupID, -1); !errors.Is(err, ErrInvalidBuilder) {
		t.Fatalf("zero BindCPU() error = %v", err)
	}
	builder := NewBuilder()
	if err := BindCPU(builder, InvalidGroupID, -1); !errors.Is(err, ErrInvalidGroupID) {
		t.Fatalf("zero group BindCPU() error = %v", err)
	}
	if err := BindCPU(builder, 7, -1); !errors.Is(err, ErrInvalidCPU) {
		t.Fatalf("negative BindCPU() error = %v", err)
	}
	if err := BindCPU(builder, 7, 0); !errors.Is(err, ErrUnknownGroup) {
		t.Fatalf("unknown BindCPU() error = %v", err)
	}
	first, err := AddNode0(builder, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddNode0(builder, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, second.NodeID(), 7); err != nil {
		t.Fatal(err)
	}
	if err := BindCPU(builder, DefaultGroupID, 3); err != nil {
		t.Fatal(err)
	}
	if err := BindCPU(builder, DefaultGroupID, 4); !errors.Is(err, ErrGroupCPUAssigned) {
		t.Fatalf("duplicate group BindCPU() error = %v", err)
	}
	if err := BindCPU(builder, 7, 3); !errors.Is(err, ErrCPUAlreadyBound) {
		t.Fatalf("duplicate CPU BindCPU() error = %v", err)
	}
	if _, exists := builder.cpuBindings[7]; exists || builder.placements[first.NodeID()] != InvalidGroupID {
		t.Fatal("rejected BindCPU mutated builder")
	}

	builder.cpuBindings = map[GroupID]int{DefaultGroupID: 3, 7: 3, 9: -1}
	report, err := builder.Validate()
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("Validate() error = %v", err)
	}
	issues := report.Issues()
	if got, want := validationCodes(issues), []ValidationCode{ValidationCodeDuplicateGroupCPU, ValidationCodeIneffectiveGroupCPU}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CPU validation codes = %v, want %v", got, want)
	}
	if issues[0].GroupID != 7 || issues[0].CPU != 3 || issues[1].GroupID != 9 || issues[1].CPU != -1 {
		t.Fatalf("CPU validation issues = %#v", issues)
	}
	if definition, err := builder.Build(); definition != nil || !errors.As(err, &validationError) || builder.state != builderOpen {
		t.Fatalf("Build() = (%p, %v), state=%d", definition, err, builder.state)
	}
}

func TestBindCPUCoverageAndCopiedPlan(t *testing.T) {
	builder := NewBuilder()
	first, err := AddNode0(builder, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddNode0(builder, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, second.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 2)
	report, err := builder.Validate()
	if err == nil || !reflect.DeepEqual(validationCodes(report.Issues()), []ValidationCode{ValidationCodeMissingGroupCPU}) || report.Issues()[0].GroupID != 9 {
		t.Fatalf("incomplete CPU coverage = (%#v, %v)", report, err)
	}
	bindCPU(t, builder, 9, 4)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := definition.groups, []groupPlan{{id: DefaultGroupID, cpu: 2, nodes: []NodeID{first.NodeID()}}, {id: 9, cpu: 4, nodes: []NodeID{second.NodeID()}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("group plan = %#v, want %#v", got, want)
	}
	builder.cpuBindings[DefaultGroupID] = 99
	if definition.groups[0].cpu != 2 {
		t.Fatal("definition retained mutable CPU binding storage")
	}
}

func TestAffinityStatusFallbackAndImmutability(t *testing.T) {
	cause := errors.New("adapter rejected CPU")
	setAffinityAdapterForTest(t, func() affinityAdapter { return testAffinity(func(int) error { return cause }) })
	builder := NewBuilder()
	if _, err := AddNode0(builder, validIsolated()); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 0)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close()
	statuses := executor.AffinityStatus()
	if len(statuses) != 1 || statuses[0].GroupID != DefaultGroupID || statuses[0].CPU != 0 || statuses[0].Outcome != AffinityOutcomeLockedThreadFallback || !errors.Is(statuses[0].Warning, ErrAffinityUnavailable) || !errors.Is(statuses[0].Warning, cause) {
		t.Fatalf("AffinityStatus() = %#v", statuses)
	}
	statuses[0].CPU = 99
	if executor.AffinityStatus()[0].CPU != 0 {
		t.Fatal("AffinityStatus leaked mutable storage")
	}
	var zero *Executor
	if zero.AffinityStatus() != nil {
		t.Fatal("zero AffinityStatus() is not nil")
	}
}

func TestAffinityStatusAppliedInGroupOrder(t *testing.T) {
	setAffinityAdapterForTest(t, func() affinityAdapter { return testAffinity(func(int) error { return nil }) })
	builder := NewBuilder()
	first, err := AddNode0(builder, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	_, err = AddNode0(builder, validIsolated())
	if err != nil {
		t.Fatal(err)
	}
	if err := Place(builder, first.NodeID(), 9); err != nil {
		t.Fatal(err)
	}
	bindCPU(t, builder, DefaultGroupID, 1)
	bindCPU(t, builder, 9, 3)
	definition, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	executor, err := definition.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close()
	if got, want := executor.AffinityStatus(), []AffinityStatus{{GroupID: DefaultGroupID, CPU: 1, Outcome: AffinityOutcomeApplied}, {GroupID: 9, CPU: 3, Outcome: AffinityOutcomeApplied}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AffinityStatus() = %#v, want %#v", got, want)
	}
}
