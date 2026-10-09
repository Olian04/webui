package json_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	_ "github.com/Olian04/webui/pkg/monaco/json"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type doc struct{ Text string }

func app(language webui.Language, editable bool) http.Handler {
	text := webui.String[doc]{Label: "Text", Load: func(d doc) string { return d.Text }}
	var submit webui.Action[doc]
	if editable {
		text.Store = func(d *doc, v string) { d.Text = v }
		submit.Run = func(context.Context, doc) (webui.Outcome, error) { return webui.Success("Saved"), nil }
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: webui.Editor[doc]{
		Title: "Doc", Language: language, Content: text, Submit: submit,
		Load: func(context.Context) (doc, error) { return doc{Text: "{}"}, nil },
	}}}}.MustCompile("/admin")
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

var workerOf = regexp.MustCompile(`"json":"([^"]+)"`)

func TestTheJSONServiceIsHandedToAnEditableJSONEditorAndServedFromTheApp(t *testing.T) {
	t.Parallel()

	h := app(webui.LangJSON, true)
	m := workerOf.FindStringSubmatch(get(h, "/admin/e").Body.String())
	assert.True(t, m != nil)
	rec := get(h, m[1])
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Header().Get("Cache-Control"), "public, max-age=31536000, immutable")
	assert.True(t, rec.Body.Len() > 1000)
}

func TestTheJSONServiceIsNotHandedToAViewerOrToAnotherLanguage(t *testing.T) {
	t.Parallel()

	assert.True(t, workerOf.FindStringSubmatch(get(app(webui.LangJSON, false), "/admin/e").Body.String()) == nil)
	assert.True(t, workerOf.FindStringSubmatch(get(app(webui.LangGo, true), "/admin/e").Body.String()) == nil)
	assert.True(t, workerOf.FindStringSubmatch(get(app(webui.LangPlainText, true), "/admin/e").Body.String()) == nil)
}
