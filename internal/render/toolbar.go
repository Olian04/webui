package render

import (
	"github.com/Olian04/webui/internal/ir"
)

// LoadedToolbar is a toolbar with what it needs of this request: what each button is for this visitor, where each link goes, and the
// model that each of its forms loaded. The runtime fills it.
type LoadedToolbar struct {
	Node *ir.Toolbar
	Page *ir.Page

	// Path and Query are the page's current address, as its arguments and view state encode.
	Path, Query map[string]string

	// Access is what each item is for this visitor, by the position it was declared at: a button whose Guard refuses them, or a link
	// to a page that does, is not drawn, or is drawn disabled. Hrefs is where each link goes.
	Access []Access
	Hrefs  []string

	// Forms is what each form of the toolbar loaded, by the position it was declared at: the model that its values show, or that it could not
	// be loaded, which its place in the row says without taking the rest of the toolbar down.
	Forms []ToolbarForm

	Failed bool // the options of a value could not be read: the toolbar says so, the page stands
}

// ToolbarForm is what a form of a toolbar loaded: its model, or why there is none.
type ToolbarForm struct {
	Model   any
	Failed  bool
	Refusal *ir.Refusal // what the visitor is told when the load was refused, nil for a failure
}

// form is what the form at item i loaded.
func (v LoadedToolbar) form(i int) ToolbarForm {
	if i < len(v.Forms) {
		return v.Forms[i]
	}
	return ToolbarForm{}
}

// hasActions is whether the toolbar has a button that posts, and so a form to post it with. A toolbar of links and values has none:
// there is nothing in it to submit.
func (v LoadedToolbar) hasActions() bool {
	for i, it := range v.Node.Items {
		if it.Action != nil && v.access(i).Drawn() {
			return true
		}
	}
	return false
}

// drawn is whether any item is drawn for this visitor. A toolbar whose items are all refused to them is not there at all.
func (v LoadedToolbar) drawn() bool {
	for i := range v.Node.Items {
		if v.access(i).Drawn() {
			return true
		}
	}
	return false
}

// access is what item i is for this visitor.
func (v LoadedToolbar) access(i int) Access {
	if i < len(v.Access) {
		return v.Access[i]
	}
	return Access{}
}
