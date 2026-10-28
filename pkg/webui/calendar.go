package webui

import (
	"context"
	"time"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/convert"
	"github.com/Olian04/webui/internal/ir"
)

// Span is a stretch of time that begins at From and ends just before To: the days that a calendar shows, as the moments the first begins at and the last ends at.
type Span struct{ From, To time.Time }

// Day is one day of a [Calendar], as its link is built from it.
type Day struct {
	// Date is the day, as the moment it begins at in UTC.
	Date time.Time

	// Items is how many items of the calendar are on it. A day with none has no link.
	Items int
}

// Calendar is a leaf that shows a month as a grid of days: each day with a mark when something is on it, and a link when Open says where a day leads. It is for a
// page that says what is on, such as releases, deadlines or what a team did, where a [Table] of the same things would be a list to scan for a date.
//
// The month that is shown is in the address, as the leaf's name and ".month" ("september-releases.month=2026-09"), with links to the month before and the month after, and, when it
// is not the current one, a link back to it: they are real links, so the calendar works without script and a copied link shows the same month, and with script they replace the calendar in place.
// Nothing is kept on the server. Load is asked for the days the grid shows, the days of the weeks round the month included, and the month is of whole weeks.
//
// A calendar is of dates and not of moments: a day is a day in UTC, as every moment is shown in UTC elsewhere in the library, and an app that keeps another zone applies it where it
// reads and where it builds the dates. Today is the day the clock says in UTC, and is marked with a ring and a bold number, and with aria-current="date", so that the mark is not colour alone.
// A week begins on Monday, as ISO 8601 has it, and on Sunday with SundayFirst; the week begins are not guessed from the visitor's language, which says nothing sure of it.
//
// Every day has a name that a reader who goes by the grid is told, such as "Sunday 6 September 2026, 3 items". It only reads.
type Calendar[M any] struct {
	// Title is the panel's heading, and names the leaf's view state in the address: "releases" for a calendar titled "Releases", and "calendar" when it has none.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the title.
	Desc string

	// Load returns the items that are in the span, which is the days the grid shows: the month's, and the days of the first and last weeks that are not. It is asked for the
	// span and not for the month, so that a day at the edge shows what is on it. Return the items in the span, and no others, in any order. If Load returns [Failed] the panel says
	// "Could not load", the cause is logged and the rest of the page stands. It is called on every request that needs the calendar, and nothing is kept between them.
	Load func(ctx context.Context, span Span) ([]M, Loaded)

	// Date is when an item is. Its day is its day in UTC. It is required.
	Date func(M) time.Time

	// Open is where a day with items leads, as a [Link] built from the [Day]: a page of the app that shows what is on it, with the date as its argument. The day is a real
	// anchor. A day whose page refuses the visitor is a plain day, and so is a day with nothing on it. It cannot be an [Action]. Left nil, no day is a link.
	Open RowClick[Day]

	// SundayFirst begins each week on Sunday. It is Monday when left false.
	SundayFirst bool

	// WeekNumbers adds a column of the ISO 8601 number of each week, in which the first week of a year is the one with its first Thursday. Those are of weeks that begin
	// on Monday, so it cannot be combined with SundayFirst.
	WeekNumbers bool

	// Today is what the calendar takes for now, to mark the day and to choose the month that it opens at. It is the clock when left nil, and is for a test, or for an app that serves
	// a day that is not today, such as the demo, whose data is of one day.
	Today func() time.Time
}

// ASSERT: Calendar implements PageBody
var _ PageBody = Calendar[struct{}]{}

func (c Calendar[M]) validateBody(v *bodyValidator) {
	if c.Load == nil {
		v.add("a Calendar has no Load", "Set Load to func(ctx, span Span) ([]M, Loaded), returning the items that are in the span.")
	}
	if c.Date == nil {
		v.add("a Calendar has no Date", "Set Date to func(M) time.Time, returning when an item is: its day, in UTC, is where it is marked.")
	}
	if c.Open != nil {
		if _, isAction := c.Open.(Action[Day]); isAction {
			v.add("Calendar.Open is an Action", "A day is read, and a link can be opened in a new tab: use a Link, which is built from the Day.")
		} else {
			c.Open.validateRow(v)
		}
	}
	if c.SundayFirst && c.WeekNumbers {
		v.add("a Calendar has SundayFirst and WeekNumbers", "ISO 8601 week numbers are of weeks that begin on Monday: drop one of the two.")
	}
}

func (c Calendar[M]) lowerBody(at ir.Addr) ir.Node {
	n := &ir.Calendar{
		At: at, Title: c.Title, Desc: c.Desc, SundayFirst: c.SundayFirst, WeekNumbers: c.WeekNumbers, Today: c.Today,
		Date: func(m any) time.Time { return c.Date(m.(M)) },
		Load: func(ctx context.Context, from, to time.Time) ([]any, error) {
			rows, loaded := c.Load(ctx, Span{From: from, To: to})
			if err := loaded.apply(ctx); err != nil {
				return nil, err
			}
			return convert.Any(rows), nil
		},
	}
	if c.Open != nil {
		link, _ := c.Open.lowerRow()
		inner := link.Args
		link.Args = func(ctx context.Context, row any) any {
			d := row.(ir.CalendarDay)
			return inner(ctx, Day{Date: d.Date, Items: d.Items})
		}
		n.Open = link
	}
	return n
}

// calendarViewID is the name of a calendar's view state when it has no title: what the address calls it.
func calendarViewID(n *ir.Calendar) string {
	if base := args.Slug(n.Title); base != "" {
		return base
	}
	return "calendar"
}
