package graph

import (
	"errors"

	"github.com/mmaveric/gpf/batch"
	"github.com/mmaveric/gpf/node"
	"github.com/mmaveric/gpf/numa"
	"github.com/mmaveric/gpf/queue"
)

// NodeID identifies a node within a graph definition.
type NodeID uint32

// InvalidNodeID is the invalid node identity.
const InvalidNodeID NodeID = 0

// PortID identifies a port within a node definition.
type PortID uint8

// InvalidPortID is the invalid port identity.
const InvalidPortID PortID = 0

const errorPortID PortID = 9

// EdgeID identifies a committed edge within a builder.
type EdgeID uint32

// InvalidEdgeID is the invalid edge identity.
const InvalidEdgeID EdgeID = 0

// GroupID identifies an execution group in a future runtime.
type GroupID uint32

const (
	// InvalidGroupID is not a valid execution group identity.
	InvalidGroupID GroupID = 0
	// DefaultGroupID is assigned to every node without an explicit placement.
	DefaultGroupID GroupID = 1
)

// Definition is an immutable graph topology produced by Builder.Build.
//
// Its structural metadata and factory bindings are private so they cannot be
// changed by callers. Factories and callbacks remain dormant until a future
// runtime stage.
type Definition struct {
	planningMode           bool
	explicitPlacements     []GroupID
	constraints            PlanningConstraints
	execution              *Definition
	nodeInstances          []nodeInstance
	groupInstances         []groupInstance
	edgeInstances          []edgeInstance
	counters               []counterDefinition
	workerTiming           *WorkerTimingConfig
	groupMetricsFlushEvery uint64
	timing                 *TimingConfig
	logger                 *LoggerConfig
	owner                  *graphOwner
	nodes                  []nodeMetadata
	edges                  []edgeMetadata
	edgeBindings           []edgeBinding
	forward                []ValidationAdjacency
	reverse                []ValidationAdjacency
	order                  []NodeID
	placements             []GroupID
	groups                 []groupPlan
	edgeRemote             []bool
	edgeQueuePlans         []EdgeQueuePlan
}

// AutomaticNodeAssignment maps one physical node to its proposed plan group.
type AutomaticNodeAssignment struct {
	NodeID, LogicalNodeID NodeID
	Replica               uint32
	GroupID               GroupID
}

// PlannedGroup is one proposed group and its explicit CPU-binding status.
type PlannedGroup struct {
	GroupID, LogicalGroupID GroupID
	Replica                 uint32
	CPU                     int
	CPUBound                bool
	NUMA                    numa.Placement
}

// AutomaticGroupPreview is a copied pure planning result, never an executable Definition.
type AutomaticGroupPreview struct {
	Groups                              []PlannedGroup
	Nodes                               []AutomaticNodeAssignment
	Edges                               []EdgeQueuePlan
	Constraints                         PlanningConstraints
	RemoteEdgesBefore, RemoteEdgesAfter uint32
	Rejections                          []MergeRejection
	CPUComplete                         bool
	owner                               *graphOwner
	revision                            uint64
	logical                             *Definition
	execution                           *Definition
	requiredGroups                      []PlannedGroup
}

// Builder collects mutable node-registration metadata. Builders must not be copied.
type Builder struct {
	revision               uint64
	automaticCPUs          map[GroupID]int
	automaticCPURevision   uint64
	planningMode           bool
	replicas               map[GroupID]replicaConfig
	nodePools              map[NodeID]nodePoolConfig
	counters               map[counterDefinition]struct{}
	workerTiming           *WorkerTimingConfig
	groupMetricsFlushEvery uint64
	timing                 *TimingConfig
	logger                 *LoggerConfig
	owner                  *graphOwner
	state                  builderState
	nextNodeID             NodeID
	nodes                  []nodeMetadata
	inputs                 map[endpointID]*inputMetadata
	outputs                map[endpointID]*outputMetadata
	edges                  []edgeMetadata
	edgeBindings           []edgeBinding
	placements             map[NodeID]GroupID
	cpuBindings            map[GroupID]int
	numaBindings           map[GroupID]numa.Placement
	definition             *Definition
}

type graphOwner struct{ _ byte }

type builderState uint8

const (
	builderInvalid builderState = iota
	builderOpen
	builderSealed
)

// ErrInvalidBuilder reports a nil or uninitialized builder.
var ErrInvalidBuilder = errors.New("invalid graph builder")

// ErrBuilderSealed reports a builder reserved by a future sealing operation.
var ErrBuilderSealed = errors.New("graph builder is sealed")

// ErrNodeIDExhausted reports that another node identity cannot be assigned.
var ErrNodeIDExhausted = errors.New("graph node IDs exhausted")

// NewBuilder constructs an open mutable builder with a unique private owner.
func NewBuilder() *Builder {
	return &Builder{
		owner:       &graphOwner{},
		state:       builderOpen,
		nextNodeID:  1,
		inputs:      make(map[endpointID]*inputMetadata),
		outputs:     make(map[endpointID]*outputMetadata),
		placements:  make(map[NodeID]GroupID),
		cpuBindings: make(map[GroupID]int),
	}
}

var (
	// ErrInvalidGroupID reports the reserved zero group identity.
	ErrInvalidGroupID = errors.New("invalid graph group ID")
	// ErrUnknownNode reports a placement for a node not registered on the builder.
	ErrUnknownNode = errors.New("unknown graph node")
	// ErrNodeAlreadyPlaced reports a second placement for one node.
	ErrNodeAlreadyPlaced = errors.New("graph node already placed")
	// ErrInvalidCPU reports a negative CPU ID.
	ErrInvalidCPU = errors.New("invalid graph CPU")
	// ErrUnknownGroup reports a binding for a group with no currently registered node.
	ErrUnknownGroup = errors.New("unknown graph group")
	// ErrGroupCPUAssigned reports a second CPU binding for one group.
	ErrGroupCPUAssigned = errors.New("graph group CPU already assigned")
	// ErrCPUAlreadyBound reports a CPU bound to another effective group.
	ErrCPUAlreadyBound = errors.New("graph CPU already bound")
	// ErrGroupNUMAConfigured reports a second NUMA annotation for one group.
	ErrGroupNUMAConfigured = errors.New("graph group NUMA placement already configured")
)

// SetGroupNUMA records optional inactive NUMA intent for an effective group.
func SetGroupNUMA(builder *Builder, group GroupID, placement numa.Placement) error {
	if builder == nil || builder.owner == nil || builder.state == builderInvalid {
		return ErrInvalidBuilder
	}
	if builder.state == builderSealed {
		return ErrBuilderSealed
	}
	if group == InvalidGroupID {
		return ErrInvalidGroupID
	}
	if err := placement.Validate(); err != nil {
		return err
	}
	if !effectiveGroups(builder.nodes, builder.placements)[group] {
		return ErrUnknownGroup
	}
	if _, exists := builder.numaBindings[group]; exists {
		return ErrGroupNUMAConfigured
	}
	if builder.numaBindings == nil {
		builder.numaBindings = make(map[GroupID]numa.Placement)
	}
	builder.numaBindings[group] = placement
	builder.revision++
	return nil
}

// AllocatorDescriptors contains typed inactive allocator hooks for edge storage.
type AllocatorDescriptors[T any] struct {
	Pool  batch.Allocator[T]
	Queue queue.Allocator[T]
}

// Place assigns one registered node to a fixed nonzero execution group.
//
// Errors are selected in builder-validity, builder-sealed, group-ID, node-ID,
// and duplicate-placement order. It only records placement metadata; it does
// not alter topology or invoke callbacks or factories.
func Place(builder *Builder, node NodeID, group GroupID) error {
	if builder == nil || builder.state == builderInvalid || builder.owner == nil {
		return ErrInvalidBuilder
	}
	if builder.state == builderSealed {
		return ErrBuilderSealed
	}
	if group == InvalidGroupID {
		return ErrInvalidGroupID
	}
	if node == InvalidNodeID || int(node) > len(builder.nodes) || builder.nodes[node-1].nodeID != node {
		return ErrUnknownNode
	}
	if _, exists := builder.placements[node]; exists {
		return ErrNodeAlreadyPlaced
	}
	if builder.placements == nil {
		builder.placements = make(map[NodeID]GroupID)
	}
	builder.placements[node] = group
	builder.revision++
	return nil
}

// EnableAutomaticPlanning opts unplaced nodes into free placement constraints.
// It does not perform automatic merging or change the fixed manual Build plan.
func EnableAutomaticPlanning(builder *Builder) error {
	if builder == nil || builder.state == builderInvalid || builder.owner == nil {
		return ErrInvalidBuilder
	}
	if builder.state == builderSealed {
		return ErrBuilderSealed
	}
	builder.planningMode = true
	builder.revision++
	return nil
}

// BindCPU assigns one nonnegative CPU to a currently effective group.
//
// Errors are selected in builder-validity, builder-sealed, group-ID, CPU,
// effective-group, existing-group-binding, and existing-CPU-binding order. It
// records only binding metadata and does not alter placement or runtime state.
func BindCPU(builder *Builder, group GroupID, cpu int) error {
	if builder == nil || builder.state == builderInvalid || builder.owner == nil {
		return ErrInvalidBuilder
	}
	if builder.state == builderSealed {
		return ErrBuilderSealed
	}
	if group == InvalidGroupID {
		return ErrInvalidGroupID
	}
	if cpu < 0 {
		return ErrInvalidCPU
	}
	effective := effectiveGroups(builder.nodes, builder.placements)
	if !effective[group] {
		return ErrUnknownGroup
	}
	if _, exists := builder.cpuBindings[group]; exists {
		return ErrGroupCPUAssigned
	}
	for boundGroup, boundCPU := range builder.cpuBindings {
		if effective[boundGroup] && boundCPU == cpu {
			return ErrCPUAlreadyBound
		}
	}
	if builder.cpuBindings == nil {
		builder.cpuBindings = make(map[GroupID]int)
	}
	builder.cpuBindings[group] = cpu
	builder.revision++
	return nil
}

// InPort is a typed input-port handle. Its zero value is invalid.
type InPort[T any] struct {
	owner  *graphOwner
	nodeID NodeID
	portID PortID
}

// NodeID returns the handle's node identity.
func (p InPort[T]) NodeID() NodeID { return p.nodeID }

// PortID returns the handle's node-local input identity.
func (p InPort[T]) PortID() PortID { return p.portID }

// Valid reports whether the handle belongs to a builder registration.
func (p InPort[T]) Valid() bool {
	return p.owner != nil && p.nodeID != InvalidNodeID && p.portID != InvalidPortID
}

// OutPort is a typed output-port handle. Its zero value is invalid.
type OutPort[T any] struct {
	owner  *graphOwner
	nodeID NodeID
	portID PortID
}

// NodeID returns the handle's node identity.
func (p OutPort[T]) NodeID() NodeID { return p.nodeID }

// PortID returns the handle's node-local output identity.
func (p OutPort[T]) PortID() PortID { return p.portID }

// Valid reports whether the handle belongs to a builder registration.
func (p OutPort[T]) Valid() bool {
	return p.owner != nil && p.nodeID != InvalidNodeID && p.portID != InvalidPortID
}

// ErrorPort is a typed failure-output handle. Its zero value is invalid.
// Error ports can only be connected through ConnectError.
type ErrorPort[E any] struct {
	owner  *graphOwner
	nodeID NodeID
	portID PortID
}

// NodeID returns the handle's node identity.
func (p ErrorPort[E]) NodeID() NodeID { return p.nodeID }

// PortID returns the node-local error-output identity.
func (p ErrorPort[E]) PortID() PortID { return p.portID }

// Valid reports whether the handle belongs to a builder registration.
func (p ErrorPort[E]) Valid() bool {
	return p.owner != nil && p.nodeID != InvalidNodeID && p.portID == errorPortID
}

// OutputRef identifies all ordinary outputs registered for one node.
type OutputRef[O node.OutputSet] struct {
	owner       *graphOwner
	nodeID      NodeID
	outputCount uint8
}

// NodeID returns the reference's node identity.
func (r OutputRef[O]) NodeID() NodeID { return r.nodeID }

// OutputCount returns the registered ordinary output count.
func (r OutputRef[O]) OutputCount() uint8 { return r.outputCount }

// Valid reports whether the reference belongs to a builder registration.
func (r OutputRef[O]) Valid() bool {
	return r.owner != nil && r.nodeID != InvalidNodeID && r.outputCount <= 8
}

// NodePorts is the typed result of a node registration.
type NodePorts[I any, O node.OutputSet, E any] struct {
	nodeID  NodeID
	inputs  I
	outputs OutputRef[O]
	errors  ErrorPort[E]
}

// NodeID returns the registration's node identity.
func (p NodePorts[I, O, E]) NodeID() NodeID { return p.nodeID }

// Inputs returns the node's typed input ports.
func (p NodePorts[I, O, E]) Inputs() I { return p.inputs }

// Outputs returns the node's ordinary output reference.
func (p NodePorts[I, O, E]) Outputs() OutputRef[O] { return p.outputs }

// Errors returns the node's typed error output.
func (p NodePorts[I, O, E]) Errors() ErrorPort[E] { return p.errors }

type registrationKind uint8

const (
	explicitRegistration registrationKind = iota + 1
	closureRegistration
	ingressRegistration
)

const (
	factoryMask uint16 = 1 << iota
	selectorMask
	handler0Mask
	handler1Mask
	handler2Mask
	handler3Mask
	handler4Mask
	handler5Mask
	handler6Mask
	handler7Mask
)

// Source and single-input handler positions share the first handler bit.
const (
	sourceMask = handler0Mask
	input1Mask = handler0Mask
	input2Mask = handler1Mask
	input3Mask = handler2Mask
	input4Mask = handler3Mask
	input5Mask = handler4Mask
	input6Mask = handler5Mask
	input7Mask = handler6Mask
	input8Mask = handler7Mask
)

type staticValidationDescriptor struct {
	required uint16
	present  uint16
}

type normalizationTarget struct {
	currentReplica     uint32
	currentLogicalNode NodeID
	executor           *Executor
	eventSink          func(GroupID) func(node.ErrorEvent) bool
	currentGroup       GroupID
	sourceAllowed      func() bool
	metrics            *metricStore
	edges              *edgeBindingTarget
	currentNode        NodeID
	runners            []runner
	lifecycle          lifecycle
	committed          bool
}

type lifecycle struct {
	init  func() error
	close func() error
}

type nodeNormalizer func(*normalizationTarget) error

type nodeMetadata struct {
	nodeID      NodeID
	kind        registrationKind
	inputCount  uint8
	outputCount uint8
	validation  staticValidationDescriptor
	normalizer  nodeNormalizer
}

type endpointID struct {
	nodeID NodeID
	portID PortID
}

type inputMetadata struct {
	hasProducer bool
}

type outputMetadata struct {
	hasConsumer bool
	isError     bool
}

type edgeMetadata struct {
	id             EdgeID
	source         endpointID
	target         endpointID
	config         queue.Config
	hasFactory     bool
	isError        bool
	fanInMerge     EdgeID
	fanInCount     uint32
	broadcastGroup EdgeID
	broadcastIndex uint32
	broadcastCount uint32
	shardGroup     EdgeID
	shardIndex     uint32
	shardCount     uint32
}

// edgeBinding retains a monomorphized path to one runtime-local edge.
type edgeBinding func(*edgeBindingTarget, EdgeQueuePlan) error

func (b *Builder) register(metadata nodeMetadata) (NodeID, error) {
	if b == nil || b.state == builderInvalid || b.owner == nil {
		return InvalidNodeID, ErrInvalidBuilder
	}
	if b.state == builderSealed {
		return InvalidNodeID, ErrBuilderSealed
	}
	if b.nextNodeID == InvalidNodeID {
		return InvalidNodeID, ErrNodeIDExhausted
	}

	nodeID := b.nextNodeID
	b.nextNodeID++
	metadata.nodeID = nodeID
	b.nodes = append(b.nodes, metadata)
	b.revision++
	if b.inputs == nil {
		b.inputs = make(map[endpointID]*inputMetadata)
	}
	if b.outputs == nil {
		b.outputs = make(map[endpointID]*outputMetadata)
	}
	for portID := PortID(1); portID <= PortID(metadata.inputCount); portID++ {
		b.inputs[endpointID{nodeID: nodeID, portID: portID}] = &inputMetadata{}
	}
	for portID := PortID(1); portID <= PortID(metadata.outputCount); portID++ {
		b.outputs[endpointID{nodeID: nodeID, portID: portID}] = &outputMetadata{}
	}
	b.outputs[endpointID{nodeID: nodeID, portID: errorPortID}] = &outputMetadata{isError: true}
	return nodeID, nil
}

func outputCount[O node.OutputSet]() uint8 {
	var outputs O
	return outputs.OutputCount()
}

// EdgeConfig configures a future typed graph edge.
type EdgeConfig[T any] struct {
	Capacity      uint32
	BatchCapacity uint32
	Policy        queue.Policy
	Factory       queue.Factory[T]
	NUMA          numa.Placement
	Allocators    AllocatorDescriptors[T]
}

var (
	// ErrInvalidPort reports a zero, malformed, or unknown graph port.
	ErrInvalidPort = errors.New("invalid graph port")
	// ErrForeignPort reports a port owned by another graph builder.
	ErrForeignPort = errors.New("graph port belongs to another builder")
	// ErrInputConnected reports an input that already has a producer.
	ErrInputConnected = errors.New("graph input already has a producer")
	// ErrFanOutUnsupported reports an output that already has a consumer.
	ErrFanOutUnsupported = errors.New("graph output fan-out is unsupported")
	ErrFanInUnsupported  = errors.New("graph fan-in requires multiple distinct producers")
	ErrInvalidFanIn      = errors.New("invalid graph fan-in producer set")
	// ErrBroadcastParallelExpansion reports broadcast endpoints expanded by replicas or node pools.
	ErrBroadcastParallelExpansion = errors.New("broadcast endpoints cannot be replicated or pooled")
	// ErrShardParallelExpansion reports sharding endpoints expanded by replicas or node pools.
	ErrShardParallelExpansion = errors.New("sharding endpoints cannot be replicated or pooled")
	// ErrInvalidEdgeConfig reports an invalid future queue configuration.
	ErrInvalidEdgeConfig = errors.New("invalid graph edge configuration")
)

// EdgeConfigError describes an invalid future queue configuration.
type EdgeConfigError struct {
	Config queue.Config

	capacityInvalid      bool
	batchCapacityInvalid bool
	policyInvalid        bool
	numaInvalid          bool
}

// Error returns the stable invalid-edge-configuration category message.
func (e *EdgeConfigError) Error() string { return ErrInvalidEdgeConfig.Error() }

// Unwrap identifies the invalid-edge-configuration error category.
func (e *EdgeConfigError) Unwrap() error { return ErrInvalidEdgeConfig }

func validateEdgeConfig(config queue.Config) error {
	capacityInvalid := config.Capacity == 0
	batchCapacityInvalid := config.BatchCapacity == 0
	policyInvalid := config.Policy != queue.Drop && config.Policy != queue.Wait
	numaInvalid := config.NUMA.Validate() != nil
	if capacityInvalid || batchCapacityInvalid || policyInvalid || numaInvalid {
		return &EdgeConfigError{
			Config:               config,
			capacityInvalid:      capacityInvalid,
			batchCapacityInvalid: batchCapacityInvalid,
			policyInvalid:        policyInvalid,
			numaInvalid:          numaInvalid,
		}
	}
	return nil
}

// Connect creates a typed edge between matching ordinary ports on an open builder.
//
// It validates all local invariants before committing structural metadata. A
// nil Factory selects the future built-in queue; factories remain dormant until
// a future runtime binding stage.
func Connect[T any](
	builder *Builder,
	output OutPort[T],
	input InPort[T],
	config EdgeConfig[T],
) (EdgeID, error) {
	if builder == nil || builder.state == builderInvalid || builder.owner == nil {
		return InvalidEdgeID, ErrInvalidBuilder
	}
	if builder.state == builderSealed {
		return InvalidEdgeID, ErrBuilderSealed
	}
	if !output.Valid() {
		return InvalidEdgeID, ErrInvalidPort
	}
	if !input.Valid() {
		return InvalidEdgeID, ErrInvalidPort
	}
	source := endpointID{nodeID: output.nodeID, portID: output.portID}
	target := endpointID{nodeID: input.nodeID, portID: input.portID}
	outputMetadata, ok := builder.outputs[source]
	if output.owner == builder.owner && !ok {
		return InvalidEdgeID, ErrInvalidPort
	}
	inputMetadata, ok := builder.inputs[target]
	if input.owner == builder.owner && !ok {
		return InvalidEdgeID, ErrInvalidPort
	}
	if output.owner != builder.owner {
		return InvalidEdgeID, ErrForeignPort
	}
	if input.owner != builder.owner {
		return InvalidEdgeID, ErrForeignPort
	}
	if outputMetadata.isError {
		return InvalidEdgeID, ErrInvalidPort
	}
	if inputMetadata.hasProducer {
		return InvalidEdgeID, ErrInputConnected
	}
	if outputMetadata.hasConsumer {
		return InvalidEdgeID, ErrFanOutUnsupported
	}

	queueConfig := queue.Config{Capacity: config.Capacity, BatchCapacity: config.BatchCapacity, Policy: config.Policy, NUMA: config.NUMA}
	if err := validateEdgeConfig(queueConfig); err != nil {
		return InvalidEdgeID, err
	}

	id := EdgeID(len(builder.edges) + 1)
	binding := futureBinding(id, source, target, queueConfig, config.Factory, false, config.Allocators)
	builder.edges = append(builder.edges, edgeMetadata{
		id:         id,
		source:     source,
		target:     target,
		config:     queueConfig,
		hasFactory: config.Factory != nil,
		isError:    false,
	})
	builder.revision++
	builder.edgeBindings = append(builder.edgeBindings, binding)
	inputMetadata.hasProducer = true
	outputMetadata.hasConsumer = true
	return id, nil
}

// ConnectError creates a typed edge from a registered ErrorPort to an ordinary
// input. It shares Connect's validation and ownership semantics while keeping
// failure routes explicit in the public type system.
func ConnectError[E any](
	builder *Builder,
	output ErrorPort[E],
	input InPort[E],
	config EdgeConfig[E],
) (EdgeID, error) {
	if builder == nil || builder.state == builderInvalid || builder.owner == nil {
		return InvalidEdgeID, ErrInvalidBuilder
	}
	if builder.state == builderSealed {
		return InvalidEdgeID, ErrBuilderSealed
	}
	if !output.Valid() {
		return InvalidEdgeID, ErrInvalidPort
	}
	if !input.Valid() {
		return InvalidEdgeID, ErrInvalidPort
	}
	source := endpointID{nodeID: output.nodeID, portID: output.portID}
	target := endpointID{nodeID: input.nodeID, portID: input.portID}
	outputMetadata, ok := builder.outputs[source]
	if output.owner == builder.owner && (!ok || !outputMetadata.isError) {
		return InvalidEdgeID, ErrInvalidPort
	}
	inputMetadata, ok := builder.inputs[target]
	if input.owner == builder.owner && !ok {
		return InvalidEdgeID, ErrInvalidPort
	}
	if output.owner != builder.owner {
		return InvalidEdgeID, ErrForeignPort
	}
	if input.owner != builder.owner {
		return InvalidEdgeID, ErrForeignPort
	}
	if inputMetadata.hasProducer {
		return InvalidEdgeID, ErrInputConnected
	}
	if outputMetadata.hasConsumer {
		return InvalidEdgeID, ErrFanOutUnsupported
	}
	queueConfig := queue.Config{Capacity: config.Capacity, BatchCapacity: config.BatchCapacity, Policy: config.Policy, NUMA: config.NUMA}
	if err := validateEdgeConfig(queueConfig); err != nil {
		return InvalidEdgeID, err
	}

	id := EdgeID(len(builder.edges) + 1)
	builder.edges = append(builder.edges, edgeMetadata{
		id:         id,
		source:     source,
		target:     target,
		config:     queueConfig,
		hasFactory: config.Factory != nil,
		isError:    true,
	})
	builder.revision++
	builder.edgeBindings = append(builder.edgeBindings, futureBinding(id, source, target, queueConfig, config.Factory, true, config.Allocators))
	inputMetadata.hasProducer = true
	outputMetadata.hasConsumer = true
	return id, nil
}
