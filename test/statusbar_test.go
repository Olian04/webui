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
)

func statusApp() http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{
		Path: "/s",
		Body: webui.Table[Device]{
			Title: "Devices",
			Load: func(context.Context, webui.Query) (webui.Window[Device], error) {
				return webui.Window[Device]{Items: []Device{{Id: "a"}}, Total: 1}, nil
			},
			Key:     func(d Device) string { return d.Id },
			Columns: []webui.Accessor[Device]{formID},
		},
	}}}.MustCompile("/admin")
}

func TestTheStatusBarSaysHowThePageCameToBe(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC().Add(-time.Second)
	rec := serve(statusApp(), http.MethodGet, "/admin/s")
	body := rec.Body.String()

	// With script off, the bar says so, in markup that only a browser with script off shows.
	assert.Contains(t, body, `<noscript><span class="sb-badge sb-warn"`)
	assert.Contains(t, body, ">JS off</span>")
	// The server's time, and when the HTML was built, in a form the script can read.
	assert.True(t, regexp.MustCompile(`data-build[^>]*>\d+(\.\d)? ms<`).MatchString(body))
	m := regexp.MustCompile(`<time class="sb-opt" data-built datetime="([^"]+)"`).FindStringSubmatch(body)
	assert.NotNil(t, m)
	built, err := time.Parse(time.RFC3339Nano, m[1])
	assert.NoError(t, err)
	assert.True(t, built.After(before) && !built.After(time.Now().UTC().Add(time.Second)))
	// The badge for a page that was not built for the request is there for the script to show.
	assert.Contains(t, body, `data-stale hidden`)

	// The browser's own tools get the time too, in the standard header, and there is no header of the framework's for it.
	assert.True(t, regexp.MustCompile(`^build;dur=\d+(\.\d)?$`).MatchString(rec.Header().Get("Server-Timing")))
	assert.Equal(t, rec.Header().Get("X-Webui-Built"), "")
}

func TestAPanelFetchedAgainCarriesItsOwnBuildTimeInElementsThatFollowIt(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/admin/s", nil)
	req.Header.Set("X-Webui-Leaf", "p")
	rec := httptest.NewRecorder()
	statusApp().ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.Equal(t, rec.Header().Get("X-Webui-Built"), "")
	assert.True(t, strings.HasPrefix(rec.Header().Get("Server-Timing"), "build;dur="))
	assert.False(t, strings.Contains(body, "statusbar")) // a panel is not the page
	// It is followed by the two elements of the bar that it says, each with what it replaces.
	oob := body[strings.Index(body, "<div hidden data-oob>"):]
	assert.True(t, regexp.MustCompile(`data-build data-replaces="\[data-build\]"[^>]*>\d+(\.\d)? ms<`).MatchString(oob))
	m := regexp.MustCompile(`data-built data-replaces="\[data-built\]" datetime="([^"]+)"`).FindStringSubmatch(oob)
	assert.NotNil(t, m)
	_, err := time.Parse(time.RFC3339Nano, m[1])
	assert.NoError(t, err)
	assert.True(t, strings.Index(body, "<div hidden data-oob>") > strings.Index(body, `data-leaf="p"`)) // after the panel, which is what the script puts in its place
}
