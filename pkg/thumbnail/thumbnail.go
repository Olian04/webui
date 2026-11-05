// Package thumbnail makes the small picture of an image that a [webui.FileInfo] shows beside a file: the picture of an uploaded image scaled down to fit a square, as a PNG. It reads PNG, JPEG
// and GIF with the standard library, and nothing else, and is its own package so that a program that does not make thumbnails does not link the scaler.
package thumbnail

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
	"github.com/Olian04/webui/pkg/webui"
)

// maxPixels is the most pixels an image may have to be scaled: it is read whole into memory, as four bytes a pixel, and
// scaled from a copy of that, so a small file that is a very large picture (which a PNG can be) is not one to open.
const maxPixels = 25_000_000

// ErrUnavailable is why [Make] could not make a thumbnail: the file is not a kind of image that it reads, or it is too large a picture, or it
// is damaged.
var ErrUnavailable = errors.New("thumbnail: no thumbnail can be made of this file")

// Make is the picture of an uploaded image scaled down to fit inside a square of size pixels, as a PNG, with the picture's own
// proportions and never enlarged. It is for a [webui.FileInfo]'s Thumbnail: a Store that is handed an image makes the small one when it keeps
// the file, so that a page shows the small picture and the visitor is not sent the whole of an image to see it at 32 pixels. Twice the
// size that it is shown at is right for a screen of a high density.
//
// It reads PNG, JPEG and GIF (the first picture of one), which the standard library decodes, and no WebP, which it does not and a
// library that took an image dependency to read would carry it for every program. An [webui.Upload] of any other kind, a picture of more than
// 25 million pixels and a damaged file are an [ErrUnavailable]. An app that wants to read more has its own scaler: a Thumbnail that the app
// made is only an address.
func Make(up *webui.Upload, size int) ([]byte, error) {
	if up == nil || up.Data == nil || size <= 0 {
		return nil, ErrUnavailable
	}
	var decode func(io.Reader) (image.Image, error)
	var config func(io.Reader) (image.Config, error)
	switch up.Kind {
	case webui.FilePNG:
		decode, config = png.Decode, png.DecodeConfig
	case webui.FileJPEG:
		decode, config = jpeg.Decode, jpeg.DecodeConfig
	case webui.FileGIF:
		decode, config = func(r io.Reader) (image.Image, error) { return gif.Decode(r) }, gif.DecodeConfig
	case webui.FileWebP, webui.FileImage, webui.FilePDF, webui.FileJSON, webui.FileCSV, webui.FileText, webui.FileAny:
		return nil, ErrUnavailable
	}
	cfg, err := config(bytes.NewReader(up.Data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, ErrUnavailable
	}
	img, err := decode(bytes.NewReader(up.Data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, favicon.Fit(img, size)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return out.Bytes(), nil
}
