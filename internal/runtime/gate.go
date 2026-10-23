package runtime

import (
	"cmp"
	"context"
	"errors"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// gateOf is what a Guard's answer makes of its control: drawn as it is when the Guard accepts the visitor, drawn
// disabled with the message of an [ir.Disabled], and not drawn for any other error. The same Guard refuses the request,
// so what is drawn and what is allowed cannot disagree. A Guard that panics is the application's bug: it hides the control,
// as an error does, and the panic is logged, so that a page does not fail for it and a visitor is not offered what could
// not be checked. what names the Guard in the log.
func (p *Program) gateOf(page *ir.Page, what string, guard func() error) render.Gate {
	_, err := guarded(what, func() (struct{}, error) { return struct{}{}, guard() })
	if err == nil {
		return render.Gate{}
	}
	var panicked *panicError
	if errors.As(err, &panicked) {
		p.log.Error("webui: a Guard panicked, so what it guards is not drawn", "page", page.PathTemplate, "err", err)
		return render.Gate{Hidden: true}
	}
	var d *ir.Disabled
	if errors.As(err, &d) {
		return render.Gate{Reason: cmp.Or(d.Message, "Not allowed.")}
	}
	return render.Gate{Hidden: true}
}

// mayFollow is whether a link on page, to the page at href, is drawn: the visitor may open it, as its own Guard says.
// A Guard that panics answers no, and is logged.
func (p *Program) mayFollow(ctx context.Context, page *ir.Page, link *ir.Link, href string) bool {
	return p.gateOf(page, "the Guard of the page "+link.Dest, func() error {
		if !p.mayOpen(ctx, href) {
			return errNoWay
		}
		return nil
	}).Open()
}

// mayLead is whether a link that the framework draws on page, to the page at href (a breadcrumb parent, Cancel, Back), is drawn: the visitor may open it.
// A Guard that panics answers no, and is logged.
func (p *Program) mayLead(ctx context.Context, page *ir.Page, href string) bool {
	return p.gateOf(page, "the Guard of a page that a link on "+page.PathTemplate+" leads to", func() error {
		if !p.mayOpen(ctx, href) {
			return errNoWay
		}
		return nil
	}).Open()
}

// errNoWay is the answer of a page that the visitor may not open, of which a link to it says no more.
var errNoWay = errors.New("the visitor may not open the page")
