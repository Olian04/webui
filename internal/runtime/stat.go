package runtime

import (
	"context"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// stat loads a stat leaf and builds its view. A failed Load fails that panel, not the page.
func (p *Program) stat(ctx context.Context, req *Request, page *ir.Page, n *ir.Stat) templ.Component {
	model, ok, refusal := loaded(p, ctx, page, "stat", "stat", n.At, func() (any, error) { return n.Load(ctx) })
	if !ok {
		path, query := req.encode(page)
		return p.render.Stat(render.StatView{Node: n, Failed: true, Refusal: refusal, Retry: p.render.PageHref(page, path, query)})
	}
	return p.render.Stat(render.StatView{Node: n, Model: model})
}
