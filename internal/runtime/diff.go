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
	model, err := n.Load(ctx)
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: diff load failed", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
		}
		return p.render.Diff(render.DiffView{Node: n, Failed: true})
	}
	req.code.Add("")
	return p.render.Diff(render.NewDiffView(n, n.Original.Get(model), n.Modified.Get(model)))
}
