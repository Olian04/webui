package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// linked is the address a page links an asset at, with the hash of its contents in it.
func linked(t *testing.T, h http.Handler, logical string) string {
	t.Helper()

	dot := strings.LastIndex(logical, ".")
	base, ext := logical, ""
	if dot > 0 {
		base, ext = logical[:dot], logical[dot:]
	}
	m := regexp.MustCompile(`(/[^"' ]*_webui/` + regexp.QuoteMeta(base) + `\.[0-9a-f]{8}` + regexp.QuoteMeta(ext) + `)["' >]`).
		FindStringSubmatch(get(h, "/admin/").Body.String())
	if m == nil {
		return ""
	}
	return m[1]
}

func TestTheHandbookIsMarkdownSetAsMarkupThatCannotRunAnything(t *testing.T) {
	body := get(handler(t), "/admin/handbook").Body.String()
	assert.Contains(t, body, `<article class="md"`)
	assert.Contains(t, body, "<h1>Collector handbook</h1>")
	assert.Contains(t, body, "<table>")
	assert.Contains(t, body, `<code class="language-bash">`)
	assert.Contains(t, body, `rel="noopener noreferrer"`)
	assert.False(t, strings.Contains(body, "<script>alert"))
}

func TestEveryLinkInTheHandbookGoesToAPageThatExistsAndTheHandbookLinksToItself(t *testing.T) {
	h := handler(t)
	pages := []string{"/admin/handbook", "/admin/handbook/incidents", "/admin/handbook/sinks", "/admin/handbook/glossary"}
	link := regexp.MustCompile(`<a href="(/admin[^"]*)"( target=[^>]*)?>`)
	seen := map[string]bool{}
	for _, page := range pages {
		own := map[string]bool{}
		for _, m := range link.FindAllStringSubmatch(get(h, page).Body.String(), -1) {
			assert.Equal(t, m[2], "") // inside the application: the same tab
			target := strings.ReplaceAll(m[1], "&amp;", "&")
			own[target] = true
			if !seen[target] {
				seen[target] = true
				if code := get(h, target).Code; code != http.StatusOK && code != http.StatusFound {
					t.Errorf("%s links to %s, which is %d", page, target, code)
				}
			}
		}
		// Each page of the handbook leads to another one, and to a page that is not in it.
		handbook, elsewhere := 0, 0
		for target := range own {
			if target != page && strings.HasPrefix(target, "/admin/handbook") {
				handbook++
			} else if !strings.HasPrefix(target, "/admin/handbook") {
				elsewhere++
			}
		}
		if handbook == 0 || elsewhere == 0 {
			t.Errorf("%s links to %d other pages of the handbook and %d pages outside it", page, handbook, elsewhere)
		}
	}
}

func TestAHandbookPageDrawsPicturesFromItsTrustedOriginsAndNoOtherPage(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/handbook/glossary")
	body := rec.Body.String()
	assert.Contains(t, body, `<img src="https://raw.githubusercontent.com/Olian04/webui/main/docs/images/first-page-light.png"`)
	assert.Contains(t, body, `<img src="https://pkg.go.dev/badge/github.com/Olian04/webui/pkg/webui.svg"`)
	assert.False(t, strings.Contains(body, `src="https://tracker.example`)) // not trusted: a link
	assert.Contains(t, body, `<a href="https://tracker.example/pixel.gif"`)
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "img-src 'self' data: https://raw.githubusercontent.com https://pkg.go.dev;")

	// Every other page of the demo has the library's policy, with no origin added.
	assert.Contains(t, get(h, "/admin/device").Header().Get("Content-Security-Policy"), "img-src 'self' data:;")
}
