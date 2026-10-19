package runtime

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// toolbarParam is the parameter of a request that applies the inputs of a toolbar: its value is the toolbar's id.
const toolbarParam = "_toolbar"

// toolbar builds a toolbar's view. Everything in it is about the page's arguments as they are for this request: why a
// button may not be used, where a link goes, and what an input shows. A source of options that fails fails the toolbar,
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
	view.Gates = make([]string, len(resolved.Items))
	view.Hrefs = make([]string, len(resolved.Items))
	for i, it := range resolved.Items {
		switch {
		case it.Action != nil && it.Action.Guard != nil:
			// The same Guard that will refuse the POST gates the button.
			if err := it.Action.Guard(ctx, req.Args); err != nil {
				view.Gates[i] = cmp.Or(err.Error(), "Not allowed.")
			}
		case it.Link != nil:
			href, err := p.open(it.Link.Dest, it.Link.Args(ctx, req.Args), req.Address)
			if err != nil {
				p.log.Error("webui: a toolbar link failed", "page", page.PathTemplate, "dest", it.Link.Dest, "err", err)
				view.Gates[i] = "This is not available."
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
	outcome, ok := p.perform(w, r, page, req, action, req.Args, fmt.Sprintf("action %q", n.Items[index].Label))
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

// toolbarApply answers a request that carries what the inputs of a toolbar were given: it applies them to the page's
// arguments, with the rules of each, and sends the visitor to the page that those arguments are, with the rest of
// the address as it was: every table's sort and filters, and its first page, since what it shows may no longer have
// a second. The address the visitor arrives at is the page's own, a link that can be copied. Inputs that were
// refused apply none of them, and the visitor is back where they were, with the reasons in a toast.
func (p *Program) toolbarApply(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Toolbar) {
	ctx := r.Context()
	resolved, err := resolve(p, page, n, func() (*ir.Toolbar, error) { return n.Resolved(ctx) })
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("toolbar options: %w", err))
		return
	}
	inputs := resolved.Inputs()
	sent := r.URL.Query()
	values := make(map[string]string, len(inputs))
	for _, f := range inputs {
		given := sent[f.Name]
		switch {
		case len(given) == 0:
		case f.Display == ir.DisplaySelect:
			values[f.Name] = strings.Join(given, ir.ListSep) // a select sends one value for each that is chosen
		default:
			values[f.Name] = given[0]
		}
	}

	next, errs := resolved.Bind(inputs, req.Args, values)
	if len(errs) > 0 {
		messages := make([]string, len(errs))
		for i, e := range errs {
			messages[i] = e.Label + ": " + e.Message
		}
		path, query := req.encode(page)
		p.flashRedirect(w, r, p.render.PageHref(page, path, query), strings.Join(messages, "; "), render.ToastError)
		return
	}
	applied := *req
	applied.Args = next
	path, query := applied.encode(page)
	//nolint:gosec // G710: an address the page's own arguments were encoded into.
	http.Redirect(w, r, p.render.PageHref(page, path, query), http.StatusSeeOther)
}
