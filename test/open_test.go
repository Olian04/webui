package webui_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// openArgs are the arguments of the page that the actions here open: a path argument, a flag and a query that is text.
type openArgs struct {
	Id    string
	Debug bool
	Q     string
}

var openDetails = webui.Page[openArgs]{Path: "/device/{id}", View: webui.Stack{}}

// openApp is a page with one action, whose Run is what a test says, and a page for it to open. What it logs is in logs.
func openApp(prefix string, run func(ctx context.Context) webui.Outcome) (h http.Handler, logs *bytes.Buffer) {
	logs = &bytes.Buffer{}
	start := webui.Page[webui.NoArgs]{Path: "/start", View: webui.Toolbar{Left: []webui.ToolbarItem{
		webui.Action[webui.NoArgs]{Label: "Go", Run: func(ctx context.Context, _ webui.NoArgs) (webui.Outcome, error) { return run(ctx), nil }},
	}}}
	return webui.App{Logger: slog.New(slog.NewTextHandler(logs, nil)), Pages: webui.Pages{start, openDetails}}.MustCompile(prefix), logs
}

// pressGo does the action, as a visitor's button does, and returns the answer.
func pressGo(h http.Handler, prefix string) *httptest.ResponseRecorder {
	return post(h, prefix+"/start", url.Values{"_view": {"p"}, "_act": {"0"}})
}

// What Open makes of a page and its arguments is the address that the visitor is sent to: the mount prefix in front, the path arguments in the path
// and escaped, and the others in the query, with the ones that are zero left out.
func TestOpenBuildsTheAddressTheVisitorIsSentTo(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		prefix string
		open   func(ctx context.Context) webui.Target
		want   string
	}{
		"a flag that is set": {"/admin", func(ctx context.Context) webui.Target {
			return webui.Open(ctx, openDetails, openArgs{Id: "abc", Debug: true})
		}, "/admin/device/abc?debug=true"},
		"a path argument and a query, escaped": {"", func(ctx context.Context) webui.Target {
			return webui.Open(ctx, openDetails, openArgs{Id: "a b/c", Q: "x&y"})
		}, "/device/a%20b%2Fc?q=x%26y"},
		"a page by its id": {"/admin", func(ctx context.Context) webui.Target {
			return webui.Open(ctx, webui.PageID[openArgs]("/device/{id}"), openArgs{Id: "x y"})
		}, "/admin/device/x%20y"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h, _ := openApp(tc.prefix, func(ctx context.Context) webui.Outcome { return webui.Success("done").Then(tc.open(ctx)) })
			rec := pressGo(h, tc.prefix)
			assert.Equal(t, rec.Code, http.StatusSeeOther)
			assert.Equal(t, rec.Header().Get("Location"), tc.want)
		})
	}
}

// A mistake in what Run opens or rejects is the app's, and is not the visitor's to see: the page says something went wrong, and the cause, which names what was
// meant, is logged. The page is a page that works, and nothing is kept for the next one.
func TestAMistakeInOpenOrRejectIsLoggedAndTheVisitorIsToldSomethingWentWrong(t *testing.T) {
	t.Parallel()

	changed := openDetails
	changed.Path = "/CHANGED/{id}" // a declaration that was changed after it was compiled is not the one that is mounted
	for name, tc := range map[string]struct {
		run  func(ctx context.Context) webui.Outcome
		said string // what the log names
	}{
		"a page that is not mounted": {func(ctx context.Context) webui.Outcome {
			return webui.Success("x").Then(webui.Open(ctx, webui.Page[openArgs]{Path: "/never-mounted/{id}"}, openArgs{Id: "x"}))
		}, "/never-mounted/{id}"},
		"a page whose declaration was changed": {func(ctx context.Context) webui.Outcome {
			return webui.Success("x").Then(webui.Open(ctx, changed, openArgs{Id: "x"}))
		}, "/CHANGED/{id}"},
		"a path argument that is empty": {func(ctx context.Context) webui.Outcome {
			return webui.Success("x").Then(webui.Open(ctx, openDetails, openArgs{}))
		}, "/device/{id}"},
		"a context of no request": {func(context.Context) webui.Outcome {
			return webui.Success("x").Then(webui.Open(context.Background(), openDetails, openArgs{Id: "x"}))
		}, "/device/{id}"},
		"a rejection about another model": {func(context.Context) webui.Outcome {
			return webui.Reject(webui.Field[int](webui.Int[int]{Label: "n"}, "bad"))
		}, "Reject"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h, logs := openApp("/admin", tc.run)
			rec := pressGo(h, "/admin")
			shown := page.Parse(t, follow(h, rec).Body.String())
			assert.True(t, strings.Contains(shown.Root().Text(), "Something went wrong"))
			assert.True(t, strings.Contains(logs.String(), tc.said))
			assert.False(t, strings.Contains(shown.Root().AllText(), tc.said)) // the cause is for the log
		})
	}
}
