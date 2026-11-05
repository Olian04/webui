// Package sfnt reads an OpenType font with CFF outlines and writes a subset of it, as a WOFF file that a browser takes. It is how the renderer cuts the icon font of
// Font Awesome Free to the icons an app can draw (see assets.IconFont): the whole font is 120 KB and a page draws a few glyphs.
//
// It is not written for this library. It is the sfnt code of github.com/tdewolff/font (commit 88a7f6b56918, October 2026) with the binary reader and writer of
// github.com/tdewolff/parse/v2 (v2.8.16), both under the MIT license (LICENSE-font.md and LICENSE-parse.md), copied here and cut down, since this library does
// not take a dependency on a module that has no release. What is kept is reading a font (the container, the tables, the CFF outlines and the character map) and
// subsetting it; what is gone is TrueType outlines, kerning and layout tables, merging, drawing, WOFF2 and the system font finder, and with them the modules that
// those need. The files keep their names, so that a change upstream can be compared with them; see VENDORED.md for what differs.
//
// The package is internal and is not to be grown into a font library: what the renderer does not use does not belong here.
package sfnt
