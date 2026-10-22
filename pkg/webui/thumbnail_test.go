package webui

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func solidImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 200, 60, 20, 255
	}
	return img
}

func uploadOf(kind FileKind, data []byte) *Upload {
	return &Upload{Kind: kind, Size: int64(len(data)), Data: data}
}

func sizeOf(t *testing.T, data []byte) image.Point {
	t.Helper()
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	assert.NoError(t, err) // the thumbnail is a PNG, whatever the image was
	return image.Point{X: cfg.Width, Y: cfg.Height}
}

func TestAThumbnailKeepsTheProportionsAndTheLongestSideIsTheSize(t *testing.T) {
	t.Parallel()

	var p, j, g bytes.Buffer
	assert.NoError(t, png.Encode(&p, solidImage(400, 200)))
	assert.NoError(t, jpeg.Encode(&j, solidImage(300, 600), nil))
	assert.NoError(t, gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 90, 30), color.Palette{color.Black, color.White}), nil))

	for _, c := range []struct {
		name string
		up   *Upload
		want image.Point
	}{
		{"a wide png", uploadOf(FilePNG, p.Bytes()), image.Point{X: 64, Y: 32}},
		{"a tall jpeg", uploadOf(FileJPEG, j.Bytes()), image.Point{X: 32, Y: 64}},
		{"a gif", uploadOf(FileGIF, g.Bytes()), image.Point{X: 64, Y: 21}},
	} {
		got, err := Thumbnail(c.up, 64)
		assert.NoError(t, err)
		assert.Equal(t, sizeOf(t, got), c.want)
	}
}

// A picture that is already small is not enlarged, and the thumbnail is much smaller than a large image.
func TestAThumbnailNeverEnlargesAndIsMuchSmallerThanTheImage(t *testing.T) {
	t.Parallel()

	var small, big bytes.Buffer
	assert.NoError(t, png.Encode(&small, solidImage(20, 10)))
	got, err := Thumbnail(uploadOf(FilePNG, small.Bytes()), 64)
	assert.NoError(t, err)
	assert.Equal(t, sizeOf(t, got), image.Point{X: 20, Y: 10})

	// Noise does not compress, so the large image is large as a file too.
	noise := image.NewNRGBA(image.Rect(0, 0, 1500, 1000))
	// A fixed seed, for a test that is the same each time: it is noise to resist compression and not a secret.
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: not for anything that needs to be unpredictable.
	for i := range noise.Pix {
		noise.Pix[i] = byte(rng.Uint32() & 0xff) // the low eight bits, which is what noise needs
	}
	assert.NoError(t, png.Encode(&big, noise))
	got, err = Thumbnail(uploadOf(FilePNG, big.Bytes()), 64)
	assert.NoError(t, err)
	assert.True(t, len(got) < big.Len()/100)
	assert.Equal(t, sizeOf(t, got), image.Point{X: 64, Y: 43})
}

func TestAThumbnailIsNotMadeOfWhatItDoesNotRead(t *testing.T) {
	t.Parallel()

	var p bytes.Buffer
	assert.NoError(t, png.Encode(&p, solidImage(10, 10)))
	for _, up := range map[string]*Upload{
		"a nil upload":      nil,
		"no data":           {Kind: FilePNG},
		"a webp":            uploadOf(FileWebP, p.Bytes()),
		"a pdf":             uploadOf(FilePDF, p.Bytes()),
		"any file":          uploadOf(FileAny, p.Bytes()),
		"damaged":           uploadOf(FilePNG, p.Bytes()[:30]),
		"a jpeg that isn't": uploadOf(FileJPEG, p.Bytes()),
	} {
		_, err := Thumbnail(up, 64)
		assert.True(t, errors.Is(err, ErrThumbnail))
	}
	_, err := Thumbnail(uploadOf(FilePNG, p.Bytes()), 0)
	assert.True(t, errors.Is(err, ErrThumbnail))
}

// A small file can be a very large picture, which is read whole into memory to be scaled: it is refused by its size before it is read.
func TestAThumbnailIsNotMadeOfAPictureOfMoreThan25MillionPixels(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	assert.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 5100, 5100)))) // 26 million pixels, and a file of a few kilobytes
	assert.True(t, buf.Len() < 200_000)
	_, err := Thumbnail(uploadOf(FilePNG, buf.Bytes()), 64)
	assert.True(t, errors.Is(err, ErrThumbnail))

	buf.Reset()
	assert.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 4000, 4000)))) // 4k, and more, is fine
	got, err := Thumbnail(uploadOf(FilePNG, buf.Bytes()), 64)
	assert.NoError(t, err)
	assert.Equal(t, sizeOf(t, got), image.Point{X: 64, Y: 64})
}
