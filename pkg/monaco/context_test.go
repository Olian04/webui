package monaco_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// folder is a context with a directory for each of the three languages that take one.
func folder() fstest.MapFS {
	return fstest.MapFS{
		"typescript/tsconfig.json": {Data: []byte(`{"compilerOptions": {"target": "es2022", "strict": true, "lib": ["es2022"]}}`)},
		"typescript/env.d.ts":      {Data: []byte(`declare const host: string;`)},
		"json/schema.json":         {Data: []byte(`{"type": "object", "properties": {"n": {"type": "number"}}}`)},
	}
}

func contextEditor(fsys fstest.MapFS, language webui.Language, guard func(context.Context, doc) error, secret []byte) http.Handler {
	return webui.App{Secret: secret, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: webui.Editor[doc]{
		Title: "Doc", Language: language, Context: fsys,
		Content: webui.String[doc]{
			Label: "Text", Load: func(d doc) string { return d.Text }, Store: func(d *doc, v string) { d.Text = v },
		},
		Submit: webui.Action[doc]{
			Guard: guard,
			Run:   func(context.Context, doc) (webui.Outcome, error) { return webui.Success("Saved"), nil },
		},
		Load: func(context.Context) (doc, error) { return doc{}, nil },
	}}}}.MustCompile("/admin")
}

type pageConfig struct {
	Workers map[string]string `json:"workers"`
	Context *struct {
		Libs     map[string][]struct{ Path, URL string } `json:"libs"`
		Compiler map[string]map[string]any               `json:"compiler"`
		Schemas  map[string][]struct{ Path, URL string } `json:"schemas"`
	} `json:"context"`
}

func configOf(t *testing.T, body string) pageConfig {
	t.Helper()

	m := regexp.MustCompile(`(?s)<script id="webui-code" type="application/json">(.*?)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no editor config in the page")
	}
	var c pageConfig
	assert.NoError(t, json.Unmarshal([]byte(m[1]), &c))
	return c
}

func TestAnEditableEditorIsHandedItsContextAsAddressesThatCarryAKey(t *testing.T) {
	t.Parallel()

	h := contextEditor(folder(), webui.LangTypeScript, nil, nil)
	body := get(h, "/admin/e").Body.String()
	c := configOf(t, body)

	libs := c.Context.Libs["typescript"]
	assert.Equal(t, len(libs), 1)
	assert.Equal(t, libs[0].Path, "/env.d.ts")
	assert.Contains(t, libs[0].URL, "/admin/_webui/code/")
	assert.Contains(t, libs[0].URL, ".d.ts?k=")
	assert.Equal[any](t, c.Context.Compiler["typescript"]["target"], float64(9)) // ES2022
	assert.Equal[any](t, c.Context.Compiler["typescript"]["strict"], true)
	assert.True(t, c.Context.Schemas == nil)                      // the JSON directory is for JSON editors
	assert.False(t, strings.Contains(body, "declare const host")) // the text is not in the page

	rec := get(h, libs[0].URL)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Body.String(), "declare const host: string;")
	assert.Equal(t, rec.Header().Get("Content-Type"), "text/plain; charset=utf-8")
	// Private to the visitor's browser, and kept there: the key is the same for the life of the process.
	assert.Equal(t, rec.Header().Get("Cache-Control"), "private, max-age=31536000, immutable")
	etag := rec.Header().Get("ETag")
	req := httptest.NewRequest(http.MethodGet, libs[0].URL, nil)
	req.Header.Set("If-None-Match", etag)
	again := httptest.NewRecorder()
	h.ServeHTTP(again, req)
	assert.Equal(t, again.Code, http.StatusNotModified)
}

func TestAContextFileIsNotServedWithoutItsKey(t *testing.T) {
	t.Parallel()

	h := contextEditor(folder(), webui.LangTypeScript, nil, nil)
	url := configOf(t, get(h, "/admin/e").Body.String()).Context.Libs["typescript"][0].URL
	bare, _, _ := strings.Cut(url, "?")

	for _, target := range []string{
		bare,                               // no key
		bare + "?k=",                       // an empty one
		bare + "?k=AAAAAAAAAAAAAAAAAAAAAA", // a wrong one
		"/admin/_webui/code/nothing.d.ts?k=AAAAAAAAAAAAAAAAAAAAAA", // a name that is no file
		"/admin/_webui/code/..%2Fmonaco%2Fmanifest.json?k=x",       // a traversal
	} {
		assert.Equal(t, get(h, target).Code, http.StatusNotFound)
	}
}

func TestOneFilesKeyOpensNoOtherFile(t *testing.T) {
	t.Parallel()

	fsys := folder()
	fsys["typescript/other.d.ts"] = &fstest.MapFile{Data: []byte(`declare const other: number;`)}
	h := contextEditor(fsys, webui.LangTypeScript, nil, nil)
	libs := configOf(t, get(h, "/admin/e").Body.String()).Context.Libs["typescript"]
	assert.Equal(t, len(libs), 2)
	nameOf := func(u string) string { return strings.TrimPrefix(strings.SplitN(u, "?", 2)[0], "/admin/_webui/code/") }
	keyOf := func(u string) string { return strings.SplitN(u, "?k=", 2)[1] }
	assert.Equal(t, get(h, "/admin/_webui/code/"+nameOf(libs[0].URL)+"?k="+keyOf(libs[0].URL)).Code, http.StatusOK)
	assert.Equal(t, get(h, "/admin/_webui/code/"+nameOf(libs[0].URL)+"?k="+keyOf(libs[1].URL)).Code, http.StatusNotFound)
}

func TestAVisitorWhoMayNotEditIsHandedNoContextAndNoKey(t *testing.T) {
	t.Parallel()

	refuse := func(context.Context, doc) error { return errors.New("no") }
	h := contextEditor(folder(), webui.LangTypeScript, refuse, nil)

	body := get(h, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, "/_webui/code/")) // not even an address
	assert.False(t, strings.Contains(body, "k="))
	assert.False(t, strings.Contains(body, "data-context"))
	assert.True(t, configOf(t, body).Context == nil)
}

func TestAJSONEditorIsHandedItsSchemaAndNamesItInItsMarkup(t *testing.T) {
	t.Parallel()

	h := contextEditor(folder(), webui.LangJSON, nil, nil)
	body := get(h, "/admin/e").Body.String()
	c := configOf(t, body)
	assert.True(t, c.Context.Libs == nil)
	id := regexp.MustCompile(`data-context="([0-9a-f]+)"`).FindStringSubmatch(body)
	assert.NotNil(t, id)
	files := c.Context.Schemas[id[1]]
	assert.Equal(t, len(files), 1)
	assert.Equal(t, files[0].Path, "/schema.json")
	rec := get(h, files[0].URL)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestTwoEditorsGivenOneFolderHandOverEachFileOnce(t *testing.T) {
	t.Parallel()

	editor := webui.Editor[doc]{
		Title: "Doc", Language: webui.LangTypeScript, Context: folder(),
		Content: webui.String[doc]{Label: "Text", Load: func(d doc) string { return d.Text }, Store: func(d *doc, v string) { d.Text = v }},
		Submit:  webui.Action[doc]{Run: func(context.Context, doc) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
		Load:    func(context.Context) (doc, error) { return doc{}, nil },
	}
	second := editor
	second.Title = "Another"
	second.Content.Label = "Other text"
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", Body: webui.Stack{editor, second}}}}.MustCompile("/admin")
	assert.Equal(t, len(configOf(t, get(h, "/admin/e").Body.String()).Context.Libs["typescript"]), 1)
}

func TestTheKeysAgreeOnlyWhenTheAppsShareASecret(t *testing.T) {
	t.Parallel()

	urlOf := func(h http.Handler) string {
		return configOf(t, get(h, "/admin/e").Body.String()).Context.Libs["typescript"][0].URL
	}
	secret := []byte("0123456789abcdef0123")
	a := contextEditor(folder(), webui.LangTypeScript, nil, secret)
	b := contextEditor(folder(), webui.LangTypeScript, nil, secret)
	assert.Equal(t, urlOf(a), urlOf(b))
	assert.Equal(t, get(b, urlOf(a)).Code, http.StatusOK) // a page from one instance, a fetch from another

	c := contextEditor(folder(), webui.LangTypeScript, nil, nil)
	d := contextEditor(folder(), webui.LangTypeScript, nil, nil)
	assert.True(t, urlOf(c) != urlOf(d))
	assert.Equal(t, get(d, urlOf(c)).Code, http.StatusNotFound)
}
