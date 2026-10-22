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
)

// arrived is what a file upload's Run was handed, and read from it.
type arrived struct {
	handle webui.FileHandle // without its Reader
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
func newUploadShop(edit func(*webui.FileUpload), pageOpts ...func(*webui.Page[webui.NoArgs])) *uploadShop {
	s := &uploadShop{}
	up := webui.FileUpload{
		Title: "Upload firmware", MaxSize: 5 << 20,
		Receive: webui.Action[webui.FileHandle]{Run: func(_ context.Context, f webui.FileHandle) (webui.Outcome, error) {
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
	page := webui.Page[webui.NoArgs]{Path: "/up", Body: up}
	for _, o := range pageOpts {
		o(&page)
	}
	s.h = webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	return s
}

// upload posts the one file of a panel, as a browser does: the leaf first.
func upload(h http.Handler, filename, content string, wrap func(io.Reader) io.Reader) *httptest.ResponseRecorder {
	return orderedPost(h, "/admin/up", wrap, rawPart{"_leaf", "", "p"}, rawPart{"file", filename, content})
}

func TestAFileUploadIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(f webui.FileUpload) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", Body: f}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	run := func(context.Context, webui.FileHandle) (webui.Outcome, error) { return webui.Success("ok"), nil }
	receive := webui.Action[webui.FileHandle]{Run: run}
	assert.Contains(t, problems(webui.FileUpload{MaxSize: 1}), "FileUpload.Receive has no Run")
	assert.Contains(t, problems(webui.FileUpload{Receive: receive}), "FileUpload has MaxSize 0")
	assert.Contains(t, problems(webui.FileUpload{Receive: receive, MaxSize: -5}), "FileUpload has MaxSize -5")
	for _, k := range []webui.FileKind{webui.FileJSON, webui.FileCSV, webui.FileText} {
		assert.Contains(t, problems(webui.FileUpload{Receive: receive, MaxSize: 1, Accept: []webui.FileKind{webui.FileImage, k}}), "told by all of the file")
	}
	assert.Contains(t, problems(webui.FileUpload{Receive: receive, MaxSize: 1, Accept: []webui.FileKind{webui.FileKind(99)}}), "is not a FileKind")
	_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/up", Body: webui.FileUpload{
		Receive: receive, MaxSize: 1 << 40, Accept: []webui.FileKind{webui.FileImage, webui.FilePDF, webui.FileAny},
	}}}}.Compile("/admin")
	assert.NoError(t, err) // as large as the app says, with no ceiling
}

// The panel is a native form of its own: the leaf comes before the one file, and the file chooser has what the script reads, so that it works as a File does.
func TestAFileUploadIsAFormOfItsOwnWithTheLeafBeforeTheFile(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload) {
		f.Accept, f.MaxSize, f.Help, f.Desc = []webui.FileKind{webui.FilePNG, webui.FileJPEG}, 2<<30, "The image that the device shows.", "Sent to the bucket."
		f.Receive.Label = "Send it"
	})
	body := serve(s.h, http.MethodGet, "/admin/up").Body.String()
	assert.Contains(t, body, "Upload firmware")
	assert.Contains(t, body, `enctype="multipart/form-data"`)
	leaf, file := strings.Index(body, `name="_leaf"`), strings.Index(body, `type="file"`)
	assert.True(t, leaf >= 0 && file > leaf)
	assert.Contains(t, body, `name="file" accept="image/png,.png,image/jpeg,.jpg,.jpeg" required`)
	assert.Contains(t, body, "A PNG or JPEG file")
	assert.Contains(t, body, "Up to 2 GiB")
	assert.Contains(t, body, "The image that the device shows.")
	assert.Contains(t, body, "Send it")
	assert.Contains(t, body, `data-file=`)
	assert.Contains(t, body, `/_webui/file.`) // the script that makes the chooser a drop zone
	assert.False(t, strings.Contains(body, `name="_rm"`))
}

func TestAFileUploadHandsTheFileToRunAsItArrives(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload) { f.Accept = []webui.FileKind{webui.FileImage} })
	rec := upload(s.h, `../../photos/x.png`, string(pngBytes), nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oUploaded x.png")
	got := s.last()
	assert.DeepEqual(t, got.data, pngBytes)
	assert.NoError(t, got.err)
	assert.Equal(t, got.handle, webui.FileHandle{Name: "x.png", ContentType: "image/png", Kind: webui.FilePNG})
}

// The file is read as it comes: Run has the first of it while the browser has not yet sent the rest, which is what lets a file of any size go on to a store.
func TestAFileUploadIsStreamedToRunAndNotHeldFirst(t *testing.T) {
	t.Parallel()

	first := make(chan struct{})
	var once sync.Once
	s := newUploadShop(func(f *webui.FileUpload) {
		f.Receive.Run = func(_ context.Context, h webui.FileHandle) (webui.Outcome, error) {
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
		_ = mw.WriteField("_leaf", "p")
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
	s := newUploadShop(func(f *webui.FileUpload) {
		f.Receive.Run = func(_ context.Context, h webui.FileHandle) (webui.Outcome, error) {
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

	s := newUploadShop(func(f *webui.FileUpload) { f.Accept = []webui.FileKind{webui.FilePNG} })
	rec := upload(s.h, "plan.png", string(pdfBytes)+strings.Repeat("x", 1<<20), nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eChoose a PNG file.")
	assert.Equal(t, s.count(), 0)
}

// A file that is larger than the panel takes is refused in the words of the field, however Run took it: its reads fail past the limit, and what it did not read is
// read by the library, to know.
func TestAFileUploadLargerThanMaxSizeIsRefused(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload) { f.MaxSize = 1000 })
	rec := upload(s.h, "big.bin", strings.Repeat("x", 100_000), nil)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eMust be at most 1000 bytes.")
	assert.Equal(t, s.count(), 1)
	assert.True(t, s.last().err != nil)
	assert.True(t, len(s.last().data) <= 1000)

	// A Run that stops early, with the file unread, is not let off: the rest of it is read by the library, and is too large.
	early := newUploadShop(func(f *webui.FileUpload) {
		f.MaxSize = 1000
		f.Receive.Run = func(context.Context, webui.FileHandle) (webui.Outcome, error) { return webui.Success("Uploaded"), nil }
	})
	assert.Equal(t, flashOf(upload(early.h, "big.bin", strings.Repeat("x", 100_000), nil)), "eMust be at most 1000 bytes.")

	// The size that it takes exactly is taken.
	assert.Equal(t, flashOf(upload(s.h, "ok.bin", strings.Repeat("x", 1000), nil)), "oUploaded ok.bin")
}

// A Run that returns before the end of the file is not an error: what is left is thrown away, and the browser is answered.
func TestAFileUploadRunThatReadsNothingIsAnswered(t *testing.T) {
	t.Parallel()

	s := newUploadShop(func(f *webui.FileUpload) {
		f.Receive.Run = func(context.Context, webui.FileHandle) (webui.Outcome, error) { return webui.Success("Noted"), nil }
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

			s := newUploadShop(func(f *webui.FileUpload) {
				f.Receive.Run = func(_ context.Context, h webui.FileHandle) (webui.Outcome, error) {
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
	h := webui.App{Pages: webui.Pages{webui.Page[args]{Path: "/up/{bucket}", Body: webui.FileUpload{
		MaxSize: 1 << 20,
		Receive: webui.Action[webui.FileHandle]{Run: func(ctx context.Context, f webui.FileHandle) (webui.Outcome, error) {
			_, _ = io.Copy(io.Discard, f.Reader)
			got = webui.ArgsOf[args](ctx).Bucket
			return webui.Success("Uploaded"), nil
		}},
	}}}}.MustCompile("/admin")
	rec := orderedPost(h, "/admin/up/firmware", nil, rawPart{"_leaf", "", "p"}, rawPart{"file", "a.bin", "data"})
	assert.Equal(t, flashOf(rec), "oUploaded")
	assert.Equal(t, got, "firmware")
}

// The Guard is asked when the panel is drawn, with a file that has not arrived: an error leaves the panel out, and Disabled draws it disabled with the reason.
func TestAFileUploadGuardDecidesHowThePanelIsDrawn(t *testing.T) {
	t.Parallel()

	hidden := newUploadShop(func(f *webui.FileUpload) {
		f.Receive.Guard = func(context.Context, webui.FileHandle) error { return errors.New("no") }
	})
	body := serve(hidden.h, http.MethodGet, "/admin/up").Body.String()
	assert.False(t, strings.Contains(body, `type="file"`))
	assert.False(t, strings.Contains(body, "Upload firmware"))

	disabled := newUploadShop(func(f *webui.FileUpload) {
		f.Receive.Guard = func(_ context.Context, h webui.FileHandle) error {
			assert.True(t, h.Reader == nil) // a file that has not arrived
			return webui.Disabled("An upload is already running")
		}
	})
	body = serve(disabled.h, http.MethodGet, "/admin/up").Body.String()
	assert.Contains(t, body, "Upload firmware")
	assert.Contains(t, body, `type="file"`)
	assert.Contains(t, body, "disabled")
	assert.Contains(t, body, "An upload is already running")
	assert.False(t, strings.Contains(body, `data-file=`)) // script leaves a chooser that is disabled alone
}

// The Guard refuses the POST too, with its message, after the first bytes and before any more: Run is not called and the rest of the file is not read.
func TestAFileUploadGuardRefusesThePostBeforeTheRestOfTheFileIsRead(t *testing.T) {
	t.Parallel()

	for name, guard := range map[string]func(context.Context, webui.FileHandle) error{
		"an error": func(context.Context, webui.FileHandle) error { return errors.New("requires the editor role") },
		"disabled": func(context.Context, webui.FileHandle) error { return webui.Disabled("requires the editor role") },
		"the file": func(_ context.Context, h webui.FileHandle) error {
			if h.Name != "a.bin" || h.Kind != webui.FileAny || h.Reader == nil {
				return errors.New("the guard was not given the file")
			}
			return errors.New("requires the editor role")
		},
		"no guard": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := newUploadShop(func(f *webui.FileUpload) { f.Receive.Guard = guard })
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

// A post with no file in it, or a chooser that was left alone, says to choose one.
func TestAFileUploadWithNoFileSaysToChooseOne(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	for name, parts := range map[string][]rawPart{
		"no part":      {{"_leaf", "", "p"}},
		"an empty one": {{"_leaf", "", "p"}, {"file", "", ""}},
		"no bytes":     {{"_leaf", "", "p"}, {"file", "a.bin", ""}},
	} {
		rec := orderedPost(s.h, "/admin/up", nil, parts...)
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.Equal(t, flashOf(rec), "eChoose a file.")
		_ = name
	}
	assert.Equal(t, s.count(), 0)

	// The file before the leaf is not anybody's.
	rec := orderedPost(s.h, "/admin/up", nil, rawPart{"file", "a.bin", "data"}, rawPart{"_leaf", "", "p"})
	assert.Equal(t, flashOf(rec), "eThat request was not understood.")
	assert.Equal(t, s.count(), 0)
}

// Uploads that arrive together are each their own.
func TestFileUploadsAtTheSameTimeAreEachTheirOwn(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := upload(s.h, "a.bin", strings.Repeat("x", 1000+i), nil)
			assert.Equal(t, flashOf(rec), "oUploaded a.bin")
		}()
	}
	wg.Wait()
	assert.Equal(t, s.count(), 20)
	sizes := map[int]bool{}
	for _, r := range s.runs {
		sizes[len(r.data)] = true
	}
	assert.Equal(t, len(sizes), 20)
}
