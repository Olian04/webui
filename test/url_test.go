package webui_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// linkApp lists devices, each with an address in its Ip field, shown three ways: as a
// plain String, as a URL with the address as its text, and as a URL that says
// "Download".
func linkApp(urls ...string) http.Handler {
	var items []Device
	for i, u := range urls {
		items = append(items, Device{Id: string(rune('a' + i)), Ip: u})
	}
	cols := []webui.Accessor[Device]{
		formID,
		webui.String[Device]{Label: "Plain", Load: func(d Device) string { return d.Ip }},
		webui.URL[Device]{Label: "Site", Load: func(d Device) string { return d.Ip }},
		webui.URL[Device]{Label: "File", Text: "Download", Load: func(d Device) string { return d.Ip }},
	}
	table := webui.Table[Device]{Title: "Links", Columns: cols, Loader: webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) { return items, webui.Ready() }}}
	detail := webui.Form[Device]{
		Title:  "One",
		Load:   func(context.Context) (Device, webui.Loaded) { return items[0], webui.Ready() },
		Fields: []webui.Accessor[Device]{cols[2], cols[3]},
	}
	return webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/list", View: table},
		webui.Page[webui.NoArgs]{Path: "/one", View: detail},
	}}.MustCompile("")
}

func TestAURLColumnIsALinkThatOpensInANewTabAndAStringNeverIs(t *testing.T) {
	t.Parallel()

	const addr = "https://example.com/a?b=1&c=2"
	p := pageOf(t, linkApp(addr), "/list")

	// Declared as a link: a new tab, with the opener cut, and said so. They are the Site column and the File column.
	out := outLinks(t, p)
	assert.Equal(t, len(out), 2)
	for _, a := range out {
		assert.Equal(t, a.Attr("href"), addr)
		assert.Equal(t, a.Attr("rel"), "noopener noreferrer")
	}
	assert.Equal(t, out[0].Name(), addr+"(opens in a new tab)")
	assert.Equal(t, out[1].Name(), "Download(opens in a new tab)")

	// The same text in a String column is text, and the link is not there.
	table := page.One(t, "the table", p.ByRole("table", ""))
	assert.DeepEqual(t, table.Rows()[0][:2], []string{"a", addr})
}

func TestOnlyAPathOrAnHTTPOrMailtoAddressIsEverALink(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"https://example.com/x", "http://example.com", "mailto:ops@example.com", "/other/page"} {
		out := outLinks(t, pageOf(t, linkApp(ok), "/list"))
		assert.Equal(t, len(out), 2)
		assert.Equal(t, out[0].Attr("href"), ok)
	}
	for _, bad := range []string{
		"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,<b>x</b>", "vbscript:x", "//evil.example/x",
		`/\evil.example`, "ftp://example.com/x", "example.com/x", "https:///nohost", "",
	} {
		p := pageOf(t, linkApp(bad), "/list")
		assert.Equal(t, len(outLinks(t, p)), 0)
		assert.Equal(t, len(linksTo(p, bad)), 0)
	}
}

func TestAURLInAFormIsALinkAndAnEmptyOneIsADash(t *testing.T) {
	t.Parallel()

	assert.Equal(t, len(outLinks(t, pageOf(t, linkApp("https://example.com/f"), "/one"))), 2)

	empty := pageOf(t, linkApp(""), "/one")
	assert.Equal(t, len(outLinks(t, empty)), 0)
	assert.Equal(t, len(empty.ByText("empty")), 2) // the Site and the File, each with nothing to show
}

func TestARowClickCannotShareTheFirstColumnWithAURL(t *testing.T) {
	t.Parallel()

	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.View = webui.Stack{}
	table := webui.Table[Device]{
		Loader:   webui.PagedLoader[Device]{Load: okRows},
		RowClick: webui.Link[Device, detailsArgs]{Page: details, Args: func(_ context.Context, d Device) detailsArgs { return detailsArgs{Id: d.Id} }},
		Columns:  []webui.Accessor[Device]{webui.URL[Device]{Label: "Site", Load: func(d Device) string { return d.Ip }}, formID},
	}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", View: table}, details}})
	assert.Contains(t, errs[0].Detail, "first column is a URL")
}

// outLinks are the links of the page's main part that open in a tab of their own: what a URL is declared to be.
func outLinks(t *testing.T, p *page.Page) []page.El {
	t.Helper()
	var out []page.El
	for _, a := range page.One(t, "main", p.ByRole("main", "")).ByRole("link", "") {
		if a.Attr("target") == "_blank" {
			out = append(out, a)
		}
	}
	return out
}
