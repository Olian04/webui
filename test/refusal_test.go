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
	"github.com/Olian04/webui/test/util/page"
)

type recordArgs struct{ ID string }

const recordsPath webui.PageID[webui.NoArgs] = "/records"

// refusalApp is a list of records and a page for one of them, whose Guard refuses in the way that its id asks.
func refusalApp(log *slog.Logger) http.Handler {
	list := webui.Page[webui.NoArgs]{Path: recordsPath, Nav: webui.Nav{Label: "Records"}, View: webui.Stack{}}
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
		View: webui.Stack{
			// Something that posts, so that the page has a POST to refuse.
			webui.Toolbar{Left: []webui.ToolbarItem{
				webui.Action[webui.NoArgs]{Label: "Close", Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }},
			}},
			webui.Table[Device]{
				Title: "Events",
				Loader: webui.PagedLoader[Device]{Load: func(_ context.Context, _ webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
					return webui.PagedWindow[Device]{}, webui.Ready()
				}},
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
		refused := page.Parse(t, rec.Body.String())
		assert.Equal(t, len(refused.ByRole("heading", want.title)), 1)
		assert.Equal(t, len(refused.ByText(want.text)), 1)
		assert.False(t, strings.Contains(refused.Root().AllText(), "ran before anything was loaded"))
		assert.Equal(t, len(refused.Views()), 0) // and nothing was read
	}

	// The link on is a real link to a page of the app.
	assert.True(t, hasLink(pageOf(t, h, "/admin/records/missing"), "Back to the records", "/admin/records"))
	assert.Equal(t, len(page.One(t, "main", pageOf(t, h, "/admin/records/closed").ByRole("main", "")).Links()), 0)
}

// Any other error is as it was: a 403 with its text, and not how the check was made.
func TestAGuardThatReturnsAnyOtherErrorStillAnswers403(t *testing.T) {
	t.Parallel()

	rec := serve(refusalApp(nil), http.MethodGet, "/admin/records/plain")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	refused := page.Parse(t, rec.Body.String())
	assert.Equal(t, len(refused.ByText("only for editors")), 1)
	assert.False(t, strings.Contains(refused.Root().AllText(), "ran before anything was loaded"))

	// An error with no words of its own says that there is no access, and nothing else.
	quiet := webui.Page[webui.NoArgs]{Path: "/quiet", View: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error { return errors.New("") }}
	rec = serve(webui.App{Pages: webui.Pages{quiet}}.MustCompile("/admin"), http.MethodGet, "/admin/quiet")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Equal(t, len(page.Parse(t, rec.Body.String()).ByText("You do not have access to this page.")), 1)
}

// A POST to a page that refuses is answered, as any is, with a redirect to the page, which says it.
func TestAPostToAPageThatRefusesIsSentToWhatSaysIt(t *testing.T) {
	t.Parallel()

	h := refusalApp(nil)
	rec := post(h, "/admin/records/missing", url.Values{"_view": {"p.0"}, "_act": {"0"}})
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
	stuff := webui.Page[webui.NoArgs]{Path: "/stuff", View: webui.Stack{
		webui.Table[Device]{
			Title: "Gone",
			Loader: webui.PagedLoader[Device]{Load: func(ctx context.Context, _ webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				return webui.PagedWindow[Device]{}, webui.Failed(webui.NotFound("This device was deleted.").WithLink("See the others", webui.Open(ctx, recordsPath, webui.NoArgs{})))
			}},
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
		},
		webui.Form[Device]{
			Title: "Details",
			Load: func(context.Context) (Device, webui.Loaded) {
				return Device{}, webui.Failed(webui.Forbidden("Not for you."))
			},
			Fields: []webui.Accessor[Device]{formID},
		},
		webui.Table[Device]{
			Title: "Broken",
			Loader: webui.PagedLoader[Device]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				return webui.PagedWindow[Device]{}, webui.Failed(errors.New("the database is down"))
			}},
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
		},
	}}
	list := webui.Page[webui.NoArgs]{Path: recordsPath, View: webui.Stack{}}
	h := webui.App{Logger: slog.New(slog.NewTextHandler(&buf, nil)), Pages: webui.Pages{stuff, list}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/stuff")
	assert.Equal(t, rec.Code, http.StatusOK)
	shown := page.Parse(t, rec.Body.String())
	assert.Equal(t, len(shown.ByText("Not found")), 1)
	assert.Equal(t, len(shown.ByText("This device was deleted.")), 1)
	assert.True(t, hasLink(shown, "See the others", "/admin/records"))
	assert.Equal(t, len(shown.ByText("Not for you.")), 1)
	// What fails is as it was: "Could not load", once, and the cause in the log and not in the page.
	assert.Equal(t, len(shown.ByText("Could not load")), 1)
	assert.False(t, strings.Contains(shown.Root().AllText(), "the database is down"))
	assert.Equal(t, strings.Count(buf.String(), "level=ERROR"), 1)
	assert.Contains(t, buf.String(), "the database is down")
}
