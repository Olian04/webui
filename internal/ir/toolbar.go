package ir

import "context"

// Toolbar is a leaf that is a row of controls for the page it is on: buttons, links, and the values of the models that its forms load, read-only.
// It is the leaf, and so what is refreshed: the forms in it have no address of their own.
type Toolbar struct {
	At Addr

	// Items are the controls: the ones for the left, and then the ones for the right, which start at RightFrom.
	Items     []ToolbarItem
	RightFrom int
}

// ToolbarItem is one control of a toolbar: exactly one of Action, Link and Form is set.
type ToolbarItem struct {
	// Label and Role are of a button, an Action or a Link.
	Label string
	Role  Role

	// Action is a button that does something. It has no subject: what it needs of the page it reads from the request.
	Action *Action

	// Link is a button that goes to another page. Its Args are built from the request, with no subject.
	Link *Link

	// Form is a model that the toolbar loads and shows the values of, read-only: its Title names the group, and its Fields are shown in a row.
	// It is never submitted from a toolbar.
	Form *Form
}

// Addr reports the node's position in the body tree.
func (t *Toolbar) Addr() Addr { return t.At }

// Values are the fields of the toolbar's forms, in order.
func (t *Toolbar) Values() []Field {
	var out []Field
	for _, it := range t.Items {
		if it.Form != nil {
			out = append(out, it.Form.Fields...)
		}
	}
	return out
}

// Resolved is the toolbar with the choices of its forms' selects read, for the request being answered (what a badge says of a value): a
// copy when a select reads them, and the toolbar itself when none does.
func (t *Toolbar) Resolved(ctx context.Context) (*Toolbar, error) {
	out := t
	for i, it := range t.Items {
		if it.Form == nil {
			continue
		}
		resolved, err := it.Form.Resolved(ctx)
		if err != nil {
			return nil, err
		}
		if resolved == it.Form {
			continue
		}
		if out == t {
			copied := *t
			copied.Items = append([]ToolbarItem(nil), t.Items...)
			out = &copied
		}
		out.Items[i].Form = resolved
	}
	return out, nil
}
