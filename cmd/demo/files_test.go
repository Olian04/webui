package main

import (
	"net/http"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestAPathThatTakesTheRestOfTheAddressIsAListOfSegmentsWithACrumbForEach(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/files/logs/2026/10")

	// One crumb for each segment, each a link to the folder on the way; the last is where we are.
	crumbs := page.One(t, "the breadcrumb", p.ByRole("navigation", "Breadcrumb")).Links()
	assert.DeepEqual(t, crumbs[1:], []page.Link{{Name: "Files", Href: "/admin/files"}, {Name: "logs", Href: "/admin/files/logs"}, {Name: "2026", Href: "/admin/files/logs/2026"}})
	assert.Equal(t, len(p.ByRole("heading", "10")), 1) // and the last is where we are

	// A row adds its name to the list, and the address is built from the elements.
	assert.Equal(t, len(linksTo(p, "/admin/files/logs/2026/10/collector.log")), 1)
	// The Markdown beside it links to paths in the app, which are given the mount prefix.
	article := page.One(t, "the note", mainOf(t, p).ByRole("article", ""))
	assert.True(t, hasLink(article, "2026", "/admin/files/logs/2026"))
	assert.True(t, hasLink(article, "all files", "/admin/files"))

	// Nothing at the path: the page's Guard says so, before anything is loaded.
	assert.Equal(t, get(h, "/admin/files/logs/nope").Code, http.StatusNotFound)
	// A doubled slash is not a path of this page.
	assert.Equal(t, get(h, "/admin/files/logs//2026").Code, http.StatusTemporaryRedirect) // cleaned by the mux
}
