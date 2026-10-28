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

// A theme that is pinned is one look, in one mode: the document says which, the stylesheet has the one block for it and no
// answer to the viewer's system, the page has no button for a choice that is not theirs, and the script that keeps their
// choice leaves the document alone.
func TestAPinnedThemeIsOneModeAndHasNoButtonForIt(t *testing.T) {
	t.Parallel()

	theme := webui.Theme{Mode: webui.ModeDark, Canvas: "#0e1224", Accent: "#4361e8"}
	h := webui.App{Theme: theme, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", Nav: webui.Nav{Label: "P"}, Body: webui.Stack{}}}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/p").Body.String()

	assert.Contains(t, page, `<html lang="en" data-theme="dark" data-theme-pinned>`)
	assert.Contains(t, page, `<meta name="color-scheme" content="dark">`)
	assert.Contains(t, page, "<style>html{background:#0e1224;color-scheme:dark}</style>") // the canvas it is drawn on, before the stylesheet
	assert.False(t, strings.Contains(page, "data-pref-toggle"))                           // no button for a choice that is not the viewer's

	css := serve(h, http.MethodGet, assetURL(t, h, "theme.css")).Body.String()
	assert.Contains(t, css, "html:root:root[data-theme='dark'] {")
	assert.Contains(t, css, "--canvas: #0e1224;")
	assert.Contains(t, css, "--blue: #4361e8;")
	for _, absent := range []string{"[data-theme='light']", "prefers-color-scheme", ":not([data-theme])"} {
		assert.False(t, strings.Contains(css, absent))
	}
	for _, token := range []string{"--panel:", "--raise:", "--input-bg:", "--text:", "--text-2:", "--text-3:", "--border:", "--border-2:", "--hover:", "--active:"} {
		assert.Contains(t, css, token)
	}

	prefs := serve(h, http.MethodGet, assetURL(t, h, "prefs.js")).Body.String()
	assert.Contains(t, prefs, "el.hasAttribute('data-theme-pinned') ? null : localStorage.getItem('webui.theme')")
}

func TestALightPinIsTheLightBlockAndTheLightEarlyBackground(t *testing.T) {
	t.Parallel()

	h := webui.App{Theme: webui.Theme{Mode: webui.ModeLight, Canvas: "#efe4d6"}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, page, `data-theme="light" data-theme-pinned`)
	assert.Contains(t, page, "<style>html{background:#efe4d6;color-scheme:light}</style>")
	assert.Contains(t, serve(h, http.MethodGet, assetURL(t, h, "theme.css")).Body.String(), "html:root:root[data-theme='light'] {")
}

// A mode with no canvas of its own is drawn on the design's own for it, and still pinned.
func TestAPinWithNoCanvasKeepsTheDesignsOwnSurfaces(t *testing.T) {
	t.Parallel()

	h := webui.App{Theme: webui.Theme{Mode: webui.ModeLight}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, page, "<style>html{background:#f4f5f5;color-scheme:light}</style>")
	assert.False(t, strings.Contains(page, "data-pref-toggle"))
}

// Left to the viewer, an app has the button, and no document attribute that would pin it.
func TestAnUnpinnedThemeLeavesTheChoiceToTheViewer(t *testing.T) {
	t.Parallel()

	h := webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, page, `<html lang="en">`)
	assert.Contains(t, page, "data-pref-toggle")
}

// Every preset is a whole look that Compile accepts, in the mode it pins, and which has the one look it says.
func TestEveryPresetCompilesAndPinsItsMode(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		theme webui.Theme
		mode  webui.Mode
	}{
		"Midnight": {webui.ThemeMidnight, webui.ModeDark},
		"Ember":    {webui.ThemeEmber, webui.ModeDark},
		"Forest":   {webui.ThemeForest, webui.ModeDark},
		"Violet":   {webui.ThemeViolet, webui.ModeDark},
		"Paper":    {webui.ThemePaper, webui.ModeLight},
		"Sand":     {webui.ThemeSand, webui.ModeLight},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.theme.Mode, tc.mode)
			assert.True(t, tc.theme.Canvas != "" && tc.theme.Accent != "")
			h, err := webui.App{Theme: tc.theme}.Compile("/admin")
			assert.NoError(t, err)
			page := serve(h, http.MethodGet, "/admin/").Body.String()
			assert.Contains(t, page, `data-theme="`+string(tc.mode)+`" data-theme-pinned`)
			assert.Contains(t, page, "<style>html{background:"+strings.ToLower(string(tc.theme.Canvas))+";color-scheme:"+string(tc.mode)+"}</style>")
		})
	}
}

// A preset is a value to start from: changing a field of a copy is a theme of its own, which is checked as any is, and does not
// change the preset.
func TestAPresetCanBeCopiedAndChanged(t *testing.T) {
	t.Parallel()

	mine := webui.ThemeForest
	mine.Accent = "#1f7a6d"
	_, err := webui.App{Theme: mine}.Compile("/admin")
	assert.NoError(t, err)
	assert.Equal(t, webui.ThemeForest.Accent, webui.Color("#2a7f5f"))

	mine.Accent = "#a7f3d0" // too light for a white label
	_, err = webui.App{Theme: mine}.Compile("/admin")
	assert.Error(t, err)
}
