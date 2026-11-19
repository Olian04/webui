package webui_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// minified is a stylesheet as the library serves it, from the folder that it is built into.
func minified(t *testing.T, name string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "internal", "render", "assets", "min", "css", name)) //nolint:gosec // G304: the name is one of the test's own literals, in the repository's asset folder
	assert.NoError(t, err)
	return string(raw)
}

// linkedTheme is the theme stylesheet that a page of the app links, found in the markup as a browser finds it, or "" when it has none.
func linkedTheme(t *testing.T, h http.Handler, prefix string) string {
	t.Helper()

	rec := serve(h, http.MethodGet, prefix+"/")
	if m := regexp.MustCompile(`href="([^"]*_webui/theme\.[0-9a-f]{8}\.css)"`).FindStringSubmatch(rec.Body.String()); m != nil {
		return m[1]
	}
	return ""
}

// A page that the app makes itself links the classless stylesheet, and its address is found without a compiled app: a file's name is a hash of its contents, which is the same in every app of this
// version of the library, so that only the mount prefix and the theme are the app's. The name is checked against a hash made here, and what is served at it against the file, and the theme's against the
// markup of a page of the app.
func TestStylesAreTheClasslessStylesheetAndTheThemeAtTheAddressesTheLibraryServes(t *testing.T) {
	t.Parallel()

	home := webui.Page[webui.NoArgs]{Path: "/", View: webui.Markdown[handbook]{Title: "Home", Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "hi"}, webui.Ready() }, Content: handbookText}}
	classless := minified(t, "classless.css")
	sum := sha256.Sum256([]byte(classless))
	name := "classless." + hex.EncodeToString(sum[:4]) + ".css"

	for _, theme := range []webui.Theme{{}, {Accent: "#1f7a6d"}, {Accent: "#1f7a6d", Light: webui.ModeColors{Canvas: "#f3f6f2"}}} {
		app := webui.App{Theme: theme, Pages: webui.Pages{home}}
		for _, prefix := range []string{"", "/ui", "/admin"} {
			h := app.MustCompile(prefix)
			got := app.Styles(prefix)
			assert.Equal(t, got[0], prefix+"/_webui/"+name)
			rec := serve(h, http.MethodGet, got[0])
			assert.Equal(t, rec.Code, http.StatusOK)
			assert.Equal(t, rec.Body.String(), classless) // what is served at the address is the file
			assert.Contains(t, rec.Header().Get("Cache-Control"), "immutable")
			if theme == (webui.Theme{}) {
				assert.Equal(t, len(got), 1) // no theme to link
				assert.Equal(t, linkedTheme(t, h, prefix), "")
			} else {
				assert.Equal(t, len(got), 2)
				assert.Equal(t, got[1], linkedTheme(t, h, prefix)) // the theme, after it, as the app's own pages link it
				assert.Equal(t, serve(h, http.MethodGet, got[1]).Code, http.StatusOK)
			}
		}
	}

	// A theme is named for its contents: another theme is another address, the same is the same.
	a := webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}
	b := webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}
	c := webui.App{Theme: webui.Theme{Accent: "#7a1f6d"}}
	assert.DeepEqual(t, a.Styles("/admin"), b.Styles("/admin"))
	assert.True(t, a.Styles("/admin")[1] != c.Styles("/admin")[1])
	assert.Equal(t, a.Styles("/admin")[0], c.Styles("/admin")[0]) // the stylesheet is the same for every theme

	// A prefix that Compile would refuse gives nothing.
	assert.Equal(t, len(webui.App{}.Styles("admin")), 0)
	assert.Equal(t, len(webui.App{}.Styles("/admin/")), 0)
}

// selectors are the selector lists of a stylesheet, as a browser reads them: the text before each opening brace, but for an at-rule.
func selectors(css string) []string {
	var out []string
	var buf strings.Builder
	for _, r := range regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "") { // a comment is not a selector
		switch r {
		case '{':
			if p := strings.TrimSpace(buf.String()); p != "" && !strings.HasPrefix(p, "@") {
				out = append(out, p)
			}
			buf.Reset()
		case '}':
			buf.Reset() // what was between the braces was declarations
		default:
			buf.WriteRune(r)
		}
	}
	return out
}

// What a page is promised is that plain markup looks like the app, and not the name of a class: the classless stylesheet has no class and no id in any selector, and so there is no name in it to
// depend on, or to keep. It is the sheet that the page gets, and the sources that it is made of.
func TestTheClasslessStylesheetHasNoClass(t *testing.T) {
	t.Parallel()

	selector := regexp.MustCompile(`[.#][A-Za-z_-]`)
	check := func(what, css string, atLeast int) {
		t.Helper()
		found := selectors(css)
		assert.True(t, len(found) >= atLeast) // it is a stylesheet, and the scan reads it
		for _, s := range found {
			if selector.MatchString(s) {
				t.Errorf("%s has a class or an id in a selector: %s", what, s)
			}
		}
	}
	check("classless.css", minified(t, "classless.css"), 40)
	for source, atLeast := range map[string]int{"tokens.css": 3, "base.css": 5, "prose.css": 20, "classless.css": 20} {
		raw, err := os.ReadFile(filepath.Join("..", "internal", "render", "assets", "css", source)) //nolint:gosec // G304: the name is one of the test's own literals, in the repository's asset folder
		assert.NoError(t, err)
		check(source, string(raw), atLeast)
	}
}

// stableTokens are the custom properties that a page may use, and that the library keeps: a name here that is no longer defined is a breaking change. Their values are the design's to change.
var stableTokens = []string{
	"--canvas", "--panel", "--raise", "--input-bg",
	"--border", "--border-2", "--control-bd",
	"--text", "--text-2", "--text-3", "--hover",
	"--blue", "--blue-text",
	"--green", "--green-bg", "--green-bd", "--orange", "--orange-bg", "--orange-bd", "--red", "--red-bg", "--red-bd",
	"--radius", "--control-h",
}

func TestTheTokensThatAPageMayUseAreDefinedInBothStylesheets(t *testing.T) {
	t.Parallel()

	for _, sheet := range []string{"classless.css", "app.css"} {
		css := minified(t, sheet)
		for _, token := range stableTokens {
			if !strings.Contains(css, token+":") {
				t.Errorf("%s does not define %s", sheet, token)
			}
		}
	}
	// The stylesheet of the app's own pages is flat: the nesting that puts prose under .md is taken out when it is built.
	assert.False(t, strings.Contains(minified(t, "app.css"), "&"))
}
