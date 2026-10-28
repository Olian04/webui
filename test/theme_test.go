package webui_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func themeCSS(t *testing.T, theme webui.Theme) string {
	t.Helper()

	h := webui.App{Theme: theme}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, assetURL(t, h, "theme.css"))
	assert.Equal(t, rec.Code, http.StatusOK)
	return rec.Body.String()
}

func TestAZeroThemeServesNoOverrides(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/").Body.String(), "theme.css"))
}

func TestTheAccentDrivesTheWholeBlueFamilyInBothModes(t *testing.T) {
	t.Parallel()

	css := themeCSS(t, webui.Theme{Accent: "#1f7a6d"})
	// Dark, the default: hover and text are lightened.
	assert.Contains(t, css, "html:root:root:not([data-theme]),\nhtml:root:root[data-theme='dark'] {")
	assert.Contains(t, css, "--blue-hover: color-mix(in srgb, #1f7a6d 88%, black);")
	assert.Contains(t, css, "--blue-text: color-mix(in srgb, #1f7a6d 62%, white);")
	// Light, chosen or by the system: deepened.
	assert.Contains(t, css, "html:root:root[data-theme='light'] {")
	assert.Contains(t, css, "@media (prefers-color-scheme: light) {\n  html:root:root:not([data-theme]) {")
	assert.Contains(t, css, "--blue-text: color-mix(in srgb, #1f7a6d 80%, black);")
	// And it touches nothing else.
	for _, other := range []string{"--green", "--orange", "--red", "--canvas", "--text"} {
		assert.False(t, strings.Contains(css, other))
	}
}

func TestAStatusColourDerivesItsTextBackgroundAndBorder(t *testing.T) {
	t.Parallel()

	css := themeCSS(t, webui.Theme{Critical: "#cc2244", OK: "#228855"})
	assert.Contains(t, css, "--red: color-mix(in srgb, #cc2244 60%, white);") // dark
	assert.Contains(t, css, "--red: color-mix(in srgb, #cc2244 85%, black);") // light
	assert.Contains(t, css, "--red-bg: color-mix(in srgb, #cc2244 14%, transparent);")
	assert.Contains(t, css, "--red-solid: #cc2244;") // the button's fill, as given
	assert.Contains(t, css, "--red-solid-hover: color-mix(in srgb, #cc2244 88%, black);")
	assert.Contains(t, css, "--red-bd: color-mix(in srgb, #cc2244 30%, transparent);")
	assert.Contains(t, css, "--green-bg:")
	assert.False(t, strings.Contains(css, "--orange")) // Warning was not set
	assert.False(t, strings.Contains(css, "--blue"))   // nor Accent
}

func TestThemeCSSIsLinkedAfterTheStylesheet(t *testing.T) {
	t.Parallel()

	page := serve(webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}.MustCompile("/admin"), http.MethodGet, "/admin/").Body.String()
	app, theme := strings.Index(page, "app."), strings.Index(page, "theme.")
	assert.True(t, app >= 0 && theme > app) // later in the cascade, so it wins
}

// A canvas is given for a mode, and from it that mode's surfaces, borders and text are derived. The other mode keeps the design's own, and the
// viewer still has the button and the system's choice: nothing is pinned.
func TestACanvasForOneModeDerivesTheSurfacesOfThatModeAndLeavesTheOtherAlone(t *testing.T) {
	t.Parallel()

	theme := webui.Theme{Accent: "#4361e8", Dark: webui.ModeColors{Canvas: "#0e1224"}}
	h := webui.App{Theme: theme, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", Nav: webui.Nav{Label: "P"}, Body: webui.Stack{}}}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/p").Body.String()
	css := serve(h, http.MethodGet, assetURL(t, h, "theme.css")).Body.String()

	dark := css[:strings.Index(css, "html:root:root[data-theme='light']")]
	assert.Contains(t, dark, "html:root:root:not([data-theme]),\nhtml:root:root[data-theme='dark'] {")
	assert.Contains(t, dark, "--canvas: #0e1224;")
	for _, token := range []string{"--panel:", "--raise:", "--input-bg:", "--text:", "--text-2:", "--text-3:", "--border:", "--border-2:", "--hover:", "--active:"} {
		assert.Contains(t, dark, token)
	}
	// The light block has the accent, which is shared, and none of the surfaces, which the design keeps.
	light := css[strings.Index(css, "html:root:root[data-theme='light'] {"):]
	assert.Contains(t, light, "--blue: #4361e8;")
	assert.False(t, strings.Contains(light, "--canvas:"))
	assert.False(t, strings.Contains(light, "--panel:"))

	// The canvas the page is drawn on before the stylesheet is the app's in the dark theme and the design's in the light.
	assert.Contains(t, page, "html{background:#0e1224;color-scheme:dark}")
	assert.Contains(t, page, "html[data-theme='light']{background:#f4f5f5;color-scheme:light}")

	// Nothing is pinned: the viewer has the button and a document that carries no choice of the app's.
	assert.Contains(t, page, `<html lang="en">`)
	assert.Contains(t, page, "data-pref-toggle")
	assert.Contains(t, page, `<meta name="color-scheme" content="dark light">`)
	prefs := serve(h, http.MethodGet, assetURL(t, h, "prefs.js")).Body.String()
	assert.False(t, strings.Contains(prefs, "pinned"))
}

func TestACanvasForEachModeIsDerivedForBoth(t *testing.T) {
	t.Parallel()

	css := themeCSS(t, webui.Theme{Dark: webui.ModeColors{Canvas: "#0e1224"}, Light: webui.ModeColors{Canvas: "#eef0f9"}})
	assert.Contains(t, css, "--canvas: #0e1224;")
	assert.Contains(t, css, "--canvas: #eef0f9;")
	// The system's own preference gets the light block too, for a viewer who has not chosen.
	assert.Contains(t, css, "@media (prefers-color-scheme: light) {\n  html:root:root:not([data-theme]) {")

	h := webui.App{Theme: webui.Theme{Dark: webui.ModeColors{Canvas: "#0e1224"}, Light: webui.ModeColors{Canvas: "#eef0f9"}}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, page, "html{background:#0e1224;color-scheme:dark}")
	assert.Contains(t, page, "html[data-theme='light']{background:#eef0f9;color-scheme:light}")
	assert.Contains(t, page, "@media (prefers-color-scheme:light){html:not([data-theme]){background:#eef0f9;color-scheme:light}}")
}

// A colour that a mode gives wins over the one given for both, in that mode only.
func TestAModesOwnColourWinsOverTheSharedOneInThatModeOnly(t *testing.T) {
	t.Parallel()

	css := themeCSS(t, webui.Theme{Accent: "#1f7a6d", Light: webui.ModeColors{Accent: "#4361e8"}})
	dark := css[:strings.Index(css, "html:root:root[data-theme='light']")]
	light := css[strings.Index(css, "html:root:root[data-theme='light'] {"):]
	assert.Contains(t, dark, "--blue: #1f7a6d;")
	assert.Contains(t, light, "--blue: #4361e8;")
	assert.False(t, strings.Contains(light, "--blue: #1f7a6d;"))
}

// A zero theme, and a theme that only gives the shared colours, have nothing the document or the script need to know of.
func TestASharedColourLeavesTheDocumentAsItWas(t *testing.T) {
	t.Parallel()

	page := serve(webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}.MustCompile("/admin"), http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, page, `<html lang="en">`)
	assert.Contains(t, page, "data-pref-toggle")
}

// Every preset is a whole look that Compile accepts, with a canvas for each mode, and the viewer keeps the choice between them.
func TestEveryPresetCompilesAndHasALookInEachMode(t *testing.T) {
	t.Parallel()

	for name, theme := range map[string]webui.Theme{
		"Midnight": webui.ThemeMidnight, "Ember": webui.ThemeEmber, "Forest": webui.ThemeForest,
		"Violet": webui.ThemeViolet, "Paper": webui.ThemePaper, "Sand": webui.ThemeSand,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.True(t, theme.Accent != "" && theme.Dark.Canvas != "" && theme.Light.Canvas != "")
			h, err := webui.App{Theme: theme}.Compile("/admin")
			assert.NoError(t, err)
			page := serve(h, http.MethodGet, "/admin/").Body.String()
			assert.Contains(t, page, "html{background:"+strings.ToLower(string(theme.Dark.Canvas))+";color-scheme:dark}")
			assert.Contains(t, page, "html[data-theme='light']{background:"+strings.ToLower(string(theme.Light.Canvas))+";color-scheme:light}")
			assert.Contains(t, page, "data-pref-toggle")
		})
	}
}

// A preset is a value to start from: changing a field of a copy is a theme of its own, which is checked as any is, and does not
// change the preset. The accent is shared, so changing it changes both modes.
func TestAPresetCanBeCopiedAndChanged(t *testing.T) {
	t.Parallel()

	mine := webui.ThemeForest
	mine.Accent = "#1f7a6d"
	_, err := webui.App{Theme: mine}.Compile("/admin")
	assert.NoError(t, err)
	css := themeCSS(t, mine)
	assert.Equal(t, strings.Count(css, "--blue: #1f7a6d;"), 3) // dark, light, and light again for a viewer whose system asks for it
	assert.Equal(t, webui.ThemeForest.Accent, webui.Color("#2a7f5f"))

	mine.Accent = "#a7f3d0" // too light for a white label
	_, err = webui.App{Theme: mine}.Compile("/admin")
	assert.Error(t, err)
}
