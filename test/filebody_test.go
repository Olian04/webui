package webui_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// rawPart is a part of a body, in the order that it is sent in; one with no filename is a value.
type rawPart struct{ name, filename, content string }

// orderedPost posts a multipart body whose parts come in exactly the order given, which a browser sends them in the order of the form.
func orderedPost(h http.Handler, target string, wrap func(io.Reader) io.Reader, parts ...rawPart) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, p := range parts {
		if p.filename == "" {
			_ = w.WriteField(p.name, p.content)
			continue
		}
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="`+p.name+`"; filename="`+p.filename+`"`)
		hdr.Set("Content-Type", "application/octet-stream")
		pw, _ := w.CreatePart(hdr)
		_, _ = pw.Write([]byte(p.content))
	}
	_ = w.Close()
	var reader io.Reader = &body
	if wrap != nil {
		reader = wrap(reader)
	}
	req := httptest.NewRequest(http.MethodPost, target, reader)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The body is read once, as it arrives, and a file is held in memory: nothing of a submission is written to the temporary directory, which the
// standard library's ParseMultipartForm would write a file of more than a megabyte to. The directory is empty while the action runs, with all of
// the file received, and after.
func TestAFileIsHeldInMemoryAndNothingIsWrittenToDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	var inRun []os.DirEntry
	s := newFileShop(plainFile(webui.FilePNG), func(p *webui.Page[webui.NoArgs]) {
		f := p.Body.(webui.Form[attachment])
		run := f.Submit.Run
		f.Submit.Run = func(ctx context.Context, a attachment) (webui.Outcome, error) {
			inRun, _ = os.ReadDir(dir)
			return run(ctx, a)
		}
		p.Body = f
	})
	data := string(pngBytes) + strings.Repeat("x", 3<<20)
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "big.png", data})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, len(s.last.Data), len(data)) // the whole of it
	assert.Equal(t, len(inRun), 0)
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Equal(t, len(entries), 0)
}

// A file that cannot be any of the kinds is refused by its first bytes, in the words of the field, and the form is drawn again.
func TestAFileOfTheWrongKindIsRefusedByItsFirstBytes(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "plan.png", string(pdfBytes) + strings.Repeat("x", 1<<20)})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose a PNG file.")
	assert.Equal(t, s.stores, 0)
}

// A file larger than the field takes is refused, the rest of it is read and thrown away, and what comes after it in the form is not lost: the
// form is drawn again with it.
func TestAFileOverTheLimitIsRefusedAndTheRestOfTheFormIsStillRead(t *testing.T) {
	t.Parallel()

	s := newFileShop(func(s *fileShop) webui.File[attachment] {
		f := plainFile()(s)
		f.MaxSize = 1000
		return f
	})
	rec := orderedPost(s.h, "/admin/f", nil,
		rawPart{"_leaf", "", "p"},
		rawPart{"f1", "big.bin", strings.Repeat("x", 100_000)},
		rawPart{"f0", "", "Late title"})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	assert.Contains(t, body, "Must be at most 1000 bytes.")
	assert.Contains(t, body, "Late title") // read after the file that was thrown away
	assert.Equal(t, s.stores, 0)
}

// A field that a browser sends after the file is read as any other, and the leaf says whose the file is, so it comes first.
func TestAFieldAfterTheFileIsKeptAndTheLeafAfterItIsNotUnderstood(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	rec := orderedPost(s.h, "/admin/f", nil,
		rawPart{"_leaf", "", "p"},
		rawPart{"f1", "a.png", string(pngBytes)},
		rawPart{"f0", "", "Late title"})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, s.last.Title, "Late title")
	assert.Equal(t, s.last.Got.Name, "a.png")

	s = newFileShop(plainFile(webui.FilePNG))
	rec = orderedPost(s.h, "/admin/f", nil,
		rawPart{"f1", "a.png", string(pngBytes)},
		rawPart{"_leaf", "", "p"})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eThat request was not understood.")
	assert.Equal(t, s.stores, 0)
}

// A body that ends before the file does is a submission that could not be read, and stores nothing.
func TestABodyThatEndsEarlyStoresNothing(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	rec := orderedPost(s.h, "/admin/f", func(r io.Reader) io.Reader { return io.LimitReader(r, 700) },
		rawPart{"_leaf", "", "p"},
		rawPart{"f1", "a.png", string(pngBytes) + strings.Repeat("x", 4000)})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eThat submission could not be read.")
	assert.Equal(t, s.stores, 0)
}
