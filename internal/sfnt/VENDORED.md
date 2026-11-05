# Vendored: github.com/tdewolff/font and github.com/tdewolff/parse

`internal/sfnt` is a copy of part of two libraries by Taco de Wolff, both MIT-licensed (`LICENSE-font.md`, `LICENSE-parse.md`):

| From | At | Files |
|---|---|---|
| `github.com/tdewolff/font` | `v0.0.0-20261010204837-88a7f6b56918` | `sfnt.go`, `sfnt_cff.go`, `sfnt_cmap.go`, `sfnt_os2.go`, `sfnt_subset.go`, `sfnt_data.go`, `sfnt_util.go`, `util.go`, `woff.go` |
| `github.com/tdewolff/parse/v2` | `v2.8.16` | `binary.go` (the `BinaryReader` and `BinaryWriter`) |

It exists so that the renderer can cut the icon font to the icons an app uses (a 5 KB font in place of 120 KB, with the metrics of the whole font), without a dependency on a module that has no release.
`woff.go` (reading WOFF 1.0) and `woffwrite.go` (writing it) use only the standard library; WOFF2 would have needed Brotli, and is not here.

## What differs from upstream

Everything not listed is as it was. Do not "update" a file by copying it over: compare it with upstream and carry the changes below.

- **Removed**: TrueType outlines (`glyf`, `loca`) and their subsetting, kerning, GPOS/GSUB/JSTF, glyph drawing (`GlyphPath`, `GlyphBounds`), the `estimateOS2` fallback, the PDF table set of `Subset`, merging, WOFF2, EOT, AFM, the system font finder, and the freetype and `golang.org/x/image` code. `ParseSFNT` takes an OpenType or a WOFF 1.0 file; `Subset` takes a CFF font and says so.
- **`sfnt.go`**: names are decoded with `unicode/utf16` and not with `golang.org/x/text` (nothing here reads a name). `Write` keeps the head table's own `modified` time, where upstream writes the time of writing, which made the same font different bytes every time: a font served under a hash of its contents has to be the same bytes.
- **`sfnt_cff.go`**: the INDEX writer wrote a three-byte offset as `uint8(o>>16), uint16(o&0x00FF)`, which drops the middle byte, so a CFF font whose glyph data is over 64 KB (more than about 340 glyphs of Font Awesome) was invalid and a browser refused it. It writes the three bytes now (`WriteUint24`), and the size at which four are needed is `1<<24`, not `2<<24`. Found by rendering every icon in Chromium; held by `TestASubsetHasEachGlyphAsTheWholeFontDrawsIt`, which asks for 400 and 1,000 glyphs.
- **`binary.go`**: the part of `parse/v2` that the font code uses, in this package; the memory-mapped file reader is not here. `BinaryWriter.WriteByte`, which `go vet` refuses for having the signature of the standard one without its result, is gone.
- **`sfnt_cff.go`**: `parseCFF2`, which returns an error on its first line, is that line alone, for `go vet` (the code after it was never reached).
- **`woffwrite.go`**: ours: `WriteWOFF`.

`sfnt_test.go` is ours, not upstream's.
