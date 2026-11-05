package assets

import (
	_ "embed" // the font, and the list of what the library draws
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Olian04/webui/internal/sfnt"
)

// The icon font is not served whole. Font Awesome Free solid is 2,000 icons in 120 KB and a page draws a few, so an app's font is cut from it when the app starts, to the icons that the
// app can draw and no others: see [IconFont]. The font below is the whole of it, as a WOFF 1.0 file, which the standard library reads.
//
//go:embed fonts/fa-solid-900.woff
var iconFontSource []byte

// coreIcons is the icons that the library draws itself, one name a line, with comments. Every app has them, whatever it names.
//
//go:embed icons_core.txt
var coreIcons string

// CoreIcons is the names of the icons that the library draws itself: what the design uses, and what a script puts in. An app's font always has them.
func CoreIcons() []string {
	var names []string
	for line := range strings.SplitSeq(coreIcons, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			names = append(names, line)
		}
	}
	return names
}

// iconCodes is the code point of each icon, read from icons.txt once.
var iconCodes = sync.OnceValue(func() map[string]rune {
	codes := map[string]rune{}
	for line := range strings.SplitSeq(iconNames, "\n") {
		name, hex, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			panic(fmt.Sprintf("icons.txt: %q: %v", line, err)) // the file is generated: this is a broken one
		}
		codes[name] = rune(code)
	}
	return codes
})

// IconCode is the code point an icon is at in the font, or false for a name that is not an icon.
func IconCode(name string) (rune, bool) {
	code, ok := iconCodes()[name]
	return code, ok
}

var (
	iconFontMu     sync.Mutex
	iconFontParsed *sfnt.SFNT
)

// IconFont is a font of the named icons and no others, as a WOFF 1.0 file: the same glyphs as the whole font has, drawn as they are in it, since the metrics are
// carried over. Every name must be an icon.
func IconFont(names []string) ([]byte, error) {
	var codes []rune
	for _, name := range names {
		code, ok := IconCode(name)
		if !ok {
			return nil, fmt.Errorf("%q is not an icon", name)
		}
		codes = append(codes, code)
	}
	slices.Sort(codes)
	codes = slices.Compact(codes)

	iconFontMu.Lock() // the parsed font is shared, and the library it is from makes no promise about reading it from two places at once
	defer iconFontMu.Unlock()
	if iconFontParsed == nil {
		font, err := sfnt.ParseSFNT(iconFontSource, 0)
		if err != nil {
			return nil, fmt.Errorf("fonts/fa-solid-900.woff: %w", err)
		}
		iconFontParsed = font
	}
	glyphs := []uint16{0} // .notdef is the first glyph of a font
	for _, code := range codes {
		id := iconFontParsed.GlyphIndex(code)
		if id == 0 {
			return nil, fmt.Errorf("the font has no glyph at U+%04X", code)
		}
		glyphs = append(glyphs, id)
	}
	subset, err := iconFontParsed.Subset(glyphs, sfnt.SubsetOptions{Tables: sfnt.KeepMinTables})
	if err != nil {
		return nil, err
	}
	return subset.WriteWOFF()
}
