package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// The demo is the library's showcase, so these tests are the list of what it
// shows. They share the demo's one in-memory service, so none of them runs in
// parallel.

// handlerAs is the whole site, with every request carrying a session for the role, as
// a browser that had signed in would.
func handlerAs(t *testing.T, role Role) http.Handler {
	t.Helper()

	compiled, err := app.Compile("/admin")
	assert.NoError(t, err)
	site := routes(compiled)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Cookie", sessionCookie+"="+sign(role))
		site.ServeHTTP(w, r)
	})
}

// handler is the site for an editor, who may do everything.
func handler(t *testing.T) http.Handler { return handlerAs(t, Editor) }

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func post(h http.Handler, target string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestEveryPageServes(t *testing.T) {
	h := handler(t)
	for _, path := range []string{
		"/admin/device", "/admin/device/dev_27c38b", "/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events",
		"/admin/site", "/admin/site/Stockholm", "/admin/alert", "/admin/alert/alt_000", "/admin/settings", "/admin/retention",
		"/admin/system", "/admin/audit", "/admin/",
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

func TestNavigationHasSectionsAndTheSitePageLightsSites(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/site/Stockholm").Body.String()
	for _, caption := range []string{"Platform", "Configuration", "Operations"} {
		assert.Contains(t, body, `<div class="nav-section">`+caption+`</div>`)
	}
	// SiteDetail has no entry of its own; it lights Sites, its ancestor.
	assert.Contains(t, body, `<a class="nav-item active" href="/admin/site"`)
	// The breadcrumb's parent is the page mounted at /site.
	assert.Contains(t, body, `<a href="/admin/site">Sites</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">Stockholm</span>`)
}

func TestSiteDetailHasTwoTablesWithTheirOwnState(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/site/Stockholm?devices.sort=id&alerts.sort=severity&alerts.desc=true").Body.String()
	assert.Equal(t, strings.Count(body, `class="panel"`), 2)
	assert.Contains(t, body, `aria-sort="ascending"`)  // devices, by its sort
	assert.Contains(t, body, `aria-sort="descending"`) // alerts, by its own
	assert.Contains(t, body, `name="devices.filter.id"`)
	assert.Contains(t, body, `name="alerts.filter.device"`)

	rec := get(h, "/admin/site/Nowhere")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), `there is no site &#34;Nowhere&#34;`)
}

func TestAQueryArgumentNarrowsTheEvents(t *testing.T) {
	h := handler(t)
	rows := func(target string) int {
		body := get(h, target).Body.String()
		body = body[strings.Index(body, "Recent events"):]
		return strings.Count(body[:strings.Index(body, "</table>")], `<tr class="">`) // body rows; the header has none
	}
	assert.Equal(t, rows("/admin/device/dev_27c38b"), 8)                                // every event
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=15"), 3)                     // the last 15 minutes
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events"), 3) // the Raw tab: the same table
}

func TestTheLandingPageIsServedAtTheRoot(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/")
	assert.Equal(t, rec.Code, http.StatusOK) // not a redirect to the first entry
	assert.Contains(t, rec.Body.String(), `<a class="side-brand" href="/admin/"`)
	assert.Contains(t, rec.Body.String(), "Disk used") // the system status on it

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

func TestAnAlertHasItsOwnPageAndItsRowsOpenIt(t *testing.T) {
	h := handler(t)
	assert.Contains(t, get(h, "/admin/alert").Body.String(), `href="/admin/alert/alt_000?webui.from=%2Fadmin%2Falert"`)
	assert.Contains(t, get(h, "/admin/site/Stockholm").Body.String(), `href="/admin/alert/alt_000?webui.from=%2Fadmin%2Fsite%2FStockholm"`)

	// It shows the alert, lights Alerts in the sidebar, and cancels to the list.
	page := get(h, "/admin/alert/alt_000").Body.String()
	assert.Contains(t, page, "Ingest lag above 5s")
	assert.Contains(t, page, `class="nav-item active" href="/admin/alert"`)
	assert.Equal(t, cancelHref(t, h, "/admin/alert/alt_000"), "/admin/alert")

	assert.Equal(t, get(h, "/admin/alert/nope").Code, http.StatusForbidden) // the Guard: no such alert
}

func TestTheAlertPageLinksToItsDeviceWithAQueryArgument(t *testing.T) {
	body := get(handler(t), "/admin/alert/alt_000").Body.String()
	assert.Contains(t, body, `href="/admin/device/dev_27c38b?minutes=15&amp;webui.from=%2Fadmin%2Falert%2Falt_000"`)
}

func TestAcknowledgingFromTheAlertPageReturnsToWhereItWasOpenedFrom(t *testing.T) {
	h := handler(t)
	rec := post(h, "/admin/alert/alt_001?webui.from=%2Fadmin%2Falert", url.Values{"_leaf": {"p.0"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/alert")
	assert.False(t, strings.Contains(get(h, "/admin/alert").Body.String(), "alt_001"))
}

func TestActionsRowBulkAndDestructive(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/alert").Body.String()
	assert.Contains(t, body, `name="_act" value="bulk:0">Acknowledge</button>`)
	assert.Contains(t, body, `class="btn btn-danger btn-sm"`) // the destructive role
	assert.Contains(t, body, `value="bulk:1">Delete</button>`)
	assert.Contains(t, body, `name="_act" value="row:0:alt_000"`)

	rec := post(h, "/admin/alert", url.Values{"_leaf": {"p"}, "_act": {"bulk:1"}, "_sel": {"alt_010"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.False(t, strings.Contains(get(h, "/admin/alert").Body.String(), "alt_010"))
}

func TestViewerIsRefusedWhereGuarded(t *testing.T) {
	h := handlerAs(t, Viewer)

	// A page Guard: nothing was loaded, and the sidebar does not offer the page.
	assert.False(t, strings.Contains(get(h, "/admin/device").Body.String(), "Audit log"))
	rec := get(h, "/admin/audit")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), "requires the editor role")

	// Gated controls: disabled, with the reason.
	body := get(h, "/admin/alert").Body.String()
	assert.Contains(t, body, `<span class="gate" data-guard="requires the editor role">`)
	assert.Contains(t, body, `disabled>Dismiss</button>`)

	assert.Contains(t, get(h, "/admin/alert/alt_000").Body.String(), `disabled>Acknowledge</button>`)

	settings := get(h, "/admin/settings").Body.String()
	assert.Contains(t, settings, `disabled>Save</button>`) // a form's submit is gated by the same Guard
}

func TestSaveRejectsADuplicateIPThenStaysWhenOpenedDirectly(t *testing.T) {
	h := handler(t)
	// Fields: Group{ID, IP} f0_0 f0_1, Group{Status, Site} f1_*, Rate f2.
	taken, _ := service.Device("dev_27c75c") // another device's address
	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {taken.IP}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "already in use by another device")
	assert.Contains(t, rec.Body.String(), `value="`+taken.IP+`"`) // what was typed is kept

	rec = post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {"10.9.9.9"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev_27c38b") // opened directly: nowhere to return to
	device, _ := service.Device("dev_27c38b")
	assert.Equal(t, device.IP, "10.9.9.9")
}

func TestFormsDemonstrateRulesFloatSliderAndPlaceholder(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `placeholder="eu-north-1"`)  // Placeholder
	assert.Contains(t, body, `name="f1_0" type="number"`) // Float
	assert.Contains(t, body, `step="any"`)                //
	assert.Contains(t, body, `type="range" name="f1_1"`)  // a Slider with a Store
	assert.Contains(t, body, `class="range-value"`)       //
	assert.Contains(t, body, `minlength="3" maxlength="32"`)

	// Rules run on the server too, and every failure is reported at once.
	rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"ab"}, "f0_1": {"70000"}, "f1_0": {"2"}, "f1_1": {"150"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	for _, want := range []string{"Must be at least 3 characters.", "Must be at most 65535.", "Must be at most 1.", "Must be at most 100."} {
		assert.Contains(t, rec.Body.String(), want)
	}
	rec = post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
}

func TestAReadOnlyFormHasNoSubmitAndShowsABadgeAndABar(t *testing.T) {
	body := get(handler(t), "/admin/system").Body.String()
	assert.False(t, strings.Contains(body, `type="submit"`))
	assert.Contains(t, body, `class="badge badge-ok"`)
	assert.Contains(t, body, `class="gauge-fill"`) // a Slider with no Store is a bar
}

func TestAnUnknownTotalPagesByFullPages(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/audit").Body.String()
	assert.Contains(t, body, "1–8") // a range with no "of N"
	assert.False(t, strings.Contains(body, " of "))
	assert.Contains(t, body, `href="/admin/audit?audit.offset=8"`)

	// Actions are recorded: the newest entry is the one just made.
	post(h, "/admin/retention", url.Values{"_leaf": {"p"}, "f0": {"45"}})
	assert.Contains(t, get(h, "/admin/audit").Body.String(), "saved retention")
}

func TestSearchAsksSeveralPages(t *testing.T) {
	rec := get(handler(t), "/admin/_webui/search?q=stock")
	var got struct {
		Results []struct{ Group, Title string }
	}
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	groups := map[string]bool{}
	for _, r := range got.Results {
		groups[r.Group] = true
	}
	assert.True(t, groups["Devices"])
	assert.True(t, groups["Sites"])
}

// linked is the address a page links an asset at, with the hash of its contents in it.
func linked(t *testing.T, h http.Handler, logical string) string {
	t.Helper()

	dot := strings.LastIndex(logical, ".")
	base, ext := logical, ""
	if dot > 0 {
		base, ext = logical[:dot], logical[dot:]
	}
	m := regexp.MustCompile(`(/[^"' ]*_webui/` + regexp.QuoteMeta(base) + `\.[0-9a-f]{8}` + regexp.QuoteMeta(ext) + `)["' >]`).
		FindStringSubmatch(get(h, "/admin/").Body.String())
	if m == nil {
		return ""
	}
	return m[1]
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
	a.Pages = append(append(webui.Pages{}, a.Pages...), Broken)
	h, err := a.Compile("/admin")
	assert.Error(t, err)
	rec := get(h, "/admin/anything")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.Contains(t, rec.Body.String(), "Failed to compile")
	assert.Contains(t, rec.Body.String(), "field When has unsupported type []string")
}

func cancelHref(t *testing.T, h http.Handler, target string) string {
	t.Helper()

	body := get(h, target).Body.String()
	const marker = `<a class="btn btn-ghost btn-sm" href="`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	body = body[i+len(marker):]
	return strings.ReplaceAll(body[:strings.Index(body, `"`)], "&amp;", "&")
}

func TestTheDeviceIsOpenedFromSeveralPagesAndEachRemembersItsOwn(t *testing.T) {
	h := handler(t)
	// Every page that links to a device remembers the address the user is on.
	for _, from := range []string{
		"/admin/device?devices.sort=ip&devices.offset=10",
		"/admin/site/Stockholm?alerts.sort=severity&devices.sort=id",
		"/admin/alert",
	} {
		list := get(h, from).Body.String()
		assert.Contains(t, list, "webui.from="+url.QueryEscape(from))
	}

	// And the device page's Cancel returns there, view state and all.
	from := "/admin/site/Stockholm?devices.sort=id&devices.offset=5"
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b?webui.from="+url.QueryEscape(from)), from)
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b?webui.from="+url.QueryEscape("/admin/alert")), "/admin/alert")
}

func TestCancelFallsBackToTheListWhenThereIsNoOrigin(t *testing.T) {
	h := handler(t)
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b"), "/admin/device")
	// A search result carries none, since a search is not a page.
	search := get(h, "/admin/_webui/search?q=dev_27c38b").Body.String()
	assert.Contains(t, search, `"href":"/admin/device/dev_27c38b"`)
	assert.False(t, strings.Contains(search, "webui.from"))
	// An address that is not in this app is ignored.
	assert.Equal(t, cancelHref(t, h, "/admin/device/dev_27c38b?webui.from="+url.QueryEscape("https://evil.example/")), "/admin/device")
}

func TestSavingReturnsToWhereTheDeviceWasOpenedFrom(t *testing.T) {
	h := handler(t)
	from := "/admin/site/Malm\u00f6?alerts.sort=severity"
	rec := post(h, "/admin/device/dev_27c75c?webui.from="+url.QueryEscape(from), url.Values{"_leaf": {"p.0.0"}, "f0_1": {"10.8.8.8"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	to, err := url.Parse(rec.Header().Get("Location")) // http.Redirect percent-encodes non-ASCII
	assert.NoError(t, err)
	assert.Equal(t, to.Path, "/admin/site/Malmö")
	assert.Equal(t, to.RawQuery, "alerts.sort=severity")

	// A rejection keeps the Cancel it had.
	taken, _ := service.Device("dev_27c38b")
	rec = post(h, "/admin/device/dev_27c75c?webui.from="+url.QueryEscape(from), url.Values{"_leaf": {"p.0.0"}, "f0_1": {taken.IP}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), `>Cancel</a>`)
}

func flashOf(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "webui_flash" {
			raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
			return string(raw)
		}
	}
	return ""
}

func ingest(sample, load string) url.Values {
	return url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {sample}, "f1_1": {load}}
}

func TestIngestShowsEveryKindOfOutcome(t *testing.T) {
	h := handler(t)

	// Failure: not accepted, so the form is shown again with what was typed, and the reason.
	rec := post(h, "/admin/settings", ingest("0.95", "40"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), `value="0.95"`)
	assert.Contains(t, rec.Body.String(), "would drop most of them")

	// Warning: accepted, with something to be aware of.
	rec = post(h, "/admin/settings", ingest("0.5", "10"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, flashOf(rec), "wSaved, but ingest will start shedding")

	// Success: accepted, and confirmed.
	rec = post(h, "/admin/settings", ingest("0.5", "60"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oSaved")
}

func TestSavingRetentionGoesOnToTheSystemPage(t *testing.T) {
	rec := post(handler(t), "/admin/retention", url.Values{"_leaf": {"p"}, "f0": {"45"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/system") // Then, in place of staying on the form
	assert.Equal(t, flashOf(rec), "oSaved")
}

func TestAcknowledgingACriticalAlertWarns(t *testing.T) {
	h := handler(t)
	// alt_003 is critical, alt_005 is info, and alt_007 a warning.
	rec := post(h, "/admin/alert", url.Values{"_leaf": {"p"}, "_act": {"bulk:0"}, "_sel": {"alt_003", "alt_005"}})
	assert.Equal(t, flashOf(rec), "wAcknowledged 2, including 1 critical")
	rec = post(h, "/admin/alert", url.Values{"_leaf": {"p"}, "_act": {"bulk:0"}, "_sel": {"alt_007"}}) // a warning
	assert.Equal(t, flashOf(rec), "oAcknowledged 1")
}
