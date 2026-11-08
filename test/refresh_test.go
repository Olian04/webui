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
	"github.com/Olian04/webui/test/util/page"
)

// refreshApp is a page of two tables, the first of which returns what it is told to of whether it is final, and the second never does.
func refreshApp(say webui.Loaded) http.Handler {
	table := func(title string, say webui.Loaded) webui.Table[stock] {
		return webui.Table[stock]{
			Title: title, Columns: stockCols,
			Loader: webui.PagedLoader[stock]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[stock], webui.Loaded) {
				return webui.PagedWindow[stock]{Items: []stock{{Name: "bolt"}}, Total: 1}, say
			}},
		}
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", View: webui.Stack{table("Job", say), table("Other", webui.Ready())}}}}.MustCompile("/admin")
}

// A panel whose Load says that it is not final says so, to the reader in its head and to script in an attribute, and only that panel does.
func TestAPanelWhoseLoadIsNotFinalSaysSoAndOnlyThatPanelDoes(t *testing.T) {
	t.Parallel()

	h := refreshApp(webui.Ready().RefreshAfter(5 * time.Second))
	p := pageOf(t, h, "/admin/s")
	assert.Equal(t, len(p.ByAttr("data-refresh-after", "")), 1)
	// It is the first table's panel that carries it, and the second has no say.
	assert.DeepEqual(t, askedAgain(p), map[string]string{"p.0": "5"})
	assert.Equal(t, len(p.ByAllText("Updating…")), 1)
	assert.Equal(t, len(p.ByAllText("Updating, reload in 5 s")), 1)

	// The same request for that panel alone, as script makes it, says it too: the answer that asks again is the one that carries it.
	panel := serveView(h, "/admin/s", "p.0")
	assert.Equal(t, panel.Code, http.StatusOK)
	assert.DeepEqual(t, askedAgain(page.Parse(t, panel.Body.String())), map[string]string{"p.0": "5"})
	assert.DeepEqual(t, askedAgain(page.Parse(t, serveView(h, "/admin/s", "p.1").Body.String())), map[string]string{})
}

// A panel that is ready and says nothing more is final: the answer that does not say it is what ends the loop.
func TestAPanelThatIsReadyIsFinal(t *testing.T) {
	t.Parallel()

	for name, loaded := range map[string]webui.Loaded{"ready": webui.Ready(), "the zero value": {}, "failed with nothing": webui.Failed(nil)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := pageOf(t, refreshApp(loaded), "/admin/s")
			assert.Equal(t, len(p.ByAttr("data-refresh-after", "")), 0)
			assert.False(t, strings.Contains(p.Root().AllText(), "Updating"))
		})
	}
}

// A load that failed has nothing to show again, so it says nothing of refreshing, however it was told to; and the failure is still the failure.
func TestAFailedLoadDoesNotRefresh(t *testing.T) {
	t.Parallel()

	rec := serve(refreshApp(webui.Failed(errors.New("the backend is down")).RefreshAfter(3*time.Second)), http.MethodGet, "/admin/s")
	failed := page.Parse(t, rec.Body.String())
	assert.Equal(t, len(failed.ByText("Could not load")), 1)
	assert.Equal(t, len(failed.ByAttr("data-refresh-after", "")), 0)
	assert.False(t, strings.Contains(failed.Root().AllText(), "the backend is down")) // the cause is logged, never shown

	// A refusal is still a refusal, with its words.
	refused := pageOf(t, refreshApp(webui.Failed(webui.Forbidden("Not for you.")).RefreshAfter(3*time.Second)), "/admin/s")
	assert.Equal(t, len(refused.ByText("Not for you.")), 1)
	assert.Equal(t, len(refused.ByAttr("data-refresh-after", "")), 0)
}

// A delay of less than a second is asked in a second, and is in whole seconds, up.
func TestTheDelayIsAtLeastASecondInWholeSeconds(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		d    time.Duration
		want string
	}{
		"zero":          {0, "1"},
		"negative":      {-time.Minute, "1"},
		"a part of one": {1500 * time.Millisecond, "2"},
		"a minute":      {time.Minute, "60"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.DeepEqual(t, askedAgain(pageOf(t, refreshApp(webui.Ready().RefreshAfter(c.d)), "/admin/s")), map[string]string{"p.0": c.want})
		})
	}
}

// Every kind of panel that has a Load says it: the three sources of a table, a form, an editor's markdown and diff, and a file upload.
func TestEveryPanelWithALoadCanSayItIsNotFinal(t *testing.T) {
	t.Parallel()

	not := webui.Ready().RefreshAfter(4 * time.Second)
	app := func(body webui.View) http.Handler {
		return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", View: body}}}.MustCompile("/admin")
	}
	for name, h := range map[string]http.Handler{
		"Rows": app(webui.Table[stock]{Title: "Items", Columns: stockCols, Loader: webui.RowsLoader[stock]{Load: func(context.Context) ([]stock, webui.Loaded) { return nil, not }}}),
		"Feed": app(webui.Table[stock]{Title: "Items", Columns: stockCols, Loader: webui.FeedLoader[stock]{Load: func(context.Context, string, int) ([]stock, string, webui.Loaded) { return nil, "", not }}}),
		"Form": app(webui.Form[stock]{Title: "Item", Fields: []webui.Accessor[stock]{stockCols[0]}, Load: func(context.Context) (stock, webui.Loaded) { return stock{}, not }}),
		"Markdown": app(webui.Markdown[handbook]{
			Title: "Handbook", Content: handbookText, Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "# Hi"}, not },
		}),
		"Diff": app(webui.Diff[snippet]{
			Title: "Changes", Original: snippetOld, Modified: snippetView, Load: func(context.Context) (snippet, webui.Loaded) { return snippet{}, not },
		}),
		"FileUpload": app(webui.FileUpload[struct{}]{
			Title: "Upload", MaxSize: 1 << 20, Load: func(context.Context) (struct{}, webui.Loaded) { return struct{}{}, not },
			Receive: webui.Action[webui.FileHandle[struct{}]]{Run: func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
				return webui.Success(""), nil
			}},
		}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := pageOf(t, h, "/admin/s")
			assert.DeepEqual(t, askedAgain(p), map[string]string{"p": "4"})
			assert.Equal(t, len(p.ByAllText("Updating, reload in 4 s")), 1)
		})
	}
}

// askedAgain are the panels of a page that ask for themselves again, by address and the seconds they wait.
func askedAgain(p *page.Page) map[string]string {
	got := map[string]string{}
	for _, v := range p.Views() {
		if v.Has("data-refresh-after") {
			got[v.Attr("data-view")] = v.Attr("data-refresh-after")
		}
	}
	return got
}
