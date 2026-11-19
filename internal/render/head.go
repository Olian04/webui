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
	Controls

	At   ir.Pos // the panel's
	Page *ir.Page

	// Path and Query are the page's current address, as its arguments and view state encode.
	Path, Query map[string]string
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
