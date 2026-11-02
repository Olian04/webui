package render

import (
	"math"
	"strconv"

	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// The size of a small chart, in the units of its box. It is drawn at the size the stylesheet gives it: the box scales.
const (
	sparkW   = 96
	sparkH   = 20
	sparkPad = 3 // room for the stroke and the mark at the edges
)

// trendProps is a trend as it is drawn: the change, when the model has one, and the line, when it has points. Either is nil when it is absent,
// and a trend with neither says nothing, which the caller draws as a dash.
func trendProps(f ir.Field, model any) c.TrendProps {
	var p c.TrendProps
	t := f.Trend
	if t == nil {
		return p
	}
	if t.Read == nil {
		return p
	}
	r := t.Read(model)
	if r.HasChange {
		d := deltaProps(t, r.Change, r.Sign)
		p.Delta = &d
	}
	if s := sparkProps(r); s.Line != "" {
		p.Spark = &s
	}
	return p
}

// deltaProps is a signed change as it is drawn: the direction it moved in, and whether that is the good one, which is a rise unless the trend
// says a lower value is the better one. A change that reads zero has no direction, and so no tone.
func deltaProps(t *ir.Trend, text string, sign int) c.DeltaProps {
	p := c.DeltaProps{Text: text, Sign: sign}
	if sign != 0 {
		good := (sign > 0) != t.Lower
		p.Tone = c.ToneCritical
		if good {
			p.Tone = c.ToneOK
		}
	}
	return p
}

// sparkProps lays out a trend's points in the box: a line through them, evenly spaced, from the lowest to the highest, with a dot on the last. It has
// no line for no points, and one point is a level line.
func sparkProps(r ir.TrendReading) c.SparkProps {
	p := c.SparkProps{W: sparkW, H: sparkH}
	vs := r.Points
	if len(vs) == 0 {
		return p
	}
	p.Label = r.Describe

	lo, hi := vs[0], vs[0]
	for _, v := range vs {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	inner := float64(sparkH - 2*sparkPad)
	span := hi - lo
	level := span == 0 || math.IsInf(span, 0) || math.IsNaN(span) // a range that does not fit in a number is drawn level
	y := func(v float64) float64 {
		if level {
			return float64(sparkH) / 2
		}
		return float64(sparkPad) + (1-(v-lo)/span)*inner
	}
	x := func(i int) float64 {
		return float64(sparkPad) + float64(i)*float64(sparkW-2*sparkPad)/float64(len(vs)-1)
	}

	if len(vs) == 1 {
		// one point is a level line, with the dot at the middle of it
		p.Line = f1(float64(sparkPad)) + "," + f1(y(vs[0])) + " " + f1(float64(sparkW-sparkPad)) + "," + f1(y(vs[0]))
		p.HasDot, p.DotX, p.DotY = true, f1(float64(sparkW)/2), f1(y(vs[0]))
		return p
	}
	var line []byte
	for i, v := range vs {
		if i > 0 {
			line = append(line, ' ')
		}
		line = append(line, f1(x(i))+","+f1(y(v))...)
	}
	last := len(vs) - 1
	p.Line = string(line)
	p.HasDot, p.DotX, p.DotY = true, f1(x(last)), f1(y(vs[last]))
	return p
}

// f1 is a coordinate, to a tenth of a unit, which is finer than a pixel.
func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
