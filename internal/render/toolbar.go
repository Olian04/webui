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

	// Controls are the buttons and links, by the position they were declared at, with what each is for this visitor: a button whose Guard refuses them, or a link to a page
	// that does, is not drawn, or is drawn disabled; and where each link goes.
	Controls

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
