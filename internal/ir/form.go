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

	// Editor, when set, says the form is a code editor: it has one field, whose text is
	// edited (or, with no way to store it, read) in an editor of a language, not in an
	// input. It is a form in every other way: it loads, guards, binds and submits as one.
	Editor *Editor

	// Bind applies submitted values, keyed by Field.Name, to a copy of base —
	// the model Load returned. Starting from the loaded model, not a zero one,
	// is what keeps read-only fields (an ID with no Store) intact for Guard and
	// Run. It checks every writable field's rules and applies every value it
	// can, returning every failure rather than stopping at the first, so one
	// round trip shows the user all of them.
	Bind func(base any, values map[string]string) (model any, errs []FieldError)
}

// Editor is what makes a Form a code editor.
type Editor struct {
	Language string // Monaco's id for it

	// Service is the language service that covers the language, "" for none: only an
	// editor that can be edited is given it.
	Service string
}

// Addr reports the node's position in the body tree.
func (f *Form) Addr() Addr { return f.At }
