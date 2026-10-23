package runtime

import (
	"context"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// markdown loads a markdown leaf and builds its view. A failed Load fails that panel,
// not the page. The page is given the editor only when the text has a fenced block
// that names a language, which is all it would colour.
func (p *Program) markdown(ctx context.Context, req *Request, page *ir.Page, n *ir.Markdown) templ.Component {
	model, ok, refusal := loaded(p, ctx, page, "markdown", "markdown", n.At, func() (any, error) { return n.Load(ctx) })
	if !ok {
		path, query := req.encode(page)
		return p.render.Markdown(render.MarkdownView{Node: n, Failed: true, Refusal: refusal, Retry: p.render.PageHref(page, path, query)})
	}
	view := p.render.MarkdownView(n, n.Content.Get(model))
	if view.Result.Highlight {
		req.code.Add("")
	}
	return p.render.Markdown(view)
}
