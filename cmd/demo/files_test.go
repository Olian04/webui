package main

import (
	"net/http"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestAPathThatTakesTheRestOfTheAddressIsAListOfSegmentsWithACrumbForEach(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/files/logs/2026/10").Body.String()

	// One crumb for each segment, each a link to the folder on the way; the last is where we are.
	assert.Contains(t, body, `<a href="/admin/files">Files</a>`)
	assert.Contains(t, body, `<a href="/admin/files/logs">logs</a>`)
	assert.Contains(t, body, `<a href="/admin/files/logs/2026">2026</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">10</span>`)

	// A row adds its name to the list, and the address is built from the elements.
	assert.Contains(t, body, `href="/admin/files/logs/2026/10/collector.log"`)
	// The Markdown beside it links to paths in the app, which are given the mount prefix.
	assert.Contains(t, body, `<a href="/admin/files/logs/2026">2026</a>`)
	assert.Contains(t, body, `<a href="/admin/files">all files</a>`)

	// Nothing at the path: the page's Guard says so, before anything is loaded.
	assert.Equal(t, get(h, "/admin/files/logs/nope").Code, http.StatusNotFound)
	// A doubled slash is not a path of this page.
	assert.Equal(t, get(h, "/admin/files/logs//2026").Code, http.StatusTemporaryRedirect) // cleaned by the mux
}
