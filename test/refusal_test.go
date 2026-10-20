package webui_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type recordArgs struct{ ID string }

const recordsPath webui.PageID[webui.NoArgs] = "/records"

// refusalApp is a list of records and a page for one of them, whose Guard refuses in the way that its id asks.
func refusalApp(log *slog.Logger) http.Handler {
	list := webui.Page[webui.NoArgs]{Path: recordsPath, Nav: webui.Nav{Label: "Records"}, Body: webui.Stack{}}
	one := webui.Page[recordArgs]{
		Path: "/records/{id}",
		Guard: func(ctx context.Context, a recordArgs) error {
			back := webui.Open(ctx, recordsPath, webui.NoArgs{})
			switch a.ID {
			case "missing":
				return webui.NotFound("There is no record missing.").WithLink("Back to the records", back)
			case "credential":
				return webui.Unauthorized("A credential is needed to read this.").WithLink("Set one up", back)
			case "closed":
				return webui.Forbidden("This record is closed.")
			case "wrapped":
				return fmt.Errorf("checking: %w", webui.NotFound("There is no record wrapped."))
			case "plain":
				return errors.New("only for editors")
			}
			return nil
		},
		Body: webui.Stack{
			// Something that posts, so that the page has a POST to refuse.
			webui.Toolbar[recordArgs]{Left: []webui.ToolbarItem[recordArgs]{
				webui.Action[recordArgs]{Label: "Close", Run: func(context.Context, recordArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }},
			}},
			webui.Table[Device]{
				Title: "Events",
				Load: func(_ context.Context, _ webui.Query) (webui.Window[Device], error) {
					return webui.Window[Device]{}, nil
				},
				Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
			},
		},
	}
	return webui.App{Logger: log, Pages: webui.Pages{list, one}}.MustCompile("/admin")
}

// A Guard says what a visitor may be told, with the status that goes with it, and a way on.
func TestAGuardThatRefusesWithAStatusAnswersWithIt(t *testing.T) {
	t.Parallel()

	h := refusalApp(nil)
	for id, want := range map[string]struct {
		status int
		title  string
		text   string
	}{
		"missing":    {http.StatusNotFound, "Not found", "There is no record missing."},
		"credential": {http.StatusUnauthorized, "Not signed in", "A credential is needed to read this."},
		"closed":     {http.StatusForbidden, "Not permitted", "This record is closed."},
		"wrapped":    {http.StatusNotFound, "Not found", "There is no record wrapped."},
	} {
		rec := serve(h, http.MethodGet, "/admin/records/"+id)
		assert.Equal(t, rec.Code, want.status)
		body := rec.Body.String()
		assert.Contains(t, body, `<p class="state-title">`+want.title+`</p>`)
		assert.Contains(t, body, want.text)
		assert.False(t, strings.Contains(body, "ran before anything was loaded"))
		assert.False(t, strings.Contains(body, `class="panel`)) // and nothing was read
	}

	// The link on is a real link to a page of the app.
	body := serve(h, http.MethodGet, "/admin/records/missing").Body.String()
	assert.Contains(t, body, `<a class="btn btn-ghost btn-sm" href="/admin/records">Back to the records</a>`)
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/records/closed").Body.String(), `class="btn btn-ghost btn-sm"`))
}

// Any other error is as it was: a 403 with its text, and not how the check was made.
func TestAGuardThatReturnsAnyOtherErrorStillAnswers403(t *testing.T) {
	t.Parallel()

	rec := serve(refusalApp(nil), http.MethodGet, "/admin/records/plain")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), `<p class="state-desc">only for editors</p>`)
	assert.False(t, strings.Contains(rec.Body.String(), "ran before anything was loaded"))

	// An error with no words of its own says that there is no access, and nothing else.
	page := webui.Page[webui.NoArgs]{Path: "/quiet", Body: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error { return errors.New("") }}
	rec = serve(webui.App{Pages: webui.Pages{page}}.MustCompile("/admin"), http.MethodGet, "/admin/quiet")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), `<p class="state-desc">You do not have access to this page.</p>`)
}

// A POST to a page that refuses is answered, as any is, with a redirect to the page, which says it.
func TestAPostToAPageThatRefusesIsSentToWhatSaysIt(t *testing.T) {
	t.Parallel()

	h := refusalApp(nil)
	rec := post(h, "/admin/records/missing", url.Values{"_leaf": {"p.0"}, "_act": {"0"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, serve(h, http.MethodGet, rec.Header().Get("Location")).Code, http.StatusNotFound)
}

// A refusal is told to the visitor and is not the app's failure, so it is not an error in its log.
func TestARefusalIsNotLoggedAsAFailure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := refusalApp(slog.New(slog.NewTextHandler(&buf, nil)))
	serve(h, http.MethodGet, "/admin/records/missing")
	serve(h, http.MethodGet, "/admin/records/credential")
	assert.Equal(t, buf.Len(), 0)
}

// A panel's Load can say that what it shows is not there. The page has a status already, so the panel says it, with its link,
// and the page stays as it is.
func TestAPanelWhoseLoadRefusesSaysSoAndThePageStays(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	page := webui.Page[webui.NoArgs]{Path: "/stuff", Body: webui.Stack{
		webui.Table[Device]{
			Title: "Gone",
			Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], error) {
				return webui.Window[Device]{}, webui.NotFound("This device was deleted.").WithLink("See the others", webui.Open(ctx, recordsPath, webui.NoArgs{}))
			},
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
		},
		webui.Form[Device]{
			Title:  "Details",
			Load:   func(context.Context) (Device, error) { return Device{}, webui.Forbidden("Not for you.") },
			Fields: []webui.Accessor[Device]{formID},
		},
		webui.Table[Device]{
			Title: "Broken",
			Load: func(context.Context, webui.Query) (webui.Window[Device], error) {
				return webui.Window[Device]{}, errors.New("the database is down")
			},
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
		},
	}}
	list := webui.Page[webui.NoArgs]{Path: recordsPath, Body: webui.Stack{}}
	h := webui.App{Logger: slog.New(slog.NewTextHandler(&buf, nil)), Pages: webui.Pages{page, list}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/stuff")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.Contains(t, body, `<div class="empty-title">Not found</div>`)
	assert.Contains(t, body, "This device was deleted.")
	assert.Contains(t, body, `<a class="btn btn-ghost btn-sm" href="/admin/records">See the others</a>`)
	assert.Contains(t, body, "Not for you.")
	// What fails is as it was: "Could not load", once, and the cause in the log and not in the page.
	assert.Equal(t, strings.Count(body, "Could not load"), 1)
	assert.False(t, strings.Contains(body, "the database is down"))
	assert.Equal(t, strings.Count(buf.String(), "level=ERROR"), 1)
	assert.Contains(t, buf.String(), "the database is down")
}
