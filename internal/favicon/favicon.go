// Package favicon scales a logo to the square PNGs a browser asks for.
//
// The standard library has no image scaler, so this is a small area-averaging
// one: every output pixel is the average of the source pixels it covers,
// weighted by how much of each it covers, in premultiplied alpha so a
// transparent edge does not bleed its colour into the logo. A logo that is not
// square is fitted inside the square and centred, never stretched.
package favicon

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// PNG is src scaled to fit a size x size square, encoded as PNG.
func PNG(src image.Image, size int) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, Scale(src, size)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Scale is src fitted inside a size x size transparent square.
func Scale(src image.Image, size int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	b := src.Bounds()
	if b.Empty() || size <= 0 {
		return out
	}

	// The picture's size in the square, and where its top-left lands.
	ratio := math.Min(float64(size)/float64(b.Dx()), float64(size)/float64(b.Dy()))
	w := max(1, int(math.Round(float64(b.Dx())*ratio)))
	h := max(1, int(math.Round(float64(b.Dy())*ratio)))
	ox, oy := (size-w)/2, (size-h)/2

	// Work in premultiplied RGBA, from a copy that has the source's origin at 0,0.
	flat := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(flat, flat.Bounds(), src, b.Min, draw.Src)

	sx, sy := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	for y := range h {
		for x := range w {
			r, g, bl, a := average(flat, float64(x)*sx, float64(y)*sy, float64(x+1)*sx, float64(y+1)*sy)
			out.Set(ox+x, oy+y, color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(bl), A: uint16(a)})
		}
	}
	return out
}

// average is the mean of the source over the rectangle x0,y0 to x1,y1, each
// source pixel counted by the share of it the rectangle covers.
func average(src *image.RGBA, x0, y0, x1, y1 float64) (r, g, b, a float64) {
	var total float64
	for y := int(math.Floor(y0)); y < int(math.Ceil(y1)); y++ {
		wy := math.Min(float64(y+1), y1) - math.Max(float64(y), y0)
		for x := int(math.Floor(x0)); x < int(math.Ceil(x1)); x++ {
			wx := math.Min(float64(x+1), x1) - math.Max(float64(x), x0)
			wgt := wx * wy
			c := src.RGBAAt(x, y)
			r += float64(c.R) * wgt
			g += float64(c.G) * wgt
			b += float64(c.B) * wgt
			a += float64(c.A) * wgt
			total += wgt
		}
	}
	if total == 0 {
		return 0, 0, 0, 0
	}
	// 8-bit premultiplied to 16-bit, which is what color.RGBA64 takes.
	return r / total * 257, g / total * 257, b / total * 257, a / total * 257
}
