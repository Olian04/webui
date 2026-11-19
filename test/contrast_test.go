package webui_test

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

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
	for i, part := range strings.FieldsFunc(m[1], func(r rune) bool {
		return r == ',' || r == '/' || unicode.IsSpace(r)
	}) {
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
		out[m[1]] = strings.Join(strings.Fields(m[2]), "")
	}
	return out
}

// The text a reader needs must be legible on the surfaces it sits on, in both
// themes: 4.5:1 for text, and 3:1 for what is only decoration. A palette edit that
// slips under fails here, with the pair that did.
func TestThePalettesTextMeetsWCAGContrastInBothThemes(t *testing.T) {
	t.Parallel()

	css := assetSource(t, "tokens.css")
	white := rgba{255, 255, 255, 1}

	for name, tok := range map[string]map[string]string{
		"dark":  tokens(t, css, ":root,\n[data-theme='dark']"),
		"light": tokens(t, css, "[data-theme='light']"),
	} {
		colour := func(k string) rgba { return parseColour(t, tok[k]) }
		panel, canvas := colour("surface-color"), colour("page-color")
		check := func(what string, ratio, atLeast float64) {
			if ratio < atLeast {
				t.Errorf("%s: %s is %.2f:1, want at least %.1f:1", name, what, ratio, atLeast)
			}
		}

		for _, surface := range []struct {
			label string
			bg    rgba
		}{{"panel", panel}, {"canvas", canvas}} {
			check("--text-color on "+surface.label, contrast(colour("text-color").over(surface.bg), surface.bg), 4.5)
			check("--text-muted-color on "+surface.label, contrast(colour("text-muted-color").over(surface.bg), surface.bg), 4.5)
			check("--text-faint-color on "+surface.label+" (decoration only)", contrast(colour("text-faint-color").over(surface.bg), surface.bg), 3)
		}
		// The quiet text is read on every surface it is drawn on, and the hover of a row is one of them.
		input, raise := colour("field-color"), colour("raised-color")
		hovered := colour("hover-color").over(panel)
		check("--text-muted-color on a hovered row", contrast(colour("text-muted-color").over(hovered), hovered), 4.5)
		check("--text-muted-color on the raised surface", contrast(colour("text-muted-color").over(raise), raise), 4.5)
		check("--text-muted-color on an input", contrast(colour("text-muted-color").over(input), input), 4.5)
		check("--accent-text-color on a hovered row", contrast(colour("accent-text-color").over(hovered), hovered), 4.5)
		// The edge of a control is what tells a person where it is: 3:1 against what it is drawn on, for a visitor who asks for more contrast.
		// The ordinary edge is the quiet one (the same as a border's), which is the look of the app and the choice of its design.
		check("--control-border-strong-color on an input", contrast(colour("control-border-strong-color").over(input), input), 3)
		check("--control-border-strong-color on the panel", contrast(colour("control-border-strong-color").over(panel), panel), 3)
		if tok["control-border-color"] != "var(--border-strong-color)" {
			t.Errorf("%s: --control-border-color is %q, want the ordinary edge, var(--border-strong-color)", name, tok["control-border-color"])
		}
		check("--accent-text-color on panel", contrast(colour("accent-text-color").over(panel), panel), 4.5)
		check("white on --accent-color (primary button)", contrast(white, colour("accent-color")), 4.5)
		check("white on --accent-hover-color", contrast(white, colour("accent-hover-color")), 4.5)
		check("white on --danger-solid-color (danger button)", contrast(white, colour("danger-solid-color")), 4.5)
		check("white on --danger-solid-hover-color", contrast(white, colour("danger-solid-hover-color")), 4.5)

		for _, tone := range []string{"success", "warning", "danger"} {
			fg := colour(tone + "-color").over(panel)
			check(fmt.Sprintf("--%s-color text on panel", tone), contrast(fg, panel), 4.5)
			badge := colour(tone + "-bg-color").over(panel)
			check(fmt.Sprintf("--%s-color badge text on its tint", tone), contrast(colour(tone+"-color").over(badge), badge), 4.5)
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

	css := assetSource(t, "tokens.css")
	dark := tokens(t, css, ":root,\n[data-theme='dark']")["page-color"]
	light := tokens(t, css, "[data-theme='light']")["page-color"]
	assert.Contains(t, page, "html{background:"+dark+";color-scheme:dark}")
	assert.Contains(t, page, "html[data-theme='light']{background:"+light+";color-scheme:light}")
	assert.Contains(t, page, "html:not([data-theme]){background:"+light+";color-scheme:light}")
}
