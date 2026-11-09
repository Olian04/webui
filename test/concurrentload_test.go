package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/lang"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// meet makes a panel's backend wait for the others': it counts itself in and waits until all of n are in, or a
// while, and says whether they were. Panels that load one after the other can never meet.
func meet(ctx context.Context, arrived *atomic.Int32, n int32, met *atomic.Int32) {
	arrived.Add(1)
	deadline := time.After(2 * time.Second)
	for arrived.Load() < n {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case <-time.After(time.Millisecond):
		}
	}
	met.Add(1)
}

func TestAPagesPanelsLoadAtTheSameTime(t *testing.T) {
	t.Parallel()

	var arrived, met atomic.Int32
	table := func(title string) webui.Table[Device] {
		return webui.Table[Device]{
			Title: title,
			Loader: webui.PagedLoader[Device]{Load: func(ctx context.Context, _ webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				meet(ctx, &arrived, 4, &met)
				return webui.PagedWindow[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, webui.Ready()
			}},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	page := webui.Page[webui.NoArgs]{Path: "/p", View: webui.Stack{
		webui.Split{
			webui.Form[Device]{
				Title: "Form",
				Load: func(ctx context.Context) (Device, webui.Loaded) {
					meet(ctx, &arrived, 4, &met)
					return Device{Id: "x"}, webui.Ready()
				},
				Fields: []webui.Accessor[Device]{formID},
			},
			table("One"),
		},
		table("Two"),
		table("Three"),
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/p")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, met.Load(), int32(4)) // each of the four, in a Split and a Stack, found the others loading
	body := rec.Body.String()
	for _, want := range []string{"One-row", "Two-row", "Three-row"} {
		assert.Contains(t, body, want)
	}
	// And in the order the page declares them, whichever answered first.
	assert.True(t, strings.Index(body, "One-row") < strings.Index(body, "Two-row"))
	assert.True(t, strings.Index(body, "Two-row") < strings.Index(body, "Three-row"))
}

// A panel's Load that panics is answered like any panic while serving, whichever goroutine it was on: with a 500
// and the process alive, and the panels beside it are not left running behind the response.
func TestAPanicInOnePanelsLoadIsAnsweredAndTheProcessLives(t *testing.T) {
	t.Parallel()

	var others atomic.Int32
	page := webui.Page[webui.NoArgs]{Path: "/p", View: webui.Stack{
		webui.Table[Device]{
			Title: "Fine",
			Loader: webui.PagedLoader[Device]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				others.Add(1)
				return webui.PagedWindow[Device]{}, webui.Ready()
			}},
			Columns: []webui.Accessor[Device]{formID},
		},
		webui.Form[Device]{
			Title:  "Broken",
			Load:   func(context.Context) (Device, webui.Loaded) { panic("the backend fell over") },
			Fields: []webui.Accessor[Device]{formID},
		},
		webui.Table[Device]{
			Title: "Also fine",
			Loader: webui.PagedLoader[Device]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				others.Add(1)
				return webui.PagedWindow[Device]{}, webui.Ready()
			}},
			Columns: []webui.Accessor[Device]{formID},
		},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/p")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.Contains(t, rec.Body.String(), "Something went wrong")
	assert.False(t, strings.Contains(rec.Body.String(), "fell over")) // the cause is logged, not shown
	assert.Equal(t, others.Load(), int32(2))                          // the others finished before the answer
	assert.Equal(t, serve(h, http.MethodGet, "/admin/p").Code, http.StatusInternalServerError)
}

// What an editor among the panels needs of the page, such as its script, reaches the page whichever panel
// recorded it.
func TestAnEditorBesideOtherPanelsStillLinksTheEditor(t *testing.T) {
	t.Parallel()

	declared := webui.Page[webui.NoArgs]{Path: "/p", View: webui.Stack{
		webui.Table[Device]{
			Title: "Table",
			Loader: webui.PagedLoader[Device]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				return webui.PagedWindow[Device]{}, webui.Ready()
			}},
			Columns: []webui.Accessor[Device]{formID},
		},
		webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView, Language: lang.JSON},
	}}
	body := serve(webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin"), http.MethodGet, "/admin/p").Body.String()
	assert.Contains(t, body, `<script type="module" src="/admin/_webui/monaco/code-`)
	read := page.Parse(t, body)
	_, editable := read.Field("f0")
	assert.False(t, editable) // the editor is a viewer, and shows what it holds
	assert.Equal(t, len(read.ByRole("heading", "Config")), 1)
}
