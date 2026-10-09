package main

import (
	"image"
	"image/color"
	"math"
)

// logo draws the brand mark: a ring that fades from orange to red. Brand.Logo is
// any image.Image; the app serves it as a PNG beside the stylesheet.
func logo() image.Image {
	const size, outer, inner = 64, 30.0, 18.0
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	centre := float64(size-1) / 2
	for y := range size {
		for x := range size {
			d := math.Hypot(float64(x)-centre, float64(y)-centre)
			if d > outer || d < inner {
				continue
			}
			t := float64(x+y) / (2 * size)
			img.Set(x, y, color.RGBA{
				R: uint8(255 - 26*t), G: uint8(152 - 80*t), B: uint8(48 + 44*t), A: 255,
			})
		}
	}
	return img
}
