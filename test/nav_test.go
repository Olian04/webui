package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func navApp() http.Handler {
	empty := func(path string, nav webui.Nav) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: nav, Body: webui.Stack{}}
	}
	return webui.App{
		Brand: webui.Brand{Name: "Acme"},
		Pages: webui.Pages{
			empty("/device", webui.Nav{Label: "Devices", Icon: "display"}),
			empty("/retention", webui.Nav{Label: "retention"}), // no Icon
		},
	}.MustCompile("/admin")
}

func TestANavEntryWithAnIconDrawsIt(t *testing.T) {
	t.Parallel()

	body := serve(navApp(), http.MethodGet, "/admin/device").Body.String()
	// The icon is its Font Awesome class, and there is no initial beside it.
	assert.Contains(t, body, `<i class="fa-solid fa-display ni" aria-hidden="true"></i><span>Devices</span>`)
	assert.False(t, strings.Contains(body, `ni-letter" aria-hidden="true">D<`))
}

func TestANavEntryWithoutAnIconCarriesItsCapitalisedInitialForTheRail(t *testing.T) {
	t.Parallel()

	body := serve(navApp(), http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `<span class="ni ni-letter" aria-hidden="true">R</span> <span>retention</span>`)
}

func TestTheBrandLinksToTheLandingPage(t *testing.T) {
	t.Parallel()

	body := serve(navApp(), http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `<a class="side-brand" href="/admin/"`)
	// And so does the first breadcrumb.
	assert.Contains(t, body, `<a href="/admin/">Acme</a>`)
}

func TestWithoutALandingPageTheRootGoesToTheFirstEntry(t *testing.T) {
	t.Parallel()

	rec := serve(navApp(), http.MethodGet, "/admin/")
	assert.Equal(t, rec.Code, http.StatusFound)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device")
}

func TestAPageAtTheRootIsTheLandingPage(t *testing.T) {
	t.Parallel()

	h := webui.App{
		Brand: webui.Brand{Name: "Acme"},
		Pages: webui.Pages{
			webui.Page[webui.NoArgs]{Path: "/", Body: webui.Stack{}},
			webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: webui.Stack{}},
		},
	}.MustCompile("/admin")

	for _, target := range []string{"/admin", "/admin/"} {
		rec := serve(h, http.MethodGet, target)
		assert.Equal(t, rec.Code, http.StatusOK) // served, not redirected
		assert.Contains(t, rec.Body.String(), `<span class="cur" aria-current="page">Acme</span>`)
	}
}

func TestCompileRefusesAnIconThatIsNotAFontAwesomeIcon(t *testing.T) {
	t.Parallel()

	for icon, want := range map[string]string{
		"rocket-ship": `Nav.Icon "rocket-ship" is not a Font Awesome Free solid icon`,
		"fa-house":    `Write the icon's name without the "fa-" prefix: "house".`,
	} {
		app := webui.App{Pages: webui.Pages{
			webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A", Icon: icon}, Body: webui.Stack{}},
		}}
		var all string
		for _, e := range compileErrors(t, app) {
			all += e.Error() + "\n"
		}
		assert.Contains(t, all, want)
	}
}

func TestTheIconFontAndItsClassesAreServedAndLinked(t *testing.T) {
	t.Parallel()

	h := navApp()
	page := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, page, `<link rel="stylesheet" href="`+assetURL(t, h, "fontawesome.css")+`">`)

	css := serve(h, http.MethodGet, assetURL(t, h, "fontawesome.css"))
	assert.Equal(t, css.Code, http.StatusOK)
	assert.Contains(t, css.Body.String(), `.fa-house::before{content:"\f015"}`)
	font0 := assetURL(t, h, "fa-solid-900.woff2")
	assert.Contains(t, css.Body.String(), `url(`+font0[strings.LastIndex(font0, "/")+1:]+`)`) // beside the stylesheet, so a relative url finds it

	font := serve(h, http.MethodGet, assetURL(t, h, "fa-solid-900.woff2"))
	assert.Equal(t, font.Code, http.StatusOK)
	assert.Equal(t, font.Header().Get("Content-Type"), "font/woff2")
}

func TestTheSidebarOffersACollapseControlOnlyWithScript(t *testing.T) {
	t.Parallel()

	body := serve(navApp(), http.MethodGet, "/admin/device").Body.String()
	// It sits in the js-only footer, so without script it is not there to be inert.
	assert.Contains(t, body, `<div class="js-only"><div class="side-foot"><button class="side-row side-collapse"`)
	assert.Contains(t, body, `data-sidebar-toggle`)
}

// ancestryApp has pages at several depths, to see which entry a page lights.
func ancestryApp() http.Handler {
	page := func(path string, label string) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: label}, Body: webui.Stack{}}
	}
	return webui.App{Pages: webui.Pages{
		page("/", "Overview"),
		page("/device", "Devices"),
		page("/device/archive", ""),     // hidden, below Devices
		page("/device/archive/old", ""), // hidden, below a hidden page: Devices still
		page("/orphan", ""),             // hidden, with no ancestor but the root
		page("/site", "Sites"),
		page("/site/alerts", "Site alerts"), // below Sites, but with an entry of its own
	}}.MustCompile("/admin")
}

func activeEntries(t *testing.T, h http.Handler, target string) []string {
	t.Helper()

	body := serve(h, http.MethodGet, target).Body.String()
	var out []string
	for _, part := range strings.Split(body, `<a class="nav-item active" href="`)[1:] {
		out = append(out, part[:strings.Index(part, `"`)])
	}
	return out
}

func TestAPageWithNoEntryLightsItsNearestAncestorsEntry(t *testing.T) {
	t.Parallel()

	h := ancestryApp()
	assert.DeepEqual(t, activeEntries(t, h, "/admin/device/archive"), []string{"/admin/device"})
	assert.DeepEqual(t, activeEntries(t, h, "/admin/device/archive/old"), []string{"/admin/device"})
	assert.DeepEqual(t, activeEntries(t, h, "/admin/device"), []string{"/admin/device"})
}

func TestTheRootIsNeverAnAncestorForTheHighlight(t *testing.T) {
	t.Parallel()

	// A labelled landing page does not light up for every hidden page below it.
	assert.Equal(t, len(activeEntries(t, ancestryApp(), "/admin/orphan")), 0)
}

func TestAPageWithItsOwnEntryLightsOnlyThat(t *testing.T) {
	t.Parallel()

	assert.DeepEqual(t, activeEntries(t, ancestryApp(), "/admin/site/alerts"), []string{"/admin/site/alerts"})
}

// guardedNavApp has a guarded entry first in its section, and in the app.
func guardedNavApp(allowed func(context.Context) error) http.Handler {
	guarded := func(path, label, section string) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{
			Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: label, Section: section},
			Guard: func(ctx context.Context, _ webui.NoArgs) error { return allowed(ctx) }, Body: webui.Stack{},
		}
	}
	open := func(path, label, section string) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: label, Section: section}, Body: webui.Stack{}}
	}
	return webui.App{Pages: webui.Pages{
		guarded("/audit", "Audit log", "Operations"),
		open("/system", "System", ""), // continues the Operations section
		open("/device", "Devices", "Platform"),
		guarded("/secret", "Secret", "Hidden section"),
	}}.MustCompile("/admin")
}

func TestASidebarEntryTheVisitorMayNotOpenIsLeftOut(t *testing.T) {
	t.Parallel()

	refuse := guardedNavApp(func(context.Context) error { return errors.New("requires the editor role") })
	body := serve(refuse, http.MethodGet, "/admin/device").Body.String()
	assert.False(t, strings.Contains(body, "Audit log"))
	assert.False(t, strings.Contains(body, "Secret"))
	assert.Contains(t, body, "Devices")
	assert.Contains(t, body, "System")

	allow := guardedNavApp(func(context.Context) error { return nil })
	body = serve(allow, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, "Audit log")
	assert.Contains(t, body, "Secret")
}

func TestASectionCaptionStaysAboveWhatIsStillShownAndGoesWithAnEmptySection(t *testing.T) {
	t.Parallel()

	refuse := guardedNavApp(func(context.Context) error { return errors.New("no") })
	body := serve(refuse, http.MethodGet, "/admin/device").Body.String()
	// The Operations entry that carried the caption is hidden; System, which it headed, keeps it.
	assert.Contains(t, body, `<div class="nav-section">Operations</div> <a class="nav-item" href="/admin/system"`)
	// A section with every entry hidden has no caption.
	assert.False(t, strings.Contains(body, "Hidden section"))
}

func TestAHiddenEntryStillAnswers403ToItsAddressAndTheRootSkipsIt(t *testing.T) {
	t.Parallel()

	refuse := guardedNavApp(func(context.Context) error { return errors.New("requires the editor role") })
	rec := serve(refuse, http.MethodGet, "/admin/audit")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), "requires the editor role")

	// The root goes to the first entry the visitor can open, not to the 403.
	root := serve(refuse, http.MethodGet, "/admin/")
	assert.Equal(t, root.Code, http.StatusFound)
	assert.Equal(t, root.Header().Get("Location"), "/admin/system")
}
