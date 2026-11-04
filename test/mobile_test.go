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
	assert.False(t, strings.Contains(css, "@media (min-width: 641px) and (max-width: 820px) {")) // a narrow desktop is a desktop
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

// What a screen reader is given on a phone: each box is laid over the button it opens, at the size of it, with the name; what is not
// drawn of the trail is still read; and script makes the page behind an open drawer inert and moves the focus into it.
func TestAPhoneIsUsableWithAScreenReader(t *testing.T) {
	t.Parallel()

	h := navApp()
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, "  .nav-toggle,\n  .top-more-toggle {\n    display: block;\n    position: fixed;\n    top: 4px;\n    width: 40px;\n    height: 40px;")
	assert.Contains(t, css, "visibility: hidden;") // a closed drawer is not read, nor reached by a key
	assert.False(t, strings.Contains(css, ".crumbs > :nth-last-child(n + 4) {\n    display: none"))

	enhance := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	for _, want := range []string{"el.setAttribute('inert', '')", "e.key !== 'Escape'", "'#nav-toggle:checked, #top-more:checked'", "addEventListener('pageshow', syncDrawers)"} {
		assert.Contains(t, enhance, want)
	}
}

// The buttons in the drawer are in the order of the bar (Refresh, the theme, the links), also the one that script draws, which a wrapper without a
// box of its own holds; and a pointer over a box hovers its button.
func TestThePhoneDrawerKeepsTheOrderOfTheBarAndTheHoverOfTheButtons(t *testing.T) {
	t.Parallel()

	h := navApp()
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, "  .top-right > *,\n  .top-right > .js-only > * {\n    order: 1;")
	assert.Contains(t, css, "  .nav-toggle:hover ~ .app .nav-open,\n  .top-more-toggle:hover + .top-more {\n    background: var(--hover);")

	assert.Contains(t, css, "  .top-right > .menu > .menu-pop {\n    position: static;")
	assert.Contains(t, css, "    .top-right > .menu::details-content {\n      content-visibility: visible;")

	assert.Contains(t, css, "    min-height: 44px;\n  }\n  /* The footer holds the switch that narrows the sidebar to its rail; the drawer has no rail, so the footer has nothing to hold. */\n  .side-foot {\n    display: none;")

	assert.Contains(t, css, "  .columns {\n    --column-min: 312px;")

	top := topBar(t, serve(h, http.MethodGet, "/admin/device").Body.String())
	refresh, theme := strings.Index(top, "data-refresh"), strings.Index(top, "theme-toggle")
	assert.True(t, refresh >= 0 && refresh < theme)
}

// What is read has a narrow form on a phone, by the stylesheet alone: a trend is its change without the chart and the arrow, a bar is its
// number, and a description is its label underlined, which opens it, in place of an icon. The chart is still named, and the description
// is still a stop of the keys, for a reader.
func TestAPhoneReadsItsValuesInTheirNarrowForm(t *testing.T) {
	t.Parallel()

	h := navApp()
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	phone := css[strings.Index(css, "@media (max-width: 640px) {\n  /* The boxes"):]
	assert.Contains(t, phone, "  .trend .spark {\n    position: absolute;")
	assert.Contains(t, phone, "  .delta .delta-arrow,\n  .gauge-track {\n    display: none;")
	assert.Contains(t, phone, "  .tip .info {\n    display: none;")
	assert.Contains(t, phone, "  .toolbar-lbl:hover + .tip .tip-body,\n  label.lbl:hover + .tip .tip-body,\n  .panel-title:hover + .tip .tip-body {\n    display: block;")

	enhance := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	assert.Contains(t, enhance, "closest('.toolbar-lbl, label.lbl, .panel-title')")
}

// Two fields of a row are side by side where each has room for 220px, which is the panel's room and not the window's, and one under the
// other where it has not; and a field is not held open by what is in it, so a slider and its number stay in the panel.
func TestAFieldRowFollowsTheRoomOfItsPanelAndNotTheWindow(t *testing.T) {
	t.Parallel()

	h := navApp()
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".field-row {\n  display: grid;\n  grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr));")
	assert.Contains(t, css, ".field-row .field {\n  margin-bottom: 0;\n  min-width: 0;\n}")
}
