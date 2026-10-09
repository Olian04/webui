package runtime

import (
	"context"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// submission is a rejected POST, carried back into the form that rendered it:
// the model as bound so far, what the user typed, and what was refused.
type submission struct {
	model  any
	values map[string]string
	errs   []ir.FieldError
}

// form loads a form leaf and builds its view. sub is nil on a plain GET.
// A failed Load fails that panel, not the page.
func (p *Program) form(ctx context.Context, req *Request, page *ir.Page, n *ir.Form, sub *submission) templ.Component {
	view := render.FormView{Node: n, Page: page}
	view.Path, view.Query = req.encode(page)
	view.CancelHref = p.cancelHref(req, page, view.Path)

	if sub != nil {
		view.Model, view.Values = sub.model, sub.values
		view.Errors = make(map[string]string, len(sub.errs))
		for _, e := range sub.errs {
			view.Errors[e.Label] = e.Message
		}
	} else {
		model, err := n.Load(ctx)
		if err != nil {
			if ctx.Err() == nil {
				p.log.Error("webui: form load failed", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
			}
			view.Failed = true
			return p.render.Form(view)
		}
		view.Model = model
	}

	// The same Guard that would reject the request gates the control, so there
	// is no second authorisation rule to keep in sync.
	if n.Submit != nil && n.Submit.Guard != nil {
		if err := n.Submit.Guard(ctx, view.Model); err != nil {
			view.SubmitGate = err.Error()
		}
	}
	if n.Editor != nil {
		// A language service goes only to an editor that can be edited by this visitor:
		// a viewer, or one whose guard refuses them, is only ever highlighted.
		service := ""
		if view.Editable() {
			service = n.Editor.Service
		}
		req.code.Add(service)
	}
	return p.render.Form(view)
}

// cancelHref is where a form's Cancel goes: back to the page the user came from,
// when a link says so, and otherwise to the parent in the breadcrumb, when there
// is one that is not the app's home.
func (p *Program) cancelHref(req *Request, page *ir.Page, path map[string]string) string {
	if req.From != "" {
		return req.From
	}
	crumbs := p.render.Crumbs(page, path)
	if len(crumbs) < 3 {
		return ""
	}
	return crumbs[len(crumbs)-2].Href
}
