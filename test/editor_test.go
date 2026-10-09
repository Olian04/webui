package webui_test

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type snippet struct{ Text, Before string }

var (
	snippetText = webui.String[snippet]{
		Label: "Snippet",
		Load:  func(s snippet) string { return s.Text },
		Store: func(s *snippet, v string) { s.Text = v },
		Rules: webui.StringRules{Required: true, MaxLen: 40},
	}
	snippetView = webui.String[snippet]{Label: "Snippet", Load: func(s snippet) string { return s.Text }}
	snippetOld  = webui.String[snippet]{Label: "Before", Load: func(s snippet) string { return s.Before }}
)

func loadSnippet(context.Context) (snippet, error) {
	return snippet{Text: "{\"a\": 1,\n\"b\": 2}\n", Before: "{\"a\": 1}\n"}, nil
}

// editorApp has one page, with the leaf as its body, and records what was saved.
func editorApp(body webui.PageBody) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: body}}}.MustCompile("/admin")
}

func editorErrors(t *testing.T, body webui.PageBody) string {
	t.Helper()

	var all string
	for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: body}}}) {
		all += e.Error() + "\n"
	}
	return all
}

func TestAnEditorWithAStoreIsATextBoxThatSavesLikeAForm(t *testing.T) {
	t.Parallel()

	var saved string
	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Load: loadSnippet, Content: snippetText, Language: webui.LangJSON,
		Submit: webui.Action[snippet]{Run: func(_ context.Context, s snippet) (webui.Outcome, error) {
			saved = s.Text
			return webui.Success("Saved"), nil
		}},
	})

	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<textarea class="code-text"`)
	assert.Contains(t, body, `data-language="json"`)
	assert.Contains(t, body, `maxlength="40"`)
	assert.Contains(t, body, ">Save<")
	assert.Contains(t, body, `<script type="module" src="/admin/_webui/monaco/code-`) // the editor is linked
	assert.Contains(t, body, "&#34;a&#34;: 1,")                                       // the text, escaped, is in the box

	assert.Equal(t, post(h, "/admin/e", formValues("f0", "{}")).Code, http.StatusSeeOther)
	assert.Equal(t, saved, "{}")

	// What is refused is shown back as it was typed, with the reason on the editor.
	rec := post(h, "/admin/e", formValues("f0", strings.Repeat("x", 41)))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), strings.Repeat("x", 41))
	assert.Contains(t, rec.Body.String(), `class="err code-error"`)
	assert.Equal(t, saved, "{}")
}

func TestAnEditorWithNoStoreIsAViewerAndHasNoSaveButton(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView, Language: webui.LangJSON})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<pre class="code-view"`)
	assert.False(t, strings.Contains(body, "<textarea"))
	assert.False(t, strings.Contains(body, ">Save<"))
}

func TestAnEditorWhoseGuardRefusesTheVisitorIsAViewerForThem(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Load: loadSnippet, Content: snippetText,
		Submit: webui.Action[snippet]{
			Guard: func(context.Context, snippet) error { return errors.New("read only for you") },
			Run:   func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil },
		},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<pre class="code-view"`)
	assert.False(t, strings.Contains(body, "<textarea"))
	assert.Equal(t, post(h, "/admin/e", formValues("f0", "{}")).Code, http.StatusForbidden)
}

func TestAnEditorPageIsOnlyGivenTheEditorsWorkerUnlessALanguageServiceIsImported(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Load: loadSnippet, Content: snippetText, Language: webui.LangJSON,
		Submit: webui.Action[snippet]{Run: func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	m := regexp.MustCompile(`<script id="webui-code" type="application/json">([^<]*)</script>`).FindStringSubmatch(body)
	assert.True(t, m != nil)
	assert.Contains(t, m[1], `"editor":"/admin/_webui/monaco/workers/editor.worker-`)
	assert.False(t, strings.Contains(m[1], `"json"`)) // this binary has not imported the json service
}

func TestAPageWithNoEditorOrDiffDoesNotLinkTheEditor(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Form[snippet]{Load: loadSnippet, Fields: []webui.Accessor[snippet]{snippetView}})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, "monaco"))
	assert.False(t, strings.Contains(body, "webui-code"))
}

func TestAnEditorPageTakesALargeBodyAndOtherPagesDoNot(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", 2<<20)
	eh := editorApp(webui.Editor[snippet]{
		Title: "Config", Load: loadSnippet, Content: webui.String[snippet]{
			Label: "Snippet", Load: func(s snippet) string { return s.Text }, Store: func(s *snippet, v string) { s.Text = v },
		},
		Submit: webui.Action[snippet]{Run: func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	})
	assert.Equal(t, post(eh, "/admin/e", formValues("f0", big)).Code, http.StatusSeeOther)

	fh := editorApp(webui.Form[snippet]{
		Load: loadSnippet, Fields: []webui.Accessor[snippet]{snippetText},
		Submit: webui.Action[snippet]{Run: func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	})
	assert.True(t, post(fh, "/admin/e", formValues("f0", big)).Code != http.StatusSeeOther)
}

func TestAFailingEditorLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Content: snippetView,
		Load: func(context.Context) (snippet, error) { return snippet{}, errors.New("disk secret-path unreadable") },
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "secret-path"))
}

func TestAnEditorIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	run := func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil }
	assert.Contains(t, editorErrors(t, webui.Editor[snippet]{Content: snippetView}), "an Editor has no Load")
	assert.Contains(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet, Content: snippetView, Language: "klingon"}), `"klingon" is not a language`)
	assert.Contains(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet, Content: snippetText}), "has no Submit.Run")
	assert.Contains(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet, Content: snippetView, Submit: webui.Action[snippet]{Run: run}}), "Content has no Store")
	assert.Contains(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet}), "Content")
}

func TestADiffShowsWhatChangedAsAUnifiedDiffWithoutScript(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Diff[snippet]{Title: "Changes", Load: loadSnippet, Original: snippetOld, Modified: snippetView, Language: webui.LangJSON})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "data-code-diff")
	assert.Contains(t, body, `class="udiff"`)
	assert.Contains(t, body, `data-side="original"`)
	assert.Contains(t, body, `data-side="modified"`)
	assert.Contains(t, body, "monaco/code-")           // a diff is highlighted by the editor
	assert.False(t, strings.Contains(body, `"json":`)) // and never given a language service

	still := webui.String[snippet]{Label: "Same", Load: func(s snippet) string { return s.Text }}
	same := editorApp(webui.Diff[snippet]{Title: "Changes", Load: loadSnippet, Original: still, Modified: snippetView})
	assert.Contains(t, serve(same, http.MethodGet, "/admin/e").Body.String(), "No differences")
}

func TestADiffIsOnlyEverRead(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Diff[snippet]{Title: "Changes", Load: loadSnippet, Original: snippetOld, Modified: snippetView})
	assert.Equal(t, post(h, "/admin/e", formValues("f0", "x")).Code, http.StatusMethodNotAllowed) // nothing on the page takes a post

	assert.Contains(t, editorErrors(t, webui.Diff[snippet]{Original: snippetOld, Modified: snippetView}), "a Diff has no Load")
	assert.Contains(t, editorErrors(t, webui.Diff[snippet]{Load: loadSnippet, Original: snippetText, Modified: snippetOld}), "a Diff has a Store")
	assert.Contains(t, editorErrors(t, webui.Diff[snippet]{Load: loadSnippet, Original: snippetView, Modified: snippetView}), "Label")
	assert.Contains(t, editorErrors(t, webui.Diff[snippet]{Load: loadSnippet, Original: snippetOld, Modified: snippetView, Language: "klingon"}), "is not a language")
}

func TestAFailingDiffLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Diff[snippet]{
		Title: "Changes", Original: snippetOld, Modified: snippetView,
		Load: func(context.Context) (snippet, error) { return snippet{}, errors.New("db secret-host is down") },
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "secret-host"))
}

func TestTheEditorAssetsAreImmutableGzippedAndNothingElseIsServed(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	entry := regexp.MustCompile(`src="(/admin/_webui/monaco/code-[A-Z0-9]+\.js)"`).FindStringSubmatch(body)
	assert.True(t, entry != nil)

	plain := serve(h, http.MethodGet, entry[1])
	assert.Equal(t, plain.Code, http.StatusOK)
	assert.Equal(t, plain.Header().Get("Cache-Control"), "public, max-age=31536000, immutable")
	assert.Contains(t, plain.Header().Get("Content-Type"), "javascript")
	assert.Equal(t, plain.Header().Get("X-Content-Type-Options"), "nosniff")

	zipped := gzipped(t, h, entry[1])
	assert.Equal(t, zipped.Header().Get("Content-Encoding"), "gzip")
	assert.Equal(t, unzip(t, zipped.Body.Bytes()), plain.Body.String())

	// The editor's worker is served from the same place, since a worker must be same-origin.
	cfg := regexp.MustCompile(`"editor":"([^"]+)"`).FindStringSubmatch(body)
	assert.True(t, cfg != nil)
	assert.Equal(t, serve(h, http.MethodGet, cfg[1]).Code, http.StatusOK)

	for _, p := range []string{"manifest.json", "workers/nope.js", "nope.js", "workers/manifest.json"} {
		assert.Equal(t, serve(h, http.MethodGet, "/admin/_webui/monaco/"+p).Code, http.StatusNotFound)
	}
	for _, p := range []string{"../app.css", "workers/../manifest.json", "%2e%2e/manifest.json", "..%2fmanifest.json"} {
		assert.True(t, serve(h, http.MethodGet, "/admin/_webui/monaco/"+p).Code != http.StatusOK) // never a file outside
	}
}
