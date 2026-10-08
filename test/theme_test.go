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
	rec := serve(h, http.MethodGet, "/admin/_webui/theme.css")
	assert.Equal(t, rec.Code, http.StatusOK)
	return rec.Body.String()
}

func TestAZeroThemeServesNoOverrides(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	assert.Equal(t, serve(h, http.MethodGet, "/admin/_webui/theme.css").Code, http.StatusNotFound)
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/").Body.String(), "theme.css"))
}

func TestTheAccentDrivesTheWholeBlueFamilyInBothModes(t *testing.T) {
	t.Parallel()

	css := themeCSS(t, webui.Theme{Accent: "#2f9e8f"})
	// Dark, the default: hover and text are lightened.
	assert.Contains(t, css, "html:root:root:not([data-theme]),\nhtml:root:root[data-theme='dark'] {")
	assert.Contains(t, css, "--blue-hover: color-mix(in srgb, #2f9e8f 88%, black);")
	assert.Contains(t, css, "--blue-text: color-mix(in srgb, #2f9e8f 62%, white);")
	// Light, chosen or by the system: deepened.
	assert.Contains(t, css, "html:root:root[data-theme='light'] {")
	assert.Contains(t, css, "@media (prefers-color-scheme: light) {\n  html:root:root:not([data-theme]) {")
	assert.Contains(t, css, "--blue-text: color-mix(in srgb, #2f9e8f 80%, black);")
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
	assert.Contains(t, css, "--red-solid: #cc2244;")                                           // the button's fill, as given
	assert.Contains(t, css, "--red-solid-hover: color-mix(in srgb, #cc2244 88%, black);")
	assert.Contains(t, css, "--red-bd: color-mix(in srgb, #cc2244 30%, transparent);")
	assert.Contains(t, css, "--green-bg:")
	assert.False(t, strings.Contains(css, "--orange")) // Warning was not set
	assert.False(t, strings.Contains(css, "--blue"))   // nor Accent
}

func TestThemeCSSIsLinkedAfterTheStylesheet(t *testing.T) {
	t.Parallel()

	page := serve(webui.App{Theme: webui.Theme{Accent: "#2f9e8f"}}.MustCompile("/admin"), http.MethodGet, "/admin/").Body.String()
	app, theme := strings.Index(page, "app.css"), strings.Index(page, "theme.css")
	assert.True(t, app >= 0 && theme > app) // later in the cascade, so it wins
}
