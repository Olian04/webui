package webui_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// streamRecorder is a response that notes when it is flushed, as the client of a page that is sent as it is built is.
type streamRecorder struct {
	mu      sync.Mutex
	header  http.Header
	code    int
	buf     bytes.Buffer
	writes  int
	flushes []int // how much was written at each flush
}

func newStreamRecorder() *streamRecorder { return &streamRecorder{header: http.Header{}} }

func (s *streamRecorder) Header() http.Header { return s.header }
func (s *streamRecorder) WriteHeader(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.code == 0 {
		s.code = code
	}
}
func (s *streamRecorder) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.code == 0 {
		s.code = http.StatusOK
	}
	s.writes++
	return s.buf.Write(b)
}
func (s *streamRecorder) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushes = append(s.flushes, s.buf.Len())
}

// sentSoFar is what had been flushed, which is what a client has.
func (s *streamRecorder) sentSoFar() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.flushes) == 0 {
		return nil
	}
	return append([]byte(nil), s.buf.Bytes()[:s.flushes[len(s.flushes)-1]]...)
}

func (s *streamRecorder) flushCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.flushes)
}

func (s *streamRecorder) all() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// waitFor polls until cond holds, or fails the test.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// gate is a panel's table whose backend answers when its channel is closed, and says so when it is asked.
type gated struct {
	asked   chan struct{}
	release chan struct{}
}

func newGated() *gated { return &gated{asked: make(chan struct{}), release: make(chan struct{})} }

func (g *gated) table(title string) webui.Table[Device] {
	return webui.Table[Device]{
		Title: title,
		Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], error) {
			close(g.asked)
			select {
			case <-g.release:
			case <-ctx.Done():
				return webui.Window[Device]{}, ctx.Err()
			}
			return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, nil
		},
		Columns: []webui.Accessor[Device]{formID},
	}
}

func quickTable(title string) webui.Table[Device] {
	return webui.Table[Device]{
		Title: title,
		Load: func(context.Context, webui.Query) (webui.Window[Device], error) {
			return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, nil
		},
		Columns: []webui.Accessor[Device]{formID},
	}
}

func streamApp(body webui.PageBody) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", Body: body}}}.MustCompile("/admin")
}

func get(h http.Handler, rec http.ResponseWriter, path string, mod ...func(*http.Request)) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, m := range mod {
		m(req)
	}
	h.ServeHTTP(rec, req)
}

// A page with a panel that is slow is sent as far as it can be while the panel is waited for, and the panels after
// it follow it, in the order the page has them, whichever answered first.
func TestAPageWithASlowPanelIsSentWhileItWaitsAndThePanelsFollowInOrder(t *testing.T) {
	t.Parallel()

	slow, later := newGated(), newGated()
	h := streamApp(webui.Stack{quickTable("First"), slow.table("Slow"), later.table("Later")})
	rec := newStreamRecorder()
	done := make(chan struct{})
	go func() { get(h, rec, "/admin/p"); close(done) }()

	// The shell and the first panel are with the client while the slow one has not answered.
	<-slow.asked
	waitFor(t, "the first flush", func() bool { return rec.flushCount() > 0 })
	sent := string(rec.sentSoFar())
	assert.Contains(t, sent, `<aside class="sidebar">`)
	assert.Contains(t, sent, "First-row")
	assert.False(t, strings.Contains(sent, "Slow-row"))
	assert.False(t, strings.Contains(rec.all(), "</html>")) // and not the end of the page

	// A panel after the slow one that is ready is held back behind it: in order.
	close(later.release)
	time.Sleep(30 * time.Millisecond)
	assert.False(t, strings.Contains(rec.all(), "Later-row"))

	close(slow.release)
	<-done
	body := rec.all()
	assert.Equal(t, rec.code, http.StatusOK)
	assert.True(t, strings.Index(body, "First-row") < strings.Index(body, "Slow-row"))
	assert.True(t, strings.Index(body, "Slow-row") < strings.Index(body, "Later-row"))
	assert.True(t, strings.HasSuffix(body, "</html>"))
	assert.Equal(t, rec.header.Get("X-Accel-Buffering"), "no")
	assert.Equal(t, rec.header.Get("Content-Length"), "")
}

// What is sent as it is built is the page that is sent whole: the same bytes, but for the time the bar says, and what
// says that the rest is still arriving (see TestAStreamedPageSaysThatItIsStillArrivingUntilItIsWhole).
func TestAPageSentAsItIsBuiltIsTheSamePageAsOneSentWhole(t *testing.T) {
	t.Parallel()

	var delay time.Duration
	var mu sync.Mutex
	set := func(d time.Duration) { mu.Lock(); delay = d; mu.Unlock() }
	table := func(title string) webui.Table[Device] {
		return webui.Table[Device]{
			Title: title, Columns: []webui.Accessor[Device]{formID},
			Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], error) {
				mu.Lock()
				d := delay
				mu.Unlock()
				sleep(ctx, d)
				return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, nil
			},
		}
	}
	h := streamApp(webui.Stack{webui.Split{table("A"), table("B")}, table("C")})

	set(0)
	whole := newStreamRecorder()
	get(h, whole, "/admin/p")
	assert.Equal(t, whole.flushCount(), 0) // answered at once: one piece, nothing flushed
	assert.Equal(t, whole.writes, 1)

	set(120 * time.Millisecond)
	streamed := newStreamRecorder()
	get(h, streamed, "/admin/p")
	assert.True(t, streamed.flushCount() > 0)

	mask := func(s string) string {
		for _, re := range []string{`datetime="[^"]*"`, `title="The HTML was built [^"]*"`, `>built [^<]*<`, `data-build title="[^"]*">[^<]*<`} {
			s = regexp.MustCompile(re).ReplaceAllString(s, "~")
		}
		return s
	}
	// A page that is sent as it is built has marks that it is still arriving, and its bar in two parts: the request
	// with the shell, and how the page came to be at the end. Put back together it is the bar of the page that is whole.
	unmarked := func(s string) string {
		for _, re := range []string{
			`<div class="statusbar statusbar-live">.*?</span></span></div>`,
			`<div class="arriving"[^>]*>.*?</div>`,
			`<i class="arrived" hidden></i>`,
		} {
			s = regexp.MustCompile(re).ReplaceAllString(s, "")
		}
		s = strings.Replace(s, `<main class="scroll scroll-arriving">`, `<main class="scroll">`, 1)
		return strings.Replace(s, `<div class="statusbar-late">`, `<div class="statusbar"><span class="method" data-method>GET</span> <span class="url" data-url>/admin/p</span>`, 1)
	}
	assert.Equal(t, mask(unmarked(streamed.all())), mask(whole.all()))
}

// The bytes sent so far are a compressed stream that can be read up to there.
func TestAStreamedPageIsCompressedSoThatItCanBeReadAsItArrives(t *testing.T) {
	t.Parallel()

	slow := newGated()
	h := streamApp(webui.Stack{quickTable("First"), slow.table("Slow")})
	rec := newStreamRecorder()
	done := make(chan struct{})
	go func() {
		get(h, rec, "/admin/p", func(r *http.Request) { r.Header.Set("Accept-Encoding", "gzip") })
		close(done)
	}()
	<-slow.asked
	waitFor(t, "the first flush", func() bool { return rec.flushCount() > 0 })
	assert.Equal(t, rec.header.Get("Content-Encoding"), "gzip")

	zr, err := gzip.NewReader(bytes.NewReader(rec.sentSoFar()))
	assert.NoError(t, err)
	partial, _ := io.ReadAll(zr) // the stream is not finished: it reads as far as there is, and then says so
	assert.Contains(t, string(partial), "First-row")
	assert.False(t, strings.Contains(string(partial), "Slow-row"))

	close(slow.release)
	<-done
	zr, err = gzip.NewReader(bytes.NewReader([]byte(rec.all())))
	assert.NoError(t, err)
	full, err := io.ReadAll(zr)
	assert.NoError(t, err)
	assert.Contains(t, string(full), "Slow-row")
	assert.True(t, strings.HasSuffix(string(full), "</html>"))
}

// The header says how long the first of the page took; the bar, drawn last, how long all of it did.
func TestTheStatusBarOfAStreamedPageSaysHowLongAllOfItTook(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{quickTable("First"), delayedTable("Slow", 250*time.Millisecond)})
	rec := newStreamRecorder()
	get(h, rec, "/admin/p")

	header := regexp.MustCompile(`build;dur=([0-9.]+)`).FindStringSubmatch(rec.header.Get("Server-Timing"))
	bar := regexp.MustCompile(`data-build title="[^"]*">([0-9.]+) ms<`).FindStringSubmatch(rec.all())
	assert.NotNil(t, header)
	assert.NotNil(t, bar)
	assert.True(t, atof(header[1]) < 150)
	assert.True(t, atof(bar[1]) >= 240)
}

// A panel that panics once the page has begun to be sent is a panel that says it could not be built: the page is
// still the page, and complete, and the cause is logged and not shown.
func TestAPanelThatPanicsAfterThePageBeganToBeSentSaysSoAndThePageStands(t *testing.T) {
	t.Parallel()

	slow := newGated()
	broken := webui.Form[Device]{
		Title: "Broken", Fields: []webui.Accessor[Device]{formID},
		Load: func(context.Context) (Device, error) { panic("the backend fell over") },
	}
	h := streamApp(webui.Stack{slow.table("Slow"), broken, quickTable("After")})
	rec := newStreamRecorder()
	done := make(chan struct{})
	go func() { get(h, rec, "/admin/p"); close(done) }()
	<-slow.asked
	waitFor(t, "the first flush", func() bool { return rec.flushCount() > 0 })
	close(slow.release)
	<-done

	body := rec.all()
	assert.Equal(t, rec.code, http.StatusOK)
	assert.Contains(t, body, "Could not load")
	assert.Contains(t, body, "After-row") // the panel after it is drawn
	assert.False(t, strings.Contains(body, "fell over"))
	assert.True(t, strings.HasSuffix(body, "</html>"))
}

// A page whose head depends on its panels, such as one with an editor, is built whole, as is a HEAD request.
func TestAPageWithAnEditorAndAHeadRequestAreNotSentAsTheyAreBuilt(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{
		delayedTable("Slow", 120*time.Millisecond),
		webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView, Language: webui.LangJSON},
	})
	rec := newStreamRecorder()
	get(h, rec, "/admin/p")
	assert.Equal(t, rec.flushCount(), 0)
	assert.Equal(t, rec.writes, 1)
	assert.Contains(t, rec.all(), `<script type="module" src="/admin/_webui/monaco/code-`)

	plain := streamApp(webui.Stack{delayedTable("Slow", 120*time.Millisecond), quickTable("Fast")})
	head := newStreamRecorder()
	plain.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/admin/p", nil))
	assert.Equal(t, head.flushCount(), 0)
}

// What the Guard refuses is refused, whatever the panels are: a 403 and nothing of the page.
func TestAGuardsRefusalIsStillAnErrorStatusForASlowPage(t *testing.T) {
	t.Parallel()

	page := webui.Page[webui.NoArgs]{
		Path:  "/p",
		Guard: func(context.Context, webui.NoArgs) error { return context.Canceled },
		Body:  webui.Stack{delayedTable("Slow", 120*time.Millisecond), quickTable("Fast")},
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	rec := newStreamRecorder()
	get(h, rec, "/admin/p")
	assert.Equal(t, rec.code, http.StatusForbidden)
	assert.False(t, strings.Contains(rec.all(), "Slow-row"))
}

// A visitor who goes ends the work: the panels that are waited for are told, and the response stops.
func TestAVisitorWhoLeavesAPageThatIsBeingSentStopsItsPanels(t *testing.T) {
	t.Parallel()

	slow := newGated()
	h := streamApp(webui.Stack{quickTable("First"), slow.table("Slow")})
	ctx, cancel := context.WithCancel(context.Background())
	rec := newStreamRecorder()
	done := make(chan struct{})
	go func() {
		get(h, rec, "/admin/p", func(r *http.Request) { *r = *r.WithContext(ctx) })
		close(done)
	}()
	<-slow.asked
	waitFor(t, "the first flush", func() bool { return rec.flushCount() > 0 })
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the handler went on after the visitor left")
	}
	assert.False(t, strings.Contains(rec.all(), "Slow-row"))
}

func delayedTable(title string, d time.Duration) webui.Table[Device] {
	return webui.Table[Device]{
		Title: title,
		Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], error) {
			sleep(ctx, d)
			return webui.Window[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, nil
		},
		Columns: []webui.Accessor[Device]{formID},
	}
}

func atof(s string) float64 {
	var f float64
	var scale = 1.0
	dot := false
	for _, c := range s {
		switch {
		case c == '.':
			dot = true
		case c >= '0' && c <= '9':
			if dot {
				scale /= 10
				f += float64(c-'0') * scale
			} else {
				f = f*10 + float64(c-'0')
			}
		}
	}
	return f
}

// A page that is sent as it is built says that the rest is coming from the first byte it sends, and what ends it takes
// that back: the spinner and the bar's "loading" are in the shell, the rest of the bar and the mark that ends the content
// are not sent until the last panel has been.
func TestAStreamedPageSaysThatItIsStillArrivingUntilItIsWhole(t *testing.T) {
	t.Parallel()

	slow := newGated()
	h := streamApp(webui.Stack{quickTable("First"), slow.table("Slow")})
	rec := newStreamRecorder()
	done := make(chan struct{})
	go func() { get(h, rec, "/admin/p"); close(done) }()
	<-slow.asked
	waitFor(t, "the first flush", func() bool { return rec.flushCount() > 0 })

	sent := string(rec.sentSoFar())
	assert.Contains(t, sent, `<div class="statusbar statusbar-live">`) // the request is on screen at once,
	assert.Contains(t, sent, `class="status-meta status-loading"`)     // with the mark that the rest is to come
	assert.Contains(t, sent, `<div class="arriving"`)                  // and the spinner is with the content so far
	assert.Contains(t, sent, `class="scroll scroll-arriving"`)
	assert.False(t, strings.Contains(sent, `class="arrived"`)) // which nothing has yet taken back
	assert.False(t, strings.Contains(sent, `statusbar-late`))
	assert.False(t, strings.Contains(sent, "data-build")) // the time is not known until it is all sent

	close(slow.release)
	<-done
	body := rec.all()
	assert.True(t, strings.Index(body, `<div class="arriving"`) < strings.Index(body, "First-row")) // before the content, which the stylesheet draws it after
	assert.True(t, strings.Index(body, "Slow-row") < strings.Index(body, `<i class="arrived" hidden></i>`))
	assert.True(t, strings.Index(body, `<i class="arrived" hidden></i>`) < strings.Index(body, `statusbar-late`))
	assert.Equal(t, strings.Count(body, `class="statusbar`), 2) // live and late, and no third
}

// A page that is sent whole has none of it: it has never been waiting.
func TestAPageSentWholeHasNoMarkThatItIsArriving(t *testing.T) {
	t.Parallel()

	body := serve(streamApp(webui.Stack{quickTable("First")}), http.MethodGet, "/admin/p").Body.String()
	for _, s := range []string{"arriving", "arrived", "statusbar-live", "statusbar-late", "status-loading"} {
		assert.False(t, strings.Contains(body, s))
	}
	assert.Equal(t, strings.Count(body, `class="statusbar"`), 1)
}

// The marks are drawn by the stylesheet alone, so a page with script off has them: the spinner is drawn after the
// content though it is sent before it, and is hidden by the mark that ends the page, and the end of the bar covers
// the "loading" of the live one.
func TestTheStylesheetDrawsTheArrivingMarksWithoutScript(t *testing.T) {
	t.Parallel()

	h := streamApp(webui.Stack{quickTable("First")})
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".arriving:not(:has(~ .arrived)) {\n    display: flex;\n    order: 2;")
	assert.Contains(t, css, ".scroll-arriving {\n  display: flex;\n  flex-direction: column;\n}")
	assert.Contains(t, css, ".statusbar-live:has(~ .statusbar-late) .status-loading {\n    display: none;\n  }")
	assert.Contains(t, css, ".statusbar-late {\n  position: fixed;\n  right: 0;")
}
