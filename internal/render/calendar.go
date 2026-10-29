package render

import (
	"maps"
	"strconv"
	"time"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/calendar"
	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// CalendarView is a calendar with the month it shows and what is on its days. The runtime fills it.
type CalendarView struct {
	Node *ir.Calendar
	Page *ir.Page

	// Path and Query are the address the page is at, so that the links to other months are the same page with another month.
	Path  map[string]string
	Query map[string]string

	// Month is the month that is shown, Current the month that is now, and Today the day that is.
	Month, Current calendar.Month
	Today          time.Time

	// Items is how many items are on each day, by calendar.Key, and Hrefs is where a day leads, "" or absent for one that leads nowhere.
	Items map[string]int
	Hrefs map[string]string

	Failed  bool // Load failed: the panel says so, the page stands
	Refusal *ir.Refusal
}

// monthHref is this page with another month: everything else in the address as it was. The current month is the default, so its address has no month in it.
func (r *Renderer) monthHref(v CalendarView, m calendar.Month) string {
	query := maps.Clone(v.Query)
	if query == nil {
		query = map[string]string{}
	}
	if m == v.Current {
		delete(query, args.ViewKey(v.Node.ID, "month"))
	} else {
		query[args.ViewKey(v.Node.ID, "month")] = m.String()
	}
	return r.PageHref(v.Page, v.Path, query)
}

// dayLabel is a day as a reader who goes by the grid is told it: its weekday and date in full, and how many items there are.
func dayLabel(day time.Time, items int) string {
	s := day.Format("Monday 2 January 2006")
	switch items {
	case 0:
	case 1:
		s += ", 1 item"
	default:
		s += ", " + strconv.Itoa(items) + " items"
	}
	return s
}

// calendarProps is the grid as the component takes it.
func (r *Renderer) calendarProps(v CalendarView) c.CalendarProps {
	p := c.CalendarProps{
		TitleID:     LeafID(v.Node.At) + "-month",
		Title:       v.Month.Title(),
		WeekNumbers: v.Node.WeekNumbers,
		LinkAttrs:   inPlace,
		PrevLabel:   "Previous month, " + v.Month.Prev().Title(),
		NextLabel:   "Next month, " + v.Month.Next().Title(),
	}
	if v.Month.Prev().InRange() {
		p.Prev = r.monthHref(v, v.Month.Prev())
	}
	if v.Month.Next().InRange() {
		p.Next = r.monthHref(v, v.Month.Next())
	}
	if v.Month != v.Current {
		p.Current = r.monthHref(v, v.Current)
	}
	for _, d := range calendar.Weekdays(v.Node.SundayFirst) {
		p.Weekdays = append(p.Weekdays, c.CalendarWeekday{Abbr: d.Abbr, Name: d.Name})
	}
	for _, week := range calendar.Weeks(v.Month, v.Node.SundayFirst) {
		w := c.CalendarWeekProps{}
		if v.Node.WeekNumbers {
			w.Number = strconv.Itoa(calendar.WeekNumber(week[0]))
		}
		for _, day := range week {
			key := calendar.Key(day)
			n := v.Items[key]
			w.Days = append(w.Days, c.CalendarDayProps{
				Num:     strconv.Itoa(day.Day()),
				Label:   dayLabel(day, n),
				Href:    v.Hrefs[key],
				Today:   day.Equal(v.Today),
				Outside: calendar.Of(day) != v.Month,
				Marked:  n > 0,
			})
		}
		p.Weeks = append(p.Weeks, w)
	}
	return p
}
