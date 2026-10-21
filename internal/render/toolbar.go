package render

import (
	"github.com/Olian04/webui/internal/ir"
)

// ToolbarView is a toolbar with what it needs of this request: the page's arguments, what each button is for this visitor,
// and where each link goes. The runtime fills it.
type ToolbarView struct {
	Node *ir.Toolbar
	Page *ir.Page

	// Path and Query are the page's current address, as its arguments and view state encode.
	Path, Query map[string]string

	// Args are the page's arguments, which the values show.
	Args any

	// Gates is what each item is for this visitor, by the position it was declared at: a button whose Guard refuses them, or a link
	// to a page that does, is not drawn, or is drawn disabled. Hrefs is where each link goes.
	Gates []Gate
	Hrefs []string

	Failed bool // the options of a value could not be read: the toolbar says so, the page stands
}

// hasActions is whether the toolbar has a button that posts, and so a form to post it with. A toolbar of links and values has none:
// there is nothing in it to submit.
func (v ToolbarView) hasActions() bool {
	for i, it := range v.Node.Items {
		if it.Action != nil && v.gate(i).Drawn() {
			return true
		}
	}
	return false
}

// drawn is whether any item is drawn for this visitor. A toolbar whose items are all refused to them is not there at all.
func (v ToolbarView) drawn() bool {
	for i := range v.Node.Items {
		if v.gate(i).Drawn() {
			return true
		}
	}
	return false
}

// gate is what item i is for this visitor.
func (v ToolbarView) gate(i int) Gate {
	if i < len(v.Gates) {
		return v.Gates[i]
	}
	return Gate{}
}
