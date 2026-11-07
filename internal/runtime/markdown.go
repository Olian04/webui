package runtime

import (
	"context"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// markdown loads a markdown panel and builds its view. A failed Load fails that panel,
// not the page. The page is given the editor only when the text has a fenced block
// that names a language, which is all it would colour.
func (p *Program) markdown(ctx context.Context, req *Request, page *ir.Page, n *ir.Markdown) templ.Component {
	model, ok, refusal := loaded(p, ctx, page, "markdown", "markdown", n.At, func() (any, error) { return n.Load(ctx) })
	if !ok {
		path, query := req.encode(page)
		return p.render.Markdown(render.LoadedMarkdown{Node: n, Failed: true, Refusal: refusal, Retry: p.render.PageHref(page, path, query)})
	}
	view := p.render.LoadedMarkdown(n, n.Content.Get(model))
	if view.Result.Highlight {
		req.code.Add("")
		p.warnNoHighlight(page)
	}
	return p.render.Markdown(view)
}

// warnNoHighlight says, once for each page, that a Markdown has a code block that names a language and the program has no editor to colour it with, which is how
// to put it right. It is said only when the app was given a logger, and the block is drawn plain either way: nothing is wrong that the visitor could be told.
func (p *Program) warnNoHighlight(page *ir.Page) {
	if p.render.HasEditor() {
		return
	}
	if _, said := p.unhighlighted.LoadOrStore(page.PathTemplate, true); said {
		return
	}
	p.log.Warn("webui: a Markdown has a code block that names a language, and it is drawn without colour because this program does not carry the code editor that colours it; "+
		`to colour it, import the editor for its effect: import _ "github.com/Olian04/webui/pkg/monaco"`, "page", page.PathTemplate)
}
