package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func statusApp() http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{
		Path: "/s",
		View: webui.Table[Device]{
			Title: "Devices",
			Loader: webui.PagedLoader[Device]{Load: func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
				return webui.PagedWindow[Device]{Items: []Device{{Id: "a"}}, Total: 1}, webui.Ready()
			}},
			Key:     func(d Device) string { return d.Id },
			Columns: []webui.Accessor[Device]{formID},
		},
	}}}.MustCompile("/admin")
}

func TestTheStatusBarSaysHowThePageCameToBe(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC().Add(-time.Second)
	rec := serve(statusApp(), http.MethodGet, "/admin/s")
	p := page.Parse(t, rec.Body.String())

	// With script off, the bar says so: a reader of the page as a client with no script gets it, and one with script does not (the browser suite).
	assert.Equal(t, len(p.ByText("JS off")), 1)
	// The server's time, and when the HTML was built, in a form the script can read.
	assert.True(t, regexp.MustCompile(`^\d+(\.\d)? ms$`).MatchString(page.One(t, "the render time", p.ByAttr("data-build", "")).Text()))
	built, err := time.Parse(time.RFC3339Nano, page.One(t, "the build time", p.ByAttr("data-built", "")).Attr("datetime"))
	assert.NoError(t, err)
	assert.True(t, built.After(before) && !built.After(time.Now().UTC().Add(time.Second)))
	// The badge for a page that was not built for the request is there for the script to show.
	assert.True(t, page.One(t, "the stale badge", p.ByAttr("data-stale", "")).Hidden())

	// The browser's own tools get the time too, in the standard header, and there is no header of the framework's for it.
	assert.True(t, regexp.MustCompile(`^build;dur=\d+(\.\d)?$`).MatchString(rec.Header().Get("Server-Timing")))
	assert.Equal(t, rec.Header().Get("X-Webui-Built"), "")
}

func TestAPanelFetchedAgainCarriesItsOwnBuildTimeInElementsThatFollowIt(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/admin/s", nil)
	req.Header.Set("X-Webui-View", "p")
	rec := httptest.NewRecorder()
	statusApp().ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.Equal(t, rec.Header().Get("X-Webui-Built"), "")
	assert.True(t, strings.HasPrefix(rec.Header().Get("Server-Timing"), "build;dur="))
	assert.True(t, page.IsFragment(body)) // a panel is not the page
	p := page.Parse(t, body)
	assert.Equal(t, len(p.ByRole("banner", "")), 0)
	// It is followed by the two elements of the bar that it says, each with what it replaces.
	oob := page.One(t, "the elements that replace the bar's", p.ByAttr("data-oob", ""))
	render := page.One(t, "the render time", oob.ByAttr("data-build", ""))
	assert.Equal(t, render.Attr("data-replaces"), "[data-build]")
	assert.True(t, regexp.MustCompile(`^\d+(\.\d)? ms$`).MatchString(render.Text()))
	built := page.One(t, "the build time", oob.ByAttr("data-built", ""))
	assert.Equal(t, built.Attr("data-replaces"), "[data-built]")
	_, err := time.Parse(time.RFC3339Nano, built.Attr("datetime"))
	assert.NoError(t, err)
	assert.True(t, page.One(t, "the panel", p.Views()).Before(oob)) // after the panel, which is what the script puts in its place
}
