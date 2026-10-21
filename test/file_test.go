package webui_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// What a visitor can send, as the first bytes of a file of each kind: a file is of a kind by what it holds.
var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	jpegBytes = append([]byte("\xff\xd8\xff\xe0\x00\x10JFIF"), bytes.Repeat([]byte{0}, 32)...)
	gifBytes  = []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00")
	webpBytes = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 32)...)
	pdfBytes  = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF\n")
)

// attachment is the model of a form with a file: a title, the file that is held, and what Store was handed.
type attachment struct {
	Title string
	Held  *webui.FileInfo
	Got   *webui.Upload
	Data  []byte
	Gone  bool // Store was called with nil
}

// fileShop is a page with a form that has a File and a Title, and what was done on it.
type fileShop struct {
	mu     sync.Mutex
	stores int
	last   attachment
	held   *webui.FileInfo
	h      http.Handler
}

func newFileShop(file func(*fileShop) webui.File[attachment], opts ...func(*webui.Page[webui.NoArgs])) *fileShop {
	s := &fileShop{}
	title := webui.String[attachment]{
		Label: "Title", Load: func(a attachment) string { return a.Title }, Store: func(a *attachment, v string) { a.Title = v },
		Rules: webui.StringRules{MinLen: 3},
	}
	page := webui.Page[webui.NoArgs]{Path: "/f", Body: webui.Form[attachment]{
		Load:   func(context.Context) (attachment, error) { return attachment{Title: "Report", Held: s.held}, nil },
		Fields: []webui.Accessor[attachment]{title, file(s)},
		Submit: webui.Action[attachment]{Run: func(_ context.Context, a attachment) (webui.Outcome, error) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.last = a
			if a.Got != nil { // copied here, as an Upload is only open until Run returns
				r, err := a.Got.Open()
				if err != nil {
					return webui.Outcome{}, err
				}
				defer func() { _ = r.Close() }() // a read-only handle: there is nothing to lose if closing fails
				a.Data, _ = io.ReadAll(r)
				s.last = a
			}
			return webui.Success("Saved"), nil
		}},
	}}
	for _, o := range opts {
		o(&page)
	}
	s.h = webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	return s
}

// plainFile is a File with the given kinds, storing what it is handed.
func plainFile(kinds ...webui.FileKind) func(*fileShop) webui.File[attachment] {
	return func(s *fileShop) webui.File[attachment] {
		return webui.File[attachment]{
			Label: "Attachment", Accept: kinds,
			Load: func(a attachment) *webui.FileInfo { return a.Held },
			Store: func(a *attachment, up *webui.Upload) {
				s.mu.Lock()
				s.stores++
				s.mu.Unlock()
				if up == nil {
					a.Gone, a.Got = true, nil
					return
				}
				a.Got = up
			},
		}
	}
}

type part struct{ field, filename, content string }

// multipartPost is a form that has files in it, as a browser sends one: the values, and a part for each file, an empty one for a chooser
// that was left alone.
func multipartPost(h http.Handler, target string, values map[string]string, parts ...part) *httptest.ResponseRecorder {
	return multipartPostBody(h, target, values, nil, parts...)
}

func multipartPostBody(h http.Handler, target string, values map[string]string, wrap func(io.Reader) io.Reader, parts ...part) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	values = withLeaf(values)
	for k, v := range values {
		_ = w.WriteField(k, v)
	}
	for _, p := range parts {
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="`+p.field+`"; filename="`+p.filename+`"`)
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

func withLeaf(values map[string]string) map[string]string {
	out := map[string]string{"_leaf": "p", "f0": "Report"}
	for k, v := range values {
		out[k] = v
	}
	return out
}

func TestAFormWithAFileIsMultipartAndOneWithoutIsNot(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG, webui.FileJPEG))
	body := serve(s.h, http.MethodGet, "/admin/f").Body.String()
	assert.Contains(t, body, `enctype="multipart/form-data"`)
	assert.Contains(t, body, `type="file" name="f1" accept="image/png,image/jpeg"`)
	assert.False(t, strings.Contains(body, `type="file" name="f1" value=`)) // a chooser has no value, which a browser would not take
	assert.Contains(t, body, `A PNG or JPEG file`)
	assert.Contains(t, body, `Up to 5 MiB`)
	assert.False(t, strings.Contains(body, `name="_rm"`)) // nothing is stored, and the field cannot be emptied

	// A form of no file is as it was.
	plain := serve(newShop().h, http.MethodGet, "/admin/device/dev1").Body.String()
	assert.False(t, strings.Contains(plain, "enctype"))
}

// The file that is stored is shown above the chooser, as a link when the app serves it, and the Remove checkbox is for a file that
// can be removed and is there to remove.
func TestTheFileThatIsStoredIsShownAboveTheChooser(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile())
	s.held = &webui.FileInfo{Name: "plan.pdf", Size: 1536, URL: "/files/plan"}
	body := serve(s.h, http.MethodGet, "/admin/f").Body.String()
	assert.Contains(t, body, `<a href="/files/plan">plan.pdf</a>`)
	assert.Contains(t, body, `<span class="dim">1.5 KiB</span>`)
	assert.False(t, strings.Contains(body, `name="_rm"`))

	s = newFileShop(func(s *fileShop) webui.File[attachment] {
		f := plainFile()(s)
		f.Removable = true
		return f
	})
	s.held = &webui.FileInfo{Name: "plan.pdf", URL: "javascript:alert(1)"} // an address that is not safe is shown as a name
	body = serve(s.h, http.MethodGet, "/admin/f").Body.String()
	assert.Contains(t, body, `<span>plan.pdf</span>`)
	assert.False(t, strings.Contains(body, "javascript:"))
	assert.Contains(t, body, `<input class="checkbox" type="checkbox" name="_rm" value="f1">`)
}

func TestAnUploadIsHandedToStoreWithWhatItHoldsAndTheBaseNameOnly(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", `../../etc/x.png`, string(pngBytes)})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, s.stores, 1)
	assert.Equal(t, s.last.Got.Name, "x.png")
	assert.Equal(t, s.last.Got.ContentType, "image/png") // what it holds, whatever the browser said
	assert.Equal(t, s.last.Got.Size, int64(len(pngBytes)))
	assert.DeepEqual(t, s.last.Data, pngBytes) // copied by Run, while it was open
}

// The name is the base name that the browser gave, made safe: the rest of the cases are the rules package's own.
func TestAFileNameIsAsTheBrowserGaveItWithNoPath(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		"a/b/c.png":                 "c.png",
		"..":                        "file",
		"  spaced name.png  ":       "spaced name.png",
		"emoji \U0001F600 face.png": "emoji \U0001F600 face.png",
		strings.Repeat("é", 300):    strings.Repeat("é", 255),
	} {
		s := newFileShop(plainFile())
		multipartPost(s.h, "/admin/f", nil, part{"f1", raw, "data"})
		assert.Equal(t, s.last.Got.Name, want)
	}
}

// A chooser left alone sends an empty part, which is nothing chosen: what is stored is left alone and Store is not called.
func TestNothingChosenLeavesTheStoredFileAlone(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	s.held = &webui.FileInfo{Name: "old.png"}
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "", ""})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, s.stores, 0)

	rec = multipartPost(s.h, "/admin/f", nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, s.stores, 0)
}

func TestRemoveCallsStoreWithNilAndANewFileReplacesTheRemoval(t *testing.T) {
	t.Parallel()

	removable := func(s *fileShop) webui.File[attachment] {
		f := plainFile(webui.FilePNG)(s)
		f.Removable = true
		return f
	}
	s := newFileShop(removable)
	s.held = &webui.FileInfo{Name: "old.png"}
	rec := multipartPost(s.h, "/admin/f", map[string]string{"_rm": "f1"})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, s.stores, 1)
	assert.True(t, s.last.Gone)

	// Both: the new file is what is stored.
	s = newFileShop(removable)
	multipartPost(s.h, "/admin/f", map[string]string{"_rm": "f1"}, part{"f1", "new.png", string(pngBytes)})
	assert.Equal(t, s.stores, 1)
	assert.False(t, s.last.Gone)
	assert.Equal(t, s.last.Got.Name, "new.png")

	// A file that cannot be removed ignores a request to.
	s = newFileShop(plainFile(webui.FilePNG))
	multipartPost(s.h, "/admin/f", map[string]string{"_rm": "f1"})
	assert.Equal(t, s.stores, 0)
}

// Each kind takes the files that hold it and refuses the others, by what they hold: a name says nothing.
func TestEachKindAcceptsItsOwnBytesAndRefusesTheOthers(t *testing.T) {
	t.Parallel()

	samples := map[string]string{
		"png": string(pngBytes), "jpeg": string(jpegBytes), "gif": string(gifBytes), "webp": string(webpBytes), "pdf": string(pdfBytes),
		"json": `{"a": [1, 2, {"b": null}]}`, "csv": "a,b\n1,2\n", "text": "just some words\n", "html": "<!doctype html><html><body>hi</body></html>",
		"badjson": `{"a": `, "twojson": `{} {}`, "badcsv": "a,\"b\n", "binary": "text\x00with a NUL", "latin1": "caf\xe9",
	}
	everything := []string{"png", "jpeg", "gif", "webp", "pdf", "json", "csv", "text", "html", "badjson", "twojson", "badcsv", "binary", "latin1"}
	for _, c := range []struct {
		name   string
		kind   webui.FileKind
		accept []string
	}{
		{"png", webui.FilePNG, []string{"png"}},
		{"jpeg", webui.FileJPEG, []string{"jpeg"}},
		{"gif", webui.FileGIF, []string{"gif"}},
		{"webp", webui.FileWebP, []string{"webp"}},
		{"image", webui.FileImage, []string{"png", "jpeg", "gif", "webp"}},
		{"pdf", webui.FilePDF, []string{"pdf"}},
		{"json", webui.FileJSON, []string{"json"}},                                                // exactly one value
		{"csv", webui.FileCSV, []string{"csv", "text", "twojson"}},                                // a line of words is a record of one field; a stray quote is not CSV
		{"text", webui.FileText, []string{"json", "csv", "text", "twojson", "badjson", "badcsv"}}, // UTF-8 with no NUL, and not HTML
		{"any", webui.FileAny, everything},
	} {
		for sample, content := range samples {
			t.Run(c.name+" takes "+sample+" or not", func(t *testing.T) {
				t.Parallel()

				s := newFileShop(plainFile(c.kind))
				rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "some." + sample, content}) // named for what it is not
				want := slices.Contains(c.accept, sample)
				assert.Equal(t, rec.Code == http.StatusSeeOther, want)
				if want {
					assert.Equal(t, s.stores, 1)
					assert.Equal(t, s.last.Got.Kind, kindOf(c.kind, sample)) // which kind it is, to tell what came from a field that takes several
				} else {
					assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
					assert.Equal(t, s.stores, 0)
				}
			})
		}
	}
}

// binaryKinds are the kinds that a file is told to be by its first bytes alone.
var binaryKinds = map[string]webui.FileKind{"png": webui.FilePNG, "jpeg": webui.FileJPEG, "gif": webui.FileGIF, "webp": webui.FileWebP, "pdf": webui.FilePDF}

// kindOf is the Kind that an upload of a sample has, from a field that takes a kind: that kind, or for the kinds that stand for
// several, the one it is. A field that takes any file knows only the images and the PDF.
func kindOf(accepted webui.FileKind, sample string) webui.FileKind {
	switch accepted {
	case webui.FileImage:
		return binaryKinds[sample]
	case webui.FileAny:
		if k, ok := binaryKinds[sample]; ok {
			return k
		}
		return webui.FileAny
	}
	return accepted
}

// A file that is more than one kind is the first of the kinds the field lists that it is.
func TestTheKindOfAFileThatIsSeveralIsTheFirstTheFieldLists(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		accept []webui.FileKind
		want   webui.FileKind
	}{
		{[]webui.FileKind{webui.FileJSON, webui.FileText}, webui.FileJSON},
		{[]webui.FileKind{webui.FileText, webui.FileJSON}, webui.FileText},
		{[]webui.FileKind{webui.FilePNG, webui.FileImage}, webui.FilePNG},
	} {
		s := newFileShop(plainFile(c.accept...))
		content := `{"a": 1}`
		if c.want == webui.FilePNG {
			content = string(pngBytes)
		}
		assert.Equal(t, multipartPost(s.h, "/admin/f", nil, part{"f1", "x", content}).Code, http.StatusSeeOther)
		assert.Equal(t, s.last.Got.Kind, c.want)
	}
}

func TestAFileThatIsNotTheKindIsRefusedInTheWordsOfTheField(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG, webui.FileJPEG))
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "photo.png", "this is not an image, whatever it is called"})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	assert.Contains(t, body, "Choose a PNG or JPEG file.")
	assert.Equal(t, s.stores, 0)

	// A PDF named .png is a PDF.
	s = newFileShop(plainFile(webui.FilePNG))
	assert.Equal(t, multipartPost(s.h, "/admin/f", nil, part{"f1", "plan.png", string(pdfBytes)}).Code, http.StatusUnprocessableEntity)

	// An image is refused as a text, and the words are those of the kinds that are taken.
	s = newFileShop(plainFile(webui.FileImage))
	rec = multipartPost(s.h, "/admin/f", nil, part{"f1", "a.png", "words"})
	assert.Contains(t, rec.Body.String(), "Choose a PNG, JPEG, GIF or WebP file.")
}

func TestAFileLargerThanMaxSizeIsRefusedAndNothingIsStored(t *testing.T) {
	t.Parallel()

	s := newFileShop(func(s *fileShop) webui.File[attachment] {
		f := plainFile()(s)
		f.MaxSize = 100
		return f
	})
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "big.bin", strings.Repeat("x", 101)})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Must be at most 100 bytes.")
	assert.Equal(t, s.stores, 0)

	assert.Equal(t, multipartPost(s.h, "/admin/f", nil, part{"f1", "ok.bin", strings.Repeat("x", 100)}).Code, http.StatusSeeOther)
}

// A body that is larger than all the files may be, with the usual, is refused as too large, and the handler does not read it all.
func TestABodyLargerThanTheFilesMayBeIsTooLarge(t *testing.T) {
	t.Parallel()

	s := newFileShop(func(s *fileShop) webui.File[attachment] {
		f := plainFile()(s)
		f.MaxSize = 1000
		return f
	})
	var read atomic.Int64
	rec := multipartPostBody(s.h, "/admin/f", nil, func(r io.Reader) io.Reader {
		return io.TeeReader(r, writerFunc(func(b []byte) { read.Add(int64(len(b))) }))
	}, part{"f1", "huge.bin", strings.Repeat("x", 3<<20)})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eThat submission was too large.")
	assert.Equal(t, s.stores, 0)
	assert.True(t, read.Load() < 3<<20) // it did not read what it would not take
}

type writerFunc func([]byte)

func (f writerFunc) Write(b []byte) (int, error) { f(b); return len(b), nil }

// A visitor that the page refuses has nothing of theirs read: the page's Guard runs before the body is parsed.
func TestAVisitorThePageRefusesHasNoBodyRead(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(), func(p *webui.Page[webui.NoArgs]) {
		p.Guard = func(context.Context, webui.NoArgs) error { return errors.New("no") }
	})
	var read atomic.Int64
	rec := multipartPostBody(s.h, "/admin/f", nil, func(r io.Reader) io.Reader {
		return io.TeeReader(r, writerFunc(func(b []byte) { read.Add(int64(len(b))) }))
	}, part{"f1", "a.bin", strings.Repeat("x", 1<<20)})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, read.Load(), int64(0))
	assert.Equal(t, s.stores, 0)
}

// A file is stored when the whole submission is accepted, and not before: a form that another field refuses stores none, and says to
// choose the file again, since a browser cannot refill a chooser.
func TestAFormThatIsRefusedStoresNoFileAndShowsTheStoredOneAgain(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	s.held = &webui.FileInfo{Name: "old.png", URL: "/files/old"}
	rec := multipartPost(s.h, "/admin/f", map[string]string{"f0": "x"}, part{"f1", "new.png", string(pngBytes)}) // the title is too short
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Equal(t, s.stores, 0)
	body := rec.Body.String()
	assert.Contains(t, body, "Must be at least 3 characters.")
	assert.Contains(t, body, `<a href="/files/old">old.png</a>`) // what is stored, and not what was chosen
	assert.Contains(t, body, "Choose the file again")
	assert.False(t, strings.Contains(body, "new.png"))
}

func TestARequiredFileIsOneThatIsStoredAndNotRemovedOrNewlyChosen(t *testing.T) {
	t.Parallel()

	required := func(removable bool) func(*fileShop) webui.File[attachment] {
		return func(s *fileShop) webui.File[attachment] {
			f := plainFile(webui.FilePNG)(s)
			f.Required, f.Removable = true, removable
			return f
		}
	}
	// Nothing stored, nothing chosen.
	s := newFileShop(required(false))
	rec := multipartPost(s.h, "/admin/f", nil)
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose a file.")
	body := serve(s.h, http.MethodGet, "/admin/f").Body.String()
	assert.Contains(t, body, `accept="image/png" required`) // the browser asks for one

	// Stored, nothing chosen: that satisfies it, and the browser does not ask for another.
	s = newFileShop(required(false))
	s.held = &webui.FileInfo{Name: "old.png"}
	assert.Equal(t, multipartPost(s.h, "/admin/f", nil).Code, http.StatusSeeOther)
	assert.False(t, strings.Contains(serve(s.h, http.MethodGet, "/admin/f").Body.String(), `accept="image/png" required`))

	// Stored, and asked to be removed, with nothing to replace it: it is gone, and so is what satisfied it.
	s = newFileShop(required(true))
	s.held = &webui.FileInfo{Name: "old.png"}
	assert.Equal(t, multipartPost(s.h, "/admin/f", map[string]string{"_rm": "f1"}).Code, http.StatusUnprocessableEntity)
	assert.Equal(t, multipartPost(s.h, "/admin/f", map[string]string{"_rm": "f1"}, part{"f1", "new.png", string(pngBytes)}).Code, http.StatusSeeOther)
}

// Uploads that arrive together are each their own: nothing is shared between the requests.
func TestUploadsAtTheSameTimeAreEachTheirOwn(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "a.png", string(pngBytes) + strings.Repeat("x", i)})
			assert.Equal(t, rec.Code, http.StatusSeeOther)
		}()
	}
	wg.Wait()
	assert.Equal(t, s.stores, 20)
}

// A page that has no file reads a multipart body as it always did: as a request it does not understand.
func TestAPageWithNoFileDoesNotReadAMultipartBody(t *testing.T) {
	t.Parallel()

	s := newShop()
	rec := multipartPost(s.h, "/admin/device/dev1", map[string]string{"f1": "10.0.0.9", "f2": "5"})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, len(s.saved), 0)
}

// A POST that is urlencoded to a page that has a file is read as before, with the usual limit.
func TestAUrlencodedPostToAPageWithAFileIsAsItWas(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile())
	rec := post(s.h, "/admin/f", url.Values{"_leaf": {"p"}, "f0": {"Report"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, s.stores, 0)
}

func TestAFileIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(f webui.File[attachment]) string {
		var all string
		page := webui.Page[webui.NoArgs]{Path: "/f", Body: webui.Form[attachment]{
			Load:   func(context.Context) (attachment, error) { return attachment{}, nil },
			Fields: []webui.Accessor[attachment]{f},
		}}
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{page}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	load := func(attachment) *webui.FileInfo { return nil }
	store := func(*attachment, *webui.Upload) {}
	assert.Contains(t, problems(webui.File[attachment]{Label: "F"}), "has no Load")
	assert.Contains(t, problems(webui.File[attachment]{Label: "F", Load: load, Removable: true}), "is Removable but has no Store")
	assert.Contains(t, problems(webui.File[attachment]{Label: "F", Load: load, Required: true}), "is Required but has no Store")
	assert.Contains(t, problems(webui.File[attachment]{Label: "F", Load: load, Store: store, MaxSize: -1}), "has MaxSize -1")
	assert.Contains(t, problems(webui.File[attachment]{Label: "F", Load: load, Store: store, MaxSize: 257 << 20}), "has MaxSize")
	assert.Contains(t, problems(webui.File[attachment]{Load: load, Store: store}), "has no Label")
}

// In a table a file is its stored name, a link when the app serves it, and a dash for none.
func TestAFileInATableIsItsNameAndALink(t *testing.T) {
	t.Parallel()

	rows := []attachment{
		{Title: "a", Held: &webui.FileInfo{Name: "plan.pdf", URL: "/files/plan"}},
		{Title: "b"},
	}
	table := webui.Table[attachment]{
		Rows: func(context.Context) ([]attachment, error) { return rows, nil },
		Columns: []webui.Accessor[attachment]{
			webui.String[attachment]{Label: "Title", Load: func(a attachment) string { return a.Title }},
			webui.File[attachment]{Label: "File", Load: func(a attachment) *webui.FileInfo { return a.Held }},
		},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: table}}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `plan.pdf`)
	assert.Contains(t, body, `href="/files/plan"`)
	assert.Contains(t, body, `<span class="sr-only">empty</span>`)
}
