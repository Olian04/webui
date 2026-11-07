package webui

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/render/assets"
	"github.com/Olian04/webui/test/util/assert"
)

// An Editor and a Diff are drawn with the code editor, which a program imports to have, so that one without either does not link it: this test binary has not imported
// it, and the app is refused when it is compiled, with the import as the fix.
func TestAnEditorOrADiffWithoutTheCodeEditorIsACompileError(t *testing.T) {
	t.Parallel()

	assert.False(t, assets.EditorInstalled())
	type doc struct{ Text string }
	text := String[doc]{Label: "Text", Load: func(d doc) string { return d.Text }}
	load := func(context.Context) (doc, Loaded) { return doc{}, Ready() }
	for name, body := range map[string]View{
		"an Editor": Editor[doc]{Title: "Doc", Load: load, Content: text},
		"a Diff":    Diff[doc]{Title: "Doc", Load: load, Original: text, Modified: String[doc]{Label: "New", Load: text.Load}},
	} {
		_, err := App{Pages: Pages{Page[NoArgs]{Path: "/e", View: body}}}.Compile("/admin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), name+" is drawn with the code editor, which this program does not carry")
		assert.True(t, strings.Contains(err.Error(), `import _ "github.com/Olian04/webui/pkg/monaco"`))
	}
}

// A Markdown whose code block names a language is coloured by the editor when the program carries it, and is plain when it does not: the page links no editor that is not
// there, and the block is the monospace one that a visitor without script has, with its copy button. It is not refused, as an Editor is.
func TestAMarkdownWithACodeBlockIsPlainWhenTheProgramHasNoEditor(t *testing.T) {
	t.Parallel()

	type doc struct{ Text string }
	h, err := App{Pages: Pages{Page[NoArgs]{Path: "/m", View: Markdown[doc]{
		Title: "Notes",
		Load: func(context.Context) (doc, Loaded) {
			return doc{Text: "Run it:\n\n```go\nfmt.Println(1)\n```\n"}, Ready()
		},
		Content: String[doc]{Label: "Text", Load: func(d doc) string { return d.Text }},
	}}}}.Compile("/admin")
	assert.NoError(t, err)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/m", nil))
	body := rec.Body.String()
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, body, `<code class="language-go">`)
	assert.Contains(t, body, `data-copy-code=`)
	assert.False(t, strings.Contains(body, "/monaco/"))
	assert.False(t, strings.Contains(body, "webui-code"))
}

// An app that has a logger is told, once for each page, that a Markdown cannot be coloured and how to put it right: the program is the one who can, and the visitor is told nothing.
func TestAMarkdownThatCannotBeColouredIsSaidOnceForEachPageWhenTheAppHasALogger(t *testing.T) {
	t.Parallel()

	type doc struct{ Text string }
	var out bytes.Buffer
	page := func(path string) Page[NoArgs] {
		return Page[NoArgs]{Path: PageID[NoArgs](path), View: Markdown[doc]{
			Title:   "Notes",
			Load:    func(context.Context) (doc, Loaded) { return doc{Text: "```go\nx\n```\n"}, Ready() },
			Content: String[doc]{Label: "Text", Load: func(d doc) string { return d.Text }},
		}}
	}
	h := App{Logger: slog.New(slog.NewTextHandler(&out, nil)), Pages: Pages{page("/a"), page("/b")}}.MustCompile("/admin")
	for _, target := range []string{"/admin/a", "/admin/a", "/admin/b"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		assert.Equal(t, rec.Code, http.StatusOK)
	}
	logged := out.String()
	assert.Equal(t, strings.Count(logged, "level=WARN"), 2) // each page once, and not at each request
	assert.Contains(t, logged, "does not carry the code editor")
	assert.Contains(t, logged, `import _ \"github.com/Olian04/webui/pkg/monaco\"`)
	assert.Contains(t, logged, "page=/a")
	assert.Contains(t, logged, "page=/b")

	// An app that has no logger says nothing, and is not the worse for it.
	quiet := App{Pages: Pages{page("/a")}}.MustCompile("/admin")
	rec := httptest.NewRecorder()
	quiet.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/a", nil))
	assert.Equal(t, rec.Code, http.StatusOK)
}
