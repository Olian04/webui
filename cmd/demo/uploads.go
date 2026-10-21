package main

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/Olian04/webui/pkg/webui"
)

// The library keeps no uploaded file: Store is handed an Upload to copy where the app keeps them, and Load says what is kept. Here
// they are kept in memory, in the service, and served from /uploads/ below, which is the app's own address: a File names it as the
// URL of what it holds.

// maxLogo and maxCertificate are the most that each takes, as the fields say, and the most that is read of an upload.
const (
	maxLogo        = 1 << 20
	maxCertificate = 5 << 20
)

// held is the file a Blob is, as a File shows what is kept.
func held(b *Blob, name string) *webui.FileInfo {
	if b == nil {
		return nil
	}
	return &webui.FileInfo{Name: b.Name, Size: int64(len(b.Data)), URL: "/uploads/" + name}
}

// keep copies an upload into where the app holds it. An upload is only open until the action has run, so it is copied here, within
// the limit that the field set, which the library has checked and the copy is held to again. A nil upload is a file that the
// visitor asked to remove.
func keep(dst **Blob, up *webui.Upload, limit int64) {
	if up == nil {
		*dst = nil
		return
	}
	r, err := up.Open()
	if err != nil {
		return // what was held stays
	}
	defer func() { _ = r.Close() }() // a read-only handle: there is nothing to lose if closing fails
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || int64(len(data)) > limit {
		return
	}
	*dst = &Blob{Name: up.Name, Type: up.ContentType, Data: data}
}

var (
	// An image of at most a megabyte. The kind is what the file holds, and not what it is called: a text file named logo.png is refused.
	Logo = webui.File[Settings]{
		Label: "Report logo", Accept: []webui.FileKind{webui.FileImage}, MaxSize: maxLogo,
		Help:  "The image at the top of each report: a PNG, JPEG, GIF or WebP file.",
		Load:  func(s Settings) *webui.FileInfo { return held(s.Logo, "logo") },
		Store: func(s *Settings, up *webui.Upload) { keep(&s.Logo, up, maxLogo) },
	}
	// A file that may be removed, and not only replaced: a checkbox beside it stores nil.
	Certificate = webui.File[Settings]{
		Label: "Certificate (PDF)", Accept: []webui.FileKind{webui.FilePDF}, MaxSize: maxCertificate, Removable: true,
		Load:  func(s Settings) *webui.FileInfo { return held(s.CertFile, "certificate") },
		Store: func(s *Settings, up *webui.Upload) { keep(&s.CertFile, up, maxCertificate) },
	}
)

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
