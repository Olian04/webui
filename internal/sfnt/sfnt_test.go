package sfnt

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// These tests are ours: the code in this package is another library's, cut down to reading a CFF font and writing a subset of it, and what is asked of it here is what
// the renderer asks. The font is the one the renderer embeds.
const fontFile = "../render/assets/fonts/fa-solid-900.woff"

func load(t *testing.T) *SFNT {
	t.Helper()
	raw, err := os.ReadFile(fontFile)
	if err != nil {
		t.Fatal(err)
	}
	font, err := ParseSFNT(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	return font
}

// outline is a glyph's contour as a string, to compare two glyphs for being the same shape.
type outline struct{ b strings.Builder }

func (o *outline) MoveTo(x, y float64) { fmt.Fprintf(&o.b, "M%.2f,%.2f", x, y) }
func (o *outline) LineTo(x, y float64) { fmt.Fprintf(&o.b, "L%.2f,%.2f", x, y) }
func (o *outline) QuadTo(a, b, c, d float64) {
	fmt.Fprintf(&o.b, "Q%.2f,%.2f,%.2f,%.2f", a, b, c, d)
}
func (o *outline) CubeTo(a, b, c, d, e, f float64) {
	fmt.Fprintf(&o.b, "C%.2f,%.2f,%.2f,%.2f,%.2f,%.2f", a, b, c, d, e, f)
}
func (o *outline) Close() { o.b.WriteString("Z") }

func shape(t *testing.T, font *SFNT, glyph uint16) string {
	t.Helper()
	var o outline
	if err := font.CFF.ToPath(&o, glyph, 0, 0, 0, 1, NoHinting); err != nil {
		t.Fatal(err)
	}
	return o.b.String()
}

// codes is n code points of the font, from the start of its private use area.
func codes(font *SFNT, n int) []rune {
	var out []rune
	for c := rune(0xe000); c < 0xf900 && len(out) < n; c++ {
		if font.GlyphIndex(c) != 0 {
			out = append(out, c)
		}
	}
	return out
}

func subsetOf(t *testing.T, font *SFNT, code []rune) *SFNT {
	t.Helper()
	glyphs := []uint16{0}
	for _, c := range code {
		glyphs = append(glyphs, font.GlyphIndex(c))
	}
	sub, err := font.Subset(glyphs, SubsetOptions{Tables: KeepMinTables})
	if err != nil {
		t.Fatal(err)
	}
	packed, err := sub.WriteWOFF()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseSFNT(packed, 0) // as a browser gets it
	if err != nil {
		t.Fatal(err)
	}
	return back
}

// A subset has the glyphs it was asked for, each the same shape and as wide as in the whole font, and the metrics that put a line of them where the whole font puts it. The
// sizes include one past the point where the CharStrings INDEX needs three bytes for an offset (about 340 glyphs of this font): the library this is from wrote those
// offsets wrong, and the font was one that a browser refused.
func TestASubsetHasEachGlyphAsTheWholeFontDrawsIt(t *testing.T) {
	t.Parallel()

	whole := load(t)
	for _, n := range []int{1, 25, 340, 400, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			asked := codes(whole, n)
			if len(asked) < min(n, 1000) {
				t.Fatalf("the font has %d of the %d code points", len(asked), n)
			}
			sub := subsetOf(t, load(t), asked)
			for _, c := range asked {
				g, w := sub.GlyphIndex(c), whole.GlyphIndex(c)
				if g == 0 {
					t.Fatalf("U+%04X is not in the subset", c)
				}
				if sub.GlyphAdvance(g) != whole.GlyphAdvance(w) {
					t.Fatalf("U+%04X is %d wide and was %d", c, sub.GlyphAdvance(g), whole.GlyphAdvance(w))
				}
				if shape(t, sub, g) != shape(t, whole, w) {
					t.Fatalf("U+%04X is not the shape it was", c)
				}
			}
			if sub.NumGlyphs() != uint16(len(asked)+1) {
				t.Fatalf("the subset has %d glyphs for %d asked and the one for what the font has not got", sub.NumGlyphs(), len(asked))
			}
			wa, wd, wg := whole.VerticalMetrics()
			sa, sd, sg := sub.VerticalMetrics()
			if wa != sa || wd != sd || wg != sg || whole.UnitsPerEm() != sub.UnitsPerEm() {
				t.Fatalf("the metrics are %d/%d/%d per %d and were %d/%d/%d per %d", sa, sd, sg, sub.UnitsPerEm(), wa, wd, wg, whole.UnitsPerEm())
			}
		})
	}
}

// Asking for the same font twice makes the same bytes: the app's font is served under an address that has a hash of it in it, and a restart must not change it.
func TestASubsetIsTheSameBytesEachTime(t *testing.T) {
	t.Parallel()

	font := load(t)
	asked := codes(font, 60)
	glyphs := []uint16{0}
	for _, c := range asked {
		glyphs = append(glyphs, font.GlyphIndex(c))
	}
	var first []byte
	for range 3 {
		sub, err := load(t).Subset(glyphs, SubsetOptions{Tables: KeepMinTables})
		if err != nil {
			t.Fatal(err)
		}
		packed, err := sub.WriteWOFF()
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = packed
		} else if string(first) != string(packed) {
			t.Fatal("two subsets of the same glyphs are not the same bytes")
		}
	}
}

func TestOnlyAFontWithCFFOutlinesCanBeSubset(t *testing.T) {
	t.Parallel()

	font := load(t)
	font.IsCFF, font.IsTrueType = false, true
	if _, err := font.Subset([]uint16{0, 1}, SubsetOptions{Tables: KeepMinTables}); err == nil {
		t.Fatal("a TrueType font was subset")
	}
}
