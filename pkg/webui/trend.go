package webui

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

// Direction says which way of a change is the good one.
type Direction string

const (
	// Higher is the good direction: a rise is good news, as in throughput or revenue. It is the default.
	Higher Direction = ""

	// Lower is the good direction when a fall is good news, as in a response time or a rate of errors.
	Lower Direction = "lower"
)

// Delta projects a change of M: a signed number, shown with its sign and an arrow, in the colour of whether the way it moved is
// the good one. It is how a figure says "up 4.35%" or "down 12 ms" at a glance, in a table column, in a [Stat] tile, or in a read-only
// form. It only reads: there is no Store.
//
// The sign is always in the text, and the arrow with it, so the colour is never the only thing saying which way it went. A change that
// is zero as it is shown (a rise of 0.001 with Precision 2 reads "0.00") has no sign, no arrow and no colour: what the text says
// and what the colour says are one thing. A column of Deltas sorts and filters on the number, and a file of it holds the number.
type Delta[M any] struct {
	// Label is the column header, and what a tile calls it. In a table it also names the column in the address and in a [Query].
	Label string

	// Load reads the change: positive when it went up, negative when it went down.
	Load func(M) float64

	// Better is the direction that is good news. It is Higher when left empty.
	Better Direction

	// Precision is the number of decimals shown. Zero shows the shortest exact representation.
	Precision int

	// Unit is what follows the number, as written: "%", " ms". A leading space is kept.
	Unit string

	// Help is a line or two that tells the visitor what the change is of, in the popover beside the label where there is one.
	Help string

	// NoSort leaves the sort link off the column's header, in a table.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table.
	NoFilter bool
}

func (a Delta[M]) label() string { return a.Label }

func (a Delta[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// SeriesStyle is how a [Series] is drawn.
type SeriesStyle string

const (
	// SeriesLine is a line through the points, a sparkline. It is the default.
	SeriesLine SeriesStyle = ""

	// SeriesBars is a bar for each point, from zero.
	SeriesBars SeriesStyle = "bars"
)

// Series projects a list of numbers of M, drawn as a small chart where it is read: a sparkline, or a row of bars. It has no axes and no
// scale, which is what a small chart is for: the shape of the figures over time, next to the figure. The values it was drawn from
// are in its text alternative, in words (how many, from where to where, the lowest and the highest), which a reader who cannot
// see it has, and a file of it holds them, separated by semicolons. It only reads, and it is not sorted or filtered by.
type Series[M any] struct {
	// Label is the column header, and what a tile calls it. It begins the text alternative.
	Label string

	// Load reads the points, in order. A point that is not a number (NaN, or infinite) is left out.
	Load func(M) []float64

	// Style is a line, which is the default, or bars.
	Style SeriesStyle

	// Highlight is the index of the point to mark, such as the current hour of a forecast, or a number that is outside the series
	// for none. Without it a line marks its last point, and bars mark none.
	Highlight func(M) int

	// Precision is the number of decimals the text alternative shows. Zero shows the shortest exact representation.
	Precision int

	// Unit follows each number in the text alternative, as written: "%", " ms".
	Unit string

	// Help is a line or two that tells the visitor what the series is of, in the popover beside the label where there is one.
	Help string
}

func (a Series[M]) label() string { return a.Label }

// meta leaves a series out of a table's sort and filter: a list of numbers has no order, and no one value to filter by.
func (a Series[M]) meta() accessorMeta {
	return accessorMeta{NoSort: true, NoFilter: true, Help: a.Help}
}

// ASSERT: Delta and Series implement Accessor
var (
	_ Accessor[struct{}] = Delta[struct{}]{}
	_ Accessor[struct{}] = Series[struct{}]{}
)

// ---------------------------------------------------------------- validate

func validateDelta[M any](v *bodyValidator, at string, a Delta[M]) {
	validatePrecision(v, at, a.Precision)
	switch a.Better {
	case Higher, Lower:
	default:
		v.add(fmt.Sprintf("%s has the unknown Better %q", at, string(a.Better)), "Use Higher, when a rise is good news, or Lower, when a fall is.")
	}
}

func validateSeries[M any](v *bodyValidator, at string, a Series[M]) {
	validatePrecision(v, at, a.Precision)
	switch a.Style {
	case SeriesLine, SeriesBars:
	default:
		v.add(fmt.Sprintf("%s has the unknown Style %q", at, string(a.Style)), "Use SeriesLine, a sparkline, or SeriesBars.")
	}
}

// ------------------------------------------------------------------- lower

func lowerDelta[M any](a Delta[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindFloat)
	f.Display = ir.DisplayDelta
	f.Get, f.Num = floatReads(a.Load, a.Precision)
	f.Show = func(_ context.Context, m any) string { return deltaText(a.Load(m.(M)), a.Precision, a.Unit) }
	f.Delta = &ir.Delta{
		Lower: a.Better == Lower,
		Sign:  func(m any) int { return deltaSign(a.Load(m.(M)), a.Precision) },
	}
	return f
}

// deltaSign is the direction of a change as it is shown, so that the text, the arrow and the colour agree: a value that rounds to zero
// at the precision shown has none.
func deltaSign(v float64, precision int) int {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if precision > 0 {
		p := math.Pow10(precision)
		v = math.Round(v*p) / p
	}
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// deltaText is a change with its sign: "+4.35%", "−4.80%" (with the minus sign and not a hyphen, which reads as one and is as wide as a plus),
// and "0%" for none.
func deltaText(v float64, precision int, unit string) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "—"
	}
	digits := -1
	if precision > 0 {
		digits = precision
	}
	body := strconv.FormatFloat(math.Abs(v), 'f', digits, 64) + unit
	switch deltaSign(v, precision) {
	case 1:
		return "+" + body
	case -1:
		return "−" + body
	}
	return strconv.FormatFloat(0, 'f', digits, 64) + unit
}

func lowerSeries[M any](a Series[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindString)
	f.Display = ir.DisplaySeries
	raw := func(m any) []float64 { return a.Load(m.(M)) }
	values := func(m any) []float64 { return finite(raw(m)) }
	number := func(v float64) string {
		digits := -1
		if a.Precision > 0 {
			digits = a.Precision
		}
		return strconv.FormatFloat(v, 'f', digits, 64)
	}
	f.Get = func(m any) string {
		vs := values(m)
		parts := make([]string, len(vs))
		for i, v := range vs {
			parts[i] = number(v)
		}
		return strings.Join(parts, ";")
	}
	f.Series = &ir.Series{
		Values: raw, // as loaded: a Highlight is an index of these, and the renderer leaves out what is not a number
		Bars:   a.Style == SeriesBars,
		Describe: func(m any) string {
			return describeSeries(a.Label, values(m), number, a.Unit)
		},
	}
	if a.Highlight != nil {
		f.Series.Highlight = func(m any) int { return a.Highlight(m.(M)) }
	}
	return f
}

// finite is the points that are numbers.
func finite(vs []float64) []float64 {
	out := make([]float64, 0, len(vs))
	for _, v := range vs {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
	}
	return out
}

// describeSeries is a series in words: what it is, how many points, where it starts and ends, and its extremes.
func describeSeries(label string, vs []float64, number func(float64) string, unit string) string {
	at := func(v float64) string { return number(v) + unit }
	switch len(vs) {
	case 0:
		return label + ": no values"
	case 1:
		return label + ": 1 value, " + at(vs[0])
	}
	lo, hi := vs[0], vs[0]
	for _, v := range vs {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return fmt.Sprintf("%s: %d values, from %s to %s, lowest %s, highest %s", label, len(vs), at(vs[0]), at(vs[len(vs)-1]), at(lo), at(hi))
}
