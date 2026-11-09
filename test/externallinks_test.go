package webui_test

import (
	"net/http"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func item(label, url string) webui.ExternalLink {
	return webui.ExternalLink{Label: label, URL: url}
}

func linksApp(items ...webui.ExternalLink) http.Handler {
	return webui.App{
		ExternalLinks: items,
		Pages:         webui.Pages{webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, View: webui.Stack{}}},
	}.MustCompile("/admin")
}

func TestThereIsNoExternalLinksButtonWithoutExternalLinks(t *testing.T) {
	t.Parallel()

	assert.Equal(t, len(pageOf(t, linksApp(), "/admin/device").ByRole("button", "External links")), 0)
}

func TestTheExternalLinksButtonOpensTheAppsLinksAsWritten(t *testing.T) {
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

func TestTheExternalLinksAreGroupedBySectionAndNotByPosition(t *testing.T) {
	t.Parallel()

	docs, bug, out, about, status := item("Docs", "https://example.com/help"), item("Report", "https://example.com/bugs"),
		item("Log out", "/logout"), item("About", "https://example.com/about"), item("Status", "https://example.com/status")
	docs.Section, bug.Section, out.Section = "Help", "Help", "Account"
	// Given out of order: an entry with no Section between sections, and a section
	// that is named again after another.
	p := pageOf(t, linksApp(docs, about, out, bug, status), "/admin/device")

	// No Section: first, in the order given, and with no caption above them. Sections in the order first named, each with its entries in the order given, and
	// one caption for each section.
	assert.DeepEqual(t, menuOutline(externalLinks(t, p)), []string{"About (opens in a new tab)", "Status (opens in a new tab)", "# Help", "Docs (opens in a new tab)", "Report (opens in a new tab)", "# Account", "Log out"})
}

func TestAnExternalLinkMustHaveALabelAndAnAddressThatIsNotAScript(t *testing.T) {
	t.Parallel()

	errorsOf := func(it webui.ExternalLink) string {
		var all string
		for _, e := range compileErrors(t, webui.App{ExternalLinks: []webui.ExternalLink{it}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	refuses(t, errorsOf(webui.ExternalLink{URL: "/x"}), "App.ExternalLinks[0]", "Label")
	badIcon := item("x", "/x")
	badIcon.Icon = "rocket-ship"
	assert.Contains(t, errorsOf(badIcon), `"rocket-ship", which is not a Font Awesome`)
	for _, evil := range []string{"", "javascript:alert(1)", "//evil.example/", `/\evil.example`, "data:text/html,x", "ftp://example.com", "relative/path", "https://"} {
		assert.Contains(t, errorsOf(item("x", evil)), "which is not a path on this site")
	}
	for _, ok := range []string{"/x", "/x?y=1", "http://example.com", "https://example.com/a?b=c", "mailto:ops@example.com"} {
		_, err := webui.App{ExternalLinks: []webui.ExternalLink{item("x", ok)}}.Compile("")
		assert.NoError(t, err)
	}
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
