package main

import (
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strings"
)

// serveArt answers /art/{name}.png with a picture made from the name: a gradient, so that the same name is the same picture every time. It is
// what stands in for the thumbnails and avatars a real app would have, and it is on this site, so the page's content security policy lets the browser
// fetch it with no origin trusted.
func serveArt(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(r.PathValue("name"), ".png")
	if name == "" || len(name) > 64 {
		http.NotFound(w, r)
		return
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	seed := h.Sum32()
	from, to := hue(float64(seed%360)), hue(float64((seed/360)%360+90))

	const width, height = 320, 180
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			t := (float64(x)/width + float64(y)/height) / 2
			img.Set(x, y, mix(from, to, t))
		}
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = png.Encode(w, img)
}

// hue is a colour of a fixed lightness and saturation at a place on the colour wheel, in degrees.
func hue(deg float64) color.RGBA {
	const s, l = 0.55, 0.42
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(deg/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch int(deg/60) % 6 {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{R: uint8((r + m) * 255), G: uint8((g + m) * 255), B: uint8((b + m) * 255), A: 255}
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	lerp := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.RGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: 255}
}
