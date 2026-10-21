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
// proportions and never enlarged. It is for a [FileInfo]'s Thumbnail: a Store that is handed an image makes the small one when it keeps
// the file, so that a page shows the small picture and the visitor is not sent the whole of an image to see it at 32 pixels. Twice the
// size that it is shown at is right for a screen of a high density.
//
// It reads PNG, JPEG and GIF (the first picture of one), which the standard library decodes, and no WebP, which it does not and a
// library that took an image dependency to read would carry it for every program. An [Upload] of any other kind, a picture of more than
// 25 million pixels and a damaged file are an [ErrThumbnail]. An app that wants to read more has its own scaler: a Thumbnail that the app
// made is only an address. The upload is opened twice, the second time to read the picture, since its size is read first, and each
// is closed.
func Thumbnail(up *Upload, size int) ([]byte, error) {
	if up == nil || up.Open == nil || size <= 0 {
		return nil, ErrThumbnail
	}
	var decode func(io.Reader) (image.Image, error)
	var config func(io.Reader) (image.Config, error)
	switch up.Kind {
	case FilePNG:
		decode, config = png.Decode, png.DecodeConfig
	case FileJPEG:
		decode, config = jpeg.Decode, jpeg.DecodeConfig
	case FileGIF:
		decode, config = func(r io.Reader) (image.Image, error) { return gif.Decode(r) }, gif.DecodeConfig
	case FileWebP, FileImage, FilePDF, FileJSON, FileCSV, FileText, FileAny:
		return nil, ErrThumbnail
	}
	var cfg image.Config
	if err := reading(up, func(r io.Reader) (err error) { cfg, err = config(r); return err }); err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxThumbnailPixels {
		return nil, ErrThumbnail
	}
	var img image.Image
	if err := reading(up, func(r io.Reader) (err error) { img, err = decode(r); return err }); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, favicon.Fit(img, size)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrThumbnail, err)
	}
	return out.Bytes(), nil
}

// reading opens the upload for read, and closes it. What is wrong with the picture is an ErrThumbnail, and what is wrong with the file,
// which is the app's, is its own error.
func reading(up *Upload, read func(io.Reader) error) error {
	r, err := up.Open()
	if err != nil {
		return err //nolint:wrapcheck // the app's own error, of its own file
	}
	defer func() { _ = r.Close() }() // a read-only handle
	if err := read(r); err != nil {
		return fmt.Errorf("%w: %w", ErrThumbnail, err)
	}
	return nil
}
