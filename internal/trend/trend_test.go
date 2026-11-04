package trend

import (
	"math"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestAChangeIsWrittenInThreeDigits(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		in   float64
		text string
		sign int
	}{
		"a hundred":               {100, "+100%", 1},
		"one decimal":             {35.4, "+35.4%", 1},
		"two decimals":            {0.35, "+0.35%", 1},
		"under ten keeps two":     {5.2, "+5.20%", 1},
		"ten keeps one":           {10, "+10.0%", 1},
		"rounds up to a hundred":  {99.96, "+100%", 1},
		"rounds up to ten":        {9.996, "+10.0%", 1},
		"a thousand keeps digits": {1234, "+1234%", 1},
		"a fall has the minus":    {-35.4, "−35.4%", -1},
		"a small fall":            {-0.35, "−0.35%", -1},
		"rounds to zero":          {0.004, "0%", 0},
		"a fall that rounds":      {-0.004, "0%", 0},
		"zero":                    {0, "0%", 0},
		"negative zero":           {math.Copysign(0, -1), "0%", 0},
		"not a number":            {math.NaN(), "—", 0},
		"infinite":                {math.Inf(1), "—", 0},
		"minus infinite":          {math.Inf(-1), "—", 0},
	} {
		t.Run(name, func(t *testing.T) {
			text, sign := Percent(tc.in)
			assert.Equal(t, text, tc.text)
			assert.Equal(t, sign, tc.sign)
		})
	}
}

func TestAValueOfASeriesIsWrittenInThreeDigits(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		in   float64
		want string
	}{
		"thousands": {1240, "1240"}, "rounds to a hundred": {99.96, "100"}, "one decimal": {35.4, "35.4"}, "two decimals": {0.35, "0.35"},
		"zero": {0, "0"}, "a fall": {-7, "−7.00"}, "a fall that rounds": {-0.001, "0"}, "not a number": {math.NaN(), "—"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, Number(tc.in), tc.want)
		})
	}
}

func TestAChangeInAFileIsANumber(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		in   float64
		want string
	}{
		"a rise": {4.4, "4.40"}, "a fall": {-35.4, "-35.4"}, "a hundred": {100, "100"}, "rounds to zero": {0.004, "0"},
		"not a number": {math.NaN(), ""}, "infinite": {math.Inf(1), ""},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, Export(tc.in), tc.want)
		})
	}
}

func TestFiniteLeavesOutWhatIsNotANumber(t *testing.T) {
	t.Parallel()
	assert.DeepEqual(t, Finite([]float64{1, math.NaN(), 2, math.Inf(1), 3}), []float64{1, 2, 3})
	assert.Equal(t, len(Finite(nil)), 0)
}

func TestASeriesIsDescribedInWords(t *testing.T) {
	t.Parallel()
	assert.Equal(t, Describe("Load", nil), "Load: no values")
	assert.Equal(t, Describe("Load", []float64{4}), "Load: 1 value, 4.00")
	assert.Equal(t, Describe("Load", []float64{3, 11, 2, 9}), "Load: 4 values, from 3.00 to 9.00, lowest 2.00, highest 11.0")
}
