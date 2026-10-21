package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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

// With script the chooser is a drop zone that shows a picture: the page links the script, and tells it what the logo takes and where the stored
// logo's picture is, which the certificate, a PDF, has none of.
func TestTheIngestFormLinksTheFileScriptAndTellsItWhatEachFieldTakes(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `/_webui/file.`)
	assert.Contains(t, body, `&#34;maxMessage&#34;:&#34;Must be at most 1 MiB.&#34;`)
	assert.False(t, strings.Contains(body, `&#34;preview&#34;`)) // nothing is stored yet

	assert.Equal(t, saveIngest(h, nil, upload{"f9_0", "logo.png", demoPNG}, upload{"f9_1", "cert.pdf", demoPDF}).Code, http.StatusSeeOther)
	body = get(h, "/admin/settings").Body.String()
	assert.Equal(t, strings.Count(body, `&#34;preview&#34;:&#34;/uploads/logo&#34;`), 1) // the logo's, and only the logo's
	assert.False(t, strings.Contains(body, `&#34;preview&#34;:&#34;/uploads/certificate`))

	// A viewer has no chooser. The page still links the script, which is the page's and not the visitor's (the Guard that refuses them is
	// asked per request), and it has nothing to do there.
	viewer := get(handlerAs(t, Viewer), "/admin/settings").Body.String()
	assert.False(t, strings.Contains(viewer, `data-file=`))
}
