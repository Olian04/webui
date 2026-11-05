package webui_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/lang"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// The script that sets the stored theme has to run before the first paint, and a file is a request that the first paint waits for: it is in the page, in front of the
// stylesheets (so that it runs as it is read and does not wait for them), and the page's policy allows it by its hash and allows no other inline script.
func TestTheThemeScriptIsInTheHeadInFrontOfTheStylesheetsAndAllowedByItsHash(t *testing.T) {
	t.Parallel()

	h := navApp()
	script := "<script>" + string(prefsScript()) + "</script>"
	for _, path := range []string{"/admin/device", "/admin/nowhere", "/admin/_webui/"} { // a page, the page for an address that is not one, and one in the asset directory
		rec := serve(h, http.MethodGet, path)
		body := rec.Body.String()
		assert.Equal(t, strings.Count(body, "<script"), 2) // this one, and the page script, which is a file
		assert.True(t, strings.Contains(body, script))
		assert.True(t, strings.Index(body, script) < strings.Index(body, `<link rel="stylesheet"`))
		assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "script-src 'self' "+prefsHash()+";")
		assert.False(t, strings.Contains(body, "/_webui/prefs")) // no file of it is linked
	}
}

func TestTheScriptThatSetsTheThemeIsNotServedAsAFile(t *testing.T) {
	t.Parallel()

	h := navApp()
	rec := serve(h, http.MethodGet, "/admin/")
	assert.False(t, strings.Contains(rec.Body.String(), "/_webui/prefs"))
}

// The icon font is found by reading the icon stylesheet, which a page has after its other stylesheet: asked for as the head is read, it is on its way before that.
func TestTheIconFontIsAskedForAsTheHeadIsRead(t *testing.T) {
	t.Parallel()

	h := navApp()
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `<link rel="preload" href="`+assetURL(t, h, "fa-solid.woff")+`" as="font" type="font/woff" crossorigin>`)
	assert.Equal(t, strings.Count(body, `rel="preload"`), 1)
	assert.False(t, strings.Contains(body, "modulepreload")) // a page with no editor has no editor to ask for
}

// An editor is a chain of files, each found by reading the one before: the entry names chunks and they name others, and the stylesheet names the icon font. The page asks
// for all of them as the head is read. The list is what the build wrote, from what the entry imports. The modules are asked for at low priority: they are a megabyte, and a
// page that asked for them at the priority of its stylesheets drew its first paint a second later, since a plain textarea is what is drawn first and the editor replaces it.
func TestAPageWithAnEditorAsksForItsModulesAndFontAsTheHeadIsRead(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "pkg", "monaco", "editor", "manifest.json")) //nolint:gosec // G304: a fixed file of this repository
	assert.NoError(t, err)
	var manifest struct {
		Entry   string   `json:"entry"`
		Preload []string `json:"preload"`
		Fonts   []string `json:"fonts"`
	}
	assert.NoError(t, json.Unmarshal(raw, &manifest))
	assert.True(t, len(manifest.Preload) >= 3)
	assert.Equal(t, len(manifest.Fonts), 1)

	body := serve(editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView, Language: lang.JSON}), http.MethodGet, "/admin/e").Body.String()
	for _, module := range manifest.Preload {
		assert.Contains(t, body, `<link rel="modulepreload" href="/admin/_webui/monaco/`+module+`" fetchpriority="low">`)
	}
	assert.Contains(t, body, `<link rel="preload" href="/admin/_webui/monaco/`+manifest.Fonts[0]+`" as="font" type="font/ttf" crossorigin>`)
	assert.Contains(t, body, `<script type="module" src="/admin/_webui/monaco/`+manifest.Entry+`"></script>`)
}
