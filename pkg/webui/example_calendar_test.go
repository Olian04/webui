package webui_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// Deadline is something that is due on a day.
type Deadline struct {
	Name string
	Due  time.Time
}

// A Calendar shows a month as a grid of days, each marked when something is on it. Its source is asked for the days that the grid shows, and the visible month is in the
// address ("deadlines.month=2026-10"), so that its links to the months either side are real links. A day is a day in UTC. Open, which is optional, is where a day with items leads.
func ExampleCalendar() {
	deadlines := []Deadline{
		{"Tax", time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)},
		{"Rent", time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)},
		{"Renewal", time.Date(2026, time.October, 2, 0, 0, 0, 0, time.UTC)},
	}
	page := webui.Page[webui.NoArgs]{
		Path: "/due",
		Body: webui.Calendar[Deadline]{
			Title: "Deadlines",
			Load: func(_ context.Context, span webui.Span) ([]Deadline, webui.Loaded) {
				var due []Deadline
				for _, d := range deadlines {
					if !d.Due.Before(span.From) && d.Due.Before(span.To) {
						due = append(due, d)
					}
				}
				return due, webui.Ready()
			},
			Date:  func(d Deadline) time.Time { return d.Due },
			Today: func() time.Time { return time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC) },
		},
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/due", nil))
	body := rec.Body.String()
	fmt.Println(strings.Contains(body, ">September 2026</h3>"), "the current month is shown")
	fmt.Println(strings.Count(body, "cal-marked"), "days are marked")
	fmt.Println(strings.Contains(body, `href="/admin/due?deadlines.month=2026-10"`), "the next month is in the address")

	// Output:
	// true the current month is shown
	// 3 days are marked
	// true the next month is in the address
}
