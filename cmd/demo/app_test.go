package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/pkg/prom"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestEveryPageServes(t *testing.T) {
	h := handler(t)
	for _, path := range []string{
		"/admin/device", "/admin/device/dev_27c38b", "/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events",
		"/admin/site", "/admin/site/Stockholm", "/admin/alert", "/admin/alert/alt_000", "/admin/settings", "/admin/retention",
		"/admin/system", "/admin/audit", "/admin/config", "/admin/rule", "/admin/files", "/admin/files/logs/2026/10", "/admin/files/logs/2026/10/collector.log", "/admin/handbook", "/admin/handbook/incidents", "/admin/handbook/sinks", "/admin/handbook/glossary", "/admin/",
		"/admin/compare", "/admin/loading/order", "/admin/loading/nested", "/admin/loading/failures", "/admin/loading/whole", "/admin/loading/guarded", "/admin/loading/fast", "/admin/loading/tabs", "/admin/layout/split", "/admin/layout/rail", "/admin/layout/columns", "/admin/layout/weights", "/admin/layout/tabs", "/admin/layout/many-tabs", "/admin/layout/deep", "/admin/layout/review", "/admin/layout/runbook", "/admin/layout/toolbars", "/admin/layout/list",
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
	p := pageAt(t, h, "/admin/site/Stockholm")
	sidebar := page.One(t, "the sidebar", p.ByRole("navigation", "Main"))
	for _, caption := range []string{"Platform", "Sites", "Configuration", "Operations"} {
		assert.True(t, strings.Contains(sidebar.Text(), caption))
	}
	// SiteDetail lists an entry for each site, loaded when the page is drawn, and lights the one
	// that is on screen; the list of sites, which is another page, is not lit.
	entries := map[string]page.Link{}
	for _, l := range sidebar.Links() {
		entries[l.Href] = l
	}
	assert.True(t, entries["/admin/site/Stockholm"].Current)
	assert.Equal(t, entries["/admin/site/Malm%C3%B6"].Name, "Malmö")
	assert.False(t, entries["/admin/site/Malm%C3%B6"].Current)
	assert.Equal(t, entries["/admin/site/G%C3%B6teborg"].Name, "Göteborg")
	assert.Equal(t, entries["/admin/site"].Name, "Sites")
	assert.False(t, entries["/admin/site"].Current)
	// The breadcrumb's parent is the page mounted at /site.
	crumbs := page.One(t, "the breadcrumb", p.ByRole("navigation", "Breadcrumb"))
	assert.True(t, hasLink(crumbs, "Sites", "/admin/site"))
	assert.Equal(t, page.One(t, "where we are", crumbs.ByAttr("aria-current", "page")).Text(), "Stockholm")
}

func TestTheLandingPageIsServedAtTheRoot(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/")
	assert.Equal(t, rec.Code, http.StatusOK)                                      // not a redirect to the first entry
	assert.True(t, len(linksTo(page.Parse(t, rec.Body.String()), "/admin/")) > 0) // the brand

	// Without the slash the server's mux adds it.
	assert.Equal(t, get(h, "/admin").Header().Get("Location"), "/admin/")
}

func TestNavEntriesHaveIconsOrTheirInitial(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/alert")
	const long = "Retention and archival policy for every kind of collected event"
	retention := page.One(t, "the entry with no icon", p.ByRole("link", long))
	assert.Equal(t, retention.AllText(), "R "+long) // its initial, and the name whole; the stylesheet cuts it short
	assert.Equal(t, retention.Attr("title"), long)  // and whole in its title
	alerts := page.One(t, "the entry with an icon", p.ByRole("link", "Alerts"))
	assert.Equal(t, alerts.AllText(), "Alerts") // it has the bell, and no initial
}

func TestATabIsNamedByItsLabelInTheAddress(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/device/dev_27c38b?tabs.tab=raw-events")
	strip := page.One(t, "the tabs", p.ByRoleContaining("navigation", "Views:"))
	assert.True(t, slices.Contains(strip.Links(), page.Link{Name: "Raw events", Href: "/admin/device/dev_27c38b?tabs.tab=raw-events", Current: true}))
}

func TestBrandLogoIsServedAndTheThemeCanBeOverridden(t *testing.T) {
	h := handler(t)
	logo := get(h, linked(t, h, "logo"))
	assert.Equal(t, logo.Header().Get("Content-Type"), "image/png")
	assert.True(t, strings.HasPrefix(logo.Body.String(), "\x89PNG"))

	a := app
	a.Theme = themeFor("#1f7a6d")
	themed, err := a.Compile("/admin")
	assert.NoError(t, err)
	assert.Contains(t, get(themed, linked(t, themed, "theme.css")).Body.String(), "--blue: #1f7a6d;")
	assert.Equal(t, linked(t, h, "theme.css"), "") // none by default

	// A colour cannot break out of its declaration: that is a Compile error.
	a.Theme = themeFor("red; } body { display: none")
	_, err = a.Compile("/admin")
	assert.Error(t, err)

	// A colour that cannot carry a white label is refused too, and not drawn as a button whose label is out of sight.
	a.Theme = themeFor("#a7f3d0")
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
	assert.Contains(t, body, "Table.Loader is a RowsLoader with no Load")
	assert.Contains(t, body, `page &#34;/broken/pattern&#34;`)
	assert.Contains(t, body, "uses a lookahead, `(?=`, which RE2 does not support")
	assert.False(t, strings.Contains(body, "Perl")) // Go's own wording names a dialect that says nothing to the reader
}

func TestTheAppsMetricsAreCollectedForTheHostToServeAndAreNotServedByTheApp(t *testing.T) {
	reg := prometheus.NewRegistry()
	a := app
	observer, err := prom.New(reg)
	assert.NoError(t, err)
	a.Observer = observer
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
