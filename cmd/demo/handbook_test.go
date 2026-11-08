package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
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
	p := page.Parse(t, body)
	assert.Equal(t, len(p.ByRole("article", "")), 1)
	assert.Equal(t, len(p.ByRole("heading", "Collector handbook")), 1)
	assert.True(t, len(p.ByRole("table", "")) > 0)
	assert.Contains(t, body, `<code class="language-bash">`) // the sanitiser's own output
	assert.True(t, len(p.ByAttr("rel", "noopener noreferrer")) > 0)
	assert.False(t, strings.Contains(body, "<script>alert"))
}

func TestEveryLinkInTheHandbookGoesToAPageThatExistsAndTheHandbookLinksToItself(t *testing.T) {
	h := handler(t)
	pages := []string{"/admin/handbook", "/admin/handbook/incidents", "/admin/handbook/sinks", "/admin/handbook/glossary"}
	seen := map[string]bool{}
	for _, at := range pages {
		own := map[string]bool{}
		for _, l := range page.One(t, "the handbook", pageAt(t, h, at).ByRole("article", "")).Links() {
			if !strings.HasPrefix(l.Href, "/admin") {
				continue
			}
			assert.False(t, l.NewTab) // inside the application: the same tab
			target := l.Href
			own[target] = true
			if !seen[target] {
				seen[target] = true
				if code := get(h, target).Code; code != http.StatusOK && code != http.StatusFound {
					t.Errorf("%s links to %s, which is %d", at, target, code)
				}
			}
		}
		// Each page of the handbook leads to another one, and to a page that is not in it.
		handbook, elsewhere := 0, 0
		for target := range own {
			if target != at && strings.HasPrefix(target, "/admin/handbook") {
				handbook++
			} else if !strings.HasPrefix(target, "/admin/handbook") {
				elsewhere++
			}
		}
		if handbook == 0 || elsewhere == 0 {
			t.Errorf("%s links to %d other pages of the handbook and %d pages outside it", at, handbook, elsewhere)
		}
	}
}

func TestAHandbookPageDrawsPicturesFromItsTrustedOriginsAndNoOtherPage(t *testing.T) {
	h := handler(t)
	rec := get(h, "/admin/handbook/glossary")
	p := page.Parse(t, rec.Body.String())
	assert.Equal(t, len(p.ByAttr("src", "https://raw.githubusercontent.com/Olian04/webui/main/docs/images/first-page-light.png")), 1)
	assert.Equal(t, len(p.ByAttr("src", "https://pkg.go.dev/badge/github.com/Olian04/webui/pkg/webui.svg")), 1)
	assert.Equal(t, len(p.ByAttr("src", "https://tracker.example/pixel.gif")), 0) // not trusted: a link
	assert.Equal(t, len(linksTo(p, "https://tracker.example/pixel.gif")), 1)
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "img-src 'self' data: https://raw.githubusercontent.com https://pkg.go.dev;")

	// Every other page of the demo has the library's policy, with no origin added.
	assert.Contains(t, get(h, "/admin/device").Header().Get("Content-Security-Policy"), "img-src 'self' data:;")
}
