package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func TestEveryPageServes(t *testing.T) {
	h := handler(t)
	for _, path := range []string{
		"/admin/device", "/admin/device/dev_27c38b", "/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events",
		"/admin/site", "/admin/site/Stockholm", "/admin/alert", "/admin/alert/alt_000", "/admin/settings", "/admin/retention",
		"/admin/system", "/admin/audit", "/admin/config", "/admin/rule", "/admin/files", "/admin/files/logs/2026/10", "/admin/files/logs/2026/10/collector.log", "/admin/handbook", "/admin/handbook/incidents", "/admin/handbook/sinks", "/admin/handbook/glossary", "/admin/",
		"/admin/compare", "/admin/loading/order", "/admin/loading/nested", "/admin/loading/failures", "/admin/loading/whole", "/admin/loading/guarded", "/admin/loading/fast", "/admin/loading/tabs", "/admin/layout/split", "/admin/layout/rail", "/admin/layout/columns", "/admin/layout/tabs", "/admin/layout/many-tabs", "/admin/layout/deep", "/admin/layout/review", "/admin/layout/runbook",
	} {
		rec := get(h, path)
		if rec.Code != http.StatusOK && rec.Code != http.StatusFound {
			t.Errorf("GET %s = %d", path, rec.Code)
		}
	}
	assert.Equal(t, get(h, "/admin/nope").Code, http.StatusNotFound)
}

func TestMustCompileAcceptsTheApp(t *testing.T) {
	assert.NotNil(t, app.MustCompile("/admin"))
}

func TestNavigationHasSectionsAndTheSitePageLightsItsOwnEntry(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/site/Stockholm").Body.String()
	for _, caption := range []string{"Platform", "Sites", "Configuration", "Operations"} {
		assert.Contains(t, body, `<div class="nav-section" title="`+caption+`">`+caption+`</div>`)
	}
	// SiteDetail lists an entry for each site, loaded when the page is drawn, and lights the one
	// that is on screen; the list of sites, which is another page, is not lit.
	assert.Contains(t, body, `<a class="nav-item active" href="/admin/site/Stockholm"`)
	assert.Contains(t, body, `<a class="nav-item" href="/admin/site/Malm%C3%B6"`)
	assert.Contains(t, body, `<a class="nav-item" href="/admin/site/G%C3%B6teborg"`)
	assert.Contains(t, body, `<a class="nav-item" href="/admin/site"`)
	assert.False(t, strings.Contains(body, `<a class="nav-item active" href="/admin/site"`))
	// The breadcrumb's parent is the page mounted at /site.
	assert.Contains(t, body, `<a href="/admin/site">Sites</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">Stockholm</span>`)
}

func TestTheLandingPageIsServedAtTheRoot(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/")
	assert.Equal(t, rec.Code, http.StatusOK) // not a redirect to the first entry
	assert.Contains(t, rec.Body.String(), `<a class="side-brand" href="/admin/"`)
	assert.Contains(t, rec.Body.String(), "Disk used") // the system status on it, as the values of a toolbar
	assert.Contains(t, rec.Body.String(), `<span class="toolbar-lbl">Health</span>`)
	assert.Contains(t, rec.Body.String(), `<span class="toolbar-lbl">Queue depth</span>`)

	// Without the slash the server's mux adds it.
	assert.Equal(t, get(h, "/admin").Header().Get("Location"), "/admin/")
}

func TestNavEntriesHaveIconsOrTheirInitial(t *testing.T) {
	body := get(handler(t), "/admin/alert").Body.String()
	assert.Contains(t, body, `<span class="ni ni-letter" aria-hidden="true">R</span> <span>Retention</span>`) // no icon
	assert.False(t, strings.Contains(body, `>A</span><span>Alerts</span>`))                                   // it has the bell
}

func TestATabIsNamedByItsLabelInTheAddress(t *testing.T) {
	body := get(handler(t), "/admin/device/dev_27c38b?tabs.tab=raw-events").Body.String()
	assert.Contains(t, body, `<a class="tab active" href="/admin/device/dev_27c38b?tabs.tab=raw-events" aria-current="page">Raw events</a>`)
}

func TestBrandLogoIsServedAndTheThemeCanBeOverridden(t *testing.T) {
	h := handler(t)
	logo := get(h, linked(t, h, "logo"))
	assert.Equal(t, logo.Header().Get("Content-Type"), "image/png")
	assert.True(t, strings.HasPrefix(logo.Body.String(), "\x89PNG"))

	a := app
	a.Theme = themeFor("#2f9e8f")
	themed, err := a.Compile("/admin")
	assert.NoError(t, err)
	assert.Contains(t, get(themed, linked(t, themed, "theme.css")).Body.String(), "--blue: #2f9e8f;")
	assert.Equal(t, linked(t, h, "theme.css"), "") // none by default

	// A colour cannot break out of its declaration: that is a Compile error.
	a.Theme = themeFor("red; } body { display: none")
	_, err = a.Compile("/admin")
	assert.Error(t, err)
}

func TestABrokenAppServesItsOwnErrors(t *testing.T) {
	a := app
	a.Pages = append(append(webui.Pages{}, a.Pages...), brokenPages...)
	h, err := a.Compile("/admin")
	assert.Error(t, err)
	rec := get(h, "/admin/anything")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.Contains(t, rec.Body.String(), "Failed to compile")
	assert.Contains(t, rec.Body.String(), "field When has unsupported type map[string]int")
	// One problem for each of the three pages, each for a reason of its own: the first field is in the path, so the
	// path is not also said to have no field for it.
	body := rec.Body.String()
	assert.Contains(t, body, "3 problems in this app")
	assert.False(t, strings.Contains(body, "the path declares"))
	assert.Contains(t, body, `page &#34;/broken/source&#34;`)
	assert.Contains(t, body, "a Table has more than one of Rows, Load and Feed")
	assert.Contains(t, body, `page &#34;/broken/pattern&#34;`)
	assert.Contains(t, body, "uses a lookahead, `(?=`, which RE2 does not support")
	assert.False(t, strings.Contains(body, "Perl")) // Go's own wording names a dialect that says nothing to the reader
}

func TestTheAppsMetricsAreCollectedForTheHostToServeAndAreNotServedByTheApp(t *testing.T) {
	reg := prometheus.NewRegistry()
	a := app
	a.Metrics = reg
	compiled, err := a.Compile("/admin")
	assert.NoError(t, err)
	h := routes(compiled)

	login := post(h, "/login", url.Values{"role": {"editor"}})
	req := httptest.NewRequest(http.MethodGet, "/admin/device", nil)
	for _, c := range login.Result().Cookies() {
		req.AddCookie(c)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)

	families, err := reg.Gather()
	assert.NoError(t, err)
	names := map[string]bool{}
	for _, f := range families {
		names[f.GetName()] = true
	}
	assert.True(t, names["webui_http_requests_total"])
	assert.True(t, names["webui_load_duration_seconds"])

	// The host serves its registry, and the app has no endpoint of its own for it.
	assert.Equal(t, get(h, "/metrics").Code, http.StatusOK)
	assert.Equal(t, get(compiled, "/admin/_webui/metrics").Code, http.StatusNotFound)
}
