package runtime

import (
	"context"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// diff loads a diff leaf and builds its view. A failed Load fails that panel, not the
// page. A diff only ever reads, so it is only ever highlighted: it asks the page for the
// editor and for no language service.
func (p *Program) diff(ctx context.Context, req *Request, page *ir.Page, n *ir.Diff) templ.Component {
	model, ok, refusal := loaded(p, ctx, page, "diff", "diff", n.At, func() (any, error) { return n.Load(ctx) })
	if !ok {
		return p.render.Diff(render.DiffView{Node: n, Failed: true, Refusal: refusal})
	}
	req.code.Add("")
	return p.render.Diff(render.NewDiffView(n, n.Original.Get(model), n.Modified.Get(model)))
}
