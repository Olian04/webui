package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
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
			Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], error) {
				meet(ctx, &arrived, 4, &met)
				return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, nil
			},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	page := webui.Page[webui.NoArgs]{Path: "/p", Body: webui.Stack{
		webui.Split{
			webui.Form[Device]{
				Title:  "Form",
				Load:   func(ctx context.Context) (Device, error) { meet(ctx, &arrived, 4, &met); return Device{Id: "x"}, nil },
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
	page := webui.Page[webui.NoArgs]{Path: "/p", Body: webui.Stack{
		webui.Table[Device]{
			Title: "Fine",
			Load: func(context.Context, webui.Query) (webui.Window[Device], error) {
				others.Add(1)
				return webui.Window[Device]{}, nil
			},
			Columns: []webui.Accessor[Device]{formID},
		},
		webui.Form[Device]{
			Title:  "Broken",
			Load:   func(context.Context) (Device, error) { panic("the backend fell over") },
			Fields: []webui.Accessor[Device]{formID},
		},
		webui.Table[Device]{
			Title: "Also fine",
			Load: func(context.Context, webui.Query) (webui.Window[Device], error) {
				others.Add(1)
				return webui.Window[Device]{}, nil
			},
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

	page := webui.Page[webui.NoArgs]{Path: "/p", Body: webui.Stack{
		webui.Table[Device]{
			Title:   "Table",
			Load:    func(context.Context, webui.Query) (webui.Window[Device], error) { return webui.Window[Device]{}, nil },
			Columns: []webui.Accessor[Device]{formID},
		},
		webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView, Language: webui.LangJSON},
	}}
	body := serve(webui.App{Pages: webui.Pages{page}}.MustCompile("/admin"), http.MethodGet, "/admin/p").Body.String()
	assert.Contains(t, body, `<script type="module" src="/admin/_webui/monaco/code-`)
	assert.Contains(t, body, `<pre class="code-view"`)
}
