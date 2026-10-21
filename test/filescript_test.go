package webui_test

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// fileData is what the page tells the script of a file chooser, as the input carries it.
type fileData struct {
	Max         int64    `json:"max"`
	MaxMessage  string   `json:"maxMessage"`
	Accept      []string `json:"accept"`
	KindMessage string   `json:"kindMessage"`
	Image       bool     `json:"image"`
	Preview     string   `json:"preview"`
}

// dataOf reads the data-file attribute of the page's file input.
func dataOf(t *testing.T, body string) fileData {
	t.Helper()

	m := regexp.MustCompile(`data-file="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("the file input carries no data-file")
	}
	var d fileData
	assert.NoError(t, json.Unmarshal([]byte(html.UnescapeString(m[1])), &d))
	return d
}

// A page links the script for a file chooser only when it has one that can be used: not a page of a form with no file, nor one whose form
// is only read.
func TestAPageLinksTheFileScriptOnlyWhenItHasAFileToChoose(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	js := assetURL(t, s.h, "file.js")
	assert.True(t, regexp.MustCompile(`/_webui/file\.[0-9a-f]{8}\.js$`).MatchString(js)) // by the hash of what it holds
	assert.Equal(t, serve(s.h, http.MethodGet, js).Code, http.StatusOK)
	assert.Contains(t, serve(s.h, http.MethodGet, "/admin/f").Body.String(), `<script src="`+js+`" defer></script>`)

	// A form with no file: no script.
	plain := newShop()
	assert.False(t, strings.Contains(serve(plain.h, http.MethodGet, "/admin/device/dev1").Body.String(), "file."))

	// A form that is only read has no chooser, and no script for one.
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", Body: webui.Form[attachment]{
		Load: func(context.Context) (attachment, error) {
			return attachment{Held: &webui.FileInfo{Name: "a.pdf"}}, nil
		},
		Fields: []webui.Accessor[attachment]{webui.File[attachment]{
			Label: "Attachment", Load: func(a attachment) *webui.FileInfo { return a.Held },
			Store: func(*attachment, *webui.Upload) {},
		}},
	}}}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/f").Body.String()
	assert.False(t, strings.Contains(body, "data-file"))
	assert.False(t, strings.Contains(body, "/_webui/file."))
}

// The page tells the script what the field takes, in the words the server says beneath it, and the script says them early.
func TestTheFileChooserTellsTheScriptWhatTheFieldTakesInTheServersWords(t *testing.T) {
	t.Parallel()

	s := newFileShop(func(s *fileShop) webui.File[attachment] {
		f := plainFile(webui.FilePNG, webui.FileJPEG)(s)
		f.MaxSize = 2 << 20
		return f
	})
	d := dataOf(t, serve(s.h, http.MethodGet, "/admin/f").Body.String())
	assert.Equal(t, d.Max, int64(2<<20))
	assert.Equal(t, d.MaxMessage, "Must be at most 2 MiB.")
	assert.DeepEqual(t, d.Accept, []string{"image/png", ".png", "image/jpeg", ".jpg", ".jpeg"})
	assert.Equal(t, d.KindMessage, "Choose a PNG or JPEG file.")
	assert.True(t, d.Image)

	// The words are the ones that the server answers with, for the same file.
	assert.Contains(t, multipartPost(s.h, "/admin/f", nil, part{"f1", "a.png", strings.Repeat("x", 2<<20+100)}).Body.String(), d.MaxMessage)
	assert.Contains(t, multipartPost(s.h, "/admin/f", nil, part{"f1", "a.png", "words"}).Body.String(), d.KindMessage)

	// A field that takes no pictures has none shown; one that takes any file has an image shown if the file is one.
	pdf := newFileShop(plainFile(webui.FilePDF))
	assert.False(t, dataOf(t, serve(pdf.h, http.MethodGet, "/admin/f").Body.String()).Image)
	anything := newFileShop(plainFile())
	d = dataOf(t, serve(anything.h, http.MethodGet, "/admin/f").Body.String())
	assert.True(t, d.Image)
	assert.Equal(t, d.KindMessage, "")
	assert.Equal(t, len(d.Accept), 0)
}

// The stored file is shown as a picture when it is one and is on this site: a field that takes only images, and an address that is a path.
func TestTheStoredFileIsShownAsAPictureOnlyWhenItIsOneAndIsOnThisSite(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		accept []webui.FileKind
		url    string
		want   string
	}{
		"an image on this site":             {[]webui.FileKind{webui.FileImage}, "/files/logo", "/files/logo"},
		"one kind of image":                 {[]webui.FileKind{webui.FilePNG}, "/files/logo", "/files/logo"},
		"an image on another site":          {[]webui.FileKind{webui.FileImage}, "https://example.com/logo.png", ""},
		"a protocol-relative address":       {[]webui.FileKind{webui.FileImage}, "//example.com/logo.png", ""},
		"no address":                        {[]webui.FileKind{webui.FileImage}, "", ""},
		"a field that takes more than that": {[]webui.FileKind{webui.FileImage, webui.FilePDF}, "/files/logo", ""},
		"any file":                          {nil, "/files/logo", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := newFileShop(plainFile(c.accept...))
			s.held = &webui.FileInfo{Name: "logo.png", URL: c.url}
			assert.Equal(t, dataOf(t, serve(s.h, http.MethodGet, "/admin/f").Body.String()).Preview, c.want)
		})
	}
}

// There is no script test harness, so this is what the shipped file carries: that a picture is read as a data: address, which the policy
// allows, and not made into a blob: one, which it does not; that a drop is put in the input, which the form then posts; and that the
// checks are the Constraint Validation API's.
func TestTheFileScriptDoesWhatItSaysItDoes(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile())
	js := serve(s.h, http.MethodGet, assetURL(t, s.h, "file.js")).Body.String()
	for _, want := range []string{
		"readAsDataURL", "new DataTransfer()", "input.files = one.files", "setCustomValidity(said)", "reportValidity()",
		"/^image\\/(png|jpeg|gif|webp)$/", "PREVIEW_MAX", "'Files'", "input[type=\"file\"][data-file]",
	} {
		assert.Contains(t, js, want)
	}
	assert.False(t, strings.Contains(js, "createObjectURL")) // a blob: address is not one that the content security policy lets an image have
	assert.False(t, strings.Contains(js, "svg"))
	assert.False(t, strings.Contains(js, "fetch(")) // the post is the browser's own, from the input

	csp := serve(s.h, http.MethodGet, "/admin/f").Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "img-src 'self' data:")
	assert.False(t, strings.Contains(csp, "blob:"))
}
