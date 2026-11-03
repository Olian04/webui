package webui_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// A phone has the sidebar as a drawer and the top bar's actions as a menu, each a checkbox and its label, so that they open without script.
func TestAPhoneHasTheSidebarAsADrawerOpenedByACheckbox(t *testing.T) {
	t.Parallel()

	h := navApp()
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	// The box is named, is before the shell, and has the drawer to control; its button and the backdrop are for the pointer only.
	assert.Contains(t, body, `<input type="checkbox" id="nav-toggle" class="nav-toggle" aria-label="Navigation menu" aria-controls="sidebar"><div class="app">`)
	assert.Contains(t, body, `<label class="iconbtn nav-open" for="nav-toggle" aria-hidden="true">`)
	assert.Contains(t, body, `<label class="nav-backdrop" for="nav-toggle" aria-hidden="true"></label>`)
	assert.Contains(t, body, `<aside class="sidebar" id="sidebar">`)

	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	phone := css[strings.Index(css, "@media (max-width: 640px) {\n  /* The boxes"):]
	// Closed it is out of the order of the keys, and ticked it slides in.
	assert.Contains(t, phone, "transform: translateX(-100%);\n    visibility: hidden;")
	assert.Contains(t, phone, ".nav-toggle:checked ~ .app .sidebar {\n    transform: none;\n    visibility: visible;")
	// None of it is drawn on a wider screen, and a stored choice of the rail does not reach a phone.
	assert.Contains(t, css, ".nav-toggle,\n.nav-open,\n.nav-backdrop,\n.top-backdrop,\n.top-more-toggle,\n.top-more {\n  display: none;")
	assert.Contains(t, css, "@media (min-width: 641px) {\n  html[data-sidebar='collapsed'] .sidebar {")
	assert.Contains(t, css, "@media (min-width: 641px) and (max-width: 820px) {")
}

func TestAPhoneHasTheTopBarsActionsAsAMenu(t *testing.T) {
	t.Parallel()

	h := navApp()
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	top := topBar(t, body)
	assert.Contains(t, top, `<input type="checkbox" id="top-more" class="top-more-toggle" aria-label="More actions" aria-controls="top-right">`)
	assert.Contains(t, top, `<label class="iconbtn top-more" for="top-more" aria-hidden="true">`)
	assert.Contains(t, top, `<div class="top-right" id="top-right">`)
	// The button that opens the drawer leads the bar.
	assert.True(t, strings.Index(top, "nav-open") < strings.Index(top, `class="crumbs"`))

	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, top, `<label class="top-backdrop" for="top-more" aria-hidden="true"></label>`)
	// It comes in from the right, with the buttons first, then the search, then its results.
	assert.Contains(t, css, "transform: translateX(100%);\n    visibility: hidden;")
	assert.Contains(t, css, ".top-right .search {\n    order: 2;")
	assert.Contains(t, css, ".top-right .palette {\n    order: 3;")
}

// An app with no sidebar has nothing for a button to open: no drawer, and no button for it.
func TestAnAppWithNoSidebarHasNoDrawerOrButtonForIt(t *testing.T) {
	t.Parallel()

	body := serve(noNav(), http.MethodGet, "/admin/devices").Body.String()
	assert.False(t, strings.Contains(body, "nav-toggle"))
	assert.False(t, strings.Contains(body, "nav-open"))
	assert.False(t, strings.Contains(body, "nav-backdrop"))
	// The menu of the top bar is still there.
	assert.Contains(t, body, `id="top-more"`)
}
