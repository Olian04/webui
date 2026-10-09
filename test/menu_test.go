package webui_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func item(label, url string) webui.MenuItem {
	return webui.MenuItem{Nav: webui.Nav{Label: label}, ExternalURL: url}
}

func menuApp(items ...webui.MenuItem) http.Handler {
	return webui.App{
		Menu:  items,
		Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: webui.Stack{}}},
	}.MustCompile("/admin")
}

func TestThereIsNoMenuButtonWithoutMenuItems(t *testing.T) {
	t.Parallel()

	assert.False(t, strings.Contains(serve(menuApp(), http.MethodGet, "/admin/device").Body.String(), `aria-label="Menu"`))
}

func TestTheMenuButtonOpensTheAppsOwnLinksAsWritten(t *testing.T) {
	t.Parallel()

	logout := item("Log out", "/logout")
	logout.Icon = "right-from-bracket"
	h := menuApp(logout, item("Docs", "https://example.com/help"), item("Mail", "mailto:ops@example.com"))
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	assert.Contains(t, body, `<summary class="iconbtn" title="Menu" aria-label="Menu">`)
	// Used as written: the mount prefix is not added to a path, and a web or mail
	// address is left alone.
	assert.Contains(t, body, `<a class="menu-item" href="/logout"><i class="fa-solid fa-right-from-bracket menu-icon" aria-hidden="true"></i><span>Log out</span></a>`)
	assert.Contains(t, body, `href="https://example.com/help"`)
	assert.Contains(t, body, `href="mailto:ops@example.com"`)

	// It sits after the page's Refresh, at the right of the top bar.
	assert.True(t, strings.Index(body, "data-refresh") < strings.Index(body, `aria-label="Menu"`))
}

func TestAMenuItemsSectionCaptionsItAndTheEntriesAfterIt(t *testing.T) {
	t.Parallel()

	help, bug, out := item("Docs", "https://example.com/help"), item("Report", "https://example.com/bugs"), item("Log out", "/logout")
	help.Section, out.Section = "Help", "Account"
	body := serve(menuApp(help, bug, out), http.MethodGet, "/admin/device").Body.String()

	assert.Equal(t, strings.Count(body, `class="menu-section"`), 2)
	assert.True(t, strings.Index(body, ">Help<") < strings.Index(body, "<span>Docs</span>"))
	assert.True(t, strings.Index(body, "<span>Report</span>") < strings.Index(body, ">Account<")) // Report continues Help
	assert.True(t, strings.Index(body, ">Account<") < strings.Index(body, "<span>Log out</span>"))
}

func TestAMenuItemMustHaveALabelAndAnAddressThatIsNotAScript(t *testing.T) {
	t.Parallel()

	errorsOf := func(it webui.MenuItem) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Menu: []webui.MenuItem{it}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	assert.Contains(t, errorsOf(webui.MenuItem{ExternalURL: "/x"}), "App.Menu[0] has no Label")
	badIcon := item("x", "/x")
	badIcon.Icon = "rocket-ship"
	assert.Contains(t, errorsOf(badIcon), `"rocket-ship", which is not a Font Awesome`)
	for _, evil := range []string{"", "javascript:alert(1)", "//evil.example/", `/\evil.example`, "data:text/html,x", "ftp://example.com", "relative/path", "https://"} {
		assert.Contains(t, errorsOf(item("x", evil)), "which is not a path on this site")
	}
	for _, ok := range []string{"/x", "/x?y=1", "http://example.com", "https://example.com/a?b=c", "mailto:ops@example.com"} {
		_, err := webui.App{Menu: []webui.MenuItem{item("x", ok)}}.Compile("")
		assert.NoError(t, err)
	}
}
