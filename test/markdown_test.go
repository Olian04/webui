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
		Load:    func(context.Context) (handbook, error) { return handbook{Text: text}, nil },
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
		webui.Markdown[handbook]{Title: "Handbook", Load: func(context.Context) (handbook, error) { return handbook{Text: "# Hello"}, nil }, Content: handbookText},
		webui.Markdown[handbook]{Title: "Other", Load: func(context.Context) (handbook, error) { return handbook{Text: "# Second"}, nil }, Content: handbookText},
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
		Load: func(context.Context) (handbook, error) { return handbook{}, errors.New("store secret-bucket is gone") },
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, "Could not load")
	assert.False(t, strings.Contains(body, "secret-bucket"))
}

func TestAMarkdownIsOnlyEverRead(t *testing.T) {
	t.Parallel()

	assert.Equal(t, post(markdownApp("# x"), "/admin/e", formValues("f0", "x")).Code, http.StatusMethodNotAllowed)

	store := webui.String[handbook]{Label: "Handbook", Load: func(h handbook) string { return h.Text }, Store: func(h *handbook, v string) { h.Text = v }}
	load := func(context.Context) (handbook, error) { return handbook{}, nil }
	assert.Contains(t, editorErrors(t, webui.Markdown[handbook]{Content: handbookText}), "a Markdown has no Load")
	assert.Contains(t, editorErrors(t, webui.Markdown[handbook]{Load: load, Content: store}), "a Markdown has a Store")
	assert.Contains(t, editorErrors(t, webui.Markdown[handbook]{Load: load}), "Markdown.Content")
}
