package webui_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// everythingThatLogs is an app with each thing that makes the library log: a panel that cannot load, a Load that panics, a
// Format that panics, a navigation page whose Guard panics, and a search that finds through a Format that panics.
func everythingThatLogs(log *slog.Logger) http.Handler {
	broken := webui.Int[blob]{Label: "Size", Load: func(b blob) int { return b.Size }, Format: func(int) string { panic("format") }}
	home := webui.Page[webui.NoArgs]{
		Path: "/home", Nav: webui.Nav{Label: "Home"},
		Body: webui.Stack{
			webui.Table[blob]{
				Title:   "Formatted",
				Rows:    func(context.Context) ([]blob, error) { return blobs(), nil },
				Key:     func(b blob) string { return b.Name },
				Columns: []webui.Accessor[blob]{blobName, broken},
			},
			webui.Table[blob]{
				Title:   "Unavailable",
				Rows:    func(context.Context) ([]blob, error) { return nil, errors.New("the backend is down") },
				Columns: []webui.Accessor[blob]{blobName},
			},
		},
		Search: webui.SearchRows(
			func(context.Context) ([]blob, error) { return blobs(), nil },
			func(b blob) webui.Hit[webui.NoArgs] { return webui.Hit[webui.NoArgs]{Title: b.Name} },
			broken,
		),
	}
	guarded := webui.Page[webui.NoArgs]{
		Path: "/guarded", Nav: webui.Nav{Label: "Guarded"}, Body: webui.Stack{},
		Guard: func(context.Context, webui.NoArgs) error { panic("guard") },
	}
	crash := webui.Page[webui.NoArgs]{
		Path: "/crash",
		Body: webui.Form[Device]{
			Load:   func(context.Context) (Device, error) { panic("load") },
			Fields: []webui.Accessor[Device]{formID},
		},
	}
	return webui.App{Logger: log, Pages: webui.Pages{home, guarded, crash}}.MustCompile("/admin")
}

func visitEverything(h http.Handler) {
	serve(h, http.MethodGet, "/admin/home")
	serve(h, http.MethodGet, "/admin/crash")
	serve(h, http.MethodGet, "/admin/_webui/search?q=a")
}

// An app that is given no logger logs nothing: not to the default logger, not to anything. A program that mounts the
// library is as quiet as it was before, until it says where the lines go.
//
// Not parallel: it sets the default logger, which is where an app that fell back to it would write.
func TestAnAppWithNoLoggerLogsNothing(t *testing.T) {
	var logged bytes.Buffer
	before := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(before)

	h := everythingThatLogs(nil)
	visitEverything(h)

	assert.Equal(t, logged.String(), "")
	// It does not stop the app from doing what it does: the page that crashed was a 500 and the others were drawn.
	assert.Equal(t, serve(h, http.MethodGet, "/admin/crash").Code, http.StatusInternalServerError)
	assert.Equal(t, serve(h, http.MethodGet, "/admin/home").Code, http.StatusOK)
}

// An app that is given a logger says what went wrong there, and only there.
func TestAnAppWithALoggerLogsToIt(t *testing.T) {
	var logged, other bytes.Buffer
	before := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&other, nil)))
	defer slog.SetDefault(before)

	visitEverything(everythingThatLogs(slog.New(slog.NewTextHandler(&logged, nil))))

	assert.Contains(t, logged.String(), "a Format panicked")
	assert.Contains(t, logged.String(), "load failed")
	assert.Contains(t, logged.String(), "the backend is down")
	assert.Contains(t, logged.String(), "a page's Guard panicked")
	assert.Contains(t, logged.String(), "panic while serving")
	assert.Equal(t, other.String(), "")
}

// A Format that panics in the search is logged to the app's logger too: it is called with no more than a value, so the
// logger reaches it through the request.
func TestAFormatThatPanicsInTheSearchIsLoggedToTheAppsLogger(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	h := everythingThatLogs(slog.New(slog.NewTextHandler(&logged, nil)))
	rec := serve(h, http.MethodGet, "/admin/_webui/search?q=a")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, logged.String(), "a Format panicked")
}

// A request that ends before it asked for the sidebar's entries, because it panicked, leaves none of the workers that were asked for them
// running: the Guard of another page, which they run, has finished when the response is complete, and so has what it logs.
func TestARequestThatPanicsLeavesNoSidebarWorkerRunningAfterIt(t *testing.T) {
	t.Parallel()

	var started, done atomic.Int32
	slow := webui.Page[webui.NoArgs]{
		Path: "/slow", Nav: webui.Nav{Label: "Slow"}, Body: webui.Stack{},
		Guard: func(context.Context, webui.NoArgs) error {
			if started.Add(1) == 1 {
				time.Sleep(300 * time.Millisecond) // the first, which is the workers of the request that panics: the error page's own are quick
			}
			done.Add(1)
			return nil
		},
	}
	crash := webui.Page[webui.NoArgs]{
		Path: "/crash",
		Body: webui.Form[Device]{
			Load:   func(context.Context) (Device, error) { panic("load") },
			Fields: []webui.Accessor[Device]{formID},
		},
	}
	h := webui.App{Pages: webui.Pages{slow, crash}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/crash")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.True(t, started.Load() > 0)
	assert.Equal(t, done.Load(), started.Load()) // every Guard that was started has run to its end by the time the response is complete
}
