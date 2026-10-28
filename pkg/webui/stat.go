package webui

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Olian04/webui/internal/ir"
)

// Stat is a leaf that shows a few figures of one model, each as a tile: what it is, its value, and, when it has them, the change in
// it and its trend. It is how a page says "37 devices, up 3 since yesterday" before it shows the table of them, and it only reads.
//
// A tile's value is any accessor that is read, so a count, a health [Select] drawn as a badge, a [Slider] as a bar and a
// formatted size are each themselves: a tile has no way of showing a value of its own. Its change is a [Delta] and its trend a
// [Series], both optional. A page that needs more of a figure than a tile says has a [Table] for it.
//
// The tiles are laid out in a row that wraps, and are a list of terms and values to a reader who goes by structure. A tile with a change
// says its direction in words and in an arrow as well as in colour. The panel is loaded as any is: it refreshes with a
// [Loaded.RefreshAfter], and a failed Load fails the panel and not the page.
type Stat[M any] struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the title.
	Desc string

	// Load returns the model that the tiles read.
	Load func(ctx context.Context) (M, Loaded)

	// Tiles are the figures, in the order they are drawn. A Stat has at least one.
	Tiles []StatTile[M]
}

// StatTile is one figure of a [Stat].
type StatTile[M any] struct {
	// Label is what the figure is called. Without one it is the Label of Value.
	Label string

	// Help is what the figure is, in a popover beside its label, where the label cannot say it.
	Help string

	// Value is the figure. It is shown and never edited: a Store of it is not used.
	Value Accessor[M]

	// Change is how the figure moved, as a signed number with an arrow. Without a Load there is none.
	Change Delta[M]

	// Trend is the figure's shape over time, as a small chart. Without a Load there is none.
	Trend Series[M]
}

// ASSERT: Stat implements PageBody
var _ PageBody = Stat[struct{}]{}

func (s Stat[M]) validateBody(v *bodyValidator) {
	needLoad(v, "a Stat", s.Load != nil)
	if len(s.Tiles) == 0 {
		v.add("a Stat has no Tiles", "Give it a tile for each figure it shows, each with a Value.")
	}
	for i, t := range s.Tiles {
		at := "Stat.Tiles[" + strconv.Itoa(i) + "]"
		switch t.Value.(type) {
		case nil:
			v.add(at+".Value is nil", "A tile shows a figure: set Value to an accessor, such as an Int or a Float.")
		case Group[M]:
			v.add(at+".Value is a Group", "A group composes form fields; a tile shows one figure.")
		default:
			validateAccessor[M](v, t.Value, at+".Value", accessorSite{where: "Stat.Tiles"}, map[string]bool{})
		}
		if t.Change.Load != nil {
			validateDelta(v, at+".Change", t.Change)
		} else if t.Change.Label != "" || t.Change.Unit != "" || t.Change.Precision != 0 || t.Change.LowerIsBetter {
			v.add(at+".Change has no Load", "Set Load to func(M) float64, returning the change, or remove the Change: a tile with no Load has none.")
		}
		if t.Trend.Load != nil {
			validateSeries(v, at+".Trend", t.Trend)
		} else if t.Trend.Label != "" || t.Trend.Unit != "" || t.Trend.Precision != 0 || t.Trend.Style != SeriesLine || t.Trend.Highlight != nil {
			v.add(at+".Trend has no Load", "Set Load to func(M) []float64, returning the points, or remove the Trend: a tile with no Load has none.")
		}
	}
}

func (s Stat[M]) lowerBody(at ir.Addr) ir.Node {
	n := &ir.Stat{At: at, Title: s.Title, Desc: s.Desc, Load: eraseLoad(s.Load)}
	for i, t := range s.Tiles {
		name := "s" + strconv.Itoa(i)
		tile := ir.StatTile{Help: t.Help, Value: lowerAccessor[M](t.Value, name)}
		tile.Label = t.Label
		if tile.Label == "" {
			tile.Label = tile.Value.Label
		}
		if t.Change.Load != nil {
			d := t.Change
			if d.Label == "" {
				d.Label = "Change"
			}
			f := lowerDelta(d, name+"c")
			tile.Change = &f
		}
		if t.Trend.Load != nil {
			sr := t.Trend
			if sr.Label == "" {
				sr.Label = fmt.Sprintf("%s, trend", tile.Label)
			}
			f := lowerSeries(sr, name+"t")
			tile.Trend = &f
		}
		n.Tiles = append(n.Tiles, tile)
	}
	return n
}
