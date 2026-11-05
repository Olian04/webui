package assets

import (
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/sfnt"
)

// icons.txt is generated from the Font Awesome download and the font is copied from it by hand. They must say the same: a name that Compile accepts, at a code point that the
// font has no glyph at, is a blank where an icon was meant.
func TestEveryIconCompileAcceptsIsAGlyphOfTheFont(t *testing.T) {
	t.Parallel()

	font, err := sfnt.ParseSFNT(iconFontSource, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for line := range strings.SplitSeq(iconNames, "\n") {
		name, _, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		seen++
		code, ok := IconCode(name)
		if !ok {
			t.Fatalf("%q is in icons.txt and has no code point", name)
		}
		if font.GlyphIndex(code) == 0 {
			t.Errorf("%q is at U+%04X, where the font has no glyph", name, code)
		}
		if !HasIcon(name) {
			t.Errorf("%q is in icons.txt and HasIcon says it is not", name)
		}
	}
	if seen < 1500 {
		t.Fatalf("icons.txt has %d icons", seen)
	}
}

func TestAnIconFontOfANameThatIsNotAnIconIsAnError(t *testing.T) {
	t.Parallel()

	if _, err := IconFont([]string{"house", "no-such-icon"}); err == nil {
		t.Fatal("no error for a name that is not an icon")
	}
	if HasIcon("house chimney") || HasIcon("") || HasIcon("hous") {
		t.Fatal("HasIcon takes a name that is not one")
	}
}

func TestEveryNameOfTheCoreListIsAnIcon(t *testing.T) {
	t.Parallel()

	if len(CoreIcons()) < 10 {
		t.Fatalf("the core list has %d icons", len(CoreIcons()))
	}
	for _, name := range CoreIcons() {
		if !HasIcon(name) {
			t.Errorf("icons_core.txt names %q, which is not an icon", name)
		}
	}
}
