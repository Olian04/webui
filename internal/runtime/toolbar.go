package runtime

import (
	"context"
	"net/http"
	"strconv"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// toolbar builds a toolbar's view: what a button is for this visitor, where a link goes, and the model that each form of it loads, for the values to show.
// A source of options that fails fails the toolbar, which says so, and not the page; a form whose Load fails is its place in the row saying so, and the
// rest of the toolbar stands.
func (p *Program) toolbar(ctx context.Context, req *Request, page *ir.Page, n *ir.Toolbar) templ.Component {
	view := render.LoadedToolbar{Node: n, Page: page}
	view.Path, view.Query = req.encode(page)
	resolved, err := resolve(p, page, n, func() (*ir.Toolbar, error) { return n.Resolved(ctx) })
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: the options of a toolbar could not be read", "page", page.PathTemplate, "view", render.ViewID(n.At), "err", err)
		}
		view.Failed = true
		return p.render.Toolbar(view)
	}
	view.Node = resolved
	view.Items = resolved.Items
	view.Access, view.Hrefs = p.controls(ctx, req, page, resolved.Items)
	view.Forms = make([]render.ToolbarForm, len(resolved.Items))
	for i, it := range resolved.Items {
		if it.Form != nil {
			model, ok, refusal := loaded(p, ctx, page, "toolbar", "toolbar form", n.At, func() (any, error) { return it.Form.Load(ctx) })
			view.Forms[i] = render.ToolbarForm{Model: model, Failed: !ok, Refusal: refusal}
		}
	}
	return p.render.Toolbar(view)
}

// toolbarAct runs a toolbar's button. It has no subject: what it needs of the page it reads from the request.
func (p *Program) toolbarAct(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Toolbar) {
	index, err := strconv.Atoi(r.PostForm.Get("_act"))
	if err != nil {
		p.refuse(w, r, notUnderstood)
		return
	}
	p.itemAct(w, r, page, req, n.Items, index)
}
