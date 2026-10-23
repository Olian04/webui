package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// refreshApp is a page of two tables, the first of which returns what it is told to of whether it is final, and the second never does.
func refreshApp(say webui.Loaded) http.Handler {
	table := func(title string, say webui.Loaded) webui.Table[stock] {
		return webui.Table[stock]{
			Title: title, Columns: stockCols,
			Load: func(context.Context, webui.Query) (webui.Window[stock], webui.Loaded) {
				return webui.Window[stock]{Items: []stock{{Name: "bolt"}}, Total: 1}, say
			},
		}
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: webui.Stack{table("Job", say), table("Other", webui.Ready())}}}}.MustCompile("/admin")
}

// A panel whose Load says that it is not final says so, to the reader in its head and to script in an attribute, and only that panel does.
func TestAPanelWhoseLoadIsNotFinalSaysSoAndOnlyThatPanelDoes(t *testing.T) {
	t.Parallel()

	h := refreshApp(webui.Ready().RefreshAfter(5 * time.Second))
	body := serve(h, http.MethodGet, "/admin/s").Body.String()
	assert.Equal(t, strings.Count(body, "data-refresh-after="), 1)
	assert.Contains(t, body, `data-refresh-after="5"`)
	assert.Contains(t, body, `<span class="refresh-js" data-refresh-state>Updating…</span>`)
	assert.Contains(t, body, `<span class="refresh-nojs">Updating, reload in 5 s</span>`)
	// It is the first table's panel that carries it, and the second has no say.
	first := body[strings.Index(body, `data-leaf="p.0"`):]
	second := first[strings.Index(first, `data-leaf="p.1"`):]
	assert.True(t, strings.Contains(first[:len(first)-len(second)], "data-refresh-after"))
	assert.False(t, strings.Contains(second, "data-refresh-after"))

	// The same request for that panel alone, as script makes it, says it too: the answer that asks again is the one that carries it.
	leaf := serveLeaf(h, "/admin/s", "p.0")
	assert.Equal(t, leaf.Code, http.StatusOK)
	assert.Contains(t, leaf.Body.String(), `data-refresh-after="5"`)
	assert.False(t, strings.Contains(serveLeaf(h, "/admin/s", "p.1").Body.String(), "data-refresh-after"))
}

// A panel that is ready and says nothing more is final: the answer that does not say it is what ends the loop.
func TestAPanelThatIsReadyIsFinal(t *testing.T) {
	t.Parallel()

	for name, loaded := range map[string]webui.Loaded{"ready": webui.Ready(), "the zero value": {}, "failed with nothing": webui.Failed(nil)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := serve(refreshApp(loaded), http.MethodGet, "/admin/s").Body.String()
			assert.False(t, strings.Contains(body, "data-refresh-after"))
			assert.False(t, strings.Contains(body, "refresh-note"))
		})
	}
}

// A load that failed has nothing to show again, so it says nothing of refreshing, however it was told to; and the failure is still the failure.
func TestAFailedLoadDoesNotRefresh(t *testing.T) {
	t.Parallel()

	rec := serve(refreshApp(webui.Failed(errors.New("the backend is down")).RefreshAfter(3*time.Second)), http.MethodGet, "/admin/s")
	body := rec.Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "data-refresh-after"))
	assert.False(t, strings.Contains(body, "the backend is down")) // the cause is logged, never shown

	// A refusal is still a refusal, with its words.
	refused := serve(refreshApp(webui.Failed(webui.Forbidden("Not for you.")).RefreshAfter(3*time.Second)), http.MethodGet, "/admin/s").Body.String()
	assert.Contains(t, refused, "Not for you.")
	assert.False(t, strings.Contains(refused, "data-refresh-after"))
}

// A delay of less than a second is asked in a second, and is in whole seconds, up.
func TestTheDelayIsAtLeastASecondInWholeSeconds(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		d    time.Duration
		want string
	}{
		"zero":          {0, `data-refresh-after="1"`},
		"negative":      {-time.Minute, `data-refresh-after="1"`},
		"a part of one": {1500 * time.Millisecond, `data-refresh-after="2"`},
		"a minute":      {time.Minute, `data-refresh-after="60"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, serve(refreshApp(webui.Ready().RefreshAfter(c.d)), http.MethodGet, "/admin/s").Body.String(), c.want)
		})
	}
}

// Every kind of leaf that has a Load says it: the three sources of a table, a form, an editor's markdown and diff, and a file upload.
func TestEveryLeafWithALoadCanSayItIsNotFinal(t *testing.T) {
	t.Parallel()

	not := webui.Ready().RefreshAfter(4 * time.Second)
	page := func(body webui.PageBody) http.Handler {
		return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: body}}}.MustCompile("/admin")
	}
	for name, h := range map[string]http.Handler{
		"Rows": page(webui.Table[stock]{Title: "Items", Columns: stockCols, Rows: func(context.Context) ([]stock, webui.Loaded) { return nil, not }}),
		"Feed": page(webui.Table[stock]{Title: "Items", Columns: stockCols, Feed: func(context.Context, string, int) ([]stock, string, webui.Loaded) { return nil, "", not }}),
		"Form": page(webui.Form[stock]{Title: "Item", Fields: []webui.Accessor[stock]{stockCols[0]}, Load: func(context.Context) (stock, webui.Loaded) { return stock{}, not }}),
		"Markdown": page(webui.Markdown[handbook]{
			Title: "Handbook", Content: handbookText, Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "# Hi"}, not },
		}),
		"Diff": page(webui.Diff[snippet]{
			Title: "Changes", Original: snippetOld, Modified: snippetView, Load: func(context.Context) (snippet, webui.Loaded) { return snippet{}, not },
		}),
		"FileUpload": page(webui.FileUpload[struct{}]{
			Title: "Upload", MaxSize: 1 << 20, Load: func(context.Context) (struct{}, webui.Loaded) { return struct{}{}, not },
			Receive: webui.Action[webui.FileHandle[struct{}]]{Run: func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
				return webui.Success(""), nil
			}},
		}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := serve(h, http.MethodGet, "/admin/s").Body.String()
			assert.Contains(t, body, `data-refresh-after="4"`)
			assert.Contains(t, body, "Updating, reload in 4 s")
		})
	}
}

// There is no harness for script, so this is what the shipped file carries: that it reads the attribute, waits for a tab that is hidden and
// for focus that is inside the panel, asks for the one panel as every refresh does, and ends on a failure and when the panel no longer says it.
func TestTheScriptThatAsksAgainDoesWhatItSaysItDoes(t *testing.T) {
	t.Parallel()

	js := serve(refreshApp(webui.Ready()), http.MethodGet, assetURL(t, refreshApp(webui.Ready()), "enhance.js")).Body.String()
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
