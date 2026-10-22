package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

var (
	demoPNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 64)...)
	demoPDF = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF\n")
)

type upload struct {
	field, name string
	data        []byte
}

// saveIngest posts the Ingest form with the values that it needs and the files given, as a browser does.
func saveIngest(h http.Handler, extra map[string]string, files ...upload) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"_leaf": "p", "f0_0": "eu-north-1", "f0_1": "8125", "f1_0": "0.5", "f1_1": "60"} {
		_ = w.WriteField(k, v)
	}
	for k, v := range extra {
		_ = w.WriteField(k, v)
	}
	for _, f := range files {
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="`+f.field+`"; filename="`+f.name+`"`)
		hdr.Set("Content-Type", "application/octet-stream")
		p, _ := w.CreatePart(hdr)
		_, _ = p.Write(f.data)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/admin/settings", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTheIngestFormTakesAnImageAndAPDFAndServesThemAsDownloads(t *testing.T) {
	h := handler(t)
	form := get(h, "/admin/settings").Body.String()
	assert.Contains(t, form, `enctype="multipart/form-data"`)
	assert.Contains(t, form, `name="f9_0" accept="image/png,.png,image/jpeg,.jpg,.jpeg,image/gif,.gif,image/webp,.webp"`)
	assert.Contains(t, form, `name="f9_1" accept="application/pdf,.pdf"`)
	assert.Contains(t, form, "Up to 1 MiB")

	rec := saveIngest(h, nil, upload{"f9_0", "../../logo.png", demoPNG}, upload{"f9_1", "cert.pdf", demoPDF})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	after := get(h, "/admin/settings").Body.String()
	assert.Contains(t, after, `<a href="/uploads/logo">logo.png</a>`)
	assert.Contains(t, after, `<a href="/uploads/certificate">cert.pdf</a>`)
	assert.Contains(t, after, `name="_rm" value="f9_1"`) // the certificate can be removed, the logo only replaced
	assert.False(t, strings.Contains(after, `name="_rm" value="f9_0"`))

	// What is served is a download, of the type that the file holds, which a browser is told not to second-guess.
	file := get(h, "/uploads/logo")
	assert.Equal(t, file.Code, http.StatusOK)
	assert.Equal(t, file.Header().Get("Content-Type"), "image/png")
	assert.Equal(t, file.Header().Get("X-Content-Type-Options"), "nosniff")
	assert.Equal(t, file.Header().Get("Content-Disposition"), `attachment; filename=logo.png`)
	assert.DeepEqual(t, file.Body.Bytes(), demoPNG)

	// A file left alone is kept, and a certificate that is removed is gone.
	assert.Equal(t, saveIngest(h, nil).Code, http.StatusSeeOther)
	assert.Equal(t, get(h, "/uploads/logo").Code, http.StatusOK)
	assert.Equal(t, saveIngest(h, map[string]string{"_rm": "f9_1"}).Code, http.StatusSeeOther)
	assert.Equal(t, get(h, "/uploads/certificate").Code, http.StatusNotFound)
	assert.Equal(t, get(h, "/uploads/logo").Code, http.StatusOK)
}

func TestTheIngestFormRefusesAFileThatIsNotTheKindWhateverItIsCalled(t *testing.T) {
	h := handler(t)
	rec := saveIngest(h, nil, upload{"f9_0", "logo.png", []byte("not an image at all")}, upload{"f9_1", "cert.pdf", demoPNG})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	assert.Contains(t, body, "Choose a PNG, JPEG, GIF or WebP file.")
	assert.Contains(t, body, "Choose a PDF file.")
	assert.Equal(t, get(h, "/uploads/logo").Code, http.StatusNotFound)

	big := saveIngest(h, nil, upload{"f9_0", "big.png", append(demoPNG, bytes.Repeat([]byte{1}, 1<<20)...)})
	assert.Equal(t, big.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, big.Body.String(), "Must be at most 1 MiB.")
}

func TestAViewerCannotUploadAndHasNoChooser(t *testing.T) {
	editor := handler(t)
	assert.Equal(t, saveIngest(editor, nil, upload{"f9_0", "logo.png", demoPNG}).Code, http.StatusSeeOther)

	viewer := handlerAs(t, Viewer)
	rec := saveIngest(viewer, nil, upload{"f9_0", "other.png", demoPNG})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "erequires the editor role")

	// The form of a viewer shows the file and has no chooser.
	body := get(viewer, "/admin/settings").Body.String()
	assert.False(t, strings.Contains(body, `type="file"`))
	assert.False(t, strings.Contains(body, "enctype"))
}

// With script the chooser is a drop zone, and the page tells the script what each field takes.
func TestTheIngestFormLinksTheFileScriptAndTellsItWhatEachFieldTakes(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `/_webui/file.`)
	assert.Contains(t, body, `&#34;maxMessage&#34;:&#34;Must be at most 1 MiB.&#34;`)

	// A viewer has no chooser. The page still links the script, which is the page's and not the visitor's (the Guard that refuses them is
	// asked per request), and it has nothing to do there.
	viewer := get(handlerAs(t, Viewer), "/admin/settings").Body.String()
	assert.False(t, strings.Contains(viewer, `data-file=`))
}

// bigPicture is a PNG of 2000 by 1000, which is a few kilobytes as a file and a lot as a picture.
func bigPicture(t *testing.T) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 2000, 1000))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 47, 158, 143, 255
	}
	var buf bytes.Buffer
	assert.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// A stored image is shown as a small picture that the app made when it kept the file, and not as the image shrunk: the page is not sent the
// whole of it to show 32 pixels. The picture links to the full file, which is only fetched when it is opened.
func TestAStoredImageIsShownAsAThumbnailTheAppMadeAndLinksToTheFullImage(t *testing.T) {
	h := handler(t)
	big := bigPicture(t)
	assert.Equal(t, saveIngest(h, nil, upload{"f9_0", "photo.png", big}, upload{"f9_1", "cert.pdf", demoPDF}).Code, http.StatusSeeOther)

	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `command="show-modal" aria-haspopup="dialog" aria-label="Preview photo.png" title="Preview photo.png"><img class="file-thumb" src="/uploads/logo/thumb" alt=""></button>`)
	assert.Contains(t, body, `<img class="preview-full" src="/uploads/logo" alt="photo.png" loading="lazy" decoding="async">`) // fetched only when the dialog opens
	assert.False(t, strings.Contains(body, "/uploads/certificate/thumb"))                                                      // a PDF has no picture

	small := get(h, "/uploads/logo/thumb")
	assert.Equal(t, small.Code, http.StatusOK)
	assert.Equal(t, small.Header().Get("Content-Type"), "image/png")
	cfg, err := png.DecodeConfig(bytes.NewReader(small.Body.Bytes()))
	assert.NoError(t, err)
	assert.Equal(t, [2]int{cfg.Width, cfg.Height}, [2]int{64, 32})
	assert.True(t, small.Body.Len() < 1000)

	// The full image is the file as it was uploaded.
	full := get(h, "/uploads/logo")
	assert.DeepEqual(t, full.Body.Bytes(), big)
	assert.Equal(t, get(h, "/uploads/certificate/thumb").Code, http.StatusNotFound)

	// A kind that cannot be scaled has no picture, and the name stands alone.
	webp := append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 32)...)
	assert.Equal(t, saveIngest(h, nil, upload{"f9_0", "photo.webp", webp}).Code, http.StatusSeeOther)
	assert.False(t, strings.Contains(get(h, "/admin/settings").Body.String(), "file-thumb"))
	assert.Equal(t, get(h, "/uploads/logo/thumb").Code, http.StatusNotFound)
}

// postFirmware posts the one file of the Firmware panel, as a browser does: the leaf first.
func postFirmware(h http.Handler, name string, data []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("_leaf", "p.0")
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
	hdr.Set("Content-Type", "application/octet-stream")
	p, _ := w.CreatePart(hdr)
	_, _ = p.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/admin/firmware", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// A file of any size goes to the panel's Receive as it arrives: here it is hashed, and what it was is kept, and none of it.
func TestTheFirmwarePanelHashesAFileAsItArrivesAndKeepsOnlyWhatItWas(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/firmware").Body.String()
	assert.Contains(t, body, "Upload firmware")
	assert.Contains(t, body, `enctype="multipart/form-data"`)
	assert.Contains(t, body, "Up to 2 GiB")

	data := bytes.Repeat([]byte("firmware "), 300_000) // a few megabytes, which are never held
	sum := sha256.Sum256(data)
	rec := postFirmware(h, "../images/collector-1.2.bin", data)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oReceived collector-1.2.bin, sha256 "+hex.EncodeToString(sum[:])[:12]+"…")

	after := get(h, "/admin/firmware").Body.String()
	assert.Contains(t, after, "collector-1.2.bin")
	assert.Contains(t, after, hex.EncodeToString(sum[:]))
	assert.Contains(t, after, strconv.Itoa(len(data)))
}

// A viewer sees the panel disabled, with the reason, and a post that is made by hand is refused.
func TestAViewerSeesTheFirmwarePanelDisabledAndCannotUpload(t *testing.T) {
	viewer := handlerAs(t, Viewer)
	body := get(viewer, "/admin/firmware").Body.String()
	assert.Contains(t, body, "Upload firmware")
	assert.Contains(t, body, "Requires the editor role")
	assert.Contains(t, body, "disabled")

	rec := postFirmware(viewer, "x.bin", []byte("data"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eRequires the editor role")
}
