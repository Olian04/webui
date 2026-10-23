package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// Delays are the time each panel's backend takes to answer: a form, a table and a second table.
const (
	slowForm   = 100 * time.Millisecond
	slowTable  = 400 * time.Millisecond
	slowTable2 = 250 * time.Millisecond
)

// slowApp is a page of three panels whose backends are slow, as a database or another service is.
func slowApp() http.Handler {
	table := func(title string, delay time.Duration) webui.Table[Device] {
		return webui.Table[Device]{
			Title: title,
			Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], webui.Loaded) {
				if !sleep(ctx, delay) {
					return webui.Window[Device]{}, webui.Failed(ctx.Err())
				}
				return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, webui.Ready()
			},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	page := webui.Page[webui.NoArgs]{
		Path: "/slow",
		Body: webui.Stack{
			webui.Form[Device]{
				Title: "Form",
				Load: func(ctx context.Context) (Device, webui.Loaded) {
					sleep(ctx, slowForm)
					return Device{Id: "x"}, webui.Ready()
				},
				Fields: []webui.Accessor[Device]{formID},
			},
			table("Slow", slowTable),
			table("Medium", slowTable2),
		},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
}

// sleep waits d, or until ctx is done, and says whether it waited the whole time.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

// BenchmarkPageSlowPanels is a page whose panels load from slow backends (100, 400 and 250 ms): what a visitor waits
// for is the time to the whole page, which is the sum of the three when they load one after the other and the longest
// when they load together. Run it with -benchtime=5x.
func BenchmarkPageSlowPanels(b *testing.B) {
	h := slowApp()
	for b.Loop() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/slow", nil))
		if rec.Code != http.StatusOK {
			b.Fatalf("status %d", rec.Code)
		}
	}
}

// BenchmarkPageFastPanels is the same page with backends that answer at once, as a cache or a slice in memory does:
// loading the panels together must not cost a page that has nothing to wait for.
func BenchmarkPageFastPanels(b *testing.B) {
	table := func(title string) webui.Table[Device] {
		return webui.Table[Device]{
			Title: title,
			Load: func(context.Context, webui.Query) (webui.Window[Device], webui.Loaded) {
				return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, webui.Ready()
			},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/fast", Body: webui.Stack{
		webui.Form[Device]{Title: "Form", Load: func(context.Context) (Device, webui.Loaded) { return Device{Id: "x"}, webui.Ready() }, Fields: []webui.Accessor[Device]{formID}},
		table("One"), table("Two"), table("Three"),
	}}}}.MustCompile("/admin")
	b.ReportAllocs()
	for b.Loop() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/fast", nil))
		if rec.Code != http.StatusOK {
			b.Fatalf("status %d", rec.Code)
		}
	}
}
