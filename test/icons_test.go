package webui_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// iconCodes is every icon an app may name, with the code point it is at, as the list that Compile checks a name against has them.
func iconCodes(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "internal", "render", "assets", "icons.txt")) //nolint:gosec // G304: a fixed file of this repository
	assert.NoError(t, err)
	codes := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if name, code, ok := strings.Cut(line, " "); ok {
			codes[name] = code
		}
	}
	return codes
}

// An app that names every icon there is, one for each page, gets a rule for each of them in its stylesheet, with the code point that Font Awesome gave it, and a font. That the
// font has every glyph is checked where the font can be read, in internal/render.
func TestAnAppCanNameAnyIconAndEachGetsItsRule(t *testing.T) {
	t.Parallel()

	codes := iconCodes(t)
	assert.True(t, len(codes) > 1500)
	pages := make(webui.Pages, 0, len(codes))
	i := 0
	for name := range codes {
		pages = append(pages, webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs]("/i" + strconv.Itoa(i)), Nav: webui.Nav{Label: name, Icon: name}, View: webui.Stack{}})
		i++
	}
	h := webui.App{Pages: pages}.MustCompile("/admin")
	sheet := serve(h, http.MethodGet, assetURL(t, h, "fontawesome.css")).Body.String()
	for name, code := range codes {
		assert.Contains(t, sheet, ".fa-"+name+`:before{content:"\`+code+`"}`)
	}
	font := serve(h, http.MethodGet, assetURL(t, h, "fa-solid.woff"))
	assert.Equal(t, font.Code, http.StatusOK)
	assert.True(t, strings.HasPrefix(font.Header().Get("Content-Type"), "font/"))
}

// The font is served under an address with a hash of its bytes in it, so two instances of one app, behind one address, have to cut the same font or an asset that one page links
// is one that the other instance has not got.
func TestTheFontOfOneAppHasTheSameAddressWheneverItIsMade(t *testing.T) {
	t.Parallel()

	app := func() http.Handler {
		return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A", Icon: "bell"}, View: webui.Stack{}}}}.MustCompile("/admin")
	}
	first, second := app(), app()
	assert.Equal(t, assetURL(t, first, "fa-solid.woff"), assetURL(t, second, "fa-solid.woff"))
	assert.Equal(t, serve(first, http.MethodGet, assetURL(t, first, "fa-solid.woff")).Body.String(), serve(second, http.MethodGet, assetURL(t, second, "fa-solid.woff")).Body.String())
}
