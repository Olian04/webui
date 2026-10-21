package ir

import "context"

// Toolbar is a leaf that is a row of controls for the page it is on: buttons, and the values of the page's arguments, read-only.
// What each is about is the page's arguments, as they are for the request.
type Toolbar struct {
	At Addr

	// Items are the controls: the ones for the left, and then the ones for the right, which start at RightFrom.
	Items     []ToolbarItem
	RightFrom int
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

	// Field is a value of the page's arguments, shown read-only: Get reads it from them.
	Field *Field
}

// Addr reports the node's position in the body tree.
func (t *Toolbar) Addr() Addr { return t.At }

// Values are the fields of the toolbar's values, in order.
func (t *Toolbar) Values() []Field {
	var out []Field
	for _, it := range t.Items {
		if it.Field != nil {
			out = append(out, *it.Field)
		}
	}
	return out
}

// Resolved is the toolbar with the choices of its selects read, for the request being answered (what a badge says of a value): a
// copy when a select reads them, and the toolbar itself when none does.
func (t *Toolbar) Resolved(ctx context.Context) (*Toolbar, error) {
	inputs := t.Values()
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
