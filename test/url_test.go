package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
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
	table := webui.Table[Device]{Title: "Links", Columns: cols, Rows: func(context.Context) ([]Device, error) { return items, nil }}
	detail := webui.Form[Device]{
		Title:  "One",
		Load:   func(context.Context) (Device, error) { return items[0], nil },
		Fields: []webui.Accessor[Device]{cols[2], cols[3]},
	}
	return webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/list", Body: table},
		webui.Page[webui.NoArgs]{Path: "/one", Body: detail},
	}}.MustCompile("")
}

func TestAURLColumnIsALinkThatOpensInANewTabAndAStringNeverIs(t *testing.T) {
	t.Parallel()

	body := serve(linkApp("https://example.com/a?b=1&c=2"), http.MethodGet, "/list").Body.String()

	// Declared as a link: a new tab, with the opener cut, and said so.
	assert.Contains(t, body, `<a class="ext" href="https://example.com/a?b=1&amp;c=2" target="_blank" rel="noopener noreferrer">`)
	assert.Contains(t, body, `>Download<`)
	assert.Contains(t, body, `<span class="sr-only">(opens in a new tab)</span>`)
	assert.Equal(t, strings.Count(body, `class="ext"`), 2) // the Site column and the File column

	// The same text in a String column is text: the two links are the two URL columns.
	assert.Equal(t, strings.Count(body, `href="https://example.com/a?b=1&amp;c=2"`), 2)
	assert.Contains(t, body, `>https://example.com/a?b=1&amp;c=2<`)
}

func TestOnlyAPathOrAnHTTPOrMailtoAddressIsEverALink(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"https://example.com/x", "http://example.com", "mailto:ops@example.com", "/other/page"} {
		assert.Equal(t, strings.Count(serve(linkApp(ok), http.MethodGet, "/list").Body.String(), `class="ext"`), 2)
	}
	for _, bad := range []string{
		"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,<b>x</b>", "vbscript:x", "//evil.example/x",
		`/\evil.example`, "ftp://example.com/x", "example.com/x", "https:///nohost", "",
	} {
		body := serve(linkApp(bad), http.MethodGet, "/list").Body.String()
		assert.False(t, strings.Contains(body, `class="ext"`))
		assert.False(t, strings.Contains(body, `href="`+bad+`"`))
	}
}

func TestAURLInAFormIsALinkAndAnEmptyOneIsADash(t *testing.T) {
	t.Parallel()

	body := serve(linkApp("https://example.com/f"), http.MethodGet, "/one").Body.String()
	assert.Equal(t, strings.Count(body, `class="ext"`), 2)

	empty := serve(linkApp(""), http.MethodGet, "/one").Body.String()
	assert.False(t, strings.Contains(empty, `class="ext"`))
	assert.Contains(t, empty, `<span class="dim" aria-hidden="true">—</span>`)
}

func TestARowClickCannotShareTheFirstColumnWithAURL(t *testing.T) {
	t.Parallel()

	var details webui.Page[detailsArgs]
	details.Path = "/device/{id}"
	details.Body = webui.Stack{}
	table := webui.Table[Device]{
		Load:     okRows,
		RowClick: webui.Link[Device, detailsArgs]{Page: details, Args: func(_ context.Context, d Device) detailsArgs { return detailsArgs{Id: d.Id} }},
		Columns:  []webui.Accessor[Device]{webui.URL[Device]{Label: "Site", Load: func(d Device) string { return d.Ip }}, formID},
	}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: table}, details}})
	assert.Contains(t, errs[0].Detail, "first column is a URL")
}
