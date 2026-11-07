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

// calendar loads a calendar panel for the month that the address names, or the current one, and builds its view. The source is asked for the days that the grid shows, and
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
	view := render.LoadedCalendar{Node: n, Page: page, Month: month, Current: calendar.Of(today), Today: today}
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
	view.ClickRefused = p.dayRefused(ctx, page, n, view.Items)
	return p.render.Calendar(view)
}

// dayHrefs is where each day that has items leads, by the day's date, for the days that lead anywhere: a day whose link cannot be built is logged and is a plain day, and so is
// one whose page refuses the visitor, since a link they cannot follow is not offered. A day whose click is an Action leads nowhere: it is a button.
func (p *Program) dayHrefs(ctx context.Context, req *Request, page *ir.Page, n *ir.Calendar, items map[string]int) map[string]string {
	click := n.Click
	if click == nil || click.Action != nil {
		return nil
	}
	hrefs := map[string]string{}
	for key, count := range items {
		day, ok := dayOf(key, count)
		if !ok {
			continue
		}
		if click.External != nil {
			if href := click.External(day); href != "" {
				hrefs[key] = href // an address has no page to ask
			}
			continue
		}
		href, err := p.open(click.Link.Dest, click.Link.Args(ctx, day), req.Address)
		if err != nil {
			p.log.Error("webui: a calendar's day link failed", "page", page.PathTemplate, "dest", click.Link.Dest, "day", key, "err", err)
			continue
		}
		if p.mayFollow(ctx, page, click.Link, href) {
			hrefs[key] = href
		}
	}
	return hrefs
}

// dayOf is a day with items as the IR's functions take it, from the day's key.
func dayOf(key string, count int) (ir.CalendarDay, bool) {
	date, err := time.Parse("2006-01-02", key)
	if err != nil {
		return ir.CalendarDay{}, false
	}
	return ir.CalendarDay{Date: date, Items: count}, true
}

// dayRefused is, for a calendar whose click on a day is an Action, which days with items this viewer may not click, by the day's key: the Action's Guard, run on the day, as it will
// be on the POST. A day it refuses is shown but does nothing, whichever error says so.
func (p *Program) dayRefused(ctx context.Context, page *ir.Page, n *ir.Calendar, items map[string]int) map[string]bool {
	if !n.Click.Posts() || n.Click.Action.Guard == nil {
		return nil
	}
	refused := map[string]bool{}
	for key, count := range items {
		if day, ok := dayOf(key, count); ok {
			refused[key] = !p.accessOf(page, "the click on a day", func() error { return n.Click.Action.Guard(ctx, day) }).Open()
		}
	}
	return refused
}
