// Package trend is the arithmetic behind webui.Trend: how a number is written in three digits, which way a change moved as it is
// written, and a series in words.
package trend

import (
	"fmt"
	"math"
	"strconv"
)

// none is what stands for a change that cannot be given.
const none = "—"

// minus is the minus sign, which is as wide as a plus and which a hyphen is not.
const minus = "−"

// decimals is how many decimals a number of this size has so that it reads in three digits, counting a leading zero: 0.35, 35.4, 100. A number
// of a thousand or more keeps the digits it has left of the point.
func decimals(abs float64) int {
	switch {
	case abs < 10:
		return 2
	case abs < 100:
		return 1
	}
	return 0
}

// magnitude is a number without its sign, in three digits, and the number as it is written. The decimals are those of the number after it is
// rounded, so 99.96 is "100" and not "100.0", and 9.996 is "10.0" and not "10.00".
func magnitude(abs float64) (text string, rounded float64) {
	d := decimals(abs)
	p := math.Pow10(d)
	rounded = math.Round(abs*p) / p
	if rounded == 0 {
		return "0", 0
	}
	return strconv.FormatFloat(rounded, 'f', decimals(rounded), 64), rounded
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Percent is a change, as a percent, with its sign and its unit: "+4.40%", "−35.4%", "+100%". It is the dash for a number that is not one. It
// also says which way the change moved as it is written, so that the text, the arrow and the colour agree: 1, -1, or 0 for a change that
// reads zero or that is the dash.
func Percent(v float64) (text string, sign int) {
	if !finite(v) {
		return none, 0
	}
	body, rounded := magnitude(math.Abs(v))
	switch {
	case rounded == 0:
		return "0%", 0
	case v > 0:
		return "+" + body + "%", 1
	}
	return minus + body + "%", -1
}

// Number is a value of a series in three digits, with the minus sign for a value below zero.
func Number(v float64) string {
	if !finite(v) {
		return none
	}
	body, rounded := magnitude(math.Abs(v))
	if rounded != 0 && v < 0 {
		return minus + body
	}
	return body
}

// Export is a change as a number that a file holds: in three digits, with a hyphen for a minus so that it reads as a number, no sign for a
// rise and no unit. It is empty for a number that is not one.
func Export(v float64) string {
	if !finite(v) {
		return ""
	}
	body, rounded := magnitude(math.Abs(v))
	if rounded != 0 && v < 0 {
		return "-" + body
	}
	return body
}

// Finite is the points that are numbers, in order.
func Finite(points []float64) []float64 {
	out := make([]float64, 0, len(points))
	for _, v := range points {
		if finite(v) {
			out = append(out, v)
		}
	}
	return out
}

// Describe is a series in words, for a reader who cannot see it: what it is, how many points, where it starts and ends, and its extremes.
// The points are those that Finite returns.
func Describe(label string, points []float64) string {
	switch len(points) {
	case 0:
		return label + ": no values"
	case 1:
		return label + ": 1 value, " + Number(points[0])
	}
	lo, hi := points[0], points[0]
	for _, v := range points {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return fmt.Sprintf("%s: %d values, from %s to %s, lowest %s, highest %s",
		label, len(points), Number(points[0]), Number(points[len(points)-1]), Number(lo), Number(hi))
}
