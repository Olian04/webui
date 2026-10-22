package main

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/Olian04/webui/pkg/webui"
)

// The library keeps no uploaded file: Open returns the writer that each one is written to as it arrives, and a token for it, and Load says what
// is kept. Here they are kept in memory, in the service, and served from /uploads/ below, which is the app's own address: a File names it as the
// URL of what it holds. An app that keeps files elsewhere writes to that: an object store would start its multipart upload under a temporary key in
// Open and return the key as the token, write each chunk to it, complete it in Close, rename it to its own key in Commit, and delete it in
// Discard, with no copy of the file on this server's disk at any point.

// maxLogo and maxCertificate are the most that each takes, which the library holds each file to as it is written.
const (
	maxLogo        = 1 << 20
	maxCertificate = 5 << 20
)

// held is the file a Blob is, as a File shows what is kept.
func held(b *Blob, name string) *webui.FileInfo {
	if b == nil {
		return nil
	}
	info := &webui.FileInfo{Name: b.Name, Size: int64(len(b.Data)), URL: "/uploads/" + name}
	if len(b.Thumb) > 0 {
		info.Thumbnail = "/uploads/" + name + "/thumb" // the small picture that the page shows, and not the image
	}
	return info
}

// pending is a file while it is received: a buffer that is written to and can be closed, which is nothing to do for memory, and what is known of it,
// which is the token that Commit is handed. A file that is not committed is dropped with it.
type pending struct {
	bytes.Buffer
	in webui.Incoming
}

func (*pending) Close() error { return nil }

// open is where a file goes while it is received.
func open(_ context.Context, _ Settings, in webui.Incoming) (io.WriteCloser, *pending, error) {
	p := &pending{in: in}
	return p, p, nil
}

// commit keeps the file of the field in slot, which is what is done when the submission is accepted, after the settings are saved, so that
// what is kept is not overwritten by them. The kind and the content type are what the first bytes said, which is all that an image or a PDF
// is told by.
func commit(slot string) func(context.Context, Settings, *pending) error {
	return func(_ context.Context, _ Settings, p *pending) error {
		blob := &Blob{Name: p.in.Name, Type: p.in.ContentType, Data: p.Bytes()}
		// An image gets a small picture of itself when it is kept, twice the 32 pixels that a page shows it at, so that the page is not
		// sent the whole of it to show a thumbnail; the full image is fetched only when the visitor opens it. Thumbnail reads PNG, JPEG and
		// GIF: for a WebP there is none, and the page shows the name alone, which is what an app that does not scale a kind of image gets.
		if thumb, err := webui.Thumbnail(bytes.NewReader(blob.Data), p.in.Kind, 64); err == nil {
			blob.Thumb = thumb
		}
		service.SetBlob(slot, blob)
		return nil
	}
}

var (
	// An image of at most a megabyte. The kind is what the file holds, and not what it is called: a text file named logo.png is refused.
	Logo = webui.File[Settings, *pending]{
		Label: "Report logo", Accept: []webui.FileKind{webui.FileImage}, MaxSize: maxLogo,
		Help:   "The image at the top of each report: a PNG, JPEG, GIF or WebP file.",
		Load:   func(s Settings) *webui.FileInfo { return held(s.Logo, "logo") },
		Open:   open,
		Commit: commit("logo"),
		Store:  func(*Settings, *webui.Upload) {}, // the file is kept by Commit, and a logo is only replaced, never removed
	}
	// A file that may be removed, and not only replaced: a checkbox beside it stores nil.
	Certificate = webui.File[Settings, *pending]{
		Label: "Certificate (PDF)", Accept: []webui.FileKind{webui.FilePDF}, MaxSize: maxCertificate, Removable: true,
		Load:   func(s Settings) *webui.FileInfo { return held(s.CertFile, "certificate") },
		Open:   open,
		Commit: commit("certificate"),
		Store: func(s *Settings, up *webui.Upload) { // a new file is kept by Commit; a removed one is gone from the model
			if up == nil {
				s.CertFile = nil
			}
		},
	}
)

// serveThumb sends back the small picture of an upload that was made when it was kept, which is a PNG that the app made and so has no
// need of the care that a visitor's own file has; it is sent the same way.
func serveThumb(w http.ResponseWriter, r *http.Request) {
	settings := service.Settings()
	var b *Blob
	switch r.PathValue("name") {
	case "logo":
		b = settings.Logo
	case "certificate":
		b = settings.CertFile
	}
	if b == nil || len(b.Thumb) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(b.Thumb)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b.Thumb)
}

// serveUpload sends back a file that was uploaded, as a visitor's file is to be sent: as a download (attachment), with the content type
// that it was found to have and with nosniff, so that a browser does not decide for itself what it is and run it. Sent inline from this
// origin, a file that someone chose is a page of the app's.
func serveUpload(w http.ResponseWriter, r *http.Request) {
	settings := service.Settings()
	var b *Blob
	switch r.PathValue("name") {
	case "logo":
		b = settings.Logo
	case "certificate":
		b = settings.CertFile
	}
	if b == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", b.Type)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": b.Name}))
	w.Header().Set("Content-Length", strconv.Itoa(len(b.Data)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b.Data)
}
