package webui_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// A phone has the sidebar as a drawer opened by a checkbox that is named and controls it, so that it opens without script.
func TestTheSidebarDrawerIsANamedCheckboxThatControlsIt(t *testing.T) {
	t.Parallel()

	h := navApp()
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	// The box is named, is before the shell, and has the drawer to control; its button and the backdrop are for the pointer only.
	assert.Contains(t, body, `<input type="checkbox" id="nav-toggle" class="nav-toggle" aria-label="Navigation menu" aria-controls="sidebar"><div class="app">`)
	assert.Contains(t, body, `<label class="iconbtn nav-open" for="nav-toggle" aria-hidden="true">`)
	assert.Contains(t, body, `<label class="nav-backdrop" for="nav-toggle" aria-hidden="true"></label>`)
	assert.Contains(t, body, `<aside class="sidebar drawer drawer-left" id="sidebar">`)
}

// The top bar's actions are a menu opened by a checkbox that is named and controls it, and the button that opens the drawer leads the bar.
func TestTheTopBarsActionsAreAMenuOpenedByANamedCheckbox(t *testing.T) {
	t.Parallel()

	h := navApp()
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	top := topBar(t, body)
	assert.Contains(t, top, `<input type="checkbox" id="top-more" class="top-more-toggle" aria-label="More actions" aria-controls="top-right">`)
	assert.Contains(t, top, `<label class="iconbtn top-more" for="top-more" aria-hidden="true">`)
	assert.Contains(t, top, `<div class="top-right drawer drawer-right" id="top-right">`)
	assert.Contains(t, top, `<label class="top-backdrop" for="top-more" aria-hidden="true"></label>`)
	// The button that opens the drawer leads the bar.
	assert.True(t, strings.Index(top, "nav-open") < strings.Index(top, `class="crumbs"`))
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

// The buttons of the bar are in the order Refresh, then the theme, which is the order of the drawer on a phone.
func TestTheTopBarHasRefreshBeforeTheThemeButton(t *testing.T) {
	t.Parallel()

	h := navApp()
	top := topBar(t, serve(h, http.MethodGet, "/admin/device").Body.String())
	refresh, theme := strings.Index(top, "data-refresh"), strings.Index(top, "theme-toggle")
	assert.True(t, refresh >= 0 && refresh < theme)
}
