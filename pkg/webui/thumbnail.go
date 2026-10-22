package webui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/Olian04/webui/internal/favicon"
)

// maxThumbnailPixels is the most pixels an image may have to be scaled: it is read whole into memory, as four bytes a pixel, and
// scaled from a copy of that, so a small file that is a very large picture (which a PNG can be) is not one to open.
const maxThumbnailPixels = 25_000_000

// ErrThumbnail is why [Thumbnail] could not make one: the file is not a kind of image that it reads, or it is too large a picture, or it
// is damaged.
var ErrThumbnail = errors.New("webui: no thumbnail can be made of this file")

// Thumbnail is the picture of an uploaded image scaled down to fit inside a square of size pixels, as a PNG, with the picture's own
// proportions and never enlarged. It is for a [FileInfo]'s Thumbnail: an app that has kept an image makes the small one, so that a page shows the small
// picture and the visitor is not sent the whole of an image to see it at 32 pixels. Twice the size that it is shown at is right for a
// screen of a high density.
//
// It reads src, an image of the kind that is given (the [Upload]'s Kind), once, from its start: the file is not kept by the library, so
// an app that is to make a thumbnail has its own copy to read, from where it wrote the file or from a buffer of it. It reads PNG, JPEG and
// GIF (the first picture of one), which the standard library decodes, and no WebP, which it does not and a library that took an image
// dependency to read would carry it for every program. Any other kind, a picture of more than 25 million pixels and a damaged
// file are an [ErrThumbnail]. An app that wants to read more has its own scaler: a Thumbnail that the app made is only an address.
func Thumbnail(src io.Reader, kind FileKind, size int) ([]byte, error) {
	if src == nil || size <= 0 {
		return nil, ErrThumbnail
	}
	var decode func(io.Reader) (image.Image, error)
	var config func(io.Reader) (image.Config, error)
	switch kind {
	case FilePNG:
		decode, config = png.Decode, png.DecodeConfig
	case FileJPEG:
		decode, config = jpeg.Decode, jpeg.DecodeConfig
	case FileGIF:
		decode, config = func(r io.Reader) (image.Image, error) { return gif.Decode(r) }, gif.DecodeConfig
	case FileWebP, FileImage, FilePDF, FileJSON, FileCSV, FileText, FileAny:
		return nil, ErrThumbnail
	}
	// The size is read first, from the start of the file, which is then read again from the start by the picture's decoder: the source is read
	// once, and the bytes that were read for the size are put back in front of it.
	var head bytes.Buffer
	cfg, err := config(io.TeeReader(src, &head))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrThumbnail, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxThumbnailPixels {
		return nil, ErrThumbnail
	}
	img, err := decode(io.MultiReader(&head, src))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrThumbnail, err)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, favicon.Fit(img, size)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrThumbnail, err)
	}
	return out.Bytes(), nil
}
