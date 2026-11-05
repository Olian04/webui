package webui

import (
	"context"
	"math"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/trend"
)

// Trend projects the shape of a figure of M over time, and how much it moved: a small chart of a list of numbers, and beside it the change as a
// percent, with its sign and an arrow, in the colour of whether the way it moved is the good one. It is how a figure says "up 4.35%" with the
// shape that led to it, in a table column (a table of one row is how a page shows a few figures side by side), or in a read-only form. It only reads: there is no Store.
//
// The chart is a line, a sparkline, drawn with no axes and no scale, which is what a small chart is for: the shape beside the figure. It marks its last
// point. Its text alternative has the figures in words (how many, from where to where, the lowest and the highest), which a reader who cannot see
// it has. The change is the app's to say, as a percent, and need not be the change of the points: a figure may be set against yesterday while
// the line is of the last hour. It is written in three digits at all times (0.35%, 35.4%, 100%), so that a column of them has one width.
//
// Either part may be absent for a model: no points draw no chart, and no change shows no change. The sign is always in the text, and the arrow with it, so
// the colour is never the only thing saying which way it went. A change that is zero as it is shown (a rise of 0.001 reads "0%") has no sign, no arrow and no
// colour: what the text says and what the colour says are one thing. A column of Trends sorts and filters on the change, and a file of it holds the
// change as a number, with no points. A model with no change has no number, so it sorts first and a filter by a range leaves it out; a Trend that never
// has one is declared with NoSort and NoFilter.
type Trend[M any] struct {
	// Label is the column header, and, in a table, it also names the column in the address and in a [PagedQuery]. It begins
	// the chart's text alternative.
	Label string

	// Load reads the points of the line, in order, and the change as a percent: 4.4 for a rise of 4.4%, and nil for no change. A point that is not
	// a number (NaN, or infinite) is left out of the line, and a change that is not one is shown as a dash. No points draw no chart.
	Load func(M) ([]float64, *float64)

	// LowerIsBetter says a fall is the good news, as in a response time or a rate of errors, so that a fall is drawn as good and a rise as bad.
	// Left false, a rise is good, as in throughput or revenue.
	LowerIsBetter bool

	// Help is a line or two that tells the visitor what the trend is of, in the popover beside the label where there is one.
	Help string

	// NoSort leaves the sort link off the column's header, in a table.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table.
	NoFilter bool
}

func (a Trend[M]) label() string { return a.Label }

func (a Trend[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// ASSERT: Trend implements Accessor
var _ Accessor[struct{}] = Trend[struct{}]{}

// ------------------------------------------------------------------- lower

func lowerTrend[M any](a Trend[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindFloat)
	f.Display = ir.DisplayTrend
	change := func(m any) (float64, bool) {
		_, pct := a.Load(m.(M))
		if pct == nil {
			return math.NaN(), false
		}
		return *pct, true
	}
	f.Num = func(m any) float64 {
		v, _ := change(m)
		return v
	}
	f.Get = func(m any) string {
		if v, ok := change(m); ok {
			return trend.Export(v)
		}
		return ""
	}
	f.Show = func(_ context.Context, m any) string {
		if v, ok := change(m); ok {
			text, _ := trend.Percent(v)
			return text
		}
		return ""
	}
	f.Trend = &ir.Trend{
		Lower: a.LowerIsBetter,
		Read: func(m any) ir.TrendReading {
			points, pct := a.Load(m.(M))
			r := ir.TrendReading{Points: trend.Finite(points)}
			if len(r.Points) > 0 {
				r.Describe = trend.Describe(a.Label, r.Points)
			}
			if pct != nil {
				r.Change, r.Sign = trend.Percent(*pct)
				r.HasChange = true
			}
			return r
		},
	}
	return f
}
