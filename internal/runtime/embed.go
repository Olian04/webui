package runtime

import (
	"context"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// embed builds an embedded page's panel: the address that its frame shows, from its Load, which is the app's own and is asked for each request. An address that is not a
// path on this site is the app's mistake, and fails that panel and not the page: it is logged, and the visitor is told no more than that it could not load, as for any panel.
func (p *Program) embed(ctx context.Context, req *Request, page *ir.Page, n *ir.Embed) templ.Component {
	path, query := req.encode(page)
	retry := p.render.PageHref(page, path, query)
	model, ok, refusal := loaded(p, ctx, page, "embed", "embed", n.At, func() (any, error) { return n.Load(ctx) })
	if !ok {
		return p.render.Embed(render.LoadedEmbed{Node: n, Failed: true, Refusal: refusal, Retry: retry})
	}
	src, _ := model.(string)
	if !ir.SamePath(src) {
		p.log.Error("webui: an Embed's Load returned an address that is not a path on this site, so the frame is not drawn", "page", page.PathTemplate, "view", render.ViewID(n.At), "address", src)
		return p.render.Embed(render.LoadedEmbed{Node: n, Failed: true, Retry: retry})
	}
	return p.render.Embed(render.LoadedEmbed{Node: n, Src: src, Retry: retry})
}
