package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/lang"
	_ "github.com/Olian04/webui/pkg/monaco" // the editor that an Editor and a Diff are drawn with, which a program imports to have
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type snippet struct{ Text, Before string }

var (
	snippetText = webui.String[snippet]{
		Label:    "Snippet",
		Load:     func(s snippet) string { return s.Text },
		Store:    func(s *snippet, v string) { s.Text = v },
		Required: true,
		Rules:    webui.StringRules{MaxLen: 40},
	}
	snippetView = webui.String[snippet]{Label: "Snippet", Load: func(s snippet) string { return s.Text }}
	snippetOld  = webui.String[snippet]{Label: "Before", Load: func(s snippet) string { return s.Before }}
)

func loadSnippet(context.Context) (snippet, webui.Loaded) {
	return snippet{Text: "{\"a\": 1,\n\"b\": 2}\n", Before: "{\"a\": 1}\n"}, webui.Ready()
}

// editorApp has one page, with the panel as its body, and records what was saved.
func editorApp(body webui.View) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", View: body}}}.MustCompile("/admin")
}

func editorErrors(t *testing.T, body webui.View) string {
	t.Helper()

	var all string
	for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", View: body}}}) {
		all += e.Error() + "\n"
	}
	return all
}

func TestAnEditorWithAStoreIsATextBoxThatSavesLikeAForm(t *testing.T) {
	t.Parallel()

	var saved string
	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Load: loadSnippet, Content: snippetText, Language: lang.JSON,
		Submit: webui.Action[snippet]{Run: func(_ context.Context, s snippet) (webui.Outcome, error) {
			saved = s.Text
			return webui.Success("Saved"), nil
		}},
	})

	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	p := page.Parse(t, body)
	box, _ := p.Field("f0")
	assert.Equal(t, box.Type, "textarea")
	assert.True(t, strings.Contains(box.Value, `"a": 1,`)) // the text is in the box
	area := page.One(t, "the text area", p.ByAttr("name", "f0"))
	assert.Equal(t, len(p.ByAttr("data-language", "json")), 1)
	assert.Equal(t, area.Attr("maxlength"), "40")
	assert.Equal(t, len(p.ByRole("button", "Save")), 1)
	assert.Contains(t, body, `<script type="module" src="/admin/_webui/monaco/code-`) // the editor is linked

	assert.Equal(t, post(h, "/admin/e", formValues("f0", "{}")).Code, http.StatusSeeOther)
	assert.Equal(t, saved, "{}")

	// What is refused is shown back as it was typed, with the reason on the editor.
	rec := post(h, "/admin/e", formValues("f0", strings.Repeat("x", 41)))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity) // the page, with the text as it was typed
	refused := page.Parse(t, rec.Body.String())
	typed, _ := refused.Field("f0")
	assert.Equal(t, typed.Value, strings.Repeat("x", 41))
	assert.True(t, strings.Contains(refused.Root().Text(), "Must be at most 40 characters."))
	assert.True(t, len(refused.ByAttr("aria-invalid", "true")) > 0)
	assert.Equal(t, saved, "{}")
}

func TestAnEditorWithNoStoreIsAViewerAndHasNoSaveButton(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView, Language: lang.JSON})
	p := pageOf(t, h, "/admin/e")
	assert.True(t, len(p.ByAllText(`"a": 1,`)) > 0) // the text is shown
	_, editable := p.Field("f0")
	assert.False(t, editable)
	assert.Equal(t, len(p.ByRole("button", "Save")), 0)
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
	p := pageOf(t, h, "/admin/e")
	assert.True(t, len(p.ByAllText(`"a": 1,`)) > 0) // the text is shown
	_, editable := p.Field("f0")
	assert.False(t, editable)
	rec := post(h, "/admin/e", formValues("f0", "{}"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, follow(h, rec).Body.String(), "read only for you")
}

func TestAnEditorPageIsOnlyGivenTheEditorsWorkerUnlessALanguageServiceIsImported(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Load: loadSnippet, Content: snippetText, Language: lang.JSON,
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
	rec := post(fh, "/admin/e", formValues("f0", big))
	assert.Contains(t, follow(fh, rec).Body.String(), "That submission was too large.") // a page without an editor refuses it
}

func TestAFailingEditorLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Content: snippetView,
		Load: func(context.Context) (snippet, webui.Loaded) {
			return snippet{}, webui.Failed(errors.New("disk secret-path unreadable"))
		},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "secret-path"))
}

func TestAnEditorIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	run := func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil }
	refuses(t, editorErrors(t, webui.Editor[snippet]{Content: snippetView}), "Editor", "Load")
	refuses(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet, Content: snippetView, Language: "klingon"}), "\"klingon\"", "language")
	refuses(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet, Content: snippetText}), "Submit.Run")
	refuses(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet, Content: snippetView, Submit: webui.Action[snippet]{Run: run}}), "Content", "Store")
	refuses(t, editorErrors(t, webui.Editor[snippet]{Load: loadSnippet}), "Content")
}

func TestADiffShowsWhatChangedAsAUnifiedDiffWithoutScript(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Diff[snippet]{Title: "Changes", Load: loadSnippet, Original: snippetOld, Modified: snippetView, Language: lang.JSON})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	p := page.Parse(t, body)
	assert.Equal(t, len(p.ByAttr("data-code-diff", "")), 1) // what script colours
	assert.Equal(t, len(p.ByAttr("data-side", "original")), 1)
	assert.Equal(t, len(p.ByAttr("data-side", "modified")), 1)
	assert.Contains(t, body, "monaco/code-")           // a diff is highlighted by the editor
	assert.False(t, strings.Contains(body, `"json":`)) // and never given a language service

	still := webui.String[snippet]{Label: "Same", Load: func(s snippet) string { return s.Text }}
	same := editorApp(webui.Diff[snippet]{Title: "Changes", Load: loadSnippet, Original: still, Modified: snippetView})
	assert.Equal(t, len(pageOf(t, same, "/admin/e").ByText("No differences")), 1)
}

func TestADiffIsOnlyEverRead(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Diff[snippet]{Title: "Changes", Load: loadSnippet, Original: snippetOld, Modified: snippetView})
	assert.Equal(t, post(h, "/admin/e", formValues("f0", "x")).Code, http.StatusMethodNotAllowed) // nothing on the page takes a post

	refuses(t, editorErrors(t, webui.Diff[snippet]{Original: snippetOld, Modified: snippetView}), "Diff", "Load")
	refuses(t, editorErrors(t, webui.Diff[snippet]{Load: loadSnippet, Original: snippetText, Modified: snippetOld}), "Diff", "Store")
	refuses(t, editorErrors(t, webui.Diff[snippet]{Load: loadSnippet, Original: snippetView, Modified: snippetView}), "Label")
	refuses(t, editorErrors(t, webui.Diff[snippet]{Load: loadSnippet, Original: snippetOld, Modified: snippetView, Language: "klingon"}), "language")
}

func TestAFailingDiffLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Diff[snippet]{
		Title: "Changes", Original: snippetOld, Modified: snippetView,
		Load: func(context.Context) (snippet, webui.Loaded) {
			return snippet{}, webui.Failed(errors.New("db secret-host is down"))
		},
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

// The editor's icon font is a TrueType file, which is not compressed inside, and half of it is gone when it is.
func TestTheEditorsIconFontIsGzippedForAClientThatAcceptsIt(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView}) // the font is linked by the stylesheet, which the editor page links
	fonts, err := filepath.Glob(filepath.Join("..", "pkg", "monaco", "editor", "assets", "codicon-*.ttf"))
	assert.NoError(t, err)
	assert.Equal(t, len(fonts), 1)
	font := []string{"", "/admin/_webui/monaco/assets/" + filepath.Base(fonts[0])}

	plain := serve(h, http.MethodGet, font[1])
	assert.Equal(t, plain.Code, http.StatusOK)
	assert.Equal(t, plain.Header().Get("Content-Type"), "font/ttf")
	assert.Equal(t, plain.Header().Get("Content-Encoding"), "")

	zipped := gzipped(t, h, font[1])
	assert.Equal(t, zipped.Header().Get("Content-Encoding"), "gzip")
	assert.True(t, zipped.Body.Len() < plain.Body.Len()*6/10)
	assert.Equal(t, unzip(t, zipped.Body.Bytes()), plain.Body.String())
}

// An editor that Monaco mounts over is used from the keyboard, so what the reader needs of the textarea that the server drew (which is
// hidden then) has to reach the control that Monaco makes, and the page has to say how to leave it.
func TestARequiredEditorSaysWhatItNeedsInTheWordsOfTheServer(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{
		Title: "Config", Load: loadSnippet, Content: snippetText, Language: lang.JSON,
		Submit: webui.Action[snippet]{Run: func(context.Context, snippet) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	})
	p := pageOf(t, h, "/admin/e")
	box, _ := p.Field("f0")
	assert.True(t, box.Required)
	assert.True(t, page.One(t, "the text area", p.ByAttr("name", "f0")).Has("data-msg-value-missing")) // for script to say when the textarea is hidden and cannot
}

func TestTheEditorScriptLeavesTheEditorWithEscapeAndSaysHowAndSpeaksForTheControlItMakes(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	entry := regexp.MustCompile(`src="(/admin/_webui/monaco/code-[A-Z0-9]+\.js)"`).FindStringSubmatch(body)
	assert.True(t, entry != nil)
	js := serve(h, http.MethodGet, entry[1]).Body.String()

	for _, token := range []string{
		"To leave the editor, press Esc and then Tab.", // how to leave it, in the page and not only in the manual
		"tabFocusMode",         // and that it is done
		"hc-black", "hc-light", // the high contrast themes, for forced colours and more contrast
		"native-edit-context",    // the control Monaco makes where the browser has an EditContext
		"data-confirm-open",      // Ctrl+S presses the button, so a button that asks, asks
		"data-msg-value-missing", // a required editor says what is missing
	} {
		assert.Contains(t, js, token)
	}
	assert.False(t, strings.Contains(js, ".requestSubmit()")) // a form is never sent from a shortcut, past its button
}

// discard is a response that keeps nothing of its body, so that what a request costs is the server's and not a recorder's.
type discard struct {
	header http.Header
	sent   int
}

func (d *discard) Header() http.Header         { return d.header }
func (d *discard) Write(b []byte) (int, error) { d.sent += len(b); return len(b), nil }
func (d *discard) WriteHeader(int)             {}

// The editor's file is the largest thing the app serves, and compressing it again for each request that accepts gzip cost over a hundred milliseconds each: it is
// compressed once, and a request after the first costs the server little however large the file is. It is not run in parallel: what it measures is memory, which
// anything else that runs would add to.
func TestAnEditorFileIsCompressedOnceHoweverManyRequestsAskForIt(t *testing.T) {
	h := editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView})
	worker := regexp.MustCompile(`"editor":"([^"]+)"`).FindStringSubmatch(serve(h, http.MethodGet, "/admin/e").Body.String()) // the largest of what it serves
	assert.True(t, worker != nil)

	first := gzipped(t, h, worker[1])
	assert.Equal(t, first.Header().Get("Content-Encoding"), "gzip")
	size := uint64(first.Body.Len()) //nolint:gosec // a length is not negative
	assert.True(t, size > 1<<16)     // large enough for compressing it again to show

	const requests = 30
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range requests {
		req := httptest.NewRequest(http.MethodGet, worker[1], nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := &discard{header: http.Header{}}
		h.ServeHTTP(w, req)
		assert.Equal(t, uint64(w.sent), size) //nolint:gosec // a length is not negative: the same bytes each time
	}
	runtime.ReadMemStats(&after)
	// Compressing for each request costs at least the compressed file for each; once, a small part of it.
	assert.True(t, (after.TotalAlloc-before.TotalAlloc)/requests < size/4)
}
