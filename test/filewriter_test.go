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
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// withSubmit changes the action of the form that a file shop is the page of.
func withSubmit(edit func(*webui.Action[attachment])) func(*webui.Page[webui.NoArgs]) {
	return func(p *webui.Page[webui.NoArgs]) {
		f := p.Body.(webui.Form[attachment])
		edit(&f.Submit)
		p.Body = f
	}
}

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

// settledOnce asserts that every file that was opened was closed once and then either submitted or discarded, once, as wanted.
func settledOnce(t *testing.T, s *fileShop, submitted bool) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.True(t, len(s.writers) > 0)
	for _, w := range s.writers {
		w.writeLock.Lock()
		got := [3]int{w.closes, w.submits, w.discards}
		w.writeLock.Unlock()
		want := [3]int{1, 0, 1}
		if submitted {
			want = [3]int{1, 1, 0}
		}
		assert.Equal(t, got, want)
	}
}

// A file reaches its writer as it arrives, and the library holds none of it: with the temporary directory empty and watched, three
// megabytes pass through the request, which is more than the one that the standard library would keep in memory before it wrote the
// rest to a file of its own.
func TestAFileIsWrittenAsItArrivesAndNothingIsSpooledToDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	s := newFileShop(plainFile(webui.FilePNG))
	w := &watched{dir: dir}
	s.watch = w
	data := string(pngBytes) + strings.Repeat("x", 3<<20)
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "big.png", data})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, s.writers[0].buf.Len(), len(data))
	assert.True(t, w.writes > 10) // in pieces, not at once
	assert.Equal(t, w.leftover, "")
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Equal(t, len(entries), 0)
}

// watched looks in a directory at every write, for anything that was put there.
type watched struct {
	dir      string
	writes   int
	leftover string
}

func (w *watched) saw() {
	w.writes++
	if entries, err := os.ReadDir(w.dir); err == nil && len(entries) > 0 && w.leftover == "" {
		w.leftover = entries[0].Name()
	}
}

// The body is read as it is written to the writer, and not before: the writer has the beginning of the file while the browser has not
// sent the rest.
func TestTheWriterHasTheStartOfAFileBeforeTheRestHasBeenSent(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	first := make(chan struct{})
	var once sync.Once
	s.onWrite = func(n int) {
		if n >= 4096 {
			once.Do(func() { close(first) })
		}
	}
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	req := httptest.NewRequest(http.MethodPost, "/admin/f", pr)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.h.ServeHTTP(rec, req)
	}()

	go func() {
		_ = mw.WriteField("_leaf", "p")
		_ = mw.WriteField("f0", "Report")
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="f1"; filename="a.png"`)
		part, _ := mw.CreatePart(hdr)
		_, _ = part.Write(append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{1}, 64<<10)...))
		select { // the rest is not sent until the first of it has been written
		case <-first:
		case <-time.After(5 * time.Second):
			_ = pw.CloseWithError(errors.New("the writer had none of the file while the rest was unsent"))
			return
		}
		_, _ = part.Write(bytes.Repeat([]byte{2}, 64<<10))
		_ = mw.Close()
		_ = pw.Close()
	}()
	<-done
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oSaved")
	assert.Equal(t, s.writers[0].buf.Len(), len(pngBytes)+128<<10)
}

// A file that cannot be any of the kinds is refused before a writer is asked for: nothing is opened, so nothing is written or left.
func TestAFileOfTheWrongKindIsRefusedBeforeAnythingIsOpened(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "plan.png", string(pdfBytes)})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose a PNG file.")
	assert.Equal(t, s.opened(), 0)
}

// A kind that is told by the whole of the file is written, and found wanting at the end: the writer is closed and discarded.
func TestAJSONFileThatIsNotJSONIsFoundOutAtTheEndAndDiscarded(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FileJSON))
	rec := multipartPost(s.h, "/admin/f", nil, part{"f1", "a.json", `{"a": `})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose a JSON file.")
	assert.Equal(t, s.stores, 0)
	assert.Equal(t, s.opened(), 1)
	settledOnce(t, s, false)

	s = newFileShop(plainFile(webui.FileJSON))
	assert.Equal(t, multipartPost(s.h, "/admin/f", nil, part{"f1", "a.json", `{"a": 1}`}).Code, http.StatusSeeOther)
	settledOnce(t, s, true)
}

// Every file that was opened ends in exactly one of Submit and Discard, whichever way the submission ends.
func TestEveryFileThatWasOpenedIsSubmittedWhenTheFormIsAcceptedAndDiscardedWhenItIsNot(t *testing.T) {
	t.Parallel()

	ok := func(*webui.Action[attachment]) {}
	for _, c := range []struct {
		name      string
		edit      func(*webui.Action[attachment])
		values    map[string]string
		wantCode  int
		submitted bool
	}{
		{"accepted", ok, nil, http.StatusSeeOther, true},
		{"a warning is accepted", func(a *webui.Action[attachment]) {
			a.Run = func(context.Context, attachment) (webui.Outcome, error) { return webui.Warning("careful"), nil }
		}, nil, http.StatusSeeOther, true},
		{"accepted and told to stay", func(a *webui.Action[attachment]) {
			a.Run = func(context.Context, attachment) (webui.Outcome, error) { return webui.Success("Saved").Stay(), nil }
		}, nil, http.StatusSeeOther, true},
		{"accepted and told to go back", func(a *webui.Action[attachment]) {
			a.Run = func(context.Context, attachment) (webui.Outcome, error) { return webui.Success("Saved").Back(), nil }
		}, nil, http.StatusSeeOther, true},
		{"a field that a rule refuses", ok, map[string]string{"f0": "x"}, http.StatusUnprocessableEntity, false},
		{"the action's guard refuses", func(a *webui.Action[attachment]) {
			a.Guard = func(context.Context, attachment) error { return errors.New("not you") }
		}, nil, http.StatusSeeOther, false},
		{"a run that errors", func(a *webui.Action[attachment]) {
			a.Run = func(context.Context, attachment) (webui.Outcome, error) { return webui.Outcome{}, errors.New("boom") }
		}, nil, http.StatusSeeOther, false},
		{"a run that fails", func(a *webui.Action[attachment]) {
			a.Run = func(context.Context, attachment) (webui.Outcome, error) { return webui.Failure("no"), nil }
		}, nil, http.StatusUnprocessableEntity, false},
		{"a run that rejects", func(a *webui.Action[attachment]) {
			a.Run = func(context.Context, attachment) (webui.Outcome, error) { return webui.Reject[attachment](), nil }
		}, nil, http.StatusUnprocessableEntity, false},
		{"a run that panics", func(a *webui.Action[attachment]) {
			a.Run = func(context.Context, attachment) (webui.Outcome, error) { panic("boom") }
		}, nil, http.StatusInternalServerError, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s := newFileShop(plainFile(webui.FilePNG), withSubmit(c.edit))
			rec := multipartPost(s.h, "/admin/f", c.values, part{"f1", "a.png", string(pngBytes)})
			if c.wantCode == http.StatusInternalServerError {
				assert.True(t, rec.Code == http.StatusInternalServerError || rec.Code == http.StatusSeeOther)
			} else {
				assert.Equal(t, rec.Code, c.wantCode)
			}
			settledOnce(t, s, c.submitted)
		})
	}
}

// A file larger than the field takes is not written past the limit, the rest is read and thrown away, and what comes after it in the form
// is not lost: the form is drawn again with it.
func TestAFileOverTheLimitStopsBeingWrittenAndTheRestOfTheFormIsStillRead(t *testing.T) {
	t.Parallel()

	s := newFileShop(func(s *fileShop) webui.File[attachment, *recWriter] {
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
	assert.True(t, s.writers[0].buf.Len() <= 1000)
	assert.Equal(t, s.stores, 0)
	settledOnce(t, s, false)
}

// A field that a browser sends after the file is read as any other.
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

	// The leaf says whose the file is, so it comes first: a file that comes before it is not anybody's.
	s = newFileShop(plainFile(webui.FilePNG))
	rec = orderedPost(s.h, "/admin/f", nil,
		rawPart{"f1", "a.png", string(pngBytes)},
		rawPart{"_leaf", "", "p"})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eThat request was not understood.")
	assert.Equal(t, s.opened(), 0)
}

// What a writer cannot do is told in the words of the field, and its cause is not.
func TestAWriterThatCannotBeOpenedWrittenOrClosedRefusesTheFile(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		fail failures
		open int
	}{
		{"an Open that fails", failures{open: errors.New("no bucket")}, 0},
		{"a Write that fails", failures{write: errors.New("disk full")}, 1},
		{"a Close that fails", failures{close: errors.New("upload could not be completed")}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s := newFileShop(plainFile(webui.FilePNG))
			s.fail = c.fail
			rec := orderedPost(s.h, "/admin/f", nil,
				rawPart{"_leaf", "", "p"},
				rawPart{"f1", "a.png", string(pngBytes) + strings.Repeat("x", 1<<20)},
				rawPart{"f0", "", "Late title"})
			assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
			body := rec.Body.String()
			assert.Contains(t, body, "Could not save this file.")
			assert.Contains(t, body, "Late title")
			for _, cause := range []string{"no bucket", "disk full", "upload could not be completed"} {
				assert.False(t, strings.Contains(body, cause))
			}
			assert.Equal(t, s.stores, 0)
			assert.Equal(t, s.opened(), c.open)
			if c.open > 0 {
				settledOnce(t, s, false)
			}
		})
	}
}

// A file that could not be made permanent is not a success, though the action has run: the visitor is told, and the other files are
// still submitted.
func TestAFileThatCannotBeSubmittedIsToldAndTheOthersAreStillSubmitted(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var writers []*recWriter
	file := func(label string, fail error) webui.File[attachment, *recWriter] {
		return webui.File[attachment, *recWriter]{
			Label: label, Load: func(attachment) *webui.FileInfo { return nil },
			Open: func(context.Context, attachment, webui.Incoming) (io.WriteCloser, *recWriter, error) {
				w := &recWriter{fail: failures{submit: fail}}
				mu.Lock()
				writers = append(writers, w)
				mu.Unlock()
				return w, w, nil
			},
			Commit: func(_ context.Context, _ attachment, w *recWriter) error { return w.Submit() },
			Store:  func(*attachment, *webui.Upload) {},
		}
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/f", Body: webui.Form[attachment]{
		Load:   func(context.Context) (attachment, error) { return attachment{Title: "Report"}, nil },
		Fields: []webui.Accessor[attachment]{file("One", errors.New("rename failed")), file("Two", nil)},
		Submit: webui.Action[attachment]{Run: func(context.Context, attachment) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	}}}}.MustCompile("/admin")

	rec := multipartPost(h, "/admin/f", map[string]string{"f0": ""}, part{"f0", "a.bin", "one"}, part{"f1", "b.bin", "two"})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eSaved, but a file could not be stored.")
	assert.Equal(t, len(writers), 2)
	for _, w := range writers {
		assert.Equal(t, [3]int{w.closes, w.submits, w.discards}, [3]int{1, 1, 0})
	}
}

// A body that ends before the file does leaves nothing behind: the writer is closed and discarded.
func TestABodyThatEndsEarlyDiscardsTheFileThatWasBeingWritten(t *testing.T) {
	t.Parallel()

	s := newFileShop(plainFile(webui.FilePNG))
	rec := orderedPost(s.h, "/admin/f", func(r io.Reader) io.Reader { return io.LimitReader(r, 700) },
		rawPart{"_leaf", "", "p"},
		rawPart{"f1", "a.png", string(pngBytes) + strings.Repeat("x", 4000)})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "eThat submission could not be read.")
	assert.Equal(t, s.stores, 0)
	settledOnce(t, s, false)
}

// Commit is given the token that Open returned for the file, and the model as the action saw it, with what Store noted in it.
func TestCommitIsGivenTheTokenAndTheModelAsTheActionSawIt(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var got attachment
	var tokenIsWriter bool
	s := newFileShop(func(s *fileShop) webui.File[attachment, *recWriter] {
		f := plainFile(webui.FilePNG)(s)
		f.Commit = func(_ context.Context, a attachment, w *recWriter) error {
			mu.Lock()
			defer mu.Unlock()
			got, tokenIsWriter = a, w == s.writers[0]
			return w.Submit()
		}
		return f
	}, withSubmit(func(a *webui.Action[attachment]) {
		a.Run = func(_ context.Context, m attachment) (webui.Outcome, error) {
			assert.Equal(t, m.Got.Name, "a.png") // Store has run before Run
			return webui.Success("Saved"), nil
		}
	}))
	rec := multipartPost(s.h, "/admin/f", map[string]string{"f0": "Late title"}, part{"f1", "a.png", string(pngBytes)})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, got.Title, "Late title")
	assert.Equal(t, got.Got.Name, "a.png")
	assert.True(t, tokenIsWriter)
}

// What is committed or discarded is reached even when the request has been cancelled, which is often why a file is discarded.
func TestCommitAndDiscardAreGivenAContextThatTheRequestsCancellationDoesNotReach(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name   string
		values map[string]string
		commit bool
	}{
		{"accepted", nil, true},
		{"refused", map[string]string{"f0": "x"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var mu sync.Mutex
			var settled []error
			s := newFileShop(func(s *fileShop) webui.File[attachment, *recWriter] {
				f := plainFile(webui.FilePNG)(s)
				f.Commit = func(ctx context.Context, _ attachment, w *recWriter) error {
					mu.Lock()
					defer mu.Unlock()
					settled = append(settled, ctx.Err())
					return w.Submit()
				}
				f.Discard = func(ctx context.Context, w *recWriter) error {
					mu.Lock()
					defer mu.Unlock()
					settled = append(settled, ctx.Err())
					return w.Discard()
				}
				return f
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.onWrite = func(int) { cancel() } // the visitor goes away as the file arrives

			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			_ = mw.WriteField("_leaf", "p")
			for k, v := range map[string]string{"f0": "Report"} {
				if c.values != nil {
					v = c.values[k]
				}
				_ = mw.WriteField(k, v)
			}
			hdr := textproto.MIMEHeader{}
			hdr.Set("Content-Disposition", `form-data; name="f1"; filename="a.png"`)
			part, _ := mw.CreatePart(hdr)
			_, _ = part.Write(pngBytes)
			_ = mw.Close()
			req := httptest.NewRequest(http.MethodPost, "/admin/f", &body).WithContext(ctx)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			s.h.ServeHTTP(httptest.NewRecorder(), req)

			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, len(settled), 1)
			assert.NoError(t, settled[0]) // not cancelled, though the request was
			submits, discards := s.settled()
			assert.Equal(t, submits == 1, c.commit)
			assert.Equal(t, discards == 1, !c.commit)
		})
	}
}

// An app with nothing to do when a file is permanent or is dropped leaves both out, and the file is written all the same.
func TestACommitAndADiscardAreOptional(t *testing.T) {
	t.Parallel()

	s := newFileShop(func(s *fileShop) webui.File[attachment, *recWriter] {
		f := plainFile(webui.FilePNG)(s)
		f.Commit, f.Discard = nil, nil
		return f
	})
	assert.Equal(t, multipartPost(s.h, "/admin/f", nil, part{"f1", "a.png", string(pngBytes)}).Code, http.StatusSeeOther)
	assert.Equal(t, s.writers[0].closes, 1)
	assert.Equal(t, s.writers[0].buf.Len(), len(pngBytes))
	assert.Equal(t, multipartPost(s.h, "/admin/f", map[string]string{"f0": "x"}, part{"f1", "a.png", string(pngBytes)}).Code, http.StatusUnprocessableEntity)
}
