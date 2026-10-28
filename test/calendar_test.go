package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
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

// calendarCell is the cell of a day in the page: from its opening td to its closing one, or "" if the day is not in the page.
func calendarCell(body string, day int, out bool) string {
	for _, part := range strings.Split(body, "<td")[1:] {
		cell := part[:strings.Index(part, "</td>")]
		if out != strings.Contains(cell, "cal-out") {
			continue
		}
		if strings.Contains(cell, `<span aria-hidden="true">`+strconv.Itoa(day)+`</span>`) {
			return cell
		}
	}
	return ""
}

// dayPages is the page a calendar is on, and the page its days open.
func dayPages(body webui.PageBody, guard func(context.Context, dayArgs) error) (http.Handler, webui.Page[dayArgs]) {
	detail := webui.Page[dayArgs]{Path: "/day/{date}", Guard: guard, Body: webui.Stack{}}
	list := webui.Page[webui.NoArgs]{Path: "/e", Body: body}
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
	body := rec.Body.String()

	assert.Contains(t, body, `<h2 class="panel-title">Releases</h2>`)
	assert.Contains(t, body, `<h3 class="cal-title"`)
	assert.Contains(t, body, ">September 2026</h3>")
	assert.Contains(t, body, "<table class=\"cal\" aria-labelledby=")
	assert.Equal(t, strings.Count(body, `<th scope="col" abbr=`), 7)
	assert.True(t, strings.Index(body, `abbr="Monday"`) < strings.Index(body, `abbr="Tuesday"`)) // a week begins on Monday
	assert.True(t, strings.Index(body, `abbr="Saturday"`) < strings.Index(body, `abbr="Sunday"`))

	// September 2026 begins on a Tuesday and ends on a Wednesday: the grid is 31 August to 4 October, five weeks.
	assert.Equal(t, strings.Count(body, `<td class="cal-cell`), 35)
	assert.Equal(t, strings.Count(body, "cal-out"), 5) // 31 August, and 1 to 4 October
	assert.False(t, strings.Contains(body, "<script>"))
}

func TestTheSourceIsAskedForTheDaysTheGridShowsAndNotForTheMonth(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()

	spans := src.asked()
	assert.Equal(t, len(spans), 1)
	assert.True(t, spans[0].From.Equal(time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)))
	assert.True(t, spans[0].To.Equal(time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC))) // exclusive: the day after the last the grid shows

	// A day of the week at the edge shows what is on it: 2 October is of the next month.
	assert.Contains(t, calendarCell(body, 2, true), "cal-marked")
	assert.Contains(t, calendarCell(body, 2, true), "Friday 2 October 2026, 1 item")
}

func TestTheMonthInTheAddressIsTheMonthThatIsShown(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	body := serve(h, http.MethodGet, "/admin/e?releases.month=2026-11").Body.String()
	assert.Contains(t, body, ">November 2026</h3>")
	spans := src.asked()
	assert.True(t, spans[0].From.Equal(time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC))) // a Monday
	assert.True(t, spans[0].To.Equal(time.Date(2026, time.December, 7, 0, 0, 0, 0, time.UTC)))   // 30 November is a Monday, so the grid has a sixth week
	assert.Contains(t, calendarCell(body, 9, false), "cal-marked")
	assert.False(t, strings.Contains(body, `aria-current="date"`)) // today is not in this grid

	// A month that is not one, or is not one that the calendar goes to, is the current month, and not an error.
	for _, bad := range []string{"soon", "2026-13", "2026-00", "0001-01", "9999-12", "2026-9", ""} {
		got := serve(h, http.MethodGet, "/admin/e?releases.month="+bad)
		assert.Equal(t, got.Code, http.StatusOK)
		assert.Contains(t, got.Body.String(), ">September 2026</h3>")
	}
}

func TestTheLinksToOtherMonthsAreRealLinksThatKeepTheRestOfTheAddress(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})

	// On the current month there is no link back to it.
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `href="/admin/e?releases.month=2026-08"`)
	assert.Contains(t, body, `href="/admin/e?releases.month=2026-10"`)
	assert.Contains(t, body, "Previous month, August 2026")
	assert.Contains(t, body, "Next month, October 2026")
	assert.Contains(t, body, "data-in-place")
	assert.False(t, strings.Contains(body, ">Today</a>"))

	// On another it is there, and its address has no month, since that is the one a page opens at.
	body = serve(h, http.MethodGet, "/admin/e?releases.month=2026-12").Body.String()
	assert.Contains(t, body, `href="/admin/e?releases.month=2027-01"`) // the year changes
	assert.Contains(t, body, `href="/admin/e?releases.month=2026-11"`)
	assert.Contains(t, body, `href="/admin/e" data-in-place>Today</a>`)
}

func TestAnotherPartOfTheAddressIsKeptByTheLinks(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Stack{
		webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.Feed[story]{Title: "Latest", Load: (&feedLoader{}).load, PageSize: 2, Headline: pTitle},
	})
	body := serve(h, http.MethodGet, "/admin/e?latest.limit=4").Body.String()
	assert.Contains(t, body, `href="/admin/e?latest.limit=4&amp;releases.month=2026-10"`)
}

func TestTheDayThatIsNowIsMarkedByMoreThanColour(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `aria-current="date"`), 1)
	cell := calendarCell(body, 15, false)
	assert.Contains(t, cell, "cal-today")
	assert.Contains(t, cell, `aria-current="date"`)

	// With the clock left alone, the day is the one that the clock says, and some day of the month it shows has the mark.
	clock := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen})
	assert.Equal(t, strings.Count(serve(clock, http.MethodGet, "/admin/e").Body.String(), `aria-current="date"`), 1)
}

func TestADayIsToldInFullWithHowManyItemsAreOnIt(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()

	assert.Contains(t, calendarCell(body, 3, false), "Thursday 3 September 2026, 2 items")
	assert.Contains(t, calendarCell(body, 3, false), "cal-marked")
	assert.Contains(t, calendarCell(body, 15, false), "Tuesday 15 September 2026, 1 item")
	assert.Contains(t, calendarCell(body, 4, false), "Friday 4 September 2026</span>") // none: the date and nothing more
	assert.False(t, strings.Contains(calendarCell(body, 4, false), "cal-marked"))

	// A moment is of the day it is on in UTC: 23:30 at UTC-5 on the 20th is the 21st.
	assert.Contains(t, calendarCell(body, 21, false), "Monday 21 September 2026, 1 item")
	assert.False(t, strings.Contains(calendarCell(body, 20, false), "cal-marked"))

	// A day of another month that is not in the grid is nowhere in it, whatever the source returned.
	assert.Equal(t, strings.Count(body, "cal-marked"), 4) // the 3rd, 15th, 21st, and 2 October; the source's November is not of this grid
}

func TestADayWithItemsLeadsWhereOpenSaysAndADayWithNoneLeadsNowhere(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	_, detail := dayPages(webui.Stack{}, nil)
	h, _ := dayPages(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, Open: openDay(detail)}, nil)
	body := serve(h, http.MethodGet, "/admin/e").Body.String()

	assert.Contains(t, calendarCell(body, 3, false), `<a class="cal-day" href="/admin/day/2026-09-03" data-in-place>`)
	assert.Contains(t, calendarCell(body, 15, false), `href="/admin/day/2026-09-15"`)
	assert.False(t, strings.Contains(calendarCell(body, 4, false), "<a "))
	assert.Contains(t, calendarCell(body, 2, true), `href="/admin/day/2026-10-02"`)
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
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, `href="/admin/day/2026-09-03"`))
	assert.Contains(t, calendarCell(body, 3, false), "Thursday 3 September 2026, 2 items") // still marked, and still told
	assert.Contains(t, body, `href="/admin/day/2026-09-15"`)
}

func TestAWeekBeginsOnSundayOrHasItsNumberShown(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	sunday := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, SundayFirst: true})
	body := serve(sunday, http.MethodGet, "/admin/e").Body.String()
	assert.True(t, strings.Index(body, `abbr="Sunday"`) < strings.Index(body, `abbr="Monday"`))
	spans := src.asked()
	assert.True(t, spans[0].From.Equal(time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC))) // the Sunday before the 1st
	assert.False(t, strings.Contains(body, `class="cal-wk"`))

	numbered := editorApp(webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember, WeekNumbers: true})
	body = serve(numbered, http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `<th scope="row" class="cal-wk">`), 5)
	assert.Contains(t, body, `<th scope="row" class="cal-wk"><span class="sr-only">Week </span>36</th>`) // the week of 31 August
	assert.Contains(t, body, `<span class="sr-only">Week </span>40</th>`)                                // and of 28 September
}

func TestACalendarRefreshesAsALeafOfItsOwn(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Stack{
		webui.Calendar[release]{Title: "Releases", Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.Feed[story]{Title: "Latest", Load: (&feedLoader{}).load, PageSize: 1, Headline: pTitle},
	})
	req := httptest.NewRequest(http.MethodGet, "/admin/e?releases.month=2026-10", nil)
	req.Header.Set("X-Webui-Leaf", "p.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.Contains(t, body, ">October 2026</h3>")
	assert.Contains(t, body, `data-leaf="p.0"`)
	assert.False(t, strings.Contains(body, "<html"))
	assert.False(t, strings.Contains(body, "Latest"))
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
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "secret-bucket"))
	assert.False(t, strings.Contains(rec.Body.String(), `class="cal"`))
}

func TestAMonthWithNothingOnItIsStillAMonth(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Calendar[release]{
		Title: "Releases", Date: releaseWhen, Today: midSeptember,
		Load: func(context.Context, webui.Span) ([]release, webui.Loaded) { return nil, webui.Ready() },
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `class="cal"`)
	assert.False(t, strings.Contains(body, "cal-marked"))
}

func TestTwoCalendarsOfAPageKeepTheirMonthsApart(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	h := editorApp(webui.Stack{
		webui.Calendar[release]{Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.Calendar[release]{Load: src.load, Date: releaseWhen, Today: midSeptember},
		webui.Calendar[release]{Title: "Deadlines", Load: src.load, Date: releaseWhen, Today: midSeptember},
	})
	body := serve(h, http.MethodGet, "/admin/e?calendar-2.month=2026-11").Body.String()
	assert.Contains(t, body, "September 2026")
	assert.Contains(t, body, "November 2026")
	assert.Contains(t, body, `href="/admin/e?calendar-2.month=2026-11&amp;calendar.month=2026-10"`)
	assert.Contains(t, body, "deadlines.month=2026-10")
}

func TestACalendarThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	src := &calendarSource{}
	_, detail := dayPages(webui.Stack{}, nil)
	act := webui.Action[webui.Day]{Label: "Go", Run: func(context.Context, webui.Day) (webui.Outcome, error) { return webui.Outcome{}, nil }}
	for name, tc := range map[string]struct {
		body webui.PageBody
		want string
	}{
		"no load":                  {webui.Calendar[release]{Date: releaseWhen}, "a Calendar has no Load"},
		"no date":                  {webui.Calendar[release]{Load: src.load}, "a Calendar has no Date"},
		"an action open":           {webui.Calendar[release]{Load: src.load, Date: releaseWhen, Open: act}, "Calendar.Open is an Action"},
		"sunday and week numbers":  {webui.Calendar[release]{Load: src.load, Date: releaseWhen, SundayFirst: true, WeekNumbers: true}, "a Calendar has SundayFirst and WeekNumbers"},
		"an open that is not here": {webui.Calendar[release]{Load: src.load, Date: releaseWhen, Open: webui.Link[webui.Day, webui.NoArgs]{Page: webui.Page[webui.NoArgs]{Path: "/elsewhere", Body: webui.Stack{}}, Args: func(context.Context, webui.Day) webui.NoArgs { return webui.NoArgs{} }}}, `a link targets "/elsewhere", which is not mounted in this app`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, editorErrors(t, tc.body), tc.want)
		})
	}
	_ = detail
}
