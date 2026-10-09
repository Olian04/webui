package webui_test

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// rgba is a colour as the stylesheet writes one, over nothing yet.
type rgba struct{ r, g, b, a float64 }

func parseColour(t *testing.T, s string) rgba {
	t.Helper()

	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		n, err := strconv.ParseUint(s[1:], 16, 32)
		assert.NoError(t, err)
		return rgba{float64(n >> 16 & 255), float64(n >> 8 & 255), float64(n & 255), 1}
	}
	m := regexp.MustCompile(`^rgba?\(([^)]*)\)$`).FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("not a colour the test understands: %q", s)
	}
	var v [4]float64
	v[3] = 1
	for i, part := range strings.FieldsFunc(m[1], func(r rune) bool { return r == ',' || r == ' ' || r == '/' }) {
		x, err := strconv.ParseFloat(part, 64)
		assert.NoError(t, err)
		v[i] = x
	}
	return rgba{v[0], v[1], v[2], v[3]}
}

// over is c laid on an opaque background.
func (c rgba) over(bg rgba) rgba {
	return rgba{c.r*c.a + bg.r*(1-c.a), c.g*c.a + bg.g*(1-c.a), c.b*c.a + bg.b*(1-c.a), 1}
}

func (c rgba) luminance() float64 {
	lin := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

// contrast is the WCAG ratio of two opaque colours.
func contrast(a, b rgba) float64 {
	la, lb := a.luminance(), b.luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// tokens reads the custom properties of the first rule with this selector.
func tokens(t *testing.T, css, selector string) map[string]string {
	t.Helper()

	i := strings.Index(css, "\n"+selector)
	if i < 0 {
		t.Fatalf("no rule %q", selector)
	}
	body := css[i:]
	body = body[strings.Index(body, "{")+1 : strings.Index(body, "}")]
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`--([\w-]+):\s*([^;]+);`).FindAllStringSubmatch(body, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// The text a reader needs must be legible on the surfaces it sits on, in both
// themes: 4.5:1 for text, and 3:1 for what is only decoration. A palette edit that
// slips under fails here, with the pair that did.
func TestThePalettesTextMeetsWCAGContrastInBothThemes(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	white := rgba{255, 255, 255, 1}

	for name, tok := range map[string]map[string]string{
		"dark":  tokens(t, css, ":root,\n[data-theme='dark']"),
		"light": tokens(t, css, "[data-theme='light']"),
	} {
		colour := func(k string) rgba { return parseColour(t, tok[k]) }
		panel, canvas := colour("panel"), colour("canvas")
		check := func(what string, ratio, atLeast float64) {
			if ratio < atLeast {
				t.Errorf("%s: %s is %.2f:1, want at least %.1f:1", name, what, ratio, atLeast)
			}
		}

		for _, surface := range []struct {
			label string
			bg    rgba
		}{{"panel", panel}, {"canvas", canvas}} {
			check("--text on "+surface.label, contrast(colour("text").over(surface.bg), surface.bg), 4.5)
			check("--text-2 on "+surface.label, contrast(colour("text-2").over(surface.bg), surface.bg), 4.5)
			check("--text-3 on "+surface.label+" (decoration only)", contrast(colour("text-3").over(surface.bg), surface.bg), 3)
		}
		check("--blue-text on panel", contrast(colour("blue-text").over(panel), panel), 4.5)
		check("white on --blue (primary button)", contrast(white, colour("blue")), 4.5)
		check("white on --blue-hover", contrast(white, colour("blue-hover")), 4.5)
		check("white on --red-solid (danger button)", contrast(white, colour("red-solid")), 4.5)
		check("white on --red-solid-hover", contrast(white, colour("red-solid-hover")), 4.5)

		for _, tone := range []string{"green", "orange", "red"} {
			fg := colour(tone).over(panel)
			check(fmt.Sprintf("--%s text on panel", tone), contrast(fg, panel), 4.5)
			badge := colour(tone + "-bg").over(panel)
			check(fmt.Sprintf("--%s badge text on its tint", tone), contrast(colour(tone).over(badge), badge), 4.5)
		}
	}
}

// Before the stylesheet arrives the page paints the colour it is told in its own
// head, which must be the canvas the stylesheet will draw, in each theme, or the
// page flashes from one to the other.
func TestThePageTellsTheBrowserItsBackgroundBeforeTheStylesheet(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	page := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, page, `<meta name="color-scheme" content="dark light">`)

	early := strings.Index(page, "<style>html{background:")
	sheet := strings.Index(page, `rel="stylesheet"`)
	assert.True(t, early > 0 && early < sheet) // the browser reads it first

	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	dark := tokens(t, css, ":root,\n[data-theme='dark']")["canvas"]
	light := tokens(t, css, "[data-theme='light']")["canvas"]
	assert.Contains(t, page, "html{background:"+dark+";color-scheme:dark}")
	assert.Contains(t, page, "html[data-theme='light']{background:"+light+";color-scheme:light}")
	assert.Contains(t, page, "html:not([data-theme]){background:"+light+";color-scheme:light}")
}
