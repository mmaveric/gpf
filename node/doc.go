// Package node defines fixed-arity typed node contracts.
//
// A future scheduler invokes one input handler per node call and uses concrete
// function values rather than interface dispatch. Ordinary output arity is
// limited to eight; ErrorPort is separate and does not count toward that limit.
package node
