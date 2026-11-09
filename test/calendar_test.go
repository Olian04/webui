package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// release is one thing on a calendar: a name, and when.
type release struct {
	Name string
	When time.Time
}

type dayArgs struct{ Date string }

// midSeptember is the day that the calendars of these tests take for now.
var midSeptember = func() time.Time { return time.Date(2026, time.September, 15, 14, 30, 0, 0, time.UTC) }

var releases = []release{
	{"alpha", time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC)},
	{"beta", time.Date(2026, time.September, 3, 17, 0, 0, 0, time.UTC)},
	{"gamma", time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)},
	// 23:30 at UTC-5 is the next day in UTC: a calendar is of days in UTC.
	{"late", time.Date(2026, time.September, 20, 23, 30, 0, 0, time.FixedZone("EST", -5*3600))},
	{"october", time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)}, // in the grid of September, as its last week has days of October
	{"november", time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)},
}

// calendarSource serves the releases that are in the span asked for, and notes the spans.
type calendarSource struct {
	mu    sync.Mutex
	spans []webui.Span
}

func (s *calendarSource) load(_ context.Context, span webui.Span) ([]release, webui.Loaded) {
	s.mu.Lock()
	s.spans = append(s.spans, span)
	s.mu.Unlock()
	var out []release
	for _, r := range releases {
		if !r.When.Before(span.From) && r.When.Before(span.To) {
			out = append(out, r)
		}
	}
	return out, webui.Ready()
}

func (s *calendarSource) asked() []webui.Span {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]webui.Span(nil), s.spans...)
}

func releaseWhen(r release) time.Time { return r.When }

// calendarDay is the cell of a day in the page, found by its date as it is told to a reader ("3 September 2026"); the test stops if the day is not in the page.
func calendarDay(t *testing.T, p *page.Page, date string) page.El {
	t.Helper()

	re := regexp.MustCompile(`^\w+ ` + regexp.QuoteMeta(date) + `(,|$)`)
	var found []page.El
	for _, c := range p.ByRole("cell", "") {
		if re.MatchString(c.Text()) {
			found = append(found, c)
		}
	}
	return page.One(t, "the cell of "+date, found)
}

// marked counts the days of a calendar that say that there are items on them.
func marked(p *page.Page) int {
	n := 0
	for _, c := range p.ByRole("cell", "") {
		if strings.Contains(c.Text(), " item") {
			n++
		}
	}
	return n
}

// dayPages is the page a calendar is on, and the page its days open.
func dayPages(body webui.View, guard func(context.Context, dayArgs) error) (http.Handler, webui.Page[dayArgs]) {
	detail := webui.Page[dayArgs]{Path: "/day/{date}", Guard: guard, View: webui.Stack{}}
	list := webui.Page[webui.NoArgs]{Path: "/e", View: body}
	return webui.App{Pages: webui.Pages{list, detail}}.MustCompile("/admin"), detail
}

func openDay(detail webui.Page[dayArgs]) webui.Link[webui.Day, dayArgs] {
	return webui.Link[webui.Day, dayArgs]{Page: detail, Args: func(_ context.Context, d webui.Day) dayArgs { return dayArgs{Date: d.Date.Format("2006-01-02")} }}
}

func TestACalendarIsAMonthOfWholeWeeksAsATableOfDays(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())

	assert.Equal(t, len(p.ByRole("heading", "Releases")), 1)
	assert.Equal(t, len(p.ByRole("heading", "September 2026")), 1)
	grid := page.One(t, "the grid, which is named by the month", p.ByRole("table", "September 2026"))
	// A week begins on Monday.
	assert.DeepEqual(t, grid.Headers(), []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"})

	// September 2026 begins on a Tuesday and ends on a Wednesday: the grid is 31 August to 4 October, five weeks.
	rows := grid.Rows()
	assert.Equal(t, len(rows), 5)
	outside := 0
	for _, week := range rows {
		assert.Equal(t, len(week), 7)
		for _, day := range week {
			if !strings.Contains(day, "September 2026") {
				outside++ // 31 August, and 1 to 4 October
			}
		}
	}
	assert.Equal(t, outside, 5)
	assert.Equal(t, strings.Count(rec.Body.String(), "<script>"), 1) // the one that sets the theme, and none that the data brought
}

func TestTheSourceIsAskedForTheDaysTheGridShowsAndNotForTheMonth(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	p := pageOf(t, h, "/admin/e")

	spans := src.asked()
	assert.Equal(t, len(spans), 1)
	assert.True(t, spans[0].From.Equal(time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)))
	assert.True(t, spans[0].To.Equal(time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC))) // exclusive: the day after the last the grid shows

	// A day of the week at the edge shows what is on it: 2 October is of the next month.
	assert.Equal(t, calendarDay(t, p, "2 October 2026").Text(), "Friday 2 October 2026, 1 item")
}

func TestTheMonthInTheAddressIsTheMonthThatIsShown(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	p := pageOf(t, h, "/admin/e?releases.month=2026-11")
	assert.Equal(t, len(p.ByRole("heading", "November 2026")), 1)
	spans := src.asked()
	assert.True(t, spans[0].From.Equal(time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC))) // a Monday
	assert.True(t, spans[0].To.Equal(time.Date(2026, time.December, 7, 0, 0, 0, 0, time.UTC)))   // 30 November is a Monday, so the grid has a sixth week
	assert.Contains(t, calendarDay(t, p, "9 November 2026").Text(), "1 item")
	assert.Equal(t, len(p.ByAttr("aria-current", "date")), 0) // today is not in this grid

	// A month that is not one, or is not one that the calendar goes to, is the current month, and not an error.
	for _, bad := range []string{"soon", "2026-13", "2026-00", "0001-01", "9999-12", "2026-9", ""} {
		got := serve(h, http.MethodGet, "/admin/e?releases.month="+bad)
		assert.Equal(t, got.Code, http.StatusOK)
		assert.Equal(t, len(page.Parse(t, got.Body.String()).ByRole("heading", "September 2026")), 1)
	}
}

func TestTheLinksToOtherMonthsAreRealLinksThatKeepTheRestOfTheAddress(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})

	// On the current month there is no link back to it.
	p := pageOf(t, h, "/admin/e")
	assert.True(t, hasLink(p, "Previous month, August 2026", "/admin/e?releases.month=2026-08"))
	assert.True(t, hasLink(p, "Next month, October 2026", "/admin/e?releases.month=2026-10"))
	assert.True(t, page.One(t, "the link to the next month", p.ByRole("link", "Next month, October 2026")).Has("data-in-place"))
	assert.Equal(t, len(p.ByRole("link", "Today")), 0)

	// On another it is there, and its address has no month, since that is the one a page opens at.
	p = pageOf(t, h, "/admin/e?releases.month=2026-12")
	assert.True(t, hasLink(p, "Next month, January 2027", "/admin/e?releases.month=2027-01")) // the year changes
	assert.True(t, hasLink(p, "Previous month, November 2026", "/admin/e?releases.month=2026-11"))
	assert.True(t, hasLink(p, "Today", "/admin/e"))
	assert.True(t, page.One(t, "the Today link", p.ByRole("link", "Today")).Has("data-in-place"))
}

func TestAnotherPartOfTheAddressIsKeptByTheLinks(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Stack{
		webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.List[story]{Title: "Latest", Loader: (&listLoader{}).loader(), PageSize: 2, Headline: pTitle},
	})
	p := pageOf(t, h, "/admin/e?latest.limit=4")
	assert.True(t, hasLink(p, "Next month, October 2026", "/admin/e?latest.limit=4&releases.month=2026-10"))
}

func TestTheDayThatIsNowIsMarkedByMoreThanColour(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	p := pageOf(t, h, "/admin/e")
	assert.Equal(t, len(p.ByAttr("aria-current", "date")), 1)
	assert.Equal(t, calendarDay(t, p, "15 September 2026").Attr("aria-current"), "date") // the day that is now says so, to a reader

	// With the clock left alone, the day is the one that the clock says, and some day of the month it shows has the mark.
	clock := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen})
	assert.Equal(t, len(pageOf(t, clock, "/admin/e").ByAttr("aria-current", "date")), 1)
}

func TestADayIsToldInFullWithHowManyItemsAreOnIt(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	p := pageOf(t, h, "/admin/e")

	assert.Equal(t, calendarDay(t, p, "3 September 2026").Text(), "Thursday 3 September 2026, 2 items")
	assert.Equal(t, calendarDay(t, p, "15 September 2026").Text(), "Tuesday 15 September 2026, 1 item")
	assert.Equal(t, calendarDay(t, p, "4 September 2026").Text(), "Friday 4 September 2026") // none: the date and nothing more

	// A moment is of the day it is on in UTC: 23:30 at UTC-5 on the 20th is the 21st.
	assert.Equal(t, calendarDay(t, p, "21 September 2026").Text(), "Monday 21 September 2026, 1 item")
	assert.Equal(t, calendarDay(t, p, "20 September 2026").Text(), "Sunday 20 September 2026")

	// A day of another month that is not in the grid is nowhere in it, whatever the source returned.
	assert.Equal(t, marked(p), 4) // the 3rd, 15th, 21st, and 2 October; the source's November is not of this grid
}

func TestADayWithItemsLeadsWhereOpenSaysAndADayWithNoneLeadsNowhere(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	_, detail := dayPages(webui.Stack{}, nil)
	h, _ := dayPages(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, Open: openDay(detail)}, nil)
	p := pageOf(t, h, "/admin/e")

	third := calendarDay(t, p, "3 September 2026").Links()
	assert.Equal(t, len(third), 1)
	assert.Equal(t, third[0].Href, "/admin/day/2026-09-03")
	assert.True(t, page.One(t, "the day's link", calendarDay(t, p, "3 September 2026").ByRole("link", "")).Has("data-in-place"))
	assert.Equal(t, calendarDay(t, p, "15 September 2026").Links()[0].Href, "/admin/day/2026-09-15")
	assert.Equal(t, len(calendarDay(t, p, "4 September 2026").Links()), 0)
	assert.Equal(t, calendarDay(t, p, "2 October 2026").Links()[0].Href, "/admin/day/2026-10-02")
}

// A day that leads to a page that refuses the visitor is a day and nothing more: a link they cannot follow is not offered.
func TestADayWhosePageRefusesTheVisitorIsNotALink(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	_, detail := dayPages(webui.Stack{}, nil)
	refuses := func(_ context.Context, a dayArgs) error {
		if a.Date == "2026-09-03" {
			return errors.New("not for you")
		}
		return nil
	}
	h, _ := dayPages(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, Open: openDay(detail)}, refuses)
	p := pageOf(t, h, "/admin/e")
	assert.Equal(t, len(calendarDay(t, p, "3 September 2026").Links()), 0)
	assert.Equal(t, calendarDay(t, p, "3 September 2026").Text(), "Thursday 3 September 2026, 2 items") // still marked, and still told
	assert.True(t, len(calendarDay(t, p, "15 September 2026").Links()) == 1)
}

func TestAWeekBeginsOnSundayOrHasItsNumberShown(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	sunday := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, SundayFirst: true})
	p := pageOf(t, sunday, "/admin/e")
	assert.Equal(t, page.One(t, "the grid", p.ByRole("table", "September 2026")).Headers()[0], "Sunday")
	spans := src.asked()
	assert.True(t, spans[0].From.Equal(time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC))) // the Sunday before the 1st
	assert.Equal(t, len(p.ByRole("rowheader", "")), 0)

	numbered := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, WeekNumbers: true})
	weeks := pageOf(t, numbered, "/admin/e").ByRole("rowheader", "")
	var got []string
	for _, w := range weeks {
		got = append(got, w.Text())
	}
	assert.DeepEqual(t, got, []string{"Week 36", "Week 37", "Week 38", "Week 39", "Week 40"}) // the week of 31 August to the week of 28 September
}

func TestACalendarRefreshesAsAPanelOfItsOwn(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Stack{
		webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.List[story]{Title: "Latest", Loader: (&listLoader{}).loader(), PageSize: 1, Headline: pTitle},
	})
	req := httptest.NewRequest(http.MethodGet, "/admin/e?releases.month=2026-10", nil)
	req.Header.Set("X-Webui-View", "p.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.True(t, page.IsFragment(body))
	p := page.Parse(t, body)
	assert.Equal(t, len(p.ByRole("heading", "October 2026")), 1)
	assert.Equal(t, page.One(t, "the panel", p.Views()).Attr("data-view"), "p.0")
	assert.Equal(t, len(p.ByRole("heading", "Latest")), 0)
}

func TestAFailingCalendarLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Calendar[release]{
		Title: "Releases", Date: releaseWhen, Today: midSeptember,
		Load: func(context.Context, webui.Span) ([]release, webui.Loaded) {
			return nil, webui.Failed(errors.New("store secret-bucket is gone"))
		},
	})
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())
	assert.Contains(t, p.Root().Text(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "secret-bucket"))
	assert.Equal(t, len(p.ByRole("table", "")), 0)
}

func TestAMonthWithNothingOnItIsStillAMonth(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Calendar[release]{
		Title: "Releases", Date: releaseWhen, Today: midSeptember,
		Load: func(context.Context, webui.Span) ([]release, webui.Loaded) { return nil, webui.Ready() },
	})
	p := pageOf(t, h, "/admin/e")
	assert.Equal(t, len(p.ByRole("table", "September 2026")), 1)
	assert.Equal(t, marked(p), 0)
}

func TestTwoCalendarsOfAPageKeepTheirMonthsApart(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Stack{
		webui.Calendar[release]{Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.Calendar[release]{Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.Calendar[release]{Title: "Deadlines", Load: src.load, Date: releaseWhen, Today: midSeptember},
	})
	p := pageOf(t, h, "/admin/e?calendar-2.month=2026-11")
	assert.Equal(t, len(p.ByRole("table", "September 2026")), 2) // the two that are not told apart by a title are told apart by their numbers in the address
	assert.Equal(t, len(p.ByRole("table", "November 2026")), 1)
	assert.True(t, hasLink(p, "Next month, October 2026", "/admin/e?calendar-2.month=2026-11&calendar.month=2026-10"))
	assert.True(t, len(p.ByAttr("href", "/admin/e?calendar-2.month=2026-11&deadlines.month=2026-10")) > 0)
}

func TestACalendarThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	_, detail := dayPages(webui.Stack{}, nil)
	for name, tc := range map[string]struct {
		body  webui.View
		names []string
	}{
		"no load":                  {webui.Calendar[release]{Date: releaseWhen}, []string{"Calendar", "Load"}},
		"no date":                  {webui.Calendar[release]{Load: src.load}, []string{"Calendar", "Date"}},
		"an href with no url":      {webui.Calendar[release]{Load: src.load, Date: releaseWhen, Open: webui.Href[webui.Day]{}}, []string{"Calendar.Open", "Href", "URL"}},
		"sunday and week numbers":  {webui.Calendar[release]{Load: src.load, Date: releaseWhen, SundayFirst: true, WeekNumbers: true}, []string{"Calendar", "SundayFirst", "WeekNumbers"}},
		"an open that is not here": {webui.Calendar[release]{Load: src.load, Date: releaseWhen, Open: webui.Link[webui.Day, webui.NoArgs]{Page: webui.Page[webui.NoArgs]{Path: "/elsewhere", View: webui.Stack{}}, Args: func(context.Context, webui.Day) webui.NoArgs { return webui.NoArgs{} }}}, []string{"\"/elsewhere\""}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			refuses(t, editorErrors(t, tc.body), tc.names...)
		})
	}
	_ = detail
}

// An Href leads a day to another site, as a link that opens in a tab of its own, for each day that has items and for no other. Its address is built from the Day.
func TestADayThatHasItemsCanLeadToAnotherSite(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, Open: webui.Href[webui.Day]{URL: func(d webui.Day) string {
		return "https://example.com/releases/" + d.Date.Format("2006-01-02")
	}}})
	p := pageOf(t, h, "/admin/e")

	day := calendarDay(t, p, "3 September 2026")
	link := page.One(t, "the day's link", day.ByRole("link", ""))
	assert.Equal(t, link.Attr("href"), "https://example.com/releases/2026-09-03")
	assert.Equal(t, link.Attr("target"), "_blank")
	assert.Equal(t, link.Attr("rel"), "noopener noreferrer")
	assert.Contains(t, day.Text(), ", opens in a new tab")                 // and a reader is told so
	assert.Equal(t, len(calendarDay(t, p, "4 September 2026").Links()), 0) // nothing on it: not a link
}

// A click on a day can be an Action: a day with items is a button that posts its date, and the action is run with the Day, which is found again in the grid with the items
// on it counted. A day with nothing on it, one the Guard refuses, and one that is not in the grid run nothing.
func TestAnActionOnADayRunsWithTheDayAndItsItems(t *testing.T) {
	t.Parallel()

	var ran []webui.Day
	open := webui.Action[webui.Day]{
		Guard: func(_ context.Context, d webui.Day) error {
			if d.Date.Day() == 15 {
				return webui.Disabled("not the 15th")
			}
			return nil
		},
		Run: func(_ context.Context, d webui.Day) (webui.Outcome, error) {
			ran = append(ran, d)
			return webui.Success("Opened"), nil
		},
	}
	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, Open: open})
	p := pageOf(t, h, "/admin/e")

	view, _ := p.Field("_view")
	assert.Equal(t, view.Value, "p")
	buttons := calendarDay(t, p, "3 September 2026").ByRole("button", "")
	assert.Equal(t, len(buttons), 1)
	assert.Equal(t, buttons[0].Attr("name"), "_act")
	assert.Equal(t, buttons[0].Attr("value"), "click:0:2026-09-03")
	assert.Equal(t, len(calendarDay(t, p, "4 September 2026").ByRole("button", "")), 0)  // nothing on it
	assert.Equal(t, len(calendarDay(t, p, "15 September 2026").ByRole("button", "")), 0) // its Guard refuses it

	assert.Equal(t, post(h, "/admin/e", formValues("_view", "p", "_act", "click:0:2026-09-03")).Code, http.StatusSeeOther)
	assert.Equal(t, len(ran), 1)
	assert.Equal(t, ran[0].Date, time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, ran[0].Items, 2)

	for _, act := range []string{"click:0:2026-09-04", "click:0:2026-09-15", "click:0:2027-01-01", "click:0:not-a-date", "row:0:2026-09-03"} {
		post(h, "/admin/e", formValues("_view", "p", "_act", act))
	}
	assert.Equal(t, len(ran), 1)
}

func TestAnActionOnADayIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	run := func(context.Context, webui.Day) (webui.Outcome, error) { return webui.Success("done"), nil }
	refuses(t, editorErrors(t, webui.Calendar[release]{Load: src.load, Date: releaseWhen, Open: webui.Action[webui.Day]{Run: run, Confirm: "Sure?"}}), "Calendar.Open", "Confirm")
	refuses(t, editorErrors(t, webui.Calendar[release]{Load: src.load, Date: releaseWhen, Open: webui.Action[webui.Day]{}}), "Calendar.Open", "Run")
}
