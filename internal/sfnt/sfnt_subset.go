package sfnt

import (
	"fmt"
	"math"
	"sort"
)

var (
	KeepAllTables = []string{"all"}
	KeepMinTables = []string{"min"}
	KeepPDFTables = []string{"pdf"}
)

type SubsetOptions struct {
	Tables []string // names of tables to include, special values are KeepAllTables, KeepMinTables, or KeepPDFTables
}

// Subset trims an SFNT font to contain only the passed glyphIDs, thereby resulting in a significant size reduction. The glyphIDs will apear in the specified order in the file and their dependencies are added to the end.
func (sfnt *SFNT) Subset(glyphIDs []uint16, options SubsetOptions) (*SFNT, error) {
	if !sfnt.IsCFF {
		return nil, fmt.Errorf("only CFF fonts can be subset here")
	}
	if _, ok := sfnt.Tables["CFF2"]; ok {
		return nil, fmt.Errorf("subsetting CFF2 fonts not supported")
	}

	// set up glyph mapping from original to subset
	glyphMap := make(map[uint16]uint16, len(glyphIDs))
	for subsetGlyphID, glyphID := range glyphIDs {
		glyphMap[glyphID] = uint16(subsetGlyphID)
	}

	if math.MaxUint16 < len(glyphIDs) {
		return nil, fmt.Errorf("too many glyphs for one font")
	}

	// specify tables to include
	var tags []string
	if len(options.Tables) == 1 && options.Tables[0] == "min" {
		tags = []string{"DSIG", "cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post"}
		tags = append(tags, "CFF ") // a CFF font: Subset refuses the others
	} else if len(options.Tables) == 1 && options.Tables[0] == "all" {
		for tag := range sfnt.Tables {
			tags = append(tags, tag)
		}
	} else {
		tags = options.Tables
	}
	sort.Strings(tags) // so that glyf is before loca

	// preliminary calculations
	indexToLocFormat := int16(1)              // for head and loca
	ulUnicodeRange := [4]uint32{0, 0, 0, 0}   // for OS/2
	advances := make([]uint16, len(glyphIDs)) // for hhea and hmtx
	for subsetGlyphID, glyphID := range glyphIDs {
		advances[subsetGlyphID] = sfnt.Hmtx.Advance(glyphID)
	}

	// https://learn.microsoft.com/en-us/typography/opentype/spec/recom#hmtx-table
	// It is recommended that for CFF fonts the numberOfHMetrics must be equal to the number
	// of glyphs, but emperically there is no difference.
	//if !sfnt.IsCFF {
	numberOfHMetrics := uint16(len(glyphIDs)) // for hhea and hmtx
	for 1 < numberOfHMetrics {
		if advances[numberOfHMetrics-1] != advances[numberOfHMetrics-2] {
			break
		}
		numberOfHMetrics--
	}
	//}

	// copy to new SFNT
	sfntOld := sfnt
	sfnt = &SFNT{
		Version: sfntOld.Version,
		Length:  12, // increased below
		IsCFF:   sfntOld.IsCFF,
		Tables:  map[string][]byte{},
	}

	// copy and rewrite tables
	for _, tag := range tags {
		if tag == "maxp" {
			maxp := *sfntOld.Maxp
			sfnt.Maxp = &maxp
			sfnt.Maxp.NumGlyphs = uint16(len(glyphIDs))
			break
		}
	}
	for _, tag := range tags {
		table, ok := sfntOld.Tables[tag]
		if !ok {
			continue
		}

		switch tag {
		// TODO: GDEF, GPOS, GSUB
		// TODO; cvt, fpgm, prep
		case "cmap":
			rs := make([]rune, 0, len(glyphIDs))
			runeMap := make(map[rune]uint16, len(glyphIDs)) // for OS/2
			for subsetGlyphID, glyphID := range glyphIDs {
				rsGlyph := sfntOld.Cmap.ToUnicode(glyphID)
				rs = append(rs, rsGlyph...)
				for _, r := range rsGlyph {
					runeMap[r] = uint16(subsetGlyphID)
				}
			}
			ulUnicodeRange = os2UlUnicodeRange(rs)

			sfnt.Tables[tag] = cmapWrite(rs, runeMap)
			if err := sfnt.parseCmap(); err != nil {
				return nil, err
			}
		case "CFF ":
			cff := *sfntOld.CFF
			cff.charStrings = &cffINDEX{}
			for _, glyphID := range glyphIDs {
				if glyphID == 0 {
					// make .notdef empty
					cff.charStrings.Add([]byte{0x0E}) // endchar
					continue
				}

				charString := sfntOld.CFF.charStrings.Get(glyphID)
				if charString == nil {
					return nil, fmt.Errorf("CFF: bad charString for glyph %v", glyphID)
				}
				cff.charStrings.Add(charString) // copies data
			}

			if cff.charset != nil {
				charset := make([]string, len(glyphIDs))
				for i, glyphID := range glyphIDs {
					charset[i] = cff.charset[glyphID]
				}
				cff.charset = charset
			}

			// trim globalSubrs and localSubrs INDEX
			if err := cff.ReindexSubrs(); err != nil {
				return nil, err
			}

			b, err := cff.Write()
			if err != nil {
				return nil, err
			}
			sfnt.Tables[tag] = b
			sfnt.CFF = &cff
		case "head":
			w := NewBinaryWriter(make([]byte, 0, len(sfntOld.Tables["head"])))
			w.WriteBytes(table[:50])
			w.WriteInt16(indexToLocFormat) // indexToLocFormat
			w.WriteBytes(table[52:])
			sfnt.Tables[tag] = w.Bytes()

			head := *sfntOld.Head
			sfnt.Head = &head
			sfnt.Head.IndexToLocFormat = indexToLocFormat
		case "hhea":
			w := NewBinaryWriter(make([]byte, 0, len(sfntOld.Tables["hhea"])))
			w.WriteBytes(table[:34])
			w.WriteUint16(numberOfHMetrics) // numberOfHMetrics
			w.WriteBytes(table[36:])
			sfnt.Tables[tag] = w.Bytes()

			hhea := *sfntOld.Hhea
			sfnt.Hhea = &hhea
			sfnt.Hhea.NumberOfHMetrics = numberOfHMetrics
		case "hmtx":
			sfnt.Hmtx = &hmtxTable{}
			sfnt.Hmtx.HMetrics = make([]hmtxLongHorMetric, numberOfHMetrics)
			sfnt.Hmtx.LeftSideBearings = make([]int16, len(glyphIDs)-int(numberOfHMetrics))

			n := 4*int(numberOfHMetrics) + 2*(len(glyphIDs)-int(numberOfHMetrics))
			w := NewBinaryWriter(make([]byte, 0, n))
			for subsetGlyphID, glyphID := range glyphIDs {
				lsb := sfntOld.Hmtx.LeftSideBearing(glyphID)
				if subsetGlyphID < int(numberOfHMetrics) {
					sfnt.Hmtx.HMetrics[subsetGlyphID].AdvanceWidth = advances[subsetGlyphID]
					sfnt.Hmtx.HMetrics[subsetGlyphID].LeftSideBearing = lsb
					w.WriteUint16(advances[subsetGlyphID])
				} else {
					sfnt.Hmtx.LeftSideBearings[subsetGlyphID-int(numberOfHMetrics)] = lsb
				}
				w.WriteInt16(lsb)
			}
			sfnt.Tables[tag] = w.Bytes()
		case "maxp":
			w := NewBinaryWriter(make([]byte, 0, len(sfntOld.Tables["maxp"])))
			w.WriteBytes(table[:4])
			w.WriteUint16(uint16(len(glyphIDs))) // numGlyphs
			w.WriteBytes(table[6:])
			sfnt.Tables[tag] = w.Bytes()
		case "name":
			w := NewBinaryWriter(make([]byte, 0, 6))
			w.WriteUint16(0) // version
			w.WriteUint16(0) // count
			w.WriteUint16(6) // storageOffset
			sfnt.Tables[tag] = w.Bytes()

			sfnt.Name = &nameTable{}
		case "OS/2":
			sfnt.OS2 = sfntOld.OS2
			sfnt.OS2.UlUnicodeRange1 = ulUnicodeRange[0]
			sfnt.OS2.UlUnicodeRange2 = ulUnicodeRange[1]
			sfnt.OS2.UlUnicodeRange3 = ulUnicodeRange[2]
			sfnt.OS2.UlUnicodeRange4 = ulUnicodeRange[3]

			w := NewBinaryWriter(make([]byte, 0, len(sfntOld.Tables["OS/2"])))
			w.WriteBytes(table[:42])
			w.WriteUint32(sfnt.OS2.UlUnicodeRange1)
			w.WriteUint32(sfnt.OS2.UlUnicodeRange2)
			w.WriteUint32(sfnt.OS2.UlUnicodeRange3)
			w.WriteUint32(sfnt.OS2.UlUnicodeRange4)
			w.WriteBytes(table[58:])
			sfnt.Tables[tag] = w.Bytes()
		case "post":
			w := NewBinaryWriter(make([]byte, 0, 32))
			w.WriteUint32(0x00030000) // version
			w.WriteBytes(table[4:32])
			sfnt.Tables[tag] = w.Bytes()

			sfnt.Post = &postTable{
				ItalicAngle:        sfntOld.Post.ItalicAngle,
				UnderlinePosition:  sfntOld.Post.UnderlinePosition,
				UnderlineThickness: sfntOld.Post.UnderlineThickness,
				IsFixedPitch:       sfntOld.Post.IsFixedPitch,
				MinMemType42:       sfntOld.Post.MinMemType42,
				MaxMemType42:       sfntOld.Post.MaxMemType42,
				MinMemType1:        sfntOld.Post.MinMemType1,
				MaxMemType1:        sfntOld.Post.MaxMemType1,
			}
		default:
			sfnt.Tables[tag] = table
		}

		sfnt.Length += uint32(16 + len(sfnt.Tables[tag]))        // 16 for the table record
		sfnt.Length += (4 - uint32(len(sfnt.Tables[tag]))&3) & 3 // padding
	}
	return sfnt, nil
}

func (sfnt *SFNT) SetGlyphNames(names []string) error {
	if sfnt.IsCFF {
		same := false
		if len(sfnt.CFF.charset) == len(names) {
			same = true
			for i, name := range sfnt.CFF.charset {
				if name != names[i] {
					same = false
					break
				}
			}
		}
		if same {
			return nil
		}

		sfnt.CFF.charset = names
		b, err := sfnt.CFF.Write()
		if err != nil {
			return err
		}
		sfnt.Tables["CFF "] = b
		return nil
	}

	if _, ok := sfnt.Tables["post"]; !ok {
		return fmt.Errorf("post table doesn't exist")
	}

	sfnt.Post.NumGlyphs = sfnt.NumGlyphs()
	sfnt.Post.glyphNameIndex = make([]uint16, sfnt.NumGlyphs())
	sfnt.Post.stringData = [][]byte{}
	sfnt.Post.nameMap = nil

	lastIndex := uint16(258)
	if int(sfnt.NumGlyphs()) < len(names) {
		names = names[:sfnt.NumGlyphs()]
	}
	for glyphID, name := range names {
		var index uint16
		if glyphID2, ok := sfnt.Post.Find(name); ok {
			index = sfnt.Post.glyphNameIndex[glyphID2]
		} else if math.MaxUint16 < len(sfnt.Post.stringData)-258 {
			return fmt.Errorf("invalid post table: stringData has too many entries")
		} else {
			sfnt.Post.stringData = append(sfnt.Post.stringData, []byte(name))
			sfnt.Post.nameMap[name] = uint16(glyphID)
			index = lastIndex
			lastIndex++
		}
		sfnt.Post.glyphNameIndex[uint16(glyphID)] = index
	}

	b, err := sfnt.Post.Write()
	if err != nil {
		return err
	}

	sfnt.Tables["post"] = b
	return nil
}
