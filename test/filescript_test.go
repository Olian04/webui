package webui_test

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// fileData is what the page tells the script of a file chooser, as the input carries it.
type fileData struct {
	Max         int64
	MaxMessage  string
	Accept      []string
	KindMessage string
	Image       bool
}

// dataOf reads what the page's file input tells the script: its plain attributes, and the accept attribute that is its kinds.
func dataOf(t *testing.T, body string) fileData {
	t.Helper()

	var input []page.El
	for _, e := range page.Parse(t, body).ByAttr("type", "file") {
		if e.Has("data-file") {
			input = append(input, e)
		}
	}
	chooser := page.One(t, "the file input that carries data-file", input)
	d := fileData{MaxMessage: chooser.Attr("data-max-message"), KindMessage: chooser.Attr("data-kind-message"), Image: chooser.Has("data-image")}
	size, err := strconv.ParseInt(chooser.Attr("data-max-size"), 10, 64)
	assert.NoError(t, err)
	d.Max = size
	if accept := chooser.Attr("accept"); accept != "" {
		d.Accept = strings.Split(accept, ",")
	}
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
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", View: webui.Form[attachment]{
		Load: func(context.Context) (attachment, webui.Loaded) {
			return attachment{Held: &webui.FileInfo{Name: "a.pdf"}}, webui.Ready()
		},
		Fields: []webui.Accessor[attachment]{webui.File[attachment]{
			Label: "Attachment", Load: func(a attachment) *webui.FileInfo { return a.Held },
			Store: func(*attachment, *webui.Upload) {},
		}},
	}}}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/f").Body.String()
	assert.Equal(t, len(page.Parse(t, body).ByAttr("data-file", "")), 0)
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
			p := pageOf(t, s.h, "/admin/f")
			preview := p.ByRole("button", "Preview a.png")
			dialog := p.ByRole("dialog", "Preview of a.png")
			if c.img == "" {
				for _, shown := range []string{c.info.Thumbnail, c.info.URL} {
					if shown != "" {
						assert.Equal(t, len(p.ByAttr("src", shown)), 0) // neither the thumbnail that is refused nor the file itself is shown as a picture
					}
				}
				assert.Equal(t, len(preview)+len(dialog), 0)
				return
			}
			assert.Equal(t, len(p.ByAttr("src", c.img)), 1) // the thumbnail
			if c.link == "" {
				assert.Equal(t, len(preview)+len(dialog), 0)
				return
			}
			// The button opens the dialog by an invoker command, so that it works without script; the dialog holds the file at full size, and a button that closes it.
			open := page.One(t, "the button that opens the preview", preview)
			shut := page.One(t, "the preview", dialog)
			assert.Equal(t, open.Attr("command"), "show-modal")
			assert.Equal(t, open.Attr("commandfor"), shut.Attr("id"))
			full := page.One(t, "the full image", shut.ByAttr("src", c.link))
			assert.Equal(t, full.Attr("alt"), "a.png")
			// The full image is lazy: in a dialog that is closed it has no box, so it is fetched when the dialog is opened and not before.
			assert.Equal(t, full.Attr("loading"), "lazy")
			closes := 0
			for _, b := range p.ByAttr("commandfor", shut.Attr("id")) {
				if b.Attr("command") == "close" {
					closes++
				}
			}
			assert.True(t, closes > 0)
		})
	}
}

// A picture is read as a data: address, which the policy allows, and not made into a blob: one, which it does not.
func TestAPageWithAFileFieldAllowsDataImagesInItsPolicyAndNotBlobs(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile())
	csp := serve(s.h, http.MethodGet, "/admin/f").Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "img-src 'self' data:")
	assert.False(t, strings.Contains(csp, "blob:"))
}
