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
	assert.Contains(t, css, "--accent-hover-color: color-mix(in srgb, #1f7a6d 88%, black);")
	assert.Contains(t, css, "--accent-text-color: color-mix(in srgb, #1f7a6d 62%, white);")
	// Light, chosen or by the system: deepened.
	assert.Contains(t, css, "html:root:root[data-theme='light'] {")
	assert.Contains(t, css, "@media (prefers-color-scheme: light) {\n  html:root:root:not([data-theme]) {")
	assert.Contains(t, css, "--accent-text-color: color-mix(in srgb, #1f7a6d 80%, black);")
	// And it touches nothing else.
	for _, other := range []string{"--success-color", "--warning-color", "--danger-color", "--page-color", "--text-color"} {
		assert.False(t, strings.Contains(css, other))
	}
}

func TestAStatusColourDerivesItsTextBackgroundAndBorder(t *testing.T) {
	t.Parallel()

	css := themeCSS(t, webui.Theme{Critical: "#cc2244", OK: "#228855"})
	assert.Contains(t, css, "--danger-color: color-mix(in srgb, #cc2244 60%, white);") // dark
	assert.Contains(t, css, "--danger-color: color-mix(in srgb, #cc2244 85%, black);") // light
	assert.Contains(t, css, "--danger-bg-color: color-mix(in srgb, #cc2244 14%, transparent);")
	assert.Contains(t, css, "--danger-solid-color: #cc2244;") // the button's fill, as given
	assert.Contains(t, css, "--danger-solid-hover-color: color-mix(in srgb, #cc2244 88%, black);")
	assert.Contains(t, css, "--danger-border-color: color-mix(in srgb, #cc2244 30%, transparent);")
	assert.Contains(t, css, "--success-bg-color:")
	assert.False(t, strings.Contains(css, "--warning-color")) // Warning was not set
	assert.False(t, strings.Contains(css, "--accent-color"))  // nor Accent
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
	h := webui.App{Theme: theme, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", View: webui.Stack{}}}, Navigation: []webui.NavItem{webui.NavPage{Page: webui.PageID[webui.NoArgs]("/p"), Label: "P"}}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/p").Body.String()
	css := serve(h, http.MethodGet, assetURL(t, h, "theme.css")).Body.String()

	dark := css[:strings.Index(css, "html:root:root[data-theme='light']")]
	assert.Contains(t, dark, "html:root:root:not([data-theme]),\nhtml:root:root[data-theme='dark'] {")
	assert.Contains(t, dark, "--page-color: #0e1224;")
	for _, token := range []string{"--surface-color:", "--raised-color:", "--field-color:", "--text-color:", "--text-muted-color:", "--text-faint-color:", "--border-color:", "--border-strong-color:", "--hover-color:", "--active-color:"} {
		assert.Contains(t, dark, token)
	}
	// The light block has the accent, which is shared, and none of the surfaces, which the design keeps.
	light := css[strings.Index(css, "html:root:root[data-theme='light'] {"):]
	assert.Contains(t, light, "--accent-color: #4361e8;")
	assert.False(t, strings.Contains(light, "--page-color:"))
	assert.False(t, strings.Contains(light, "--surface-color:"))

	// The canvas the page is drawn on before the stylesheet is the app's in the dark theme and the design's in the light.
	assert.Contains(t, page, "html{background:#0e1224;color-scheme:dark}")
	assert.Contains(t, page, "html[data-theme='light']{background:#f4f5f5;color-scheme:light}")

	// Nothing is pinned: the viewer has the button and a document that carries no choice of the app's.
	assert.Contains(t, page, `<html lang="en">`)
	assert.Contains(t, page, "data-pref-toggle")
	assert.Contains(t, page, `<meta name="color-scheme" content="dark light">`)
}

func TestACanvasForEachModeIsDerivedForBoth(t *testing.T) {
	t.Parallel()

	css := themeCSS(t, webui.Theme{Dark: webui.ModeColors{Canvas: "#0e1224"}, Light: webui.ModeColors{Canvas: "#eef0f9"}})
	assert.Contains(t, css, "--page-color: #0e1224;")
	assert.Contains(t, css, "--page-color: #eef0f9;")
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
	assert.Contains(t, dark, "--accent-color: #1f7a6d;")
	assert.Contains(t, light, "--accent-color: #4361e8;")
	assert.False(t, strings.Contains(light, "--accent-color: #1f7a6d;"))
}

// A zero theme, and a theme that only gives the shared colours, have nothing the document or the script need to know of.
func TestASharedColourLeavesTheDocumentAsItWas(t *testing.T) {
	t.Parallel()

	page := serve(webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}.MustCompile("/admin"), http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, page, `<html lang="en">`)
	assert.Contains(t, page, "data-pref-toggle")
}
