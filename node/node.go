package node

import "github.com/asharipov/gpf/batch"

// Result is the typed outcome of a node handler.
type Result[E any] struct {
	Value  E
	Failed bool
}

// ErrorPort is a typed error-output declaration. It is separate from ordinary
// outputs and therefore does not count toward the eight-output limit.
type ErrorPort[E any] struct {
	_ [0]func(E)
}

// Outputs0 declares a node with no ordinary outputs.
type Outputs0 struct{}

// Outputs1 declares a node with one ordinary typed output.
type Outputs1[O1 any] struct{ _ [0]func(O1) }

// Outputs2 declares a node with two ordinary typed outputs.
type Outputs2[O1, O2 any] struct{ _ [0]func(O1, O2) }

// Outputs3 declares a node with three ordinary typed outputs.
type Outputs3[O1, O2, O3 any] struct{ _ [0]func(O1, O2, O3) }

// Outputs4 declares a node with four ordinary typed outputs.
type Outputs4[O1, O2, O3, O4 any] struct{ _ [0]func(O1, O2, O3, O4) }

// Outputs5 declares a node with five ordinary typed outputs.
type Outputs5[O1, O2, O3, O4, O5 any] struct{ _ [0]func(O1, O2, O3, O4, O5) }

// Outputs6 declares a node with six ordinary typed outputs.
type Outputs6[O1, O2, O3, O4, O5, O6 any] struct {
	_ [0]func(O1, O2, O3, O4, O5, O6)
}

// Outputs7 declares a node with seven ordinary typed outputs.
type Outputs7[O1, O2, O3, O4, O5, O6, O7 any] struct {
	_ [0]func(O1, O2, O3, O4, O5, O6, O7)
}

// Outputs8 declares a node with eight ordinary typed outputs.
type Outputs8[O1, O2, O3, O4, O5, O6, O7, O8 any] struct {
	_ [0]func(O1, O2, O3, O4, O5, O6, O7, O8)
}

// Handler0 processes a source-like node invocation.
type Handler0[S, O, E any] func(state *S, outputs *O) Result[E]

// Handler1 processes one input batch. The handler owns input for the duration
// of the call; a future runtime releases it unless the handler moves it to a
// compatible output.
type Handler1[S, I, O, E any] func(state *S, input batch.Batch[I], outputs *O) Result[E]

// Node0 declares a source-like node with no inputs.
type Node0[S, O, E any] struct {
	Handle Handler0[S, O, E]
	Errors ErrorPort[E]
}

// Node1 declares a node with one input.
type Node1[S, I1, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Errors  ErrorPort[E]
}

// Node2 declares a node with two inputs and separate handlers for each input.
type Node2[S, I1, I2, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Handle2 Handler1[S, I2, O, E]
	Errors  ErrorPort[E]
}

// Node3 declares a node with three inputs and separate handlers for each input.
type Node3[S, I1, I2, I3, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Handle2 Handler1[S, I2, O, E]
	Handle3 Handler1[S, I3, O, E]
	Errors  ErrorPort[E]
}

// Node4 declares a node with four inputs and separate handlers for each input.
type Node4[S, I1, I2, I3, I4, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Handle2 Handler1[S, I2, O, E]
	Handle3 Handler1[S, I3, O, E]
	Handle4 Handler1[S, I4, O, E]
	Errors  ErrorPort[E]
}

// Node5 declares a node with five inputs and separate handlers for each input.
type Node5[S, I1, I2, I3, I4, I5, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Handle2 Handler1[S, I2, O, E]
	Handle3 Handler1[S, I3, O, E]
	Handle4 Handler1[S, I4, O, E]
	Handle5 Handler1[S, I5, O, E]
	Errors  ErrorPort[E]
}

// Node6 declares a node with six inputs and separate handlers for each input.
type Node6[S, I1, I2, I3, I4, I5, I6, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Handle2 Handler1[S, I2, O, E]
	Handle3 Handler1[S, I3, O, E]
	Handle4 Handler1[S, I4, O, E]
	Handle5 Handler1[S, I5, O, E]
	Handle6 Handler1[S, I6, O, E]
	Errors  ErrorPort[E]
}

// Node7 declares a node with seven inputs and separate handlers for each input.
type Node7[S, I1, I2, I3, I4, I5, I6, I7, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Handle2 Handler1[S, I2, O, E]
	Handle3 Handler1[S, I3, O, E]
	Handle4 Handler1[S, I4, O, E]
	Handle5 Handler1[S, I5, O, E]
	Handle6 Handler1[S, I6, O, E]
	Handle7 Handler1[S, I7, O, E]
	Errors  ErrorPort[E]
}

// Node8 declares a node with eight inputs and separate handlers for each input.
type Node8[S, I1, I2, I3, I4, I5, I6, I7, I8, O, E any] struct {
	Handle1 Handler1[S, I1, O, E]
	Handle2 Handler1[S, I2, O, E]
	Handle3 Handler1[S, I3, O, E]
	Handle4 Handler1[S, I4, O, E]
	Handle5 Handler1[S, I5, O, E]
	Handle6 Handler1[S, I6, O, E]
	Handle7 Handler1[S, I7, O, E]
	Handle8 Handler1[S, I8, O, E]
	Errors  ErrorPort[E]
}
