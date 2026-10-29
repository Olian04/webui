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
	sparkH   = 28
	sparkPad = 3 // room for the stroke and the mark at the edges
)

// deltaProps is a signed change as it is drawn: the direction it moved in, and whether that is the good one, which is a rise unless the
// field says a lower value is the better one. A change that reads zero has no direction, and so no tone.
func deltaProps(f ir.Field, text string, model any) c.DeltaProps {
	sign := 0
	if f.Delta != nil && f.Delta.Sign != nil {
		sign = f.Delta.Sign(model)
	}
	p := c.DeltaProps{Text: text, Sign: sign}
	if sign != 0 {
		good := (sign > 0) != (f.Delta != nil && f.Delta.Lower)
		p.Tone = c.ToneCritical
		if good {
			p.Tone = c.ToneOK
		}
	}
	return p
}

// sparkProps lays out a series in the box. A line runs through the points, evenly spaced, from the lowest to the highest of them; bars stand
// on zero, so that their heights are the values and not only their differences. A point that is not a number is left out, and the index
// that a Highlight gives is of the points as they were loaded.
func sparkProps(f ir.Field, model any) c.SparkProps {
	s := f.Series
	p := c.SparkProps{W: sparkW, H: sparkH}
	if s == nil || s.Values == nil {
		return p
	}
	p.Bars = s.Bars
	if s.Describe != nil {
		p.Label = s.Describe(model)
	}

	var vs []float64
	var from []int // the index each point had as it was loaded
	for i, v := range s.Values(model) {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			vs, from = append(vs, v), append(from, i)
		}
	}
	if len(vs) == 0 {
		return p
	}
	marked := -1 // the index in vs of the point that is marked
	if s.Highlight != nil {
		at := s.Highlight(model)
		for i, orig := range from {
			if orig == at {
				marked = i
			}
		}
	} else if !s.Bars {
		marked = len(vs) - 1
	}

	lo, hi := vs[0], vs[0]
	for _, v := range vs {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if s.Bars {
		lo, hi = math.Min(lo, 0), math.Max(hi, 0)
	}
	inner := float64(sparkH - 2*sparkPad)
	y := func(v float64) float64 {
		if hi == lo {
			return float64(sparkH) / 2
		}
		return float64(sparkPad) + (1-(v-lo)/(hi-lo))*inner
	}
	n := f1

	if s.Bars {
		slot := float64(sparkW-2*sparkPad) / float64(len(vs))
		width := math.Max(1, slot*0.7)
		base := y(0)
		for i, v := range vs {
			top, bottom := y(v), base
			if top > bottom {
				top, bottom = bottom, top
			}
			height := math.Max(1, bottom-top) // a value of zero is still a mark, so that it is there
			p.Rects = append(p.Rects, c.SparkRect{
				X: n(float64(sparkPad) + slot*float64(i) + (slot-width)/2), Y: n(top), W: n(width), H: n(height), Mark: i == marked,
			})
		}
		return p
	}

	x := func(i int) float64 {
		if len(vs) == 1 {
			return float64(sparkW) / 2
		}
		return float64(sparkPad) + float64(i)*float64(sparkW-2*sparkPad)/float64(len(vs)-1)
	}
	var line []byte
	for i, v := range vs {
		if i > 0 {
			line = append(line, ' ')
		}
		line = append(line, n(x(i))+","+n(y(v))...)
	}
	p.Line = string(line)
	if len(vs) == 1 {
		p.Line = n(float64(sparkPad)) + "," + n(y(vs[0])) + " " + n(float64(sparkW-sparkPad)) + "," + n(y(vs[0])) // one point is a level line
	}
	if marked >= 0 {
		p.HasDot, p.DotX, p.DotY = true, n(x(marked)), n(y(vs[marked]))
	}
	return p
}

// f1 is a coordinate, to a tenth of a unit, which is finer than a pixel.
func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
