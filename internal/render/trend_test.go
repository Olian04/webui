package render

import (
	"math"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

func TestASparkIsLaidOutWithoutNaN(t *testing.T) {
	t.Parallel()
	for name, points := range map[string][]float64{
		"none":          nil,
		"one":           {4},
		"level":         {3, 3, 3},
		"a rising line": {1, 2, 3},
		"a huge range":  {-math.MaxFloat64, math.MaxFloat64},
	} {
		t.Run(name, func(t *testing.T) {
			p := sparkProps(ir.TrendReading{Points: points, Describe: "x"})
			assert.Equal(t, strings.Contains(p.Line, "NaN") || strings.Contains(p.DotY, "NaN"), false)
			assert.Equal(t, p.Line == "", len(points) == 0)
		})
	}
}

func TestADeltaIsGoodWhenItMovedTheWayThatIsBetter(t *testing.T) {
	t.Parallel()
	rise := deltaProps(&ir.Trend{}, "+1.00%", 1)
	fall := deltaProps(&ir.Trend{Lower: true}, "−1.00%", -1)
	flat := deltaProps(&ir.Trend{}, "0%", 0)
	assert.Equal(t, rise.Tone, fall.Tone)
	assert.Equal(t, flat.Tone == rise.Tone, false)
}
