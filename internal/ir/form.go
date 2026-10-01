package ir

import (
	"context"
)

// Form is a leaf holding one model.
type Form struct {
	At Addr

	// Load returns the model. Its concrete type is known only to the closures
	// pkg/webui built; downstream passes it back opaquely.
	Load func(ctx context.Context) (any, error)

	Fields []Field
	Submit *Action

	// Bind applies submitted values to a fresh model. It applies everything it
	// can and returns every failure, rather than stopping at the first, so one
	// round trip shows the user all of them.
	Bind func(ctx context.Context, values map[string]string) (model any, errs []FieldError)
}

// Kind reports the node kind.
func (f *Form) Kind() NodeKind { return NodeForm }

// Addr reports the node's position in the body tree.
func (f *Form) Addr() Addr { return f.At }
