package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/Olian04/webui/test/util/assert"
)

// The demo is the library's showcase, so these tests are the list of what it
// shows. They share the demo's one in-memory service, so none of them runs in
// parallel.

// handlerAs is the whole site, with every request carrying a session for the role, as
// a browser that had signed in would.
func handlerAs(t *testing.T, role Role) http.Handler {
	t.Helper()

	service = newService() // each test starts from the seed data, however many times it runs

	compiled, err := app.Compile("/admin")
	assert.NoError(t, err)
	site := routes(compiled)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//nolint:gosec // G124: a request the test makes, not a cookie the app sets.
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: sign(role)}) // beside what the request carries, such as the flash
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

// follow answers the redirect that rec is, as a browser does: a GET of its Location with the cookies it set. Every POST
// is answered with one, so that a reload does not send it again.
func follow(h http.Handler, rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, rec.Header().Get("Location"), nil)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge >= 0 {
			req.AddCookie(c)
		}
	}
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

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

// The Loading pages are the cases of a page that is sent as it is built, and each says what it is for.
func TestTheLoadingPagesShowTheirCases(t *testing.T) {
	h := handler(t)

	// Panels are drawn in the order the page has them, whichever answered first.
	order := get(h, "/admin/loading/order").Body.String()
	last := -1
	for _, panel := range []string{"Quick (10 ms) 1", "Slow (400 ms) 1", "Medium (250 ms) 1", "Last (50 ms) 1"} {
		at := strings.Index(order, panel)
		assert.True(t, at > last)
		last = at
	}

	// Layouts inside layouts.
	nested := get(h, "/admin/loading/nested").Body.String()
	for _, panel := range []string{"Split, left (300 ms) 1", "Split, right (20 ms) 1", "Selected tab (200 ms) 1", "After the layouts (100 ms) 1"} {
		assert.Contains(t, nested, panel)
	}

	// A panel that fails, by error or by panic, says so, and the panel after it is there.
	failures := get(h, "/admin/loading/failures")
	assert.Equal(t, failures.Code, http.StatusOK)
	assert.Equal(t, strings.Count(failures.Body.String(), "Could not load"), 2)
	assert.Contains(t, failures.Body.String(), "After the failures (10 ms) 1")
	assert.False(t, strings.Contains(failures.Body.String(), "fell over"))

	// A page with an editor is built whole.
	assert.Contains(t, get(h, "/admin/loading/whole").Body.String(), `<script type="module" src="/admin/_webui/monaco/code-`)

	// A Guard that refuses is a 403, and the page is for editors.
	assert.Equal(t, get(handlerAs(t, Viewer), "/admin/loading/guarded").Code, http.StatusForbidden)
	assert.Equal(t, get(h, "/admin/loading/guarded").Code, http.StatusOK)
	assert.Contains(t, get(h, "/admin/loading/fast").Body.String(), "Instant 1")
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

func TestSiteDetailHasTwoTablesWithTheirOwnState(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/site/Stockholm?devices.sort=id&open-alerts.sort=severity&open-alerts.desc=true").Body.String()
	assert.Equal(t, strings.Count(body, `class="panel grows"`), 2)
	assert.Contains(t, body, `aria-sort="ascending"`)  // devices, by its sort
	assert.Contains(t, body, `aria-sort="descending"`) // alerts, by its own
	assert.Contains(t, body, `name="devices.filter.id"`)
	assert.Contains(t, body, `name="open-alerts.filter.device"`)

	rec := get(h, "/admin/site/Nowhere")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), `there is no site &#34;Nowhere&#34;`)
}

func TestAQueryArgumentNarrowsTheRawEventsAndNotTheRecentOnes(t *testing.T) {
	h := handler(t)
	rows := func(target, title string) int {
		body := get(h, target).Body.String()
		body = body[strings.Index(body, title):]
		return strings.Count(body[:strings.Index(body, "</table>")], `<tr class="">`) // body rows; the header has none
	}
	// The recent events are the last 15 minutes, whatever the page is narrowed to.
	assert.Equal(t, rows("/admin/device/dev_27c38b", "Recent events"), 3)
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=60", "Recent events"), 3)
	// The raw ones are as far back as the argument says, and all of them without it.
	assert.Equal(t, rows("/admin/device/dev_27c38b?tabs.tab=raw-events", "Raw events"), 8)
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events", "Raw events"), 3)
	assert.Equal(t, rows("/admin/device/dev_27c38b?minutes=30&tabs.tab=raw-events", "Raw events"), 5)
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

	alert := get(h, "/admin/alert/alt_000").Body.String() // a form: its submit is not offered, a way back is
	assert.False(t, strings.Contains(alert, ">Acknowledge</button>"))
	assert.Contains(t, alert, `href="/admin/alert">Back</a>`)

	// The same goes for a device's form: no Save and no Cancel, but a way back.
	assert.False(t, strings.Contains(get(h, "/admin/settings").Body.String(), ">Save</button>"))
	device := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.False(t, strings.Contains(device, ">Save</button>"))
	assert.False(t, strings.Contains(device, ">Cancel</a>"))
	assert.Contains(t, device, `href="/admin/device">Back</a>`)
	assert.False(t, strings.Contains(device, `name="f0_1"`)) // the IP is shown, not offered for editing
}

func TestSaveRejectsADuplicateIPThenStaysWhenOpenedDirectly(t *testing.T) {
	h := handler(t)
	// Fields: Group{ID, IP} f0_0 f0_1, Group{Status, Site} f1_*, Rate f2.
	taken, _ := service.Device("dev_27c75c") // another device's address
	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {taken.IP}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity) // the form, drawn again with what was typed, as the answer
	page := rec
	assert.Contains(t, page.Body.String(), "already in use by another device")
	assert.Contains(t, page.Body.String(), `value="`+taken.IP+`"`) // what was typed

	rec = post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {"10.9.9.9"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev_27c38b") // opened directly: nowhere to return to
	device, _ := service.Device("dev_27c38b")
	assert.Equal(t, device.IP, "10.9.9.9")
}

// The sites to choose from are read when the page is asked for, from where the program keeps them: with how many
// devices each has now, and a device that is moved changes them.
func TestADevicesSiteIsChosenFromTheSitesThereAreNow(t *testing.T) {
	h := handler(t)
	device, _ := service.Device("dev_27c38b")
	var other string
	counts := map[string]int{}
	for _, s := range service.Sites() {
		counts[s.Name] = s.Devices
		if s.Name != device.Site {
			other = s.Name
		}
	}
	body := get(h, "/admin/device/dev_27c38b").Body.String()
	for name, n := range counts {
		assert.Contains(t, body, fmt.Sprintf(">%s (%d devices)</option>", name, n))
	}

	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f1_1": {other}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	moved, _ := service.Device("dev_27c38b")
	assert.Equal(t, moved.Site, other)
	body = get(h, "/admin/device/dev_27c38b").Body.String()
	assert.Contains(t, body, fmt.Sprintf(">%s (%d devices)</option>", other, counts[other]+1))
	assert.Contains(t, body, fmt.Sprintf(">%s (%d devices)</option>", device.Site, counts[device.Site]-1))

	// A site that is not one of them is refused.
	rec = post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f1_1": {"Atlantis"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose from the options offered.")
}

// A column that has no use for a sort leaves it out: the devices' tags are a list of words, and an event's detail is read.
func TestTheDemoLeavesSortingOutOfTheColumnsThatHaveNoUseForIt(t *testing.T) {
	h := handler(t)
	devices := get(h, "/admin/device").Body.String()
	assert.Contains(t, devices, `href="/admin/device?devices.sort=ip"`)
	assert.False(t, strings.Contains(devices, "devices.sort=tags"))
	assert.Contains(t, devices, `aria-label="Filter Tags"`) // it still filters

	events := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.Contains(t, events, "recent-events.sort=kind")
	assert.False(t, strings.Contains(events, "recent-events.sort=detail"))
}

// The device list has a toolbar: a link to the page that makes a device, and a button that does something. An editor may
// press it, and a viewer sees it disabled with the reason.
func TestTheDeviceListHasAToolbarOfALinkAndAnAction(t *testing.T) {
	editor := get(handler(t), "/admin/device").Body.String()
	// The page it leads to has a form, so it is told where the visitor came from, and Cancel and a save return there.
	assert.Contains(t, editor, `<a class="btn btn-primary btn-sm" href="/admin/device/new?webui.from=%2Fadmin%2Fdevice">Create device</a>`)
	assert.Contains(t, editor, `<button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1">Sync now</button>`)

	rec := post(handler(t), "/admin/device", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), fmt.Sprintf("oSynced %d devices", len(service.devices)))

	viewer := get(handlerAs(t, Viewer), "/admin/device").Body.String()
	assert.Contains(t, viewer, `<span class="gate" data-guard="requires the editor role">`)
	rec = post(handlerAs(t, Viewer), "/admin/device", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, flashOf(rec), "erequires the editor role")
}

// The page that makes a device is a form over a device that is new, and saving it opens the device it made.
func TestTheDeviceThatIsCreatedHasAnAddressOfItsOwn(t *testing.T) {
	h := handler(t)
	before := len(service.devices)
	t.Cleanup(func() {
		service.mu.Lock()
		service.devices = service.devices[:before]
		service.mu.Unlock()
	})

	form := get(h, "/admin/device/new").Body.String()
	assert.Contains(t, form, "New device")
	assert.Contains(t, form, `>Create</button>`)

	taken, _ := service.Device("dev_27c75c")
	rec := post(h, "/admin/device/new", url.Values{"_leaf": {"p"}, "f0": {taken.IP}, "f1_0": {"healthy"}, "f1_1": {"Malmö"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "already in use by another device")

	rec = post(h, "/admin/device/new", url.Values{"_leaf": {"p"}, "f0": {"10.7.7.7"}, "f1_0": {"quiet"}, "f1_1": {"Malmö"}, "f2": {"edge", "gpu"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, len(service.devices), before+1)
	created := service.devices[before]
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/"+created.ID)
	assert.Equal(t, created.IP, "10.7.7.7")
	assert.Equal(t, created.Site, "Malmö")
	assert.DeepEqual(t, created.Tags, []string{"edge", "gpu"})
	assert.Equal(t, get(h, rec.Header().Get("Location")).Code, http.StatusOK)
}

// A page's argument that only a link could set has an input on the page: it is applied, and the visitor is sent to the page
// that the arguments make, with the events narrowed.
func TestTheEventsOfADeviceAreNarrowedByAnInputForTheMinutesArgument(t *testing.T) {
	h := handler(t)
	// The input is in the Raw events tab, which it is for, and not on the page.
	overview := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.False(t, strings.Contains(overview, "Events from"))
	raw := get(h, "/admin/device/dev_27c38b?minutes=30&tabs.tab=raw-events").Body.String()
	assert.Contains(t, raw, `<option value="30" data-badge="badge-mute" selected>The last 30 minutes</option>`)

	// Applying it keeps the tab, since the address does.
	rec := get(h, "/admin/device/dev_27c38b?tabs.tab=raw-events&_toolbar=p.1.0&t0=15")
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events")
	rec = get(h, "/admin/device/dev_27c38b?minutes=15&tabs.tab=raw-events&_toolbar=p.1.0&t0=")
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev_27c38b?tabs.tab=raw-events") // nothing chosen is all of them
	rec = get(h, "/admin/device/dev_27c38b?tabs.tab=raw-events&_toolbar=p.1.0&t0=7")
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev_27c38b?tabs.tab=raw-events") // not an option: refused, and where they were
}

func TestFormsDemonstrateRulesFloatSliderAndPlaceholder(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `placeholder="eu-north-1"`)  // Placeholder
	assert.Contains(t, body, `name="f1_0" type="number"`) // Float
	assert.Contains(t, body, `step="any"`)                //
	assert.Contains(t, body, `name="f1_1" type="number"`) // a Slider with a Store: a number, with a range to drag
	assert.Contains(t, body, `class="range-input"`)       //
	assert.Contains(t, body, `minlength="3" maxlength="32"`)

	// Rules run on the server too, and every failure is reported at once.
	rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"ab"}, "f0_1": {"70000"}, "f1_0": {"2"}, "f1_1": {"150"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	page := rec
	for _, want := range []string{"Must be at least 3 characters.", "Must be at most 65535.", "Must be at most 1.", "Must be at most 100."} {
		assert.Contains(t, page.Body.String(), want)
	}
	rec = post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
}

// The collector name has text rules and a pattern, which the browser holds as one pattern and the page tells apart.
func TestTheCollectorNameHasTextRulesAndAPattern(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `data-parts="`)
	for _, line := range []string{`Does not start with &#34;-&#34;`, `Does not end with &#34;-&#34;`, `Does not contain &#34;--&#34;`, "must be lowercase letters, digits and dashes"} {
		assert.Contains(t, body, `<span class="tip-item">`+line+`</span>`)
	}
	for value, want := range map[string]string{
		"-eu-north":  `Must not start with &#34;-&#34;.`,
		"eu-north-":  `Must not end with &#34;-&#34;.`,
		"eu--north":  `Must not contain &#34;--&#34;.`,
		"EU-north-1": "must be lowercase letters, digits and dashes",
	} {
		rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {value}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.Contains(t, rec.Body.String(), want)
	}
}

// The collector's API key is a secret: it is replaced and never read back.
func TestTheApiKeyIsASecretThatIsOnlyReplaced(t *testing.T) {
	h := handler(t)
	service.mu.Lock()
	service.settings.APIKey = "sk-live-0123456789"
	service.mu.Unlock()
	t.Cleanup(func() {
		service.mu.Lock()
		service.settings.APIKey = ""
		service.mu.Unlock()
	})

	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `type="password"`)
	assert.Contains(t, body, `placeholder="••••••••"`) // there is one, and that is all that is said
	assert.False(t, strings.Contains(body, "sk-live-0123456789"))

	rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f3": {""}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().APIKey, "sk-live-0123456789") // empty leaves it
	rec = post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f3": {"sk-new-abcdefghij"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().APIKey, "sk-new-abcdefghij")
}

// A device has notes, in a text area: its lines are kept, and a line break is one character whatever the browser sent.
func TestADevicesNotesAreATextAreaThatKeepsItsLines(t *testing.T) {
	h := handler(t)
	t.Cleanup(func() {
		service.mu.Lock()
		for i := range service.devices {
			if service.devices[i].ID == "dev_27c38b" {
				service.devices[i].Notes = ""
			}
		}
		service.mu.Unlock()
	})
	assert.Contains(t, get(h, "/admin/device/dev_27c38b").Body.String(), `<textarea id="p-0-0-f3" class="input textarea" name="f3"`)

	device, _ := service.Device("dev_27c38b")
	rec := post(h, "/admin/device/dev_27c38b", url.Values{"_leaf": {"p.0.0"}, "f0_1": {device.IP}, "f1_0": {device.Status}, "f3": {"Looked after by Ines.\r\nCall before a restart."}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	device, _ = service.Device("dev_27c38b")
	assert.Equal(t, device.Notes, "Looked after by Ines.\nCall before a restart.")
	assert.Contains(t, get(h, "/admin/device/dev_27c38b").Body.String(), ">\nLooked after by Ines.\nCall before a restart.</textarea>")
}

func TestAReadOnlyFormHasNoSubmitAndShowsABadgeAndABar(t *testing.T) {
	body := get(handler(t), "/admin/system").Body.String()
	assert.False(t, strings.Contains(body, `type="submit"`))
	assert.Contains(t, body, `class="badge badge-ok"`)
	assert.Contains(t, body, `class="gauge-fill"`) // a Slider with no Store is a bar
}

func TestAFeedPagesByACursorAndSortsOnlyThePageInHandWithNoFilters(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/audit").Body.String()
	assert.Contains(t, body, "Showing rows 1–8") // no total, but which rows
	assert.Contains(t, body, `href="/admin/audit?audit-log.after=8&amp;audit-log.back=1%3A&amp;audit-log.row=9">Next`)
	assert.Contains(t, body, `disabled>First page</button>`)
	assert.False(t, strings.Contains(body, `aria-sort="ascending"`)) // the source's order, until a column is chosen
	assert.False(t, strings.Contains(body, `aria-sort="descending"`))
	assert.False(t, strings.Contains(body, `class="filter`))
	assert.Contains(t, body, `audit-log.sort=`) // a column can be chosen: it orders the page in hand
	assert.Contains(t, get(h, "/admin/audit?audit-log.sort=who").Body.String(), "sorted within this page")

	// The cursor comes back, and the last page has no Next.
	last := get(h, "/admin/audit?audit-log.after=40").Body.String()
	assert.Contains(t, last, `href="/admin/audit">First page`)
	assert.Contains(t, last, `disabled>Next</button>`)

	// Actions are recorded: the newest entry is the one just made.
	post(h, "/admin/retention", url.Values{"_leaf": {"p"}, "f0": {"45"}})
	assert.Contains(t, get(h, "/admin/audit").Body.String(), "saved retention")
}

func TestSearchAsksTheDetailPagesThatOfferThemselves(t *testing.T) {
	h := handler(t)
	results := func(q string) (groups map[string][]string) {
		var got struct {
			Results []struct{ Group, Title, Href string }
		}
		assert.NoError(t, json.Unmarshal(get(h, "/admin/_webui/search?q="+url.QueryEscape(q)).Body.Bytes(), &got))
		groups = map[string][]string{}
		for _, r := range got.Results {
			groups[r.Group] = append(groups[r.Group], r.Href)
		}
		return groups
	}

	// Each detail page is found under the entry it lights: devices under Devices, sites under Sites.
	groups := results("stock")
	assert.True(t, len(groups["Devices"]) > 0)
	assert.True(t, strings.HasPrefix(groups["Devices"][0], "/admin/device/dev_"))
	assert.DeepEqual(t, groups["Sites"], []string{"/admin/site/Stockholm"})

	// An alert is found by its message, though its page is not the one the alerts are listed on.
	alert := results("ingest lag")
	assert.True(t, len(alert["Alerts"]) > 0)
	assert.True(t, strings.HasPrefix(alert["Alerts"][0], "/admin/alert/alt_"))
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
		"/admin/site/Stockholm?open-alerts.sort=severity&devices.sort=id",
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
	from := "/admin/site/Malm\u00f6?open-alerts.sort=severity"
	rec := post(h, "/admin/device/dev_27c75c?webui.from="+url.QueryEscape(from), url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {"10.8.8.8"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	to, err := url.Parse(rec.Header().Get("Location")) // http.Redirect percent-encodes non-ASCII
	assert.NoError(t, err)
	assert.Equal(t, to.Path, "/admin/site/Malmö")
	assert.Equal(t, to.RawQuery, "open-alerts.sort=severity")

	// A rejection keeps the Cancel it had.
	taken, _ := service.Device("dev_27c38b")
	rec = post(h, "/admin/device/dev_27c75c?webui.from="+url.QueryEscape(from), url.Values{"_leaf": {"p.0.0"}, "f1_0": {"healthy"}, "f0_1": {taken.IP}})
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
	page := rec
	assert.Contains(t, page.Body.String(), `value="0.95"`)
	assert.Contains(t, page.Body.String(), "would drop most of them")

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

func TestMomentsAreShownInUTCAndFilteredAsMoments(t *testing.T) {
	h := handler(t)

	// The first alert was raised at the seed's "now". It reads "just now", by the column's Format, and
	// the <time> element still holds the moment.
	body := get(h, "/admin/alert").Body.String()
	assert.Contains(t, body, `<time datetime="2026-10-09T11:30:00Z">just now</time>`)
	assert.Contains(t, body, `<time datetime="2026-10-09T10:37:00Z">53 min ago</time>`)

	// An end narrows the alerts to those raised by then: they are 53 minutes apart,
	// counting back from the first, so by 08:00 there are seven.
	narrow := get(h, "/admin/alert?alerts.max.raised=2026-10-09T08:00").Body.String()
	assert.False(t, strings.Contains(narrow, ">alt_000<"))
	assert.Contains(t, narrow, ">alt_010<")

	// A Timestamp reads the same on a device.
	device := get(h, "/admin/device/dev_27c38b").Body.String()
	assert.Contains(t, device, `<time datetime="2026-10-09T11:30:00Z">2026-10-09 11:30</time>`)
}

func TestATimestampWithAStoreIsADateAndTimePicker(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `type="datetime-local" value="2026-10-10T23:30:00" step="1"`) // 36 hours after the seed's now

	// Saved from the picker, in UTC, as Unix seconds.
	post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.25"}, "f1_1": {"80"}, "f2": {"2026-11-01T02:00"}})
	assert.Equal(t, service.Settings().Maintain, int(time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC).Unix()))
}

func TestAURLColumnIsTheOnlyWayAValueBecomesALink(t *testing.T) {
	body := get(handler(t), "/admin/alert/alt_000").Body.String()
	assert.Contains(t, body, `<a class="ext" href="https://example.com/runbooks/critical" target="_blank" rel="noopener noreferrer">`)
	assert.Contains(t, body, "Open the runbook")
}

func TestTheConfigPageEditsATextAndShowsWhatChangedSinceTheDeploy(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/config").Body.String()
	assert.Contains(t, body, `<textarea class="code-text"`)
	assert.Contains(t, body, `data-language="json"`)
	assert.Contains(t, body, "No differences")

	rec := post(h, "/admin/config", url.Values{"_leaf": {"p.0"}, "f0": {"{\n  \"collector\": \"eu-west-2\"\n}\n"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	body = get(h, "/admin/config").Body.String()
	assert.False(t, strings.Contains(body, "No differences"))
	assert.Contains(t, body, "eu-west-2")

	// A viewer is shown the text and is not let save it.
	v := handlerAs(t, Viewer)
	body = get(v, "/admin/config").Body.String()
	assert.Contains(t, body, `<pre class="code-view"`)
	assert.False(t, strings.Contains(body, "<textarea"))
	refused := post(v, "/admin/config", url.Values{"_leaf": {"p.0"}, "f0": {"{}"}})
	assert.Equal(t, refused.Code, http.StatusSeeOther)
	assert.Contains(t, follow(v, refused).Body.String(), `class="toast err"`)
}

func TestTheHandbookIsMarkdownSetAsMarkupThatCannotRunAnything(t *testing.T) {
	body := get(handler(t), "/admin/handbook").Body.String()
	assert.Contains(t, body, `<article class="md"`)
	assert.Contains(t, body, "<h1>Collector handbook</h1>")
	assert.Contains(t, body, "<table>")
	assert.Contains(t, body, `<code class="language-bash">`)
	assert.Contains(t, body, `rel="noopener noreferrer"`)
	assert.False(t, strings.Contains(body, "<script>alert"))
}

func TestEveryLinkInTheHandbookGoesToAPageThatExistsAndTheHandbookLinksToItself(t *testing.T) {
	h := handler(t)
	pages := []string{"/admin/handbook", "/admin/handbook/incidents", "/admin/handbook/sinks", "/admin/handbook/glossary"}
	link := regexp.MustCompile(`<a href="(/admin[^"]*)"( target=[^>]*)?>`)
	seen := map[string]bool{}
	for _, page := range pages {
		own := map[string]bool{}
		for _, m := range link.FindAllStringSubmatch(get(h, page).Body.String(), -1) {
			assert.Equal(t, m[2], "") // inside the application: the same tab
			target := strings.ReplaceAll(m[1], "&amp;", "&")
			own[target] = true
			if !seen[target] {
				seen[target] = true
				if code := get(h, target).Code; code != http.StatusOK && code != http.StatusFound {
					t.Errorf("%s links to %s, which is %d", page, target, code)
				}
			}
		}
		// Each page of the handbook leads to another one, and to a page that is not in it.
		handbook, elsewhere := 0, 0
		for target := range own {
			if target != page && strings.HasPrefix(target, "/admin/handbook") {
				handbook++
			} else if !strings.HasPrefix(target, "/admin/handbook") {
				elsewhere++
			}
		}
		if handbook == 0 || elsewhere == 0 {
			t.Errorf("%s links to %d other pages of the handbook and %d pages outside it", page, handbook, elsewhere)
		}
	}
}

func TestTablesExportTheirRowsWhateverTheSource(t *testing.T) {
	h := handler(t)

	// Load: the device table is 10 to a page, and the export is every device the filter keeps.
	rec := get(h, "/admin/device?devices.export=csv&devices.sort=id&devices.desc=true&devices.offset=10")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Header().Get("Content-Disposition"), "attachment; filename=devices.csv")
	assert.Equal(t, strings.Count(rec.Body.String(), "\n"), 38) // the header and 37 devices, and a line ends each

	// Rows: the open alerts, as JSON.
	var alerts []map[string]any
	assert.NoError(t, json.Unmarshal(get(h, "/admin/alert?alerts.export=json").Body.Bytes(), &alerts))
	assert.True(t, len(alerts) > 0)

	// Feed: the whole audit log, not the page of eight.
	audit := get(h, "/admin/audit?audit-log.export=csv")
	assert.Equal(t, audit.Header().Get("Content-Disposition"), "attachment; filename=audit-log.csv")
	assert.True(t, strings.Count(audit.Body.String(), "\n") > 9)

	// The viewer who may not see the audit log may not download it.
	assert.Equal(t, get(handlerAs(t, Viewer), "/admin/audit?audit-log.export=csv").Code, http.StatusForbidden)
}

func TestAHandbookPageDrawsPicturesFromItsTrustedOriginsAndNoOtherPage(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/handbook/glossary")
	body := rec.Body.String()
	assert.Contains(t, body, `<img src="https://raw.githubusercontent.com/Olian04/webui/main/docs/images/first-page-light.png"`)
	assert.Contains(t, body, `<img src="https://pkg.go.dev/badge/github.com/Olian04/webui/pkg/webui.svg"`)
	assert.False(t, strings.Contains(body, `src="https://tracker.example`)) // not trusted: a link
	assert.Contains(t, body, `<a href="https://tracker.example/pixel.gif"`)
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "img-src 'self' data: https://raw.githubusercontent.com https://pkg.go.dev;")

	// Every other page of the demo has the library's policy, with no origin added.
	assert.Contains(t, get(h, "/admin/device").Header().Get("Content-Security-Policy"), "img-src 'self' data:;")
}

func TestAPathThatTakesTheRestOfTheAddressIsAListOfSegmentsWithACrumbForEach(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/files/logs/2026/10").Body.String()

	// One crumb for each segment, each a link to the folder on the way; the last is where we are.
	assert.Contains(t, body, `<a href="/admin/files">Files</a>`)
	assert.Contains(t, body, `<a href="/admin/files/logs">logs</a>`)
	assert.Contains(t, body, `<a href="/admin/files/logs/2026">2026</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">10</span>`)

	// A row adds its name to the list, and the address is built from the elements.
	assert.Contains(t, body, `href="/admin/files/logs/2026/10/collector.log"`)
	// The Markdown beside it links to paths in the app, which are given the mount prefix.
	assert.Contains(t, body, `<a href="/admin/files/logs/2026">2026</a>`)
	assert.Contains(t, body, `<a href="/admin/files">all files</a>`)

	// Nothing at the path: the page's Guard says so, before anything is loaded.
	assert.Equal(t, get(h, "/admin/files/logs/nope").Code, http.StatusForbidden)
	// A doubled slash is not a path of this page.
	assert.Equal(t, get(h, "/admin/files/logs//2026").Code, http.StatusTemporaryRedirect) // cleaned by the mux
}

func TestAnOutcomeCanSayWhereToGoWhateverItsKind(t *testing.T) {
	h := handler(t)

	// Stay: the ingest form is one the user goes on tuning, so a save that was reached
	// from a link still stays on it, with where it came from kept in the address.
	own := "/admin/settings?webui.from=" + url.QueryEscape("/admin/system")
	rec := post(h, own, url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), own)

	// Back: someone else acknowledged the alert first. A Failure would show the form again,
	// for an alert with nothing left to do; Back leaves for the page the user came from, with
	// the reason in a toast, and for the parent when it was opened directly.
	from := "?webui.from=" + url.QueryEscape("/admin/alert")
	post(h, "/admin/alert/alt_001"+from, url.Values{"_leaf": {"p.0"}})
	rec = post(h, "/admin/alert/alt_001"+from, url.Values{"_leaf": {"p.0"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/alert")
	rec = post(h, "/admin/alert/alt_001", url.Values{"_leaf": {"p.0"}})
	assert.Equal(t, rec.Header().Get("Location"), "/admin/alert")
}

func TestThePagingRuleIsTypeScriptWithItsLanguageServiceForAnEditorAndNotForAViewer(t *testing.T) {
	workers := func(h http.Handler) string {
		body := get(h, "/admin/rule").Body.String()
		i := strings.Index(body, `<script id="webui-code"`)
		assert.True(t, i >= 0)
		return body[i : i+strings.Index(body[i:], "</script>")]
	}
	editor := handler(t)
	body := get(editor, "/admin/rule").Body.String()
	assert.Contains(t, body, `data-language="typescript"`)
	assert.Contains(t, body, `<textarea class="code-text"`)
	assert.Contains(t, workers(editor), `"ts":"/admin/_webui/monaco/workers/ts.worker-`) // the service the program imported

	// The same text is read by a viewer, who is sent no service: nothing there can be changed.
	viewer := handlerAs(t, Viewer)
	assert.Contains(t, get(viewer, "/admin/rule").Body.String(), `<pre class="code-view"`)
	assert.False(t, strings.Contains(workers(viewer), `"ts"`))

	// The text has type errors on purpose, and saving it is the editor's.
	assert.Contains(t, body, "pageOnCall(signal, ")
	rec := post(editor, "/admin/rule", url.Values{"_leaf": {"p"}, "f0": {"export const ok: number = 1;\n"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, get(editor, "/admin/rule").Body.String(), "export const ok: number = 1;")
	refused := post(viewer, "/admin/rule", url.Values{"_leaf": {"p"}, "f0": {"x"}})
	assert.Equal(t, refused.Code, http.StatusSeeOther)
	assert.Contains(t, follow(viewer, refused).Body.String(), `class="toast err"`)
}

func TestTheEditorsAreGivenTheFolderOfContextTheProgramEmbeds(t *testing.T) {
	// One folder serves both editors, each reading the directory named for its own language.
	editor := handler(t)
	rule := get(editor, "/admin/rule").Body.String()
	m := regexp.MustCompile(`"libs":\{"typescript":\[\{"path":"/rule\.d\.ts","url":"([^"]+)"\}\]`).FindStringSubmatch(rule)
	assert.NotNil(t, m)
	assert.Contains(t, rule, `"compiler":{"typescript":{`)
	file := get(editor, m[1])
	assert.Equal(t, file.Code, http.StatusOK)
	assert.Contains(t, file.Body.String(), "declare function pageOnCall(")

	config := get(editor, "/admin/config").Body.String()
	assert.Contains(t, config, `"schemas":{`)
	assert.Contains(t, config, "data-context=")

	// A viewer is shown the text and handed none of it.
	viewer := handlerAs(t, Viewer)
	for _, page := range []string{"/admin/rule", "/admin/config"} {
		body := get(viewer, page).Body.String()
		assert.False(t, strings.Contains(body, "/_webui/code/"))
	}
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

func TestAFormattedColumnReadsAsItsFormatAndSortsAndFiltersByItsValue(t *testing.T) {
	h := handler(t)

	// Stored reads "1 GiB" and sorts by the bytes: as text, "1 GiB" would come before "1 KiB" and "9 ..." after "10 ...".
	asc := get(h, "/admin/device?devices.sort=stored").Body.String()
	assert.Contains(t, asc, ">1 KiB<")
	assert.False(t, strings.Contains(asc, ">1 GiB<"))
	desc := get(h, "/admin/device?devices.sort=stored&devices.desc=true").Body.String()
	assert.Contains(t, desc, ">1 GiB<")
	assert.False(t, strings.Contains(desc, ">1 KiB<"))

	// The bounds of its filter are bytes, and the export holds the number.
	big := get(h, "/admin/device?devices.min.stored=1073741824").Body.String()
	assert.Contains(t, big, ">1 GiB<")
	assert.False(t, strings.Contains(big, ">512 MiB<"))
	assert.Contains(t, get(h, "/admin/device?devices.export=csv").Body.String(), ",1073761364,") // dev 20: 2^30 + 20*977 bytes
}

// The Layout pages are combinations that no other page of the demo has, and each is the layout it says.
func TestTheLayoutPagesAreTheCombinationsTheySay(t *testing.T) {
	h := handler(t)

	// A rail: a Stack in a Split, beside a table.
	rail := get(h, "/admin/layout/rail").Body.String()
	assert.Contains(t, rail, `<div class="split"><div class="panel grows"`)
	assert.Contains(t, rail, `</div><div class="stack">`)

	// Columns in columns: a Split that is the first child of a Split.
	assert.Contains(t, get(h, "/admin/layout/columns").Body.String(), `<div class="split"><div class="split">`)

	// Tabs each have a name of their own in the address, and only the selected tab of each is built.
	tabs := get(h, "/admin/layout/tabs?tabs-2.tab=status").Body.String()
	for _, name := range []string{"tabs.tab=", "tabs-2.tab=", "tabs-3.tab="} {
		assert.Contains(t, tabs, name)
	}
	assert.Contains(t, tabs, "Open alerts")
	assert.False(t, strings.Contains(tabs, "Mean rate"))

	// Five layouts down, and the choice of an inner Tabs is in the address.
	deep := get(h, "/admin/layout/deep?tabs-2.tab=status-and-a-form").Body.String()
	assert.Contains(t, deep, "A note")
	assert.False(t, strings.Contains(deep, "Mean rate"))

	// A page with an editor, a diff and Markdown links what they need in its head.
	review := get(h, "/admin/layout/review").Body.String()
	assert.Contains(t, review, `<script type="module" src="/admin/_webui/monaco/code-`)
	assert.Contains(t, review, "Collector configuration")
	assert.Contains(t, review, "Changes since the last deploy")
	assert.Contains(t, get(h, "/admin/layout/runbook").Body.String(), `<article class="md"`)
}

// The staircase has ten panels, the first after a second and each next one a second later, so a visitor who goes
// after a second and a half has the first of them and the spinner, and none of the rest.
func TestTheStaircaseArrivesOnePanelAtATime(t *testing.T) {
	h := handler(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/loading/staircase", nil).WithContext(ctx))

	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, body, `<div class="arriving"`)
	assert.Contains(t, body, "Panel 1 (1 s) 1")
	assert.False(t, strings.Contains(body, "Panel 2 (2 s)"))
	assert.False(t, strings.Contains(body, `class="arrived"`))
}

// The tabs page is read with its tab, and a request for the Tabs alone is read without the rest: the stamp of a panel outside
// it is the moment that panel was read, and only the panels of the tab that was asked for have a new one.
func TestTheTabsPageReadsOnlyTheTabThatIsAskedFor(t *testing.T) {
	h := handler(t)
	page := get(h, "/admin/loading/tabs").Body.String()
	assert.Contains(t, page, "Above the tabs (read with the page)")
	assert.Contains(t, page, "Below the tabs (read with the page)")
	assert.Contains(t, page, "First, left (500 ms)")

	req := httptest.NewRequest(http.MethodGet, "/admin/loading/tabs?tabs.tab=second", nil)
	req.Header.Set("X-Webui-Leaf", "p.2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, strings.HasPrefix(body, `<div class="tabset" data-tabs="p.2"`))
	assert.Contains(t, body, "Second (500 ms)")
	assert.False(t, strings.Contains(body, "Above the tabs"))
	assert.False(t, strings.Contains(body, "First, left"))
}

// A Tabs is not limited to a few tabs: the page has strips of three, six and sixteen, and the one that is chosen is the one that
// is read, wherever it is in the strip.
func TestTheManyTabsPageHasStripsOfThreeSixAndSixteenTabs(t *testing.T) {
	h := handler(t)
	page := get(h, "/admin/layout/many-tabs").Body.String()
	strips := regexp.MustCompile(`<nav class="tabs".*?</nav>`).FindAllString(page, -1)
	assert.Equal(t, len(strips), 4)
	var counts []int
	for _, s := range strips {
		counts = append(counts, strings.Count(s, `<a class="tab`))
	}
	assert.DeepEqual(t, counts, []int{3, 6, 6, 16})

	// The last of the sixteen is the one that is read when it is asked for, and the first is not.
	last := get(h, "/admin/layout/many-tabs?tabs-4.tab=system-status").Body.String()
	assert.Contains(t, last, "Sixteen / System status")
	assert.False(t, strings.Contains(last, "Sixteen / Overview"))
}

// The compare page takes two lists in its address, a parameter that repeats, and shows the devices they name: each device by
// its id and every device at each site, once. A sort or a pager on the page keeps every element of both.
func TestTheComparePageTakesLists(t *testing.T) {
	h := handler(t)

	two := get(h, "/admin/compare?device=dev_27c38b&device=dev_27c75c").Body.String()
	assert.Contains(t, two, "dev_27c38b")
	assert.Contains(t, two, "dev_27c75c")
	assert.False(t, strings.Contains(two, "dev_27cb2d"))

	// A site is every device in it, and a device that is also in a site is not shown twice.
	mixed := get(h, "/admin/compare?site=Malm%C3%B6&device=dev_27c75c").Body.String()
	assert.Equal(t, strings.Count(mixed, `>dev_27c75c<`), 1)
	assert.Contains(t, mixed, "Malmö")

	// The page's own links keep both lists whole, in order.
	body := get(h, "/admin/compare?site=Malm%C3%B6&site=G%C3%B6teborg&device=dev_27c38b&device=dev_27c75c").Body.String()
	assert.Contains(t, body, `device=dev_27c38b&amp;device=dev_27c75c&amp;devices.sort=id&amp;site=Malm%C3%B6&amp;site=G%C3%B6teborg`)

	// No lists, no devices: an empty list is no parameter, and not an error.
	assert.Equal(t, get(h, "/admin/compare").Code, http.StatusOK)
	assert.Equal(t, get(h, "/admin/compare?device=").Code, http.StatusOK)
}
