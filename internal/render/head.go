package render

import (
	"context"
	"io"

	"github.com/a-h/templ"

	c "github.com/Olian04/webui/internal/render/templates/components"

	"github.com/Olian04/webui/internal/ir"
)

// LoadedHead is the controls in the head of a panel, with what they are for this visitor: the same as a toolbar's buttons and links. The runtime fills it.
type LoadedHead struct {
	At   ir.Pos // the panel's
	Page *ir.Page

	// Path and Query are the page's current address, as its arguments and view state encode.
	Path, Query map[string]string

	// Items are the controls, by the position they were declared at, which is what a button posts. Access is what each is for this visitor, and Hrefs where each link goes.
	Items  []ir.ToolbarItem
	Access []Access
	Hrefs  []string
}

func (v LoadedHead) access(i int) Access {
	if i < len(v.Access) {
		return v.Access[i]
	}
	return Access{}
}

// posts is whether the head has a button that posts, and so a form to post it with.
func (v LoadedHead) posts() bool {
	for i, it := range v.Items {
		if it.Action != nil && v.access(i).Drawn() {
			return true
		}
	}
	return false
}

// drawn is whether any control is drawn for this visitor.
func (v LoadedHead) drawn() bool {
	for i := range v.Items {
		if v.access(i).Drawn() {
			return true
		}
	}
	return false
}

// Head draws a panel with the controls in its head, which the panel puts at the right of its title.
func (r *Renderer) Head(comp templ.Component, h LoadedHead) templ.Component {
	if !h.drawn() {
		return comp
	}
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return comp.Render(c.WithHead(ctx, r.headActions(h)), w) //nolint:wrapcheck // a component's own error
	})
}
