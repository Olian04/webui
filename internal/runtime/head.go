package runtime

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// controls is what each button and link of a toolbar or of a panel's head is for this visitor, by position: whether its Guard lets them, and where each link goes.
// A control has no subject: what it needs of the page it reads from the request. A form of a toolbar has neither.
func (p *Program) controls(ctx context.Context, req *Request, page *ir.Page, items []ir.ToolbarItem) ([]render.Access, []string) {
	access := make([]render.Access, len(items))
	hrefs := make([]string, len(items))
	for i, it := range items {
		switch {
		case it.Action != nil && it.Action.Guard != nil:
			// The same Guard that will refuse the POST decides the button.
			access[i] = p.accessOf(page, "the action "+strconv.Quote(it.Label), func() error { return it.Action.Guard(ctx, nil) })
		case it.Link != nil:
			href, err := p.open(it.Link.Dest, it.Link.Args(ctx, nil), req.Address)
			if err != nil {
				p.log.Error("webui: a link of a toolbar or of a panel's head failed", "page", page.PathTemplate, "dest", it.Link.Dest, "err", err)
				access[i] = render.Access{Hidden: true}
				continue
			}
			if !p.mayFollow(ctx, page, it.Link, href) {
				access[i] = render.Access{Hidden: true}
				continue
			}
			hrefs[i] = href
		}
	}
	return access, hrefs
}

// headed is a panel drawn with the controls that were declared for its head, when it has any.
func (p *Program) headed(ctx context.Context, req *Request, page *ir.Page, n ir.Node, comp templ.Component) templ.Component {
	h, ok := n.(ir.HasHead)
	if !ok || len(h.Head()) == 0 {
		return comp
	}
	view := render.LoadedHead{At: n.Pos(), Page: page, Controls: render.Controls{Items: h.Head()}}
	view.Path, view.Query = req.encode(page)
	view.Access, view.Hrefs = p.controls(ctx, req, page, view.Items)
	return p.render.Head(comp, view)
}

// headAct runs a button in the head of the panel n. It is posted as "head:<position>".
func (p *Program) headAct(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n ir.Node, position string) {
	h, ok := n.(ir.HasHead)
	index, err := strconv.Atoi(position)
	if !ok || err != nil {
		p.refuse(w, r, notUnderstood)
		return
	}
	p.itemAct(w, r, page, req, h.Head(), index)
}

// itemAct runs the button of a toolbar or of a panel's head that is at index of items. It has no subject: what it needs of the page it reads from the request.
func (p *Program) itemAct(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, items []ir.ToolbarItem, index int) {
	if index < 0 || index >= len(items) || items[index].Action == nil {
		p.refuse(w, r, notUnderstood)
		return
	}
	action := items[index].Action
	outcome, ok := p.perform(w, r, page, req, action, nil, fmt.Sprintf("action %q", items[index].Label))
	if !ok {
		return
	}
	switch outcome.Kind {
	case ir.OutcomeFailure, ir.OutcomeReject:
		// There are no fields to point at, so the messages are the reason.
		p.refuse(w, r, outcomeText(outcome))
	case ir.OutcomeSuccess, ir.OutcomeWarning:
		p.leave(w, r, outcome, r.URL.RequestURI())
	}
}

// headPrefix is what the "_act" of a button in a panel's head starts with.
const headPrefix = "head:"

// headPosition is the position that a head button's "_act" names, and whether it is one.
func headPosition(act string) (string, bool) { return strings.CutPrefix(act, headPrefix) }
