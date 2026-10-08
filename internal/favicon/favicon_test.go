package favicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func TestAWideLogoIsFittedAndCentredNotStretched(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	got := Scale(solid(64, 32, red), 16) // 2:1 into a square: 16 wide, 8 tall

	if at := got.NRGBAAt(8, 8); at != red {
		t.Fatalf("the middle is %v, want the logo", at)
	}
	if at := got.NRGBAAt(8, 1); at.A != 0 {
		t.Fatalf("above the picture is %v, want transparent", at)
	}
	if at := got.NRGBAAt(8, 14); at.A != 0 {
		t.Fatalf("below the picture is %v, want transparent", at)
	}
}

func TestScalingKeepsAColourAndItsOpacity(t *testing.T) {
	c := color.NRGBA{R: 10, G: 200, B: 90, A: 255}
	got := Scale(solid(100, 100, c), 32)
	for _, p := range [][2]int{{0, 0}, {16, 16}, {31, 31}} {
		if at := got.NRGBAAt(p[0], p[1]); at != c {
			t.Fatalf("pixel %v is %v, want %v", p, at, c)
		}
	}
}

func TestATransparentEdgeDoesNotBleedItsColourIn(t *testing.T) {
	// Opaque white left half, fully transparent "black" right half.
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	got := Scale(img, 2)
	if at := got.NRGBAAt(0, 0); at != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("the opaque side is %v", at)
	}
	if at := got.NRGBAAt(1, 0); at.A != 0 {
		t.Fatalf("the transparent side is %v", at)
	}
}

func TestPNGEncodesTheRequestedSizeAndSmallLogosScaleUp(t *testing.T) {
	data, err := PNG(solid(4, 4, color.NRGBA{B: 255, A: 255}), 32)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Fatalf("size %v", b)
	}
}

func TestAnEmptyImageIsATransparentSquare(t *testing.T) {
	got := Scale(image.NewNRGBA(image.Rectangle{}), 8)
	if got.Bounds().Dx() != 8 || got.NRGBAAt(4, 4).A != 0 {
		t.Fatal("want an empty 8x8")
	}
}
