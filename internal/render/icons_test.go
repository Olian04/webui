package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render/assets"
	"github.com/Olian04/webui/internal/sfnt"
	"github.com/Olian04/webui/test/util/assert"
)

// iconNames is every icon an app may name, as Compile has them.
func iconNames(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("assets", "icons.txt"))
	assert.NoError(t, err)
	var names []string
	for _, line := range strings.Split(string(raw), "\n") {
		if name, _, ok := strings.Cut(line, " "); ok {
			names = append(names, name)
		}
	}
	return names
}

// iconSheetAndFont is what an app is served for its icons: the stylesheet, and the font it names, parsed.
func iconSheetAndFont(t *testing.T, app *ir.App) (string, *sfnt.SFNT) {
	t.Helper()
	r, err := New(app, "/admin", []byte("0123456789abcdef"))
	assert.NoError(t, err)
	sheet, ok := r.Asset(r.served["fontawesome.css"])
	assert.True(t, ok)
	file, ok := r.Asset(r.served["fa-solid.woff"])
	assert.True(t, ok)
	assert.Equal(t, file.Mime, "font/woff")
	font, err := sfnt.ParseSFNT(file.Body, 0)
	assert.NoError(t, err)
	return string(sheet.Body), font
}

// An app that names every icon there is, one for each page, is the case that tests the font cut hardest: every icon has to be drawn, which is its class having the code point that
// Font Awesome gave it, and the font that is served having a glyph there. (That the glyph is the right one is checked in internal/sfnt, against the whole font, and in a browser:
// the fonts of most apps are far smaller, and this is the one that holds the 1,400 glyphs of all of them.)
func TestAnAppCanNameAnyIconAndTheFontItIsServedHasEachGlyph(t *testing.T) {
	t.Parallel()

	names := iconNames(t)
	assert.True(t, len(names) > 1500)
	app := &ir.App{}
	for i, name := range names {
		app.Pages = append(app.Pages, &ir.Page{PathTemplate: "/i" + strconv.Itoa(i), Nav: ir.Nav{Label: name, Icon: name}})
	}
	sheet, font := iconSheetAndFont(t, app)

	for _, name := range names {
		code, ok := assets.IconCode(name)
		assert.True(t, ok)
		assert.Contains(t, sheet, ".fa-"+name+`:before{content:"\`+strconv.FormatInt(int64(code), 16)+`"}`)
		if font.GlyphIndex(code) == 0 {
			t.Fatalf("the icon %q (U+%04X) has no glyph in the font that the app is served", name, code)
		}
	}
}

func TestAnAppThatNamesNoIconCarriesTheOnesTheLibraryDrawsAndNothingElse(t *testing.T) {
	t.Parallel()

	sheet, font := iconSheetAndFont(t, &ir.App{})
	core := assets.CoreIcons()
	assert.Equal(t, strings.Count(sheet, "@font-face"), 1)
	assert.Equal(t, strings.Count(sheet, ":before{"), len(core))
	codes := map[rune]bool{}
	for _, name := range core {
		code, _ := assets.IconCode(name)
		codes[code] = true
		assert.Contains(t, sheet, ".fa-"+name+":before{")
		assert.True(t, font.GlyphIndex(code) != 0)
	}
	assert.True(t, int(font.NumGlyphs()) <= len(codes)+1) // the glyphs, and the one that stands for a character the font has not got
}

// An external link's icon is the second place an app names one, and its font has the glyph too.
func TestAnExternalLinksIconIsDrawnFromTheAppsFont(t *testing.T) {
	t.Parallel()

	sheet, font := iconSheetAndFont(t, &ir.App{ExternalLinks: []ir.ExternalLink{{Label: "Docs", Icon: "book-skull", URL: "/docs"}}})
	code, ok := assets.IconCode("book-skull")
	assert.True(t, ok)
	assert.Contains(t, sheet, `.fa-book-skull:before{content:"\`+strconv.FormatInt(int64(code), 16)+`"}`)
	assert.True(t, font.GlyphIndex(code) != 0)
}

// The core is what every app has, so an icon that the library draws and that is not in it is a blank where an icon was meant, on the pages whose app did not name it. The
// library names an icon in three ways: a constant of IconName, a class in a script or in markup, and a name that an app gave, which is not the library's to list.
func TestEveryIconThatTheLibraryDrawsIsInTheCoreList(t *testing.T) {
	t.Parallel()

	known := map[string]bool{}
	for _, name := range iconNames(t) {
		known[name] = true
	}
	drawn := map[string]string{} // the icon, and where it is named
	constant := regexp.MustCompile(`IconName = "([a-z0-9-]+)"`)
	class := regexp.MustCompile(`\bfa-([a-z0-9]+(?:-[a-z0-9]+)*)\b`)
	skip := map[string]bool{"fonts": true, "min": true, "css": true, "sfnt": true}
	assert.NoError(t, filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skip[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_templ.go") || strings.HasSuffix(path, "_gen.go") || strings.HasSuffix(path, ".txt") || strings.HasSuffix(path, ".br") {
			return nil
		}
		raw, err := os.ReadFile(path) //nolint:gosec // G304: the files of this repository
		if err != nil {
			return err
		}
		for _, m := range constant.FindAllStringSubmatch(string(raw), -1) {
			drawn[m[1]] = path
		}
		for _, m := range class.FindAllStringSubmatch(string(raw), -1) {
			if known[m[1]] {
				drawn[m[1]] = path
			}
		}
		return nil
	}))
	assert.True(t, len(drawn) > 10)

	core := map[string]bool{}
	for _, name := range assets.CoreIcons() {
		core[name] = true
	}
	for name, where := range drawn {
		if !core[name] {
			t.Errorf("the icon %q is named in %s and is not in internal/render/assets/icons_core.txt: add it there", name, where)
		}
	}
	for name := range core {
		if _, ok := drawn[name]; !ok {
			t.Errorf("the icon %q is in icons_core.txt and the library draws no such icon: remove it there", name)
		}
	}
}
