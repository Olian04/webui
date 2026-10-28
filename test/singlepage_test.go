package webui_test

import (
	"context"
	"errors"
	"image"
	"image/color"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// noNav is an app with pages and not one entry in the navigation: the pages are reached by link, and by the address.
func noNav() http.Handler {
	home := webui.Page[webui.NoArgs]{Path: "/devices", Body: webui.Stack{}}
	alerts := webui.Page[webui.NoArgs]{Path: "/alerts", Body: webui.Stack{}}
	return webui.App{Brand: webui.Brand{Name: "Demo"}, Pages: webui.Pages{home, alerts}}.MustCompile("/admin")
}

// onePage is an app whose one page has an entry in the navigation.
func onePage(guard func(context.Context, webui.NoArgs) error) http.Handler {
	only := webui.Page[webui.NoArgs]{Path: "/devices", Nav: webui.Nav{Label: "Devices"}, Guard: guard, Body: webui.Stack{}}
	return webui.App{Brand: webui.Brand{Name: "Demo"}, Pages: webui.Pages{only}}.MustCompile("/admin")
}

// topBar is the markup of the header, which is where the chrome of an app that has no sidebar is.
func topBar(t *testing.T, body string) string {
	t.Helper()

	start := strings.Index(body, `<header class="topbar">`)
	end := strings.Index(body, "</header>")
	if start < 0 || end < start {
		t.Fatalf("no top bar in %s", body)
	}
	return body[start:end]
}

// Whether an app has a sidebar is the author's to say, by the pages that have an entry: where no page declares one, there is nothing for a sidebar to
// list, and the page has its width. What the sidebar held that is not navigation stays, in the top bar.
func TestAnAppWhosePagesDeclareNoEntryHasNoSidebar(t *testing.T) {
	t.Parallel()

	rec := serve(noNav(), http.MethodGet, "/admin/devices")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()

	for _, absent := range []string{`class="sidebar"`, "<aside", `class="side-brand"`, `class="side-scroll"`, `class="nav-item`, "data-sidebar-toggle"} {
		assert.False(t, strings.Contains(body, absent))
	}
	assert.Contains(t, body, `<main id="main"`)

	// The first crumb is the brand, which is where it leads back to.
	top := topBar(t, body)
	assert.Contains(t, top, `Demo`)

	// The theme is one icon button in the top bar, once. It is a switch, named by the theme it turns on, and carries both icons: the stylesheet shows the one
	// that a press brings.
	assert.Equal(t, strings.Count(body, `data-pref-toggle="theme"`), 1)
	assert.Contains(t, top, `<button class="iconbtn theme-toggle" type="button" role="switch" aria-checked="false" aria-label="Light theme" title="Light theme" data-pref-toggle="theme">`)
	assert.Contains(t, top, `fa-sun`)
	assert.Contains(t, top, `fa-moon`)
}

// An app with no page at all has no entry to list either.
func TestAnAppWithNoPageHasNoSidebar(t *testing.T) {
	t.Parallel()

	body := serve(webui.App{}.MustCompile("/admin"), http.MethodGet, "/admin/").Body.String()
	assert.False(t, strings.Contains(body, `class="sidebar"`))
	assert.Equal(t, strings.Count(body, `data-pref-toggle="theme"`), 1)
}

// The states that the library answers with, a missing page and a method that the page does not take, are drawn in the same shell as a page is.
func TestTheStatesOfAnAppWithNoEntryHaveNoSidebar(t *testing.T) {
	t.Parallel()

	h := noNav()
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/nowhere").Body.String(), `class="sidebar"`))
	assert.False(t, strings.Contains(serve(h, http.MethodPost, "/admin/devices").Body.String(), `class="sidebar"`))
}

// One page is enough, if it has an entry: the app says it wants a sidebar by giving a page one, and the first example of the README is that.
// The theme is the top bar's, with or without a sidebar, and the sidebar has no switch of its own.
func TestAnAppWithOnePageAndAnEntryHasASidebar(t *testing.T) {
	t.Parallel()

	body := serve(onePage(nil), http.MethodGet, "/admin/devices").Body.String()
	assert.Contains(t, body, `<aside class="sidebar" id="sidebar">`)
	assert.Equal(t, strings.Count(body, `class="nav-item`), 1)
	assert.Equal(t, strings.Count(body, `data-pref-toggle="theme"`), 1)
	assert.Contains(t, topBar(t, body), `data-pref-toggle="theme"`)
	assert.False(t, strings.Contains(body, `data-pref="theme"`))
	assert.False(t, strings.Contains(body, `id="pref-theme"`))
}

// A page that lists its entries from the app's data has an entry too, though it may list none or one: the sidebar is not for the number.
func TestAPageThatListsEntriesGivesTheAppASidebar(t *testing.T) {
	t.Parallel()

	for name, names := range map[string][]string{"none": nil, "one": {"Stockholm"}, "three": siteNames} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			site := webui.Page[siteArgs]{
				Path: "/site/{name}", Nav: webui.Nav{Icon: "location-dot"},
				NavEntries: siteEntries(names...), Body: webui.Stack{},
			}
			h := webui.App{Pages: webui.Pages{site}}.MustCompile("/admin")
			body := serve(h, http.MethodGet, "/admin/site/Stockholm").Body.String()
			assert.Contains(t, body, `<aside class="sidebar" id="sidebar">`)
			assert.Equal(t, strings.Count(body, `class="nav-item`), len(names))
		})
	}
}

// The declaration decides, and not what a visitor may open of it: a visitor whom the Guard of the only entry refuses sees the sidebar the author gave the
// app, with nothing in its list, and not a page that changes shape with who is looking. (The page itself answers 403, in the same shell.)
func TestTheSidebarDoesNotDependOnWhatTheVisitorMayOpen(t *testing.T) {
	t.Parallel()

	refused := func(context.Context, webui.NoArgs) error { return errors.New("not for you") }
	rec := serve(onePage(refused), http.MethodGet, "/admin/devices")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	body := rec.Body.String()
	assert.Contains(t, body, `<aside class="sidebar" id="sidebar">`)
	assert.False(t, strings.Contains(body, `class="nav-item`))
	assert.Contains(t, topBar(t, body), `data-pref-toggle="theme"`)
}

// The mark of the app, which is in the sidebar's brand, is at the head of the top bar when there is no sidebar, so that the app still has its logo on
// screen: the library's own when the app has none, and the app's when it has.
func TestTheMarkOfAnAppWithNoSidebarLeadsTheTopBar(t *testing.T) {
	t.Parallel()

	top := topBar(t, serve(noNav(), http.MethodGet, "/admin/devices").Body.String())
	assert.Contains(t, top, `<span class="top-mark" aria-hidden="true"><div class="logo" aria-hidden="true">`)
	assert.True(t, strings.Index(top, `class="top-mark"`) < strings.Index(top, `class="crumbs"`))

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	img.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	only := webui.Page[webui.NoArgs]{Path: "/devices", Body: webui.Stack{}}
	h := webui.App{Brand: webui.Brand{Name: "Demo", Logo: img}, Pages: webui.Pages{only}}.MustCompile("/admin")
	top = topBar(t, serve(h, http.MethodGet, "/admin/devices").Body.String())
	assert.Contains(t, top, `<img class="logo-img" src="`+assetURL(t, h, "logo")+`" alt="">`)
}

// With a sidebar the mark is in the sidebar's brand, and is not drawn twice.
func TestTheMarkOfAnAppWithASidebarIsNotInTheTopBar(t *testing.T) {
	t.Parallel()

	body := serve(onePage(nil), http.MethodGet, "/admin/devices").Body.String()
	assert.False(t, strings.Contains(topBar(t, body), "top-mark"))
	assert.Equal(t, strings.Count(body, `class="logo"`), 1)
}

// The icon button is script's: it has the toggle's name for a reader, and the stylesheet and the script say which icon is drawn and what the button does.
func TestTheThemeToggleIsDrivenByScriptAndTheStylesheet(t *testing.T) {
	t.Parallel()

	h := noNav()
	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	for _, want := range []string{
		`'[data-pref-toggle="theme"]'`,
		"localStorage.setItem('webui.theme', theme);",
		"btn.setAttribute('aria-checked', isLight() ? 'true' : 'false');",
	} {
		assert.Contains(t, js, want)
	}

	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	for _, want := range []string{
		".theme-toggle .ti-moon {\n  display: none;\n}",
		"html[data-theme='light'] .theme-toggle .ti-sun {\n  display: none;\n}",
		"html:not([data-theme]) .theme-toggle .ti-moon {\n    display: inline-block;\n  }",
	} {
		assert.Contains(t, css, want)
	}
}
