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
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// arrived is what a file upload's Run was handed, and read from it.
type arrived struct {
	handle webui.FileHandle[struct{}] // without its Reader
	data   []byte
	err    error // what reading the whole of it failed with
}

// uploadShop is a page with a FileUpload, and what its Run was handed.
type uploadShop struct {
	mu   sync.Mutex
	runs []arrived
	h    http.Handler
}

func (s *uploadShop) last() arrived {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[len(s.runs)-1]
}

func (s *uploadShop) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.runs)
}

// newUploadShop is a page with a FileUpload whose Run reads all of the file and says that it was uploaded; edit changes the panel, or its Run.
func newUploadShop(edit func(*webui.FileUpload[struct{}]), pageOpts ...func(*webui.Page[webui.NoArgs])) *uploadShop {
	s := &uploadShop{}
	up := webui.FileUpload[struct{}]{
		Title: "Upload firmware", MaxSize: 5 << 20,
		Receive: webui.Action[webui.FileHandle[struct{}]]{Run: func(_ context.Context, f webui.FileHandle[struct{}]) (webui.Outcome, error) {
			data, err := io.ReadAll(f.Reader)
			f.Reader = nil
			s.mu.Lock()
			s.runs = append(s.runs, arrived{handle: f, data: data, err: err})
			s.mu.Unlock()
			if err != nil {
				return webui.Outcome{}, err
			}
			return webui.Success("Uploaded " + f.Name), nil
		}},
	}
	if edit != nil {
		edit(&up)
	}
	page := webui.Page[webui.NoArgs]{Path: "/up", View: up}
	for _, o := range pageOpts {
		o(&page)
	}
	s.h = webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	return s
}

// upload posts the one file of a panel, as a browser does: the panel first.
func upload(h http.Handler, filename, content string, wrap func(io.Reader) io.Reader) *httptest.ResponseRecorder {
	return orderedPost(h, "/admin/up", wrap, rawPart{"_view", "", "p"}, rawPart{"file", filename, content})
}

func TestAFileUploadIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(f webui.FileUpload[struct{}]) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", View: f}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	run := func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
		return webui.Success("ok"), nil
	}
	receive := webui.Action[webui.FileHandle[struct{}]]{Run: run}
	assert.Contains(t, problems(webui.FileUpload[struct{}]{MaxSize: 1}), "FileUpload.Receive has no Run")
	assert.Contains(t, problems(webui.FileUpload[struct{}]{Receive: receive}), "FileUpload has MaxSize 0")
	assert.Contains(t, problems(webui.FileUpload[struct{}]{Receive: receive, MaxSize: -5}), "FileUpload has MaxSize -5")
	for _, k := range []webui.FileKind{webui.FileJSON, webui.FileCSV, webui.FileText} {
		assert.Contains(t, problems(webui.FileUpload[struct{}]{Receive: receive, MaxSize: 1, Accept: []webui.FileKind{webui.FileImage, k}}), "told by all of the file")
	}
	assert.Contains(t, problems(webui.FileUpload[struct{}]{Receive: receive, MaxSize: 1, Accept: []webui.FileKind{webui.FileKind(99)}}), "is not a FileKind")
	_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", View: webui.FileUpload[struct{}]{
		Receive: receive, MaxSize: 1 << 40, Accept: []webui.FileKind{webui.FileImage, webui.FilePDF, webui.FileAny},
	}}}}.Compile("/admin")
	assert.NoError(t, err) // as large as the app says, with no ceiling
}

// The panel is a native form of its own: the panel comes before the one file, and the file chooser has what the script reads, so that it works as a File does.
func TestAFileUploadIsAFormOfItsOwnWithTheViewIDBeforeTheFile(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Accept, f.MaxSize, f.Help, f.Desc = []webui.FileKind{webui.FilePNG, webui.FileJPEG}, 2<<30, "The image that the device shows.", "Sent to the bucket."
		f.Receive.Label = "Send it"
	})
	body := serve(s.h, http.MethodGet, "/admin/up").Body.String()
	p := page.Parse(t, body)
	assert.Equal(t, len(p.ByRole("heading", "Upload firmware")), 1)
	assert.Equal(t, len(p.ByAttr("enctype", "multipart/form-data")), 1)
	assert.DeepEqual(t, fieldNames(p), []string{"_view", "file"}) // the panel before the one file
	chooser := page.One(t, "the chooser", p.ByAttr("type", "file"))
	assert.Equal(t, chooser.Attr("name"), "file")
	assert.Equal(t, chooser.Attr("accept"), "image/png,.png,image/jpeg,.jpg,.jpeg")
	assert.True(t, chooser.Has("required"))
	for _, said := range []string{"A PNG or JPEG file", "Up to 2 GiB", "The image that the device shows."} {
		assert.True(t, len(p.ByAllText(said)) > 0)
	}
	assert.Equal(t, len(p.ByRole("button", "Send it")), 1)
	assert.True(t, chooser.Has("data-file"))
	assert.Contains(t, body, `/_webui/file.`) // the script that makes the chooser a drop zone
	_, removes := p.Field("_rm")
	assert.False(t, removes)
}

func TestAFileUploadHandsTheFileToRunAsItArrives(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload[struct{}]) { f.Accept = []webui.FileKind{webui.FileImage} })
	rec := upload(s.h, `../../photos/x.png`, string(pngBytes), nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oUploaded x.png")
	got := s.last()
	assert.DeepEqual(t, got.data, pngBytes)
	assert.NoError(t, got.err)
	assert.Equal(t, got.handle, webui.FileHandle[struct{}]{
		Name: "x.png", Path: "photos", RawName: "../../photos/x.png", ContentType: "image/png", DeclaredType: "application/octet-stream", Kind: webui.FilePNG,
	})
}

// The file is read as it comes: Run has the first of it while the browser has not yet sent the rest, which is what lets a file of any size go on to a store.
func TestAFileUploadIsStreamedToRunAndNotHeldFirst(t *testing.T) {
	t.Parallel()

	first := make(chan struct{})
	var once sync.Once
	s := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Run = func(_ context.Context, h webui.FileHandle[struct{}]) (webui.Outcome, error) {
			buf := make([]byte, 64<<10)
			n, err := io.ReadAtLeast(h.Reader, buf, 4096)
			if err != nil {
				return webui.Outcome{}, err
			}
			if n >= 4096 {
				once.Do(func() { close(first) })
			}
			_, err = io.Copy(io.Discard, h.Reader)
			return webui.Success("Uploaded"), err
		}
	})
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	req := httptest.NewRequest(http.MethodPost, "/admin/up", pr)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.h.ServeHTTP(rec, req)
	}()
	go func() {
		_ = mw.WriteField("_view", "p")
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="a.bin"`)
		part, _ := mw.CreatePart(hdr)
		_, _ = part.Write(bytes.Repeat([]byte{1}, 128<<10))
		select { // the rest is not sent until Run has the first of it
		case <-first:
		case <-time.After(5 * time.Second):
			_ = pw.CloseWithError(errors.New("Run had none of the file while the rest was unsent"))
			return
		}
		_, _ = part.Write(bytes.Repeat([]byte{2}, 128<<10))
		_ = mw.Close()
		_ = pw.Close()
	}()
	<-done
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oUploaded")
}

// Nothing is written to disk, whatever the size: the temporary directory is empty while Run reads the file and after.
func TestAFileUploadWritesNothingToDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	var inRun int
	s := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Run = func(_ context.Context, h webui.FileHandle[struct{}]) (webui.Outcome, error) {
			n, err := io.Copy(io.Discard, h.Reader)
			entries, _ := os.ReadDir(dir)
			inRun = len(entries)
			if n != 3<<20 {
				return webui.Failure("short"), err
			}
			return webui.Success("Uploaded"), err
		}
	})
	rec := upload(s.h, "big.bin", strings.Repeat("x", 3<<20), nil)
	assert.Equal(t, flashOf(rec), "oUploaded")
	assert.Equal(t, inRun, 0)
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Equal(t, len(entries), 0)
}

// A file of the wrong kind is refused by its first bytes, in the words of the panel, and Run is not called.
func TestAFileUploadOfTheWrongKindIsRefusedWithoutCallingRun(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload[struct{}]) { f.Accept = []webui.FileKind{webui.FilePNG} })
	rec := upload(s.h, "plan.png", string(pdfBytes)+strings.Repeat("x", 1<<20), nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eChoose a PNG file.")
	assert.Equal(t, s.count(), 0)
}

// A file that is larger than the panel takes is refused in the words of the field, however Run took it: its reads fail past the limit, and what it did not read is
// read by the library, to know.
func TestAFileUploadLargerThanMaxSizeIsRefused(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload[struct{}]) { f.MaxSize = 1000 })
	rec := upload(s.h, "big.bin", strings.Repeat("x", 100_000), nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eMust be at most 1000 bytes.")
	assert.Equal(t, s.count(), 1)
	assert.True(t, s.last().err != nil)
	assert.True(t, len(s.last().data) <= 1000)

	// A Run that stops early, with the file unread, is not let off: the rest of it is read by the library, and is too large.
	early := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.MaxSize = 1000
		f.Receive.Run = func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
			return webui.Success("Uploaded"), nil
		}
	})
	assert.Equal(t, flashOf(upload(early.h, "big.bin", strings.Repeat("x", 100_000), nil)), "eMust be at most 1000 bytes.")

	// The size that it takes exactly is taken.
	assert.Equal(t, flashOf(upload(s.h, "ok.bin", strings.Repeat("x", 1000), nil)), "oUploaded ok.bin")
}

// A Run that returns before the end of the file is not an error: what is left is thrown away, and the browser is answered.
func TestAFileUploadRunThatReadsNothingIsAnswered(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Run = func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
			return webui.Success("Noted"), nil
		}
	})
	rec := upload(s.h, "a.bin", strings.Repeat("x", 1<<20), nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oNoted")
}

func TestAFileUploadOutcomeIsToldAsAnyActionsIs(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		outcome webui.Outcome
		err     error
		want    string
	}{
		"success": {outcome: webui.Success("Stored"), want: "oStored"},
		"warning": {outcome: webui.Warning("Stored, but slowly"), want: "wStored, but slowly"},
		"failure": {outcome: webui.Failure("The bucket is full"), want: "eThe bucket is full"},
		"error":   {err: errors.New("boom"), want: "eSomething went wrong. The cause was logged on the server."},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := newUploadShop(func(f *webui.FileUpload[struct{}]) {
				f.Receive.Run = func(_ context.Context, h webui.FileHandle[struct{}]) (webui.Outcome, error) {
					_, _ = io.Copy(io.Discard, h.Reader)
					return c.outcome, c.err
				}
			})
			rec := upload(s.h, "a.bin", "data", nil)
			assert.Equal(t, rec.Code, http.StatusSeeOther)
			assert.Equal(t, flashOf(rec), c.want)
		})
	}
}

// A page's arguments, if Run wants them, are read as in any action.
func TestAFileUploadRunCanReadThePagesArguments(t *testing.T) {
	t.Parallel()

	type args struct {
		Bucket string `webui:"bucket"`
	}
	var got string
	h := webui.App{Pages: webui.Pages{webui.Page[args]{Path: "/up/{bucket}", View: webui.FileUpload[struct{}]{
		MaxSize: 1 << 20,
		Receive: webui.Action[webui.FileHandle[struct{}]]{Run: func(ctx context.Context, f webui.FileHandle[struct{}]) (webui.Outcome, error) {
			_, _ = io.Copy(io.Discard, f.Reader)
			got = webui.ArgsOf[args](ctx).Bucket
			return webui.Success("Uploaded"), nil
		}},
	}}}}.MustCompile("/admin")
	rec := orderedPost(h, "/admin/up/firmware", nil, rawPart{"_view", "", "p"}, rawPart{"file", "a.bin", "data"})
	assert.Equal(t, flashOf(rec), "oUploaded")
	assert.Equal(t, got, "firmware")
}

// The Guard is asked when the panel is drawn, with a file that has not arrived: an error leaves the panel out, and Disabled draws it disabled with the reason.
func TestAFileUploadGuardDecidesHowThePanelIsDrawn(t *testing.T) {
	t.Parallel()

	hidden := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Guard = func(context.Context, webui.FileHandle[struct{}]) error { return errors.New("no") }
	})
	gone := pageOf(t, hidden.h, "/admin/up")
	assert.Equal(t, len(gone.ByAttr("type", "file")), 0)
	assert.Equal(t, len(gone.ByRole("heading", "Upload firmware")), 0)

	disabled := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Guard = func(_ context.Context, h webui.FileHandle[struct{}]) error {
			assert.True(t, h.Reader == nil) // a file that has not arrived
			return webui.Disabled("An upload is already running")
		}
	})
	off := pageOf(t, disabled.h, "/admin/up")
	assert.Equal(t, len(off.ByRole("heading", "Upload firmware")), 1)
	chooser, _ := off.Field("file")
	assert.True(t, chooser.Disabled)
	assert.Equal(t, len(off.ByText("Upload Not available: An upload is already running")), 1)
	assert.False(t, page.One(t, "the chooser", off.ByAttr("type", "file")).Has("data-file")) // script leaves a chooser that is disabled alone
}

// The Guard refuses the POST too, with its message, after the first bytes and before any more: Run is not called and the rest of the file is not read.
func TestAFileUploadGuardRefusesThePostBeforeTheRestOfTheFileIsRead(t *testing.T) {
	t.Parallel()

	for name, guard := range map[string]func(context.Context, webui.FileHandle[struct{}]) error{
		"an error": func(context.Context, webui.FileHandle[struct{}]) error { return errors.New("requires the editor role") },
		"disabled": func(context.Context, webui.FileHandle[struct{}]) error {
			return webui.Disabled("requires the editor role")
		},
		"the file": func(_ context.Context, h webui.FileHandle[struct{}]) error {
			if h.Name != "a.bin" || h.Kind != webui.FileAny || h.Reader == nil {
				return errors.New("the guard was not given the file")
			}
			return errors.New("requires the editor role")
		},
		"no guard": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := newUploadShop(func(f *webui.FileUpload[struct{}]) { f.Receive.Guard = guard })
			var read atomic.Int64
			rec := upload(s.h, "a.bin", strings.Repeat("x", 4<<20), func(r io.Reader) io.Reader {
				return io.TeeReader(r, writerFunc(func(b []byte) { read.Add(int64(len(b))) }))
			})
			assert.Equal(t, rec.Code, http.StatusSeeOther)
			if guard == nil {
				assert.Equal(t, s.count(), 1)
				return
			}
			assert.Contains(t, flashOf(rec), "requires the editor role")
			assert.Equal(t, s.count(), 0)
			assert.True(t, read.Load() < 1<<20) // the start of the file, and none of the rest
		})
	}
}

// A visitor that the page refuses has nothing of theirs read.
func TestAFileUploadOnAPageTheVisitorMayNotOpenHasNothingRead(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil, func(p *webui.Page[webui.NoArgs]) {
		p.Guard = func(context.Context, webui.NoArgs) error { return errors.New("no") }
	})
	var read atomic.Int64
	rec := upload(s.h, "a.bin", strings.Repeat("x", 1<<20), func(r io.Reader) io.Reader {
		return io.TeeReader(r, writerFunc(func(b []byte) { read.Add(int64(len(b))) }))
	})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, read.Load(), int64(0))
	assert.Equal(t, s.count(), 0)
}

// A post with no file in it, or a chooser that was left alone, says to choose one. A file of no bytes is a file: it is not "nothing chosen", which says the wrong thing of
// a file that was chosen, and a folder has such files in it.
func TestAFileUploadWithNoFileSaysToChooseOneAndAnEmptyFileIsAFile(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	for name, parts := range map[string][]rawPart{
		"no part":      {{"_view", "", "p"}},
		"an empty one": {{"_view", "", "p"}, {"file", "", ""}},
	} {
		t.Run(name, func(t *testing.T) {
			rec := orderedPost(s.h, "/admin/up", nil, parts...)
			assert.Equal(t, rec.Code, http.StatusSeeOther)
			assert.Equal(t, flashOf(rec), "eChoose a file.")
		})
	}
	assert.Equal(t, s.count(), 0)

	// A file with a name and no bytes is handed to Run, as any file is, with a reader that is at its end.
	rec := orderedPost(s.h, "/admin/up", nil, rawPart{"_view", "", "p"}, rawPart{"file", "dir/installer.yaml", ""})
	assert.Equal(t, flashOf(rec), "oUploaded installer.yaml")
	assert.Equal(t, s.count(), 1)
	assert.Equal(t, len(s.last().data), 0)
	assert.Equal(t, s.last().handle.Path, "dir")

	// One that the panel does not take by its kind is told that it is empty, and not that a kind of file is to be chosen.
	png := newUploadShop(func(f *webui.FileUpload[struct{}]) { f.Accept = []webui.FileKind{webui.FilePNG} })
	assert.Equal(t, flashOf(orderedPost(png.h, "/admin/up", nil, rawPart{"_view", "", "p"}, rawPart{"file", "a.png", ""})), "eThis file is empty.")
	assert.Equal(t, png.count(), 0)

	// The same as a fragment for script, which sends a folder's empty files as it sends the others.
	fragment := postFragment(s.h, "dir/empty.txt", "")
	assert.Equal(t, len(items(page.Parse(t, fragment.Body.String()))), 1)
	assert.True(t, strings.HasPrefix(items(page.Parse(t, fragment.Body.String()))[0], "Done: "))
	assert.False(t, strings.Contains(fragment.Body.String(), "Choose a file."))

	// The file before the panel is not anybody's.
	before := orderedPost(s.h, "/admin/up", nil, rawPart{"file", "a.bin", "data"}, rawPart{"_view", "", "p"})
	assert.Equal(t, flashOf(before), "eThat request was not understood.")
	assert.Equal(t, s.count(), 2)
}

// Uploads that arrive together are each their own.
func TestFileUploadsAtTheSameTimeAreEachTheirOwn(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			rec := upload(s.h, "a.bin", strings.Repeat("x", 1000+i), nil)
			assert.Equal(t, flashOf(rec), "oUploaded a.bin")
		})
	}
	wg.Wait()
	assert.Equal(t, s.count(), 20)
	sizes := map[int]bool{}
	for _, r := range s.runs {
		sizes[len(r.data)] = true
	}
	assert.Equal(t, len(sizes), 20)
}

// typedUpload posts the one file of a panel with a filename and a Content-Type of the test's choosing, which is what the browser says of a file
// and what a folder's files carry in their names.
func typedUpload(h http.Handler, filename, declared, content string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("_view", "p")
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	if declared != "" {
		hdr.Set("Content-Type", declared)
	}
	part, _ := w.CreatePart(hdr)
	_, _ = part.Write([]byte(content))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/admin/up", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The type that the browser said is the visitor's word and is handed over beside what the first bytes hold, without its parameters.
func TestAFileUploadHandsOverTheTypeThatTheBrowserSaid(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	for declared, want := range map[string]string{
		"application/json":                "application/json",
		"Application/JSON; charset=utf-8": "application/json",
		"application/octet-stream":        "application/octet-stream",
		"":                                "",
		"not a media type":                "",
	} {
		assert.Equal(t, typedUpload(s.h, "a.json", declared, `{"a": 1}`).Code, http.StatusSeeOther)
		got := s.last().handle
		assert.Equal(t, got.DeclaredType, want)
		assert.Equal(t, got.ContentType, "text/plain; charset=utf-8") // what it holds, which is not what was said
	}
}

// A file that was sent in a folder has the folder apart from its name, made safe, and what the browser sent as it was.
func TestAFileUploadHandsOverTheFolderAndTheNameAsSent(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	for raw, want := range map[string]struct{ name, path string }{
		"a.jpg":             {"a.jpg", ""},
		"photos/2024/a.jpg": {"a.jpg", "photos/2024"},
		`photos\2024\a.jpg`: {"a.jpg", "photos/2024"},
		"../../etc/passwd":  {"passwd", "etc"},
		"a/../../b/c.txt":   {"c.txt", "a/b"},
		"dir/":              {"file", "dir"},
	} {
		assert.Equal(t, typedUpload(s.h, raw, "", "data").Code, http.StatusSeeOther)
		got := s.last().handle
		assert.Equal(t, got.Name, want.name)
		assert.Equal(t, got.Path, want.path)
		assert.Equal(t, got.RawName, raw) // as the browser sent it, so that the app can say that it was changed
	}
}

// choice is the options of a panel that takes some: what the visitor chooses beside the file.
type choice struct {
	Note    string
	Channel string
}

// choiceShop is a page with a FileUpload that has options, and what its Run was handed.
type choiceShop struct {
	mu   sync.Mutex
	runs []webui.FileHandle[choice]
	h    http.Handler
}

func (s *choiceShop) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.runs)
}

func newChoiceShop(edit func(*webui.FileUpload[choice])) *choiceShop {
	s := &choiceShop{}
	up := webui.FileUpload[choice]{
		Title: "Upload", MaxSize: 5 << 20,
		Load: func(context.Context) (choice, webui.Loaded) {
			return choice{Note: "a note", Channel: "stable"}, webui.Ready()
		},
		Fields: []webui.Accessor[choice]{
			webui.String[choice]{
				Label: "Note", Rules: webui.StringRules{MinLen: 3},
				Load: func(c choice) string { return c.Note }, Store: func(c *choice, v string) { c.Note = v },
			},
			webui.Select[choice]{
				Label: "Channel", Required: true,
				Options: []webui.Option{{Value: "stable"}, {Value: "beta"}},
				Load:    func(c choice) []string { return []string{c.Channel} }, Store: func(c *choice, v []string) { c.Channel = v[0] },
			},
		},
		Receive: webui.Action[webui.FileHandle[choice]]{Run: func(_ context.Context, f webui.FileHandle[choice]) (webui.Outcome, error) {
			_, _ = io.Copy(io.Discard, f.Reader)
			f.Reader = nil
			s.mu.Lock()
			s.runs = append(s.runs, f)
			s.mu.Unlock()
			return webui.Success("Uploaded " + f.Name), nil
		}},
	}
	if edit != nil {
		edit(&up)
	}
	s.h = webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", View: up}}}.MustCompile("/admin")
	return s
}

// chosen posts a panel with options: the panel, the fields in the order that the form has them, and then the one file.
func chosen(h http.Handler, wrap func(io.Reader) io.Reader, note, channel, content string) *httptest.ResponseRecorder {
	return orderedPost(h, "/admin/up", wrap,
		rawPart{"_view", "", "p"}, rawPart{"f0", "", note}, rawPart{"f1", "", channel}, rawPart{"file", "a.bin", content})
}

func TestAFileUploadHasInputsBesideTheFileWithTheirDefaultsAndTheirRules(t *testing.T) {
	t.Parallel()

	s := newChoiceShop(nil)
	p := pageOf(t, s.h, "/admin/up")
	assert.DeepEqual(t, fieldNames(p), []string{"_view", "f0", "f1", "file"}) // in the order of the form, the file last, so that they arrive first
	note, _ := p.Field("f0")
	assert.Equal(t, note.Value, "a note")                                                   // the defaults that Load gave
	assert.Equal(t, page.One(t, "the note", p.ByAttr("name", "f0")).Attr("minlength"), "3") // the rules, as a form's inputs have them
	assert.Equal(t, len(p.ByAttr("enctype", "multipart/form-data")), 1)
}

func TestAFileUploadHandsRunWhatWasChosenBesideTheFile(t *testing.T) {
	t.Parallel()

	s := newChoiceShop(nil)
	rec := chosen(s.h, nil, "my note", "beta", "data")
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oUploaded a.bin")
	assert.Equal(t, s.runs[0].Options, choice{Note: "my note", Channel: "beta"})
}

// The Guard is given the options as bound, since what the visitor chose may be what decides whether they may.
func TestAFileUploadGuardIsGivenTheOptions(t *testing.T) {
	t.Parallel()

	s := newChoiceShop(func(f *webui.FileUpload[choice]) {
		f.Receive.Guard = func(_ context.Context, h webui.FileHandle[choice]) error {
			if h.Reader != nil && h.Options.Channel == "beta" {
				return errors.New("beta is not for you")
			}
			return nil
		}
	})
	assert.Equal(t, flashOf(chosen(s.h, nil, "note", "stable", "data")), "oUploaded a.bin")
	assert.Equal(t, flashOf(chosen(s.h, nil, "note", "beta", "data")), "ebeta is not for you")
	assert.Equal(t, s.count(), 1)
}

// What is chosen is checked by the rules of its fields before any of the file is read: a refusal costs the file nothing, and says which field.
func TestAFileUploadChoiceThatIsRefusedIsRefusedBeforeTheFileIsRead(t *testing.T) {
	t.Parallel()

	s := newChoiceShop(nil)
	for name, c := range map[string]struct{ note, channel, want string }{
		"too short":      {"x", "stable", "Note: Must be at least 3 characters."},
		"not an option":  {"note", "nightly", "Channel:"},
		"nothing chosen": {"note", "", "Channel: "},
	} {
		var read atomic.Int64
		rec := chosen(s.h, func(r io.Reader) io.Reader {
			return io.TeeReader(r, writerFunc(func(b []byte) { read.Add(int64(len(b))) }))
		}, c.note, c.channel, strings.Repeat("x", 4<<20))
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.Contains(t, flashOf(rec), c.want)
		assert.True(t, read.Load() < 1<<20) // the fields, and none of the file
		_ = name
	}
	assert.Equal(t, s.count(), 0)
}

// A field that comes after the file is not a choice: it was not there when the file began, and is not read.
func TestAFileUploadFieldAfterTheFileIsIgnored(t *testing.T) {
	t.Parallel()

	s := newChoiceShop(nil)
	rec := orderedPost(s.h, "/admin/up", nil,
		rawPart{"_view", "", "p"}, rawPart{"f0", "", "first"}, rawPart{"f1", "", "stable"}, rawPart{"file", "a.bin", "data"}, rawPart{"f0", "", "late"})
	assert.Equal(t, flashOf(rec), "oUploaded a.bin")
	assert.Equal(t, s.runs[0].Options.Note, "first")
}

// A panel with no Load has the zero options as its defaults, and one with no Fields has none to choose.
func TestAFileUploadOptionsAreOptional(t *testing.T) {
	t.Parallel()

	s := newChoiceShop(func(f *webui.FileUpload[choice]) { f.Load, f.Fields = nil, nil })
	_, offered := pageOf(t, s.h, "/admin/up").Field("f0")
	assert.False(t, offered)
	rec := orderedPost(s.h, "/admin/up", nil, rawPart{"_view", "", "p"}, rawPart{"file", "a.bin", "data"})
	assert.Equal(t, flashOf(rec), "oUploaded a.bin")
	assert.Equal(t, s.runs[0].Options, choice{})
}

func TestAFileUploadFieldsAreCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(f webui.FileUpload[choice]) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", View: f}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	note := webui.String[choice]{Label: "Note", Load: func(c choice) string { return c.Note }}
	receive := webui.Action[webui.FileHandle[choice]]{Run: func(context.Context, webui.FileHandle[choice]) (webui.Outcome, error) {
		return webui.Success("ok"), nil
	}}
	assert.Contains(t, problems(webui.FileUpload[choice]{MaxSize: 1, Receive: receive, Fields: []webui.Accessor[choice]{note, note}}), "used twice")
	assert.Contains(t, problems(webui.FileUpload[choice]{
		MaxSize: 1, Receive: receive,
		Fields: []webui.Accessor[choice]{webui.File[choice]{Label: "F", Load: func(choice) *webui.FileInfo { return nil }}},
	}), "is a File")
	_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", View: webui.FileUpload[choice]{
		MaxSize: 1, Receive: receive, Fields: []webui.Accessor[choice]{note},
	}}}}.Compile("/admin")
	assert.NoError(t, err)
}

// postFragment posts the one file of a panel as the upload script does: a request for the panel's HTML, which carries the panel's header.
func postFragment(h http.Handler, filename, content string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("_view", "p")
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	part, _ := w.CreatePart(hdr)
	_, _ = part.Write([]byte(content))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/admin/up", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Webui-View", "p")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// A post that script made, which asks for the panel's HTML, is answered with HTML: one list item with the tone and the text of the outcome, and no flash
// and no redirect, which a browser that navigates is given. Nothing of it is data.
func TestAFileUploadThatScriptSendsIsAnsweredWithAnHTMLFragment(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		outcome webui.Outcome
		err     error
		status  int
		tone    string
		text    string
	}{
		"success": {outcome: webui.Success("Stored it"), status: 200, tone: "Done: ", text: "Stored it"},
		"warning": {outcome: webui.Warning("Stored it slowly"), status: 200, tone: "Warning: ", text: "Stored it slowly"},
		"failure": {outcome: webui.Failure("The bucket is full"), status: 422, tone: "Failed: ", text: "The bucket is full"},
		"error":   {err: errors.New("boom"), status: 500, tone: "Failed: ", text: "Something went wrong. The cause was logged on the server."},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := newUploadShop(func(f *webui.FileUpload[struct{}]) {
				f.Receive.Run = func(_ context.Context, h webui.FileHandle[struct{}]) (webui.Outcome, error) {
					_, _ = io.Copy(io.Discard, h.Reader)
					return c.outcome, c.err
				}
			})
			rec := postFragment(s.h, "a.bin", "data")
			assert.Equal(t, rec.Code, c.status)
			assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
			assert.Equal(t, rec.Header().Get("Location"), "")
			assert.Equal(t, flashOf(rec), "") // no flash: the HTML is the answer
			told := page.Parse(t, rec.Body.String())
			assert.DeepEqual(t, items(told), []string{c.tone + c.text})
			assert.Equal(t, len(told.Links()), 0) // it goes nowhere else
		})
	}
}

// The text is the server's, and is escaped; a file that was refused says so in the same item.
func TestAnUploadFragmentEscapesTheTextAndSaysWhatWasRefused(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload[struct{}]) { f.Accept = []webui.FileKind{webui.FilePNG} })
	assert.DeepEqual(t, items(page.Parse(t, postFragment(s.h, "a.png", string(pdfBytes)).Body.String())), []string{"Failed: Choose a PNG file."})

	named := newUploadShop(func(f *webui.FileUpload[struct{}]) {
		f.Receive.Run = func(_ context.Context, h webui.FileHandle[struct{}]) (webui.Outcome, error) {
			_, _ = io.Copy(io.Discard, h.Reader)
			return webui.Success("Uploaded " + h.Name), nil
		}
	})
	body := postFragment(named.h, "<b>x.bin", "data").Body.String()
	assert.False(t, strings.Contains(body, "<b>"))
	assert.Contains(t, body, "Uploaded &lt;b&gt;x.bin")
}

// An outcome that goes somewhere is a link in the answer: the next action is in the HTML, and the script does not need to know it.
func TestAnUploadFragmentOffersALinkToWhereTheOutcomeGoes(t *testing.T) {
	t.Parallel()

	noop := webui.Action[webui.FileHandle[struct{}]]{Run: func(context.Context, webui.FileHandle[struct{}]) (webui.Outcome, error) {
		return webui.Success("ok"), nil
	}}
	done := webui.Page[webui.NoArgs]{Path: "/done", View: webui.FileUpload[struct{}]{MaxSize: 1, Receive: noop}}
	up := webui.FileUpload[struct{}]{
		MaxSize: 1 << 20,
		Receive: webui.Action[webui.FileHandle[struct{}]]{Run: func(ctx context.Context, h webui.FileHandle[struct{}]) (webui.Outcome, error) {
			_, _ = io.Copy(io.Discard, h.Reader)
			return webui.Success("Uploaded").Then(webui.Open(ctx, done, webui.NoArgs{})), nil
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", View: up}, done}}.MustCompile("/admin")
	told := page.Parse(t, postFragment(h, "a.bin", "data").Body.String())
	assert.True(t, hasLink(told, "Continue", "/admin/done"))
	assert.DeepEqual(t, items(told), []string{"Done: Uploaded Continue"})
}

// A page that refuses the visitor refuses a post of script's as it does any: with the status, and the words as the HTML of the item.
func TestAFileUploadPostOnAPageTheVisitorMayNotOpenIsRefusedAsAFragment(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil, func(p *webui.Page[webui.NoArgs]) {
		p.Guard = func(context.Context, webui.NoArgs) error { return errors.New("requires the editor role") }
	})
	rec := postFragment(s.h, "a.bin", "data")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.DeepEqual(t, items(page.Parse(t, rec.Body.String())), []string{"Failed: requires the editor role"})
	assert.Equal(t, s.count(), 0)
}

// The panel tells the script how it sends, in data-upload, and the chooser takes one file without script: the script is what lets it take several, so that a post
// never drops files that the visitor chose.
func TestAFileUploadTellsTheScriptWhetherItTakesSeveralAndTakesOneWithout(t *testing.T) {
	t.Parallel()

	chooserOf := func(h http.Handler) page.El {
		return page.One(t, "the chooser", pageOf(t, h, "/admin/up").ByAttr("type", "file"))
	}
	one := chooserOf(newUploadShop(nil).h)
	assert.True(t, len(pageOf(t, newUploadShop(nil).h, "/admin/up").ByAttr("data-upload", "")) > 0)
	assert.Equal(t, len(pageOf(t, newUploadShop(nil).h, "/admin/up").ByAttr("data-multiple", "")), 0) // one file
	several := newUploadShop(func(f *webui.FileUpload[struct{}]) { f.Multiple = true })
	assert.Equal(t, len(pageOf(t, several.h, "/admin/up").ByAttr("data-multiple", "")), 1)
	for _, c := range []page.El{one, chooserOf(several.h)} {
		assert.False(t, c.Has("multiple")) // not in the markup: script adds it
		assert.False(t, c.Has("webkitdirectory"))
	}
}

// The scripts are linked where they are used, and the upload's after the file's, which has made the chooser a drop zone when it runs.
func TestThePageLinksTheUploadScriptOnlyForAFileUploadAndAfterTheFileScript(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	body := serve(s.h, http.MethodGet, "/admin/up").Body.String()
	file, upload := strings.Index(body, `/_webui/file.`), strings.Index(body, `/_webui/upload.`)
	assert.True(t, file >= 0 && upload > file)
	assert.Contains(t, body, `<script src="`+assetURL(t, s.h, "upload.js")+`" defer></script>`)
	assert.False(t, strings.Contains(serve(newFileShop(plainFile()).h, http.MethodGet, "/admin/f").Body.String(), "/_webui/upload."))
	assert.False(t, strings.Contains(serve(newShop().h, http.MethodGet, "/admin/device/dev1").Body.String(), "/_webui/upload."))
}

// fieldNames are the names of what a page posts, in the order that a browser would send them.
func fieldNames(p *page.Page) []string {
	var out []string
	for _, f := range p.Fields() {
		out = append(out, f.Name)
	}
	return out
}
