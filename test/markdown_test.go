package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type handbook struct{ Text string }

var handbookText = webui.String[handbook]{Label: "Handbook", Load: func(h handbook) string { return h.Text }}

func markdownApp(text string) http.Handler {
	return editorApp(webui.Markdown[handbook]{
		Title:   "Handbook",
		Load:    func(context.Context) (handbook, webui.Loaded) { return handbook{Text: text}, webui.Ready() },
		Content: handbookText,
	})
}

func TestAMarkdownIsShownAsMarkupInAPanelAndNeedsNoScript(t *testing.T) {
	t.Parallel()

	body := serve(markdownApp("# Runbook\n\nSome **strong** text and `code`.\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<article class="md" aria-label="Handbook">`)
	assert.Contains(t, body, "<h1>Runbook</h1>")
	assert.Contains(t, body, "<strong>strong</strong>")
	assert.Contains(t, body, "<table>")
	assert.Contains(t, body, "Handbook")              // the panel's heading
	assert.False(t, strings.Contains(body, "monaco")) // nothing to colour: the editor is not linked
}

func TestATextFromOutsideCannotPutScriptOrAPictureInThePage(t *testing.T) {
	t.Parallel()

	body := serve(markdownApp(strings.Join([]string{
		"<script>alert(1)</script>",
		"<img src=x onerror=alert(2)>",
		"[a](javascript:alert(3))",
		"![pixel](https://tracker.example/p.gif)",
		"[ok](https://example.com)",
		"</article><script>alert(4)</script>",
	}, "\n\n")), http.MethodGet, "/admin/e").Body.String()

	assert.False(t, strings.Contains(body, "<script>alert"))
	assert.False(t, strings.Contains(body, "<img"))
	assert.False(t, strings.Contains(body, "javascript:"))
	assert.Contains(t, body, `href="https://tracker.example/p.gif"`) // a link to the picture, not the picture
	assert.Contains(t, body, `<a href="https://example.com" target="_blank" rel="noopener noreferrer">ok</a>`)
	assert.Equal(t, strings.Count(body, "</article>"), 1) // the text cannot close the panel's markup
}

func TestAFencedBlockThatNamesALanguageAsksForTheEditorAndNeverForAService(t *testing.T) {
	t.Parallel()

	body := serve(markdownApp("```json\n{\"a\": 1}\n```\n"), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<code class="language-json">`)
	assert.Contains(t, body, "monaco/code-")
	assert.False(t, strings.Contains(body, `"json":"`)) // a viewer: no language service

	// Without a language, or without a fence, there is nothing to colour.
	assert.False(t, strings.Contains(serve(markdownApp("```\nplain\n```\n"), http.MethodGet, "/admin/e").Body.String(), "monaco"))
	assert.False(t, strings.Contains(serve(markdownApp("    indented\n"), http.MethodGet, "/admin/e").Body.String(), "monaco"))
}

func TestAMarkdownRefreshesAsALeafOfItsOwn(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Stack{
		webui.Markdown[handbook]{Title: "Handbook", Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "# Hello"}, webui.Ready() }, Content: handbookText},
		webui.Markdown[handbook]{Title: "Other", Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "# Second"}, webui.Ready() }, Content: handbookText},
	})
	req := httptest.NewRequest(http.MethodGet, "/admin/e", nil)
	req.Header.Set("X-Webui-Leaf", "p.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "<h1>Second</h1>")
	assert.False(t, strings.Contains(rec.Body.String(), "Hello"))
	assert.False(t, strings.Contains(rec.Body.String(), "<html"))
}

func TestAFailingMarkdownLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Markdown[handbook]{
		Title: "Handbook", Content: handbookText,
		Load: func(context.Context) (handbook, webui.Loaded) {
			return handbook{}, webui.Failed(errors.New("store secret-bucket is gone"))
		},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "secret-bucket"))
}

func TestAMarkdownIsOnlyEverRead(t *testing.T) {
	t.Parallel()

	assert.Equal(t, post(markdownApp("# x"), "/admin/e", formValues("f0", "x")).Code, http.StatusMethodNotAllowed)

	store := webui.String[handbook]{Label: "Handbook", Load: func(h handbook) string { return h.Text }, Store: func(h *handbook, v string) { h.Text = v }}
	load := func(context.Context) (handbook, webui.Loaded) { return handbook{}, webui.Ready() }
	assert.Contains(t, editorErrors(t, webui.Markdown[handbook]{Content: handbookText}), "a Markdown has no Load")
	assert.Contains(t, editorErrors(t, webui.Markdown[handbook]{Load: load, Content: store}), "a Markdown has a Store")
	assert.Contains(t, editorErrors(t, webui.Markdown[handbook]{Load: load}), "Markdown.Content")
}

func TestAPathInAMarkdownLinkStaysInsideTheAppWhereverItIsMounted(t *testing.T) {
	t.Parallel()

	text := handbook{Text: "[here](/e) and [there](/device/d1?minutes=5) and [out](//evil.example/x) and [odd](/\\evil.example)"}
	page := webui.Page[webui.NoArgs]{Path: "/e", Body: webui.Markdown[handbook]{
		Title: "Handbook", Content: handbookText,
		Load: func(context.Context) (handbook, webui.Loaded) { return text, webui.Ready() },
	}}
	for prefix, want := range map[string]string{"/admin": "/admin", "/a/b": "/a/b", "": ""} {
		h := webui.App{Pages: webui.Pages{page}}.MustCompile(prefix)
		body := serve(h, http.MethodGet, prefix+"/e").Body.String()
		assert.Contains(t, body, `<a href="`+want+`/e">here</a>`) // the same tab: no target
		assert.Contains(t, body, `<a href="`+want+`/device/d1?minutes=5">there</a>`)
		assert.False(t, strings.Contains(body, "evil.example\"")) // another host is never a link
		assert.False(t, strings.Contains(body, `href="//`))
	}
}

func markdownWithOrigins(text string, origins ...string) webui.Markdown[handbook] {
	return webui.Markdown[handbook]{
		Title: "Handbook", Content: handbookText, TrustedOrigins: origins,
		Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: text}, webui.Ready() },
	}
}

func policyOf(h http.Handler, target string) string {
	return serve(h, http.MethodGet, target).Header().Get("Content-Security-Policy")
}

const basePolicy = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

func TestAPictureFromATrustedOriginIsDrawnAndTheBrowserIsAllowedToFetchIt(t *testing.T) {
	t.Parallel()

	h := editorApp(markdownWithOrigins("![The logo](https://cdn.example.com/l.png) ![Other](https://evil.example/o.png)", "https://cdn.example.com"))
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<img src="https://cdn.example.com/l.png" alt="The logo"`)
	assert.False(t, strings.Contains(body, `src="https://evil.example`)) // not trusted: a link
	assert.Contains(t, body, `<a href="https://evil.example/o.png"`)

	// The page's policy, and only this page's, allows that origin to serve pictures.
	assert.Equal(t, policyOf(h, "/admin/e"), strings.Replace(basePolicy, "img-src 'self' data:", "img-src 'self' data: https://cdn.example.com", 1))
}

func TestAPageThatTrustsNoOriginHasTheSamePolicyAsEveryOtherPage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, policyOf(markdownApp("![x](https://cdn.example.com/l.png)"), "/admin/e"), basePolicy)
	other := webui.App{Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/with", Body: markdownWithOrigins("x", "https://cdn.example.com")},
		webui.Page[webui.NoArgs]{Path: "/without", Body: markdownWithOrigins("x")},
		webui.Page[webui.NoArgs]{Path: "/plain", Body: webui.Stack{}},
	}}.MustCompile("/admin")
	assert.True(t, policyOf(other, "/admin/with") != basePolicy)
	assert.Equal(t, policyOf(other, "/admin/without"), basePolicy)
	assert.Equal(t, policyOf(other, "/admin/plain"), basePolicy)
	assert.Equal(t, policyOf(other, "/admin/nothing-here"), basePolicy) // the 404 too
}

func TestThePolicyHasTheOriginsOfEveryMarkdownOnThePageOnceEach(t *testing.T) {
	t.Parallel()

	page := webui.Page[webui.NoArgs]{Path: "/e", Body: webui.Stack{
		markdownWithOrigins("a", "https://a.example", "https://shared.example"),
		webui.Tabs{
			{Label: "One", Body: markdownWithOrigins("b", "https://shared.example", "https://B.example:443")},
			{Label: "Two", Body: markdownWithOrigins("c", "https://c.example:8443")}, // a tab that is not selected still counts
		},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	want := strings.Replace(basePolicy, "img-src 'self' data:", "img-src 'self' data: https://a.example https://shared.example https://b.example https://c.example:8443", 1)
	assert.Equal(t, policyOf(h, "/admin/e"), want)
	assert.Equal(t, policyOf(h, "/admin/e?tabs.tab=two"), want) // the same whichever tab is shown
	// A refresh of one leaf is a fetch inside the document that has the policy.
	req := httptest.NewRequest(http.MethodGet, "/admin/e", nil)
	req.Header.Set("X-Webui-Leaf", "p.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Header().Get("Content-Security-Policy"), want)
}

func TestTheErrorPagesOfAPageHaveItsPolicyToo(t *testing.T) {
	t.Parallel()

	page := webui.Page[webui.NoArgs]{
		Path:  "/e",
		Guard: func(context.Context, webui.NoArgs) error { return errors.New("no") },
		Body:  markdownWithOrigins("x", "https://cdn.example.com"),
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "https://cdn.example.com")
}

func TestATrustedOriginMustBeAnOriginAndNothingThatCouldChangeThePolicy(t *testing.T) {
	t.Parallel()

	problems := func(origin string) string {
		return editorErrors(t, markdownWithOrigins("x", origin))
	}
	for _, bad := range []string{"", "cdn.example.com", "http://cdn.example.com", "https://cdn.example.com/img", "https://*.example.com",
		"https://cdn.example.com; script-src *", "https://cdn.example.com 'unsafe-inline'", "https://user@cdn.example.com", "https://cdn.exämple.com"} {
		assert.Contains(t, problems(bad), "is not an origin")
	}
	assert.Contains(t, problems("http://cdn.example.com"), "must start with https://")
	assert.Contains(t, problems("https://cdn.example.com/img"), "no path, query or fragment")
	assert.Contains(t, problems("https://cdn.example.com"+";"), "does not belong in an origin")
	assert.Contains(t, problems("https://cdn.example.com/"), `Markdown.TrustedOrigins[0], "https://cdn.example.com/"`)
	assert.Contains(t, problems("nope"), `Write it as "https://example.com"`)

	for _, ok := range []string{"https://cdn.example.com", "https://cdn.example.com:8443", "https://CDN.example.com"} {
		_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: markdownWithOrigins("x", ok)}}}.Compile("/admin")
		assert.NoError(t, err)
	}
}
