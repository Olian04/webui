package render

import (
	"context"

	"github.com/Olian04/webui/internal/ir"
)

// ToolbarParam is the parameter of the form that applies a toolbar's inputs: its value is the toolbar's id.
const ToolbarParam = "_toolbar"

// ToolbarView is a toolbar with what it needs of this request: the page's arguments, why each button may not be used,
// and where each link goes. The runtime fills it.
type ToolbarView struct {
	Node *ir.Toolbar
	Page *ir.Page

	// Path and Query are the page's current address, as its arguments and view state encode.
	Path, Query map[string]string

	// Args are the page's arguments, which the inputs show.
	Args any

	// Gates is, for each item, why the visitor may not use it, "" when they may. Hrefs is where each link goes.
	Gates, Hrefs []string

	Failed bool // the options of an input could not be read: the toolbar says so, the page stands
}

// formID is the id of the form that applies the inputs.
func (v ToolbarView) formID() string { return fieldID(v.Node.At, "args") }

// hasInputs is whether the toolbar has an input, and so a form to apply them.
func (v ToolbarView) hasInputs() bool {
	for _, it := range v.Node.Items {
		if it.Field != nil {
			return true
		}
	}
	return false
}

// toolbarInput is an input of a toolbar as the field of a form that is about the page's arguments: it is always
// editable, since the page is, and belongs to the form that applies the inputs.
func (r *Renderer) toolbarInput(ctx context.Context, v ToolbarView, f ir.Field) fieldView {
	about := FormView{Node: &ir.Form{At: v.Node.At, Submit: &ir.Action{}}, Page: v.Page, Model: v.Args}
	fv := r.fieldView(ctx, about, f)
	fv.FormID = v.formID()
	return fv
}
