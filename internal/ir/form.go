package ir

import (
	"context"
)

// Form is a leaf holding one model.
type Form struct {
	At Addr

	Title string
	Desc  string

	// Load returns the model. Its concrete type is known only to the closures
	// pkg/webui built; downstream passes it back opaquely.
	Load func(ctx context.Context) (any, error)

	Fields []Field
	Submit *Action

	// Bind applies submitted values, keyed by Field.Name, to a copy of base —
	// the model Load returned. Starting from the loaded model, not a zero one,
	// is what keeps read-only fields (an ID with no Store) intact for Guard and
	// Run. It checks every writable field's rules and applies every value it
	// can, returning every failure rather than stopping at the first, so one
	// round trip shows the user all of them.
	Bind func(base any, values map[string]string) (model any, errs []FieldError)
}

// Kind reports the node kind.
func (f *Form) Kind() NodeKind { return NodeForm }

// Addr reports the node's position in the body tree.
func (f *Form) Addr() Addr { return f.At }
