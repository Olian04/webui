package runtime

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// toolbar builds a toolbar's view. Everything in it is about the page's arguments as they are for this request: what a
// button is for this visitor, where a link goes, and what a value shows. A source of options that fails fails the toolbar,
// which says so, and not the page.
func (p *Program) toolbar(ctx context.Context, req *Request, page *ir.Page, n *ir.Toolbar) templ.Component {
	view := render.ToolbarView{Node: n, Page: page, Args: req.Args}
	view.Path, view.Query = req.encode(page)
	resolved, err := resolve(p, page, n, func() (*ir.Toolbar, error) { return n.Resolved(ctx) })
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: the options of a toolbar could not be read", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
		}
		view.Failed = true
		return p.render.Toolbar(view)
	}
	view.Node = resolved
	view.Gates = make([]render.Gate, len(resolved.Items))
	view.Hrefs = make([]string, len(resolved.Items))
	for i, it := range resolved.Items {
		switch {
		case it.Action != nil && it.Action.Guard != nil:
			// The same Guard that will refuse the POST decides the button.
			view.Gates[i] = p.gateOf(page, "the action "+strconv.Quote(it.Label), func() error { return it.Action.Guard(ctx, req.Args) })
		case it.Link != nil:
			href, err := p.open(it.Link.Dest, it.Link.Args(ctx, req.Args), req.Address)
			if err != nil {
				p.log.Error("webui: a toolbar link failed", "page", page.PathTemplate, "dest", it.Link.Dest, "err", err)
				view.Gates[i] = render.Gate{Hidden: true}
				continue
			}
			if !p.mayFollow(ctx, page, it.Link, href) {
				view.Gates[i] = render.Gate{Hidden: true}
				continue
			}
			view.Hrefs[i] = href
		}
	}
	return p.render.Toolbar(view)
}

// toolbarAct runs a toolbar's button. Its subject is the page's arguments, which the request has decoded.
func (p *Program) toolbarAct(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Toolbar) {
	index, err := strconv.Atoi(r.PostForm.Get("_act"))
	if err != nil || index < 0 || index >= len(n.Items) || n.Items[index].Action == nil {
		p.refuse(w, r, notUnderstood)
		return
	}
	action := n.Items[index].Action
	outcome, ok := p.perform(w, r, page, req, action, req.Args, fmt.Sprintf("action %q", n.Items[index].Label), nil)
	if !ok {
		return
	}
	switch outcome.Kind {
	case ir.OutcomeFailure, ir.OutcomeReject:
		// A toolbar has no fields to point at, so the messages are the reason.
		p.refuse(w, r, outcomeText(outcome))
	case ir.OutcomeSuccess, ir.OutcomeWarning:
		p.leave(w, r, outcome, r.URL.RequestURI())
	}
}
