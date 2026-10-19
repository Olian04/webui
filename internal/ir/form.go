package ir

import (
	"context"

	"github.com/Olian04/webui/internal/editorctx"
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
	//
	// fields are the form's Fields as they are for this request, with the choices of its selects read (see
	// [ResolveChoices]), which a select's values are checked against.
	Bind func(fields []Field, base any, values map[string]string) (model any, errs []FieldError)
}

// Editor is what makes a Form a code editor.
type Editor struct {
	Language string // Monaco's id for it

	// Service is the language service that covers the language, "" for none: only an
	// editor that can be edited is given it.
	Service string

	// Context is what the language service is given besides the text: declaration files and
	// compiler options for TypeScript, schemas for JSON. Nil for none. Like Service, it goes
	// only to an editor that can be edited.
	Context *editorctx.Context
}

// Addr reports the node's position in the body tree.
func (f *Form) Addr() Addr { return f.At }

// Resolved is the form with the choices of its selects read, for the request being answered: a copy when a select
// reads them, and the form itself when none does.
func (f *Form) Resolved(ctx context.Context) (*Form, error) {
	fields, err := ResolveChoices(ctx, f.Fields)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 || len(f.Fields) == 0 || &fields[0] == &f.Fields[0] {
		return f, nil
	}
	out := *f
	out.Fields = fields
	return &out, nil
}
