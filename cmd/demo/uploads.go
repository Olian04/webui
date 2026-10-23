package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

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
	info := &webui.FileInfo{Name: b.Name, Size: int64(len(b.Data)), URL: "/uploads/" + name}
	if len(b.Thumb) > 0 {
		info.Thumbnail = "/uploads/" + name + "/thumb" // the small picture that the page shows, and not the image
	}
	return info
}

// keep copies an upload into where the app holds it. An upload is only held until the action has run, so it is copied here, within the
// limit that the field set, which the library has checked and the copy is held to again. A nil upload is a file that the visitor asked to
// remove.
func keep(dst **Blob, up *webui.Upload, limit int64) {
	if up == nil {
		*dst = nil
		return
	}
	if int64(len(up.Data)) > limit {
		return // what was held stays
	}
	blob := &Blob{Name: up.Name, Type: up.ContentType, Data: bytes.Clone(up.Data)}
	// An image gets a small picture of itself when it is kept, twice the 32 pixels that a page shows it at, so that the page is not sent
	// the whole of it to show a thumbnail; the full image is fetched only when the visitor opens it. Thumbnail reads PNG, JPEG and GIF: for
	// a WebP there is none, and the page shows the name alone, which is what an app that does not scale a kind of image gets.
	if thumb, err := webui.Thumbnail(up, 64); err == nil {
		blob.Thumb = thumb
	}
	*dst = blob
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

// FirmwareChoice is what a visitor chooses beside the firmware image, which is the model of the upload panel: the options, with the defaults that Load gives.
type FirmwareChoice struct {
	Taken   string // what to do when an image of the same name was uploaded already: "refuse" it, or "replace" it, which removes the old one from the history
	Channel string // "stable" or "beta"
}

// A file too large to be held, which is a panel of its own and not a field of a form: a firmware image of up to 2 GiB. Receive is handed the file as
// it arrives, and reads it from the Reader wherever it is to go, which for an app is an object store. Here it is hashed, as it is read, and only what it
// was is kept: nothing of the file is held, in memory or on disk. The Guard is asked when the page is drawn, which is how a viewer sees a panel that is
// disabled and says why, and again when the file arrives, which is what refuses a post that is made by hand.
var FirmwarePage = webui.Page[webui.NoArgs]{
	Path: "/firmware",
	Nav:  webui.Nav{Label: "Firmware", Icon: "database", Section: "Operations"},
	Body: webui.Stack{
		webui.FileUpload[FirmwareChoice]{
			Title: "Upload firmware", Desc: "A FileUpload: one file, of any size up to its MaxSize, handed to Receive as it arrives. The demo hashes it as it is read and keeps only what it was.",
			Help:    "The image to roll out to the collectors. It is hashed on the way in, and not kept.",
			MaxSize: 2 << 30,
			// With script, several files and folders can be chosen and are sent one after another, with a progress bar and a result for each.
			Multiple: true,
			// What is chosen beside the file: fields of a form, checked before any of the file is read, and handed to Receive as f.Options.
			Load: func(context.Context) (FirmwareChoice, webui.Loaded) {
				return FirmwareChoice{Taken: "refuse", Channel: "stable"}, webui.Ready()
			},
			Fields: []webui.Accessor[FirmwareChoice]{
				webui.Select[FirmwareChoice]{
					Label: "If an image has the same name", Required: true,
					Options: []webui.Option{{Value: "refuse", Label: "Refuse it"}, {Value: "replace", Label: "Replace it"}},
					Load:    func(c FirmwareChoice) []string { return []string{c.Taken} },
					Store:   func(c *FirmwareChoice, v []string) { c.Taken = v[0] },
				},
				webui.Select[FirmwareChoice]{
					Label: "Release channel", Required: true,
					Options: []webui.Option{{Value: "stable", Label: "Stable"}, {Value: "beta", Label: "Beta"}},
					Load:    func(c FirmwareChoice) []string { return []string{c.Channel} },
					Store:   func(c *FirmwareChoice, v []string) { c.Channel = v[0] },
				},
			},
			Receive: webui.Action[webui.FileHandle[FirmwareChoice]]{
				Label: "Upload",
				Guard: func(ctx context.Context, _ webui.FileHandle[FirmwareChoice]) error {
					if RoleOf(ctx) != Editor {
						return webui.Disabled("Requires the editor role")
					}
					return nil
				},
				Run: func(_ context.Context, f webui.FileHandle[FirmwareChoice]) (webui.Outcome, error) {
					// A choice that is made beside the file is known before it is read, so a refusal costs none of it: what Run did not read is thrown away.
					fw := Firmware{Name: f.Name, Folder: f.Path}
					replace := f.Options.Taken == "replace"
					if service.HasFirmware(fw.Key()) && !replace {
						return webui.Failure(fw.Key() + " was uploaded already: choose to replace it to upload it again"), nil
					}
					sum := sha256.New()
					size, err := io.Copy(sum, f.Reader) // all of it, as it comes: this is how the file is known to be whole
					if err != nil {
						return webui.Outcome{}, err //nolint:wrapcheck // a failing read of the body, which the library answers
					}
					digest := hex.EncodeToString(sum.Sum(nil))
					fw.Channel, fw.Type, fw.Size, fw.SHA256, fw.At = f.Options.Channel, f.DeclaredType, int(size), digest, time.Now().Format("Jan 2 15:04")
					if !service.SetFirmware(fw, replace) { // another upload of the name came first, while this one was being read
						return webui.Failure(fw.Key() + " was uploaded already: choose to replace it to upload it again"), nil
					}
					return webui.Success("Received " + f.Name + ", sha256 " + digest[:12] + "…"), nil
				},
			},
		},
		// Every image that was uploaded, newest first: what was kept of each, and none of the file.
		webui.Table[Firmware]{
			Title: "Upload history", Desc: "Every image that was uploaded, newest first. Only what it was is kept: the file is hashed as it arrives and never held.",
			Rows: func(context.Context) ([]Firmware, webui.Loaded) { return service.FirmwareHistory(), webui.Ready() },
			Columns: []webui.Accessor[Firmware]{
				webui.String[Firmware]{Label: "File", Load: func(f Firmware) string { return f.Name }},
				webui.String[Firmware]{Label: "Folder", Load: func(f Firmware) string { return f.Folder }},
				webui.String[Firmware]{Label: "Channel", Load: func(f Firmware) string { return f.Channel }},
				webui.String[Firmware]{Label: "Type said by the browser", Load: func(f Firmware) string { return f.Type }},
				webui.Int[Firmware]{Label: "Bytes", Load: func(f Firmware) int { return f.Size }},
				webui.String[Firmware]{Label: "SHA-256", Load: func(f Firmware) string { return f.SHA256 }},
				webui.String[Firmware]{Label: "Received", Load: func(f Firmware) string { return f.At }},
			},
		},
	},
}
