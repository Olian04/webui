package webui_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func navApp() http.Handler {
	empty := func(path string, nav webui.Nav) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: nav, View: webui.Stack{}}
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

	entry := page.One(t, "the Devices entry", pageOf(t, navApp(), "/admin/device").ByRole("link", "Devices"))
	icon := entry.Children()[0]
	assert.Contains(t, icon.Attr("class"), "fa-display") // Nav.Icon is the name of a Font Awesome icon
	assert.Equal(t, icon.Attr("aria-hidden"), "true")    // decoration: the entry is called by its label
	assert.Equal(t, entry.AllText(), "Devices")          // and there is no initial beside it
}

func TestANavEntryWithoutAnIconCarriesItsCapitalisedInitialForTheRail(t *testing.T) {
	t.Parallel()

	entry := page.One(t, "the retention entry", pageOf(t, navApp(), "/admin/device").ByRole("link", "retention"))
	letter := entry.Children()[0]
	assert.Equal(t, letter.AllText(), "R")              // for the rail, which has no room for the label
	assert.Equal(t, letter.Attr("aria-hidden"), "true") // a reader has the label, and not the letter as well
}

func TestTheBrandLinksToTheLandingPage(t *testing.T) {
	t.Parallel()

	h := webui.App{Brand: webui.Brand{Name: "Acme"}, Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/", View: webui.Stack{}},
		webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, View: webui.Stack{}},
	}}.MustCompile("/admin")
	p := pageOf(t, h, "/admin/device")
	home := linksTo(p, "/admin/")
	assert.Equal(t, len(home), 2) // the brand, and the first breadcrumb
	assert.Equal(t, named(home, "Acme"), 2)
	assert.Equal(t, crumbs(t, p).Links()[0].Href, "/admin/")
}

func TestTheBrandOfAnAppWithNoLandingPageLinksToThePageTheRootWouldSendYouTo(t *testing.T) {
	t.Parallel()

	p := pageOf(t, navApp(), "/admin/retention")
	assert.Equal(t, named(linksTo(p, "/admin/device"), "Acme"), 2) // the brand and the first breadcrumb lead to the first entry, not the root
	assert.Equal(t, len(linksTo(p, "/admin/")), 0)
}

func TestTheBrandLinksToTheFirstPageThatNeedsNoArgumentsWhenNothingIsInTheNavigation(t *testing.T) {
	t.Parallel()

	type one struct{ ID string }
	byID := webui.Page[one]{Path: "/device/{id}", View: webui.Stack{}}
	locked := webui.Page[webui.NoArgs]{Path: "/locked", Guard: func(context.Context, webui.NoArgs) error { return errors.New("no") }, View: webui.Stack{}}
	open := webui.Page[webui.NoArgs]{Path: "/overview", View: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{byID, locked, open}}.MustCompile("/admin")

	// No page declares an entry, so there is no sidebar: the brand is the first crumb, which leads where the sidebar's would.
	assert.DeepEqual(t, crumbs(t, pageOf(t, h, "/admin/device/x")).Links(), []page.Link{{Name: "Home", Href: "/admin/overview"}})
}

func TestTheBrandIsNotALinkWhenThereIsNowhereToGo(t *testing.T) {
	t.Parallel()

	type one struct{ ID string }
	h := webui.App{Brand: webui.Brand{Name: "Acme"}, Pages: webui.Pages{webui.Page[one]{Path: "/device/{id}", View: webui.Stack{}}}}.MustCompile("/admin")

	p := pageOf(t, h, "/admin/device/x")
	assert.Equal(t, len(linksTo(p, "/admin/")), 0)
	assert.True(t, strings.HasPrefix(crumbs(t, p).Text(), "Acme")) // plain text, as the first breadcrumb
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
			webui.Page[webui.NoArgs]{Path: "/", View: webui.Stack{}},
			webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, View: webui.Stack{}},
		},
	}.MustCompile("/admin")

	for _, target := range []string{"/admin", "/admin/"} {
		rec := serve(h, http.MethodGet, target)
		assert.Equal(t, rec.Code, http.StatusOK) // served, not redirected
		c := crumbs(t, page.Parse(t, rec.Body.String()))
		assert.Equal(t, c.Text(), "Acme") // the landing page is the brand, and it is where the reader is
		assert.Equal(t, len(c.Links()), 0)
	}
}

func TestCompileRefusesAnIconThatIsNotAFontAwesomeIcon(t *testing.T) {
	t.Parallel()

	for icon, names := range map[string][]string{
		"rocket-ship": {"Nav.Icon", `"rocket-ship"`},
		"fa-house":    {"Nav.Icon", `"fa-house"`, `"house"`}, // and what to write instead
	} {
		app := webui.App{Pages: webui.Pages{
			webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A", Icon: icon}, View: webui.Stack{}},
		}}
		var all string
		for _, e := range compileErrors(t, app) {
			all += e.Error() + "\n"
		}
		refuses(t, all, names...)
	}
}

func TestTheIconFontAndItsClassesAreServedAndLinked(t *testing.T) {
	t.Parallel()

	h := navApp()
	sheets := pageOf(t, h, "/admin/device").ByAttr("href", assetURL(t, h, "fontawesome.css"))
	assert.Equal(t, page.One(t, "link to the icon stylesheet", sheets).Attr("rel"), "stylesheet")

	css := serve(h, http.MethodGet, assetURL(t, h, "fontawesome.css"))
	assert.Equal(t, css.Code, http.StatusOK)
	sheet := css.Body.String()
	assert.Contains(t, sheet, ".fa-display:before")              // the icon the app names
	assert.Contains(t, sheet, ".fa-gear:before")                 // and the ones the library draws itself
	assert.False(t, strings.Contains(sheet, ".fa-house:before")) // and no other: an app does not carry the two thousand

	font := serve(h, http.MethodGet, assetURL(t, h, "fa-solid.woff"))
	assert.Equal(t, font.Code, http.StatusOK)
	assert.True(t, strings.HasPrefix(font.Header().Get("Content-Type"), "font/")) // a font, in whatever format is served
	assert.True(t, font.Body.Len() < 10_000)                                      // and not the hundred and twenty kilobytes of all of them
}

func TestTheSidebarOffersACollapseControlOnlyWithScript(t *testing.T) {
	t.Parallel()

	// That it is not there without script is a scenario of the browser suite.
	assert.Equal(t, len(pageOf(t, navApp(), "/admin/device").ByAttr("data-sidebar-toggle", "")), 1)
}

// ancestryApp has pages at several depths, to see which entry a page lights.
func ancestryApp() http.Handler {
	page := func(path string, label string) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: label}, View: webui.Stack{}}
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
			Guard: func(ctx context.Context, _ webui.NoArgs) error { return allowed(ctx) }, View: webui.Stack{},
		}
	}
	open := func(path, label, section string) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: label, Section: section}, View: webui.Stack{}}
	}
	return webui.App{Pages: webui.Pages{
		guarded("/audit", "Audit log", "Operations"),
		open("/system", "System", "Operations"),
		open("/device", "Devices", "Platform"),
		guarded("/secret", "Secret", "Hidden section"),
	}}.MustCompile("/admin")
}

func TestASidebarEntryTheVisitorMayNotOpenIsLeftOut(t *testing.T) {
	t.Parallel()

	refuse := guardedNavApp(func(context.Context) error { return errors.New("requires the editor role") })
	assert.DeepEqual(t, outline(t, pageOf(t, refuse, "/admin/device")), []string{"# Operations", "System", "# Platform", "Devices"})

	allow := guardedNavApp(func(context.Context) error { return nil })
	assert.DeepEqual(t, outline(t, pageOf(t, allow, "/admin/device")), []string{"# Operations", "Audit log", "System", "# Platform", "Devices", "# Hidden section", "Secret"})
}

func TestASectionCaptionStaysAboveWhatIsStillShownAndGoesWithAnEmptySection(t *testing.T) {
	t.Parallel()

	refuse := guardedNavApp(func(context.Context) error { return errors.New("no") })
	// Audit log, first in Operations, is hidden; the caption goes to System, the first one shown. A section with every entry hidden has no caption.
	assert.DeepEqual(t, outline(t, pageOf(t, refuse, "/admin/device")), []string{"# Operations", "System", "# Platform", "Devices"})
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

func TestTheSidebarIsGroupedBySectionAndNotByPosition(t *testing.T) {
	t.Parallel()

	entry := func(path, label, section string) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: label, Section: section}, View: webui.Stack{}}
	}
	// Given out of order: sections interleaved, and entries with no Section among them.
	h := webui.App{Pages: webui.Pages{
		entry("/device", "Devices", "Platform"),
		entry("/retention", "Retention", "Configuration"),
		entry("/home", "Home", ""),
		entry("/site", "Sites", "Platform"),
		entry("/ingest", "Ingest", "Configuration"),
		entry("/about", "About", ""),
	}}.MustCompile("/admin")
	// No Section: first, in the order given, under no caption. Sections in the order first named, entries in the order given, one caption for each section.
	assert.DeepEqual(t, outline(t, pageOf(t, h, "/admin/device")), []string{
		"Home", "About", "# Platform", "Devices", "Sites", "# Configuration", "Retention", "Ingest",
	})

	// And the root goes to the first entry the sidebar lists.
	assert.Equal(t, serve(h, http.MethodGet, "/admin/").Header().Get("Location"), "/admin/home")
}

func TestAPageWithPathArgumentsCannotHaveANavLabelBecauseItHasNoAddress(t *testing.T) {
	t.Parallel()

	type one struct{ ID string }
	declared := webui.Page[one]{Path: "/device/{id}", Nav: webui.Nav{Label: "Device"}, View: webui.Stack{}}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{declared}})
	refuses(t, errs[0].Error(), `"Device"`, "/device/{id}")
}

func TestTheRootOfAnAppWithPagesButNoStartSaysThereIsNothingToOpen(t *testing.T) {
	t.Parallel()

	type one struct{ ID string }
	declared := webui.Page[one]{Path: "/device/{id}", View: webui.Stack{}} // no Nav, and none at "/"
	h := webui.App{Brand: webui.Brand{Name: "Acme"}, Pages: webui.Pages{declared}}.MustCompile("/admin")

	for _, at := range []string{"/admin", "/admin/"} {
		rec := serve(h, http.MethodGet, at)
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Contains(t, rec.Body.String(), "Nothing to open from here")
	}
}

// flashOfRoot is the toast a response left for the next page: its tone and its text.
func flashOfRoot(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "webui_flash" {
			raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
			return string(raw)
		}
	}
	return ""
}

func TestTheRootOfAnAppWithNoPageThereSendsYouToTheFirstEntryWithoutAToast(t *testing.T) {
	t.Parallel()

	rec := serve(navApp(), http.MethodGet, "/admin/")
	assert.Equal(t, rec.Code, http.StatusFound)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device")
	assert.Equal(t, flashOfRoot(rec), "")
}

func TestWithNoEntryTheRootSendsYouToTheFirstPageThatNeedsNoArgumentsAndYouMayOpen(t *testing.T) {
	t.Parallel()

	type one struct{ ID string }
	locked := webui.Page[webui.NoArgs]{Path: "/locked", Guard: func(context.Context, webui.NoArgs) error { return errors.New("no") }, View: webui.Stack{}}
	byID := webui.Page[one]{Path: "/device/{id}", View: webui.Stack{}} // needs an argument: not a start
	open := webui.Page[webui.NoArgs]{Path: "/overview", View: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{byID, locked, open}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/")
	assert.Equal(t, rec.Code, http.StatusFound)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/overview") // not the one with arguments, not the one it may not open
	assert.Equal(t, flashOfRoot(rec), "")
}

func TestARootThatHasAPageIsServedWithNoRedirectAndNoWarning(t *testing.T) {
	t.Parallel()

	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/", View: webui.Stack{}}}}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, flashOfRoot(rec), "")
}

func TestTheRootHasNoWarningToGiveWhenThereIsNowhereToSendYou(t *testing.T) {
	t.Parallel()

	type one struct{ ID string }
	h := webui.App{Pages: webui.Pages{webui.Page[one]{Path: "/device/{id}", View: webui.Stack{}}}}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/")
	assert.Equal(t, rec.Code, http.StatusOK) // the page that says so
	assert.Equal(t, flashOfRoot(rec), "")
}
