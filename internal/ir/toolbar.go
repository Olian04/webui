package ir

import "context"

// Toolbar is a leaf that is a row of controls for the page it is on: buttons, and inputs for the page's arguments.
// What each is about is the page's arguments, as they are for the request.
type Toolbar struct {
	At Addr

	// Items are the controls: the ones for the left, and then the ones for the right, which start at RightFrom.
	Items     []ToolbarItem
	RightFrom int

	// Bind applies the submitted values of the inputs, keyed by Field.Name, to a copy of the page's arguments, and
	// returns it for the runtime to encode. An input that was emptied sets its argument to nothing. fields are the
	// inputs as they are for the request, with the choices of their selects read (see [ResolveChoices]); every
	// failure is returned, as for a form.
	Bind func(fields []Field, base any, values map[string]string) (args any, errs []FieldError)
}

// ToolbarItem is one control of a toolbar: exactly one of Action, Link and Field is set.
type ToolbarItem struct {
	// Label and Role are of a button, an Action or a Link.
	Label string
	Role  Role

	// Action is a button that does something: its subject is the page's arguments.
	Action *Action

	// Link is a button that goes to another page: its Args are built from the page's arguments.
	Link *Link

	// Field is an input for the page's arguments: Get reads its value from them, and Set writes it to them.
	Field *Field
}

// Addr reports the node's position in the body tree.
func (t *Toolbar) Addr() Addr { return t.At }

// Inputs are the fields of the toolbar's inputs, in order.
func (t *Toolbar) Inputs() []Field {
	var out []Field
	for _, it := range t.Items {
		if it.Field != nil {
			out = append(out, *it.Field)
		}
	}
	return out
}

// Resolved is the toolbar with the choices of its selects read, for the request being answered: a copy when a select
// reads them, and the toolbar itself when none does.
func (t *Toolbar) Resolved(ctx context.Context) (*Toolbar, error) {
	inputs := t.Inputs()
	fields, err := ResolveChoices(ctx, inputs)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 || &fields[0] == &inputs[0] {
		return t, nil
	}
	out := *t
	out.Items = make([]ToolbarItem, len(t.Items))
	next := 0
	for i, it := range t.Items {
		if it.Field != nil {
			f := fields[next]
			it.Field = &f
			next++
		}
		out.Items[i] = it
	}
	return &out, nil
}
