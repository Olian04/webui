package webui_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func item(label, url string) webui.NavURL {
	return webui.NavURL{Label: label, URL: url}
}

func linksApp(items ...webui.NavItem) http.Handler {
	device := webui.Page[webui.NoArgs]{Path: "/device", View: webui.Stack{}}
	return webui.App{
		Menu:       items,
		Pages:      webui.Pages{device},
		Navigation: []webui.NavItem{webui.NavPage{Page: device, Label: "Devices"}},
	}.MustCompile("/admin")
}

func TestThereIsNoMenuButtonWithoutMenuItems(t *testing.T) {
	t.Parallel()

	assert.Equal(t, len(pageOf(t, linksApp(), "/admin/device").ByRole("button", "External links")), 0)
}

func TestTheMenuButtonOpensTheAppsMenuAsWritten(t *testing.T) {
	t.Parallel()

	logout := item("Log out", "/logout")
	logout.Icon = "right-from-bracket"
	h := linksApp(logout, item("Docs", "https://example.com/help"), item("Mail", "mailto:ops@example.com"))
	p := pageOf(t, h, "/admin/device")

	// The button is in the top bar, beside the page's Refresh, and opens the links.
	top := page.One(t, "top bar", p.ByRole("banner", ""))
	assert.Equal(t, len(top.ByRole("button", "External links")), 1)
	assert.Equal(t, len(top.ByRole("link", "Refresh")), 1)

	// Used as written: the mount prefix is not added to a path, and a web or mail address is left alone.
	menu := externalLinks(t, p)
	assert.DeepEqual(t, menu.Links(), []page.Link{
		{Name: "Log out", Href: "/logout"},
		{Name: "Docs (opens in a new tab)", Href: "https://example.com/help", NewTab: true}, // a web address opens in a tab of its own, and a reader is told
		{Name: "Mail", Href: "mailto:ops@example.com"},
	})
	assert.Contains(t, iconOf(page.One(t, "the Log out link", menu.ByRole("link", "Log out"))), "fa-right-from-bracket")
	// A tab of its own cannot reach back into this one.
	assert.Equal(t, page.One(t, "the Docs link", menu.ByRole("link", "Docs (opens in a new tab)")).Attr("rel"), "noopener noreferrer")
}

func TestTheMenuIsInTheOrderWrittenWithItsGroupsAsCaptions(t *testing.T) {
	t.Parallel()

	docs, bug, out, about, status := item("Docs", "https://example.com/help"), item("Report", "https://example.com/bugs"),
		item("Log out", "/logout"), item("About", "https://example.com/about"), item("Status", "https://example.com/status")
	// Entries with no group first, then a group for each caption: the menu is in the order it is written.
	p := pageOf(t, linksApp(about, status,
		webui.NavGroup{Label: "Help", Items: []webui.NavItem{docs, bug}},
		webui.NavGroup{Label: "Account", Items: []webui.NavItem{out}},
	), "/admin/device")

	// The entries outside a group have no caption above them; each group has one caption, above its entries in the order written.
	assert.DeepEqual(t, menuOutline(externalLinks(t, p)), []string{"About (opens in a new tab)", "Status (opens in a new tab)", "# Help", "Docs (opens in a new tab)", "Report (opens in a new tab)", "# Account", "Log out"})
}

func TestAMenuItemMustHaveALabelAndAnAddressThatIsNotAScript(t *testing.T) {
	t.Parallel()

	errorsOf := func(it webui.NavItem) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Menu: []webui.NavItem{it}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	refuses(t, errorsOf(webui.NavURL{URL: "/x"}), "App.Menu[0]", "Label")
	badIcon := item("x", "/x")
	badIcon.Icon = "rocket-ship"
	refuses(t, errorsOf(badIcon), "App.Menu[0]", `"rocket-ship"`, "Icon")
	for _, evil := range []string{"", "javascript:alert(1)", "//evil.example/", `/\evil.example`, "data:text/html,x", "ftp://example.com", "relative/path", "https://"} {
		refuses(t, errorsOf(item("x", evil)), "App.Menu[0]", "URL")
	}
	for _, ok := range []string{"/x", "/x?y=1", "http://example.com", "https://example.com/a?b=c", "mailto:ops@example.com"} {
		_, err := webui.App{Menu: []webui.NavItem{item("x", ok)}}.Compile("")
		assert.NoError(t, err)
	}

	// The same rules hold for a link inside a group, which is named by where it is declared.
	refuses(t, errorsOf(webui.NavGroup{Label: "Help", Items: []webui.NavItem{item("x", "javascript:alert(1)")}}), "App.Menu[0].Items[0]", "URL")
	refuses(t, errorsOf(webui.NavGroup{Label: "Help"}), "App.Menu[0]", "Items")
	refuses(t, errorsOf(webui.NavGroup{Items: []webui.NavItem{item("x", "/x")}}), "App.Menu[0]", "Label")
	refuses(t, errorsOf(webui.NavGroup{Label: "A", Items: []webui.NavItem{webui.NavGroup{Label: "B", Items: []webui.NavItem{item("x", "/x")}}}}), "App.Menu[0].Items[0]", "group")
}

func TestAMenuRefusesAPageThatListsSeveralAddresses(t *testing.T) {
	t.Parallel()

	site := webui.Page[navSiteArgs]{Path: "/site/{name}", View: webui.Stack{}}
	app := webui.App{Pages: webui.Pages{site}, Menu: []webui.NavItem{webui.NavPages[navSiteArgs]{
		Page:    site,
		Entries: func(context.Context) ([]webui.NavEntry[navSiteArgs], error) { return nil, nil },
	}}}
	var all string
	for _, e := range compileErrors(t, app) {
		all += e.Error() + "\n"
	}
	refuses(t, all, "App.Menu[0]", "NavPages")
}

func TestOnlyAWebAddressOpensInATabOfItsOwn(t *testing.T) {
	t.Parallel()

	h := linksApp(item("Out", "/logout"), item("Mail", "mailto:ops@example.com"), item("Plain", "http://example.com"), item("Secure", "HTTPS://example.com"))
	opens := map[string]bool{}
	for _, l := range externalLinks(t, pageOf(t, h, "/admin/device")).Links() {
		opens[l.Name] = l.NewTab
	}
	assert.DeepEqual(t, opens, map[string]bool{"Out": false, "Mail": false, "Plain (opens in a new tab)": true, "Secure (opens in a new tab)": true})
}
