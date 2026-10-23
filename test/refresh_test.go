package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// refreshApp is a page of two tables, the first of which says what it is told to of whether it is final, and the second never does.
func refreshApp(say func(ctx context.Context)) http.Handler {
	table := func(title string, say func(context.Context)) webui.Table[stock] {
		return webui.Table[stock]{
			Title: title, Columns: stockCols,
			Load: func(ctx context.Context, _ webui.Query) (webui.Window[stock], error) {
				if say != nil {
					say(ctx)
				}
				return webui.Window[stock]{Items: []stock{{Name: "bolt"}}, Total: 1}, nil
			},
		}
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: webui.Stack{table("Job", say), table("Other", nil)}}}}.MustCompile("/admin")
}

// A panel whose Load says that it is not final says so, to the reader in its head and to script in an attribute, and only that panel does.
func TestAPanelWhoseLoadIsNotFinalSaysSoAndOnlyThatPanelDoes(t *testing.T) {
	t.Parallel()

	h := refreshApp(func(ctx context.Context) { webui.RefreshAfter(ctx, 5*time.Second) })
	body := serve(h, http.MethodGet, "/admin/s").Body.String()
	assert.Equal(t, strings.Count(body, "data-refresh-after="), 1)
	assert.Contains(t, body, `data-refresh-after="5"`)
	assert.Contains(t, body, `<span class="refresh-js">Updating…</span>`)
	assert.Contains(t, body, `<span class="refresh-nojs">Updating, reload in 5 s</span>`)
	// It is the panel of the first table that carries it, and the second has no say.
	first := body[strings.Index(body, `data-leaf="p.0"`):]
	assert.True(t, strings.Index(first, "data-refresh-after") < strings.Index(first, `data-leaf="p.1"`))
	assert.True(t, strings.Index(first, `data-leaf="p.1"`) > 0)
	second := first[strings.Index(first, `data-leaf="p.1"`):]
	assert.False(t, strings.Contains(second, "data-refresh-after"))

	// The same request for that panel alone, as script makes it, says it too: the answer that asks again is the one that carries it.
	leaf := serveLeaf(h, "/admin/s", "p.0")
	assert.Equal(t, leaf.Code, http.StatusOK)
	assert.Contains(t, leaf.Body.String(), `data-refresh-after="5"`)
	assert.False(t, strings.Contains(serveLeaf(h, "/admin/s", "p.1").Body.String(), "data-refresh-after"))
}

// A panel that is final, which says nothing, has nothing of it: the answer that does not say it is what ends the loop.
func TestAPanelThatSaysNothingIsFinal(t *testing.T) {
	t.Parallel()

	body := serve(refreshApp(nil), http.MethodGet, "/admin/s").Body.String()
	assert.False(t, strings.Contains(body, "data-refresh-after"))
	assert.False(t, strings.Contains(body, "refresh-note"))
}

// A panel that asks to be asked again sooner than a second is asked for in a second, and of several calls the shortest counts, whichever order they come in.
func TestTheDelayIsAtLeastASecondAndTheShortestCallCounts(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		say  func(ctx context.Context)
		want string
	}{
		"zero":          {func(ctx context.Context) { webui.RefreshAfter(ctx, 0) }, `data-refresh-after="1"`},
		"negative":      {func(ctx context.Context) { webui.RefreshAfter(ctx, -time.Minute) }, `data-refresh-after="1"`},
		"a part of one": {func(ctx context.Context) { webui.RefreshAfter(ctx, 1500*time.Millisecond) }, `data-refresh-after="2"`}, // whole seconds, up
		"shortest, first": {func(ctx context.Context) {
			webui.RefreshAfter(ctx, 3*time.Second)
			webui.RefreshAfter(ctx, 9*time.Second)
		}, `data-refresh-after="3"`},
		"shortest, last": {func(ctx context.Context) {
			webui.RefreshAfter(ctx, 9*time.Second)
			webui.RefreshAfter(ctx, 3*time.Second)
		}, `data-refresh-after="3"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, serve(refreshApp(c.say), http.MethodGet, "/admin/s").Body.String(), c.want)
		})
	}
}

// Outside a Load there is nothing to say it to, and it does nothing.
func TestRefreshAfterOutsideALoadDoesNothing(t *testing.T) {
	t.Parallel()

	webui.RefreshAfter(context.Background(), time.Second)
}

// Every kind of leaf that has a Load with a context says it, and a form is one: a Form that a job is shown in has the same head.
func TestAFormSaysItIsNotFinalToo(t *testing.T) {
	t.Parallel()

	f := okForm()
	load := f.Load
	f.Load = func(ctx context.Context) (Device, error) {
		webui.RefreshAfter(ctx, 4*time.Second)
		return load(ctx)
	}
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()
	assert.Contains(t, body, `data-refresh-after="4"`)
	assert.Contains(t, body, "Updating, reload in 4 s")
}

// There is no harness for script, so this is what the shipped file carries: that it reads the attribute, waits for a tab that is hidden and
// for focus that is inside the panel, asks for the one panel as every refresh does, and ends on a failure and when the panel no longer says it.
func TestTheScriptThatAsksAgainDoesWhatItSaysItDoes(t *testing.T) {
	t.Parallel()

	js := serve(refreshApp(nil), http.MethodGet, assetURL(t, refreshApp(nil), "enhance.js")).Body.String()
	for _, want := range []string{
		"[data-leaf][data-refresh-after]", "function armRefresh", "async function pollLeaf", "document.hidden", "'visibilitychange'",
		"el.contains(document.activeElement)", "fetchLeaf(id, location.pathname + location.search", "el.replaceWith(next)",
	} {
		assert.Contains(t, js, want)
	}
	// A poll is not a navigation: it neither pushes the address nor asks for the panels around it.
	poll := js[strings.Index(js, "async function pollLeaf"):]
	poll = poll[:strings.Index(poll, "armRefresh(document);")]
	assert.False(t, strings.Contains(poll, "pushState"))
	assert.False(t, strings.Contains(poll, "settle("))
	assert.False(t, strings.Contains(poll, "location.href"))
}
