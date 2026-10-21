package render

import (
	"github.com/Olian04/webui/internal/ir"
)

// ToolbarView is a toolbar with what it needs of this request: the page's arguments, why each button may not be used,
// and where each link goes. The runtime fills it.
type ToolbarView struct {
	Node *ir.Toolbar
	Page *ir.Page

	// Path and Query are the page's current address, as its arguments and view state encode.
	Path, Query map[string]string

	// Args are the page's arguments, which the values show.
	Args any

	// Gates is, for each item, why the visitor may not use it, "" when they may. Hrefs is where each link goes.
	Gates, Hrefs []string

	Failed bool // the options of a value could not be read: the toolbar says so, the page stands
}

// hasActions is whether the toolbar has a button that posts, and so a form to post it with. A toolbar of links and values has none:
// there is nothing in it to submit.
func (v ToolbarView) hasActions() bool {
	for _, it := range v.Node.Items {
		if it.Action != nil {
			return true
		}
	}
	return false
}
