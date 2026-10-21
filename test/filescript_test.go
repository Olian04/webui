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

// The stored file's picture is the one that the app made: a small one, which is in the page and not made by script, and which opens the
// file at full size in a dialog that is in the page too, with an invoker command as a button that asks does, so that it works without script.
// The library never shows the file itself as a thumbnail, and shows a picture only from a path on this site.
func TestTheStoredFilesPictureIsTheThumbnailTheAppMadeFromThisSite(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		info *webui.FileInfo
		img  string // the thumbnail that is shown, "" for none
		link string // the file that its dialog shows at full size, "" for no dialog
	}{
		"a thumbnail and the file":    {&webui.FileInfo{Name: "a.png", URL: "/files/a", Thumbnail: "/files/a/thumb"}, "/files/a/thumb", "/files/a"},
		"a thumbnail and no file":     {&webui.FileInfo{Name: "a.png", Thumbnail: "/files/a/thumb"}, "/files/a/thumb", ""},
		"no thumbnail, only the file": {&webui.FileInfo{Name: "a.png", URL: "/files/a"}, "", ""}, // the file is never shown shrunk
		"a thumbnail on another site": {&webui.FileInfo{Name: "a.png", URL: "/files/a", Thumbnail: "https://example.com/a.png"}, "", ""},
		"a protocol-relative address": {&webui.FileInfo{Name: "a.png", URL: "/files/a", Thumbnail: "//example.com/a.png"}, "", ""},
		"a backslash":                 {&webui.FileInfo{Name: "a.png", URL: "/files/a", Thumbnail: `/\example.com/a.png`}, "", ""},
		"a data address":              {&webui.FileInfo{Name: "a.png", URL: "/files/a", Thumbnail: "data:image/png;base64,AAAA"}, "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := newFileShop(plainFile(webui.FileImage))
			s.held = c.info
			body := serve(s.h, http.MethodGet, "/admin/f").Body.String()
			if c.img == "" {
				assert.False(t, strings.Contains(body, `class="file-thumb"`))
				assert.False(t, strings.Contains(body, "file-thumb-link"))
				return
			}
			assert.Contains(t, body, `<img class="file-thumb" src="`+c.img+`" alt="">`)
			if c.link == "" {
				assert.False(t, strings.Contains(body, "file-thumb-link"))
				assert.False(t, strings.Contains(body, "preview-dialog"))
				return
			}
			assert.Contains(t, body, `<button class="file-thumb-link" type="button" commandfor="p-f1-preview" command="show-modal" aria-haspopup="dialog" aria-label="Preview a.png" title="Preview a.png"><img class="file-thumb" src="`+c.img+`" alt=""></button>`)
			assert.Contains(t, body, `<dialog id="p-f1-preview" class="preview-dialog" closedby="any" aria-label="Preview of a.png">`)
			// The full image is lazy: in a dialog that is closed it has no box, so it is fetched when the dialog is opened and not before.
			assert.Contains(t, body, `<img class="preview-full" src="`+c.link+`" alt="a.png" loading="lazy" decoding="async">`)
			assert.Contains(t, body, `commandfor="p-f1-preview" command="close"`)
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
		"dialog.showModal()", "toDataURL('image/png')", "full.src = data", "img.removeAttribute('src')",
	} {
		assert.Contains(t, js, want)
	}
	// Every function that the script calls is one that it has: a script that is cut about by an edit still has all its tokens, and fails at once
	// in the browser, where there is no harness to say so. These are the ones that it calls and does not take from the page.
	for _, fn := range []string{"config", "problem", "element", "readAsData", "openPreview", "enhance", "enhanceAll"} {
		assert.Contains(t, js, "function "+fn+"(")
	}
	assert.False(t, strings.Contains(js, "createObjectURL")) // a blob: address is not one that the content security policy lets an image have
	assert.False(t, strings.Contains(js, "svg"))
	assert.False(t, strings.Contains(js, "fetch(")) // the post is the browser's own, from the input

	csp := serve(s.h, http.MethodGet, "/admin/f").Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "img-src 'self' data:")
	assert.False(t, strings.Contains(csp, "blob:"))
}
