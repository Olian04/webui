package webui

import (
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestASliderThatHasNoStepGetsAPowerOfTenOfAHundredStepsOrMore(t *testing.T) {
	for _, tc := range []struct {
		lo, hi, step float64
	}{
		{0, 100, 1}, {0, 1, 0.01}, {0, 8, 0.01}, {0, 255, 1}, {-50, 50, 1}, {0, 50, 0.1}, {0, 1000, 10},
	} {
		assert.Equal(t, defaultStep(tc.lo, tc.hi), tc.step)
	}
}

func TestASlidersValueIsTheNearestStepWithTheDecimalsOfIt(t *testing.T) {
	read := func(v, lo, step float64) string {
		get, num := sliderReads(func(x float64) float64 { return x }, lo, step)
		assert.Equal(t, num(v), v)
		return get(v)
	}
	assert.Equal(t, read(81.6719540426991, 0, 1), "82")
	assert.Equal(t, read(671.95, 0, 10), "670")
	assert.Equal(t, read(0.30000000000000004, 0, 0.1), "0.3")
	assert.Equal(t, read(0.6719, 0, 0.01), "0.67")
	assert.Equal(t, read(0.6719, 0, 0.25), "0.75")
	assert.Equal(t, read(67, 2, 5), "67")
	assert.Equal(t, read(68, 2, 5), "67")
	assert.Equal(t, read(81.6719540426991, 0, 0), "81.6719540426991") // no step: as it is
	assert.Equal(t, read(3, 0, 0), "3")
}
