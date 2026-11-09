package webui_test

import (
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// A phone has the sidebar as a drawer opened by a checkbox that is named and controls it, so that it opens without script.
func TestTheSidebarDrawerIsANamedCheckboxThatControlsIt(t *testing.T) {
	t.Parallel()

	p := pageOf(t, navApp(), "/admin/device")

	// The box is named, is before the shell, and has the drawer to control.
	box := page.One(t, "the box", p.ByRole("checkbox", "Navigation menu"))
	drawer := page.One(t, "the drawer", p.ByAttr("id", box.Attr("aria-controls")))
	assert.Equal(t, drawer.Role(), "complementary")
	assert.True(t, len(drawer.ByRole("navigation", "Main")) > 0)
	assert.True(t, box.Before(drawer))
	assert.True(t, box.Before(page.One(t, "the breadcrumb", p.ByRole("navigation", "Breadcrumb")))) // the button that opens the drawer leads the bar
}

// The top bar's actions are a menu opened by a checkbox that is named and controls it.
func TestTheTopBarsActionsAreAMenuOpenedByANamedCheckbox(t *testing.T) {
	t.Parallel()

	p := pageOf(t, navApp(), "/admin/device")
	box := page.One(t, "the box", p.ByRole("checkbox", "More actions"))
	menu := page.One(t, "the menu", p.ByAttr("id", box.Attr("aria-controls")))
	assert.True(t, len(menu.Links()) > 0 || len(menu.ByRole("switch", "")) > 0) // what it holds is the bar's actions
}

// An app with no sidebar has nothing for a button to open: no drawer, and no button for it.
func TestAnAppWithNoSidebarHasNoDrawerOrButtonForIt(t *testing.T) {
	t.Parallel()

	p := pageOf(t, noNav(), "/admin/devices")
	assert.Equal(t, len(p.ByRole("checkbox", "Navigation menu")), 0)
	assert.Equal(t, len(p.ByRole("complementary", "")), 0)
	// The menu of the top bar is still there.
	assert.Equal(t, len(p.ByRole("checkbox", "More actions")), 1)
}

// The buttons of the bar are in the order Refresh, then the theme, which is the order of the drawer on a phone.
func TestTheTopBarHasRefreshBeforeTheThemeButton(t *testing.T) {
	t.Parallel()

	p := pageOf(t, navApp(), "/admin/device")
	refresh := page.One(t, "Refresh", p.ByRole("link", "Refresh"))
	theme := page.One(t, "the theme switch", p.ByRole("switch", "Light theme"))
	assert.True(t, refresh.Before(theme))
}
