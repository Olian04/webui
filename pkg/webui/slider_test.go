package webui

import (
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestASliderStepIsAPowerOfTenOfAHundredStepsOrMore(t *testing.T) {
	for _, tc := range []struct {
		lo, hi    float64
		precision int
		step      float64
		decimals  int
	}{
		{0, 100, 0, 1, 0},
		{0, 1, 0, 0.01, 2},
		{0, 8, 0, 0.01, 2},
		{0, 255, 0, 1, 0},
		{-50, 50, 0, 1, 0},
		{0, 50, 0, 0.1, 1},
		{0, 1000, 0, 10, 0},
		{0, 100, 1, 0.1, 1},
		{0, 100, 3, 0.001, 3},
	} {
		step, decimals := sliderStep(tc.lo, tc.hi, tc.precision)
		assert.Equal(t, step, tc.step)
		assert.Equal(t, decimals, tc.decimals)
	}
}
