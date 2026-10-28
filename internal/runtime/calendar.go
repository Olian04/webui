package runtime

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/calendar"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// calendar loads a calendar leaf for the month that the address names, or the current one, and builds its view. The source is asked for the days that the grid shows, and
// the items are counted by their day in UTC. A day with items leads where Open says, if the visitor may go there. A failed Load fails that panel, not the page.
func (p *Program) calendar(ctx context.Context, req *Request, page *ir.Page, n *ir.Calendar) templ.Component {
	now := time.Now
	if n.Today != nil {
		now = n.Today
	}
	today := calendar.Today(now())
	month := calendar.Of(today)
	if m, ok := calendar.Parse(req.Raw[args.ViewKey(n.ID, "month")]); ok {
		month = m
	}
	view := render.CalendarView{Node: n, Page: page, Month: month, Current: calendar.Of(today), Today: today}
	view.Path, view.Query = req.encode(page)

	from, to := calendar.Span(month, n.SundayFirst)
	rows, ok, refusal := loaded(p, ctx, page, "calendar", "calendar", n.At, func() ([]any, error) { return n.Load(ctx, from, to) })
	if !ok {
		view.Failed, view.Refusal = true, refusal
		return p.render.Calendar(view)
	}
	view.Items = map[string]int{}
	for _, row := range rows {
		if d := n.Date(row).UTC(); !d.Before(from) && d.Before(to) {
			view.Items[calendar.Key(d)]++
		}
	}
	view.Hrefs = p.dayHrefs(ctx, req, page, n, view.Items)
	return p.render.Calendar(view)
}

// dayHrefs is where each day that has items leads, by the day's date, for the days that lead anywhere: a day whose link cannot be built is logged and is a plain day, and so is
// one whose page refuses the visitor, since a link they cannot follow is not offered.
func (p *Program) dayHrefs(ctx context.Context, req *Request, page *ir.Page, n *ir.Calendar, items map[string]int) map[string]string {
	if n.Open == nil {
		return nil
	}
	hrefs := map[string]string{}
	for key, count := range items {
		date, err := time.Parse("2006-01-02", key)
		if err != nil {
			continue
		}
		href, err := p.open(n.Open.Dest, n.Open.Args(ctx, ir.CalendarDay{Date: date, Items: count}), req.Address)
		if err != nil {
			p.log.Error("webui: a calendar's day link failed", "page", page.PathTemplate, "dest", n.Open.Dest, "day", key, "err", err)
			continue
		}
		if p.mayFollow(ctx, page, n.Open, href) {
			hrefs[key] = href
		}
	}
	return hrefs
}
