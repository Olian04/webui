package webui_test

import (
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
	assert.Contains(t, page, `<link rel="stylesheet" href="/admin/_webui/fontawesome.css">`)

	css := serve(h, http.MethodGet, "/admin/_webui/fontawesome.css")
	assert.Equal(t, css.Code, http.StatusOK)
	assert.Contains(t, css.Body.String(), `.fa-house::before{content:"\f015"}`)
	assert.Contains(t, css.Body.String(), `url(fa-solid-900.woff2)`) // beside the stylesheet, so a relative url finds it

	font := serve(h, http.MethodGet, "/admin/_webui/fa-solid-900.woff2")
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
