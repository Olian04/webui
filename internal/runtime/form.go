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
	view := render.FormView{Node: n, Page: page, Languages: req.Languages}
	// A select that reads its options has them as they are for this request. A source that fails fails the panel.
	resolved, err := resolve(p, page, n, func() (*ir.Form, error) { return n.Resolved(ctx) })
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: the options of a form could not be read", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
		}
		view.Failed = true
		return p.render.Form(view)
	}
	n = resolved
	view.Node = n
	view.Path, view.Query = req.encode(page)
	view.CancelHref = p.returnHref(ctx, req, page, view.Path)

	if sub != nil {
		view.Model, view.Values = sub.model, sub.values
		view.Errors = make(map[string]string, len(sub.errs))
		for _, e := range sub.errs {
			view.Errors[e.Label] = e.Message
		}
	} else {
		model, ok, refusal := loaded(p, ctx, page, formKind(n), "form", n.At, func() (any, error) { return n.Load(ctx) })
		if !ok {
			view.Failed, view.Refusal = true, refusal
			return p.render.Form(view)
		}
		view.Model = model
	}

	// The same Guard that would reject the request decides the control, so there
	// is no second authorisation rule to keep in sync. A form that may not be saved is read, whichever
	// error says so: there is no disabled form to draw that is any use to the visitor.
	if n.Submit != nil && n.Submit.Guard != nil {
		gate := p.gateOf(page, "the Submit of a form", func() error { return n.Submit.Guard(ctx, view.Model) })
		view.SubmitRefused, view.SubmitReason = !gate.Open(), gate.Reason
	}
	if n.Editor != nil {
		// A language service goes only to an editor that can be edited by this visitor:
		// a viewer, or one whose guard refuses them, is only ever highlighted.
		service := ""
		if view.Editable() {
			service = n.Editor.Service
			req.code.AddContext(n.Editor.Context)
		}
		req.code.Add(service)
	}
	return p.render.Form(view)
}

// returnHref is where a form's Cancel goes, and where an outcome that says Back leaves
// for: back to the page the user came from, when a link says so, and otherwise to the
// parent in the breadcrumb, when there is one that is not the app's home. It is empty
// when there is neither.
func (p *Program) returnHref(ctx context.Context, req *Request, page *ir.Page, path map[string]string) string {
	href := req.From
	if href == "" {
		crumbs := p.render.Crumbs(page, path, nil) // only where they lead
		if len(crumbs) < 3 {
			return ""
		}
		href = crumbs[len(crumbs)-2].Href
	}
	// A way back that the visitor may not open is no way back: Cancel and Back are left out, and not drawn to lead to a 403.
	if href == "" || !p.mayLead(ctx, page, href) {
		return ""
	}
	return href
}

// formKind is what a form's panel is called in the metrics: a code editor is not a form to
// whoever reads them.
func formKind(n *ir.Form) string {
	if n.Editor != nil {
		return "editor"
	}
	return "form"
}
