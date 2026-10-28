// Package theme is the colour arithmetic behind webui.Theme: parsing, mixing the way CSS does, the contrast
// of one colour on another, the tokens an app's colours derive, and the check that what they derive is legible.
// The stylesheet's tokens are what the design draws; this package is what an app's colours become beside them,
// so that what is checked at Compile is the same number the browser will be given.
package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Color is an sRGB colour with an alpha, each component from 0 to 1.
type Color struct{ R, G, B, A float64 }

var (
	white       = Color{1, 1, 1, 1}
	black       = Color{0, 0, 0, 1}
	transparent = Color{0, 0, 0, 0}
)

// RGB is an opaque colour from 0-255 components, as a stylesheet writes it.
func RGB(r, g, b int) Color { return Color{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1} }

// RGBA is RGB with an alpha from 0 to 1.
func RGBA(r, g, b int, a float64) Color {
	c := RGB(r, g, b)
	c.A = a
	return c
}

// Parse reads "#rgb", "#rgba", "#rrggbb" or "#rrggbbaa". It is the same set that webui.Color allows.
func Parse(s string) (Color, bool) {
	if !strings.HasPrefix(s, "#") {
		return Color{}, false
	}
	h := s[1:]
	switch len(h) {
	case 3, 4:
		var long strings.Builder
		for _, r := range h {
			long.WriteRune(r)
			long.WriteRune(r)
		}
		h = long.String()
	case 6, 8:
	default:
		return Color{}, false
	}
	var v [4]float64
	v[3] = 1
	for i := 0; i < len(h)/2; i++ {
		n, err := strconv.ParseUint(h[2*i:2*i+2], 16, 8)
		if err != nil {
			return Color{}, false
		}
		v[i] = float64(n) / 255
	}
	return Color{v[0], v[1], v[2], v[3]}, true
}

// Hex is the colour as the stylesheet is given it: "#rrggbb", or "#rrggbbaa" when it is not opaque.
func (c Color) Hex() string {
	b := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	if c.A >= 1 {
		return fmt.Sprintf("#%02x%02x%02x", b(c.R), b(c.G), b(c.B))
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", b(c.R), b(c.G), b(c.B), b(c.A))
}

// CSS is the colour as a declaration's value: its hex when opaque, and rgb with an alpha when it is not, which is how the
// stylesheet writes the translucent tokens.
func (c Color) CSS() string {
	if c.A >= 1 {
		return c.Hex()
	}
	b := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", b(c.R), b(c.G), b(c.B), strconv.FormatFloat(math.Round(c.A*1000)/1000, 'f', -1, 64))
}

// withAlpha is the colour with its alpha set, for the text colour at the strength a token gives it.
func (c Color) withAlpha(a float64) Color {
	c.A = a
	return c
}

// Mix is color-mix(in srgb, a pct%, b): the two interpolated in premultiplied sRGB, which is what a browser does,
// so that a mix with "transparent" is the colour at a fraction of its alpha.
func Mix(a Color, pct float64, b Color) Color {
	wa, wb := pct/100, 1-pct/100
	alpha := a.A*wa + b.A*wb
	if alpha == 0 {
		return Color{}
	}
	return Color{
		R: (a.R*a.A*wa + b.R*b.A*wb) / alpha,
		G: (a.G*a.A*wa + b.G*b.A*wb) / alpha,
		B: (a.B*a.A*wa + b.B*b.A*wb) / alpha,
		A: alpha,
	}
}

// Over is the colour drawn on an opaque background.
func (c Color) Over(bg Color) Color {
	return Color{
		R: c.R*c.A + bg.R*(1-c.A),
		G: c.G*c.A + bg.G*(1-c.A),
		B: c.B*c.A + bg.B*(1-c.A),
		A: 1,
	}
}

// luminance is the relative luminance of an opaque colour, as WCAG defines it.
func (c Color) luminance() float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Contrast is the WCAG contrast ratio of fg drawn on bg, which is opaque: a translucent fg is first laid over it.
func Contrast(fg, bg Color) float64 {
	l1, l2 := fg.Over(bg).luminance(), bg.luminance()
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}
