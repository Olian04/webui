package webui_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"

	"github.com/Olian04/webui/pkg/lang"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// monacoLinks is every file of the editor that a page links: its script, its stylesheet, its font and the modules it asks for as the head is read.
var monacoLinks = regexp.MustCompile(`(?:href|src)="(/admin/_webui/monaco/[^"]+)"`)

// accepting is a GET that says which codings the client takes.
func accepting(h http.Handler, target, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept-Encoding", accept)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func unbrotli(t *testing.T, b []byte) string {
	t.Helper()

	out, err := io.ReadAll(brotli.NewReader(bytes.NewReader(b)))
	assert.NoError(t, err)
	return string(out)
}

// A client that takes Brotli is sent it, and one that takes only gzip is sent gzip: Chrome says "gzip, deflate, br" over HTTPS and "gzip, deflate" over plain HTTP.
func TestAnAssetIsSentInBrotliToAClientThatTakesItAndInGzipToOneThatDoesNot(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	target := assetURL(t, h, "app.css")
	plain := serve(h, http.MethodGet, target)
	assert.Equal(t, plain.Header().Get("Content-Encoding"), "")

	for accept, want := range map[string]string{
		"gzip, deflate, br": "br", "br": "br", "gzip, br": "br", "gzip, deflate": "gzip", "gzip": "gzip",
		"br;q=0, gzip": "gzip", "gzip;q=0.9, br;q=0.1": "gzip", "identity": "", "deflate": "",
	} {
		rec := accepting(h, target, accept)
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Equal(t, rec.Header().Get("Content-Encoding"), want)
		assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
		switch want {
		case "br":
			assert.Equal(t, unbrotli(t, rec.Body.Bytes()), plain.Body.String())
			assert.True(t, rec.Body.Len() < plain.Body.Len()/3)
		case "gzip":
			assert.Equal(t, unzip(t, rec.Body.Bytes()), plain.Body.String())
		default:
			assert.Equal(t, rec.Body.String(), plain.Body.String())
		}
	}

	// Each coding is a file of its own to a cache, and to a client that asks again.
	br, gz := accepting(h, target, "br"), accepting(h, target, "gzip")
	assert.True(t, br.Header().Get("ETag") != gz.Header().Get("ETag"))
	assert.True(t, strings.HasSuffix(br.Header().Get("ETag"), `-br"`))
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept-Encoding", "br")
	req.Header.Set("If-None-Match", br.Header().Get("ETag"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusNotModified)
}

// The Brotli copy of the app's own files is the one the build wrote, at the best level: it is read, not made, when the app starts or when the file is asked for.
func TestTheBrotliCopyOfAFileTheBuildCompressedIsThatCopy(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	for name, source := range map[string]string{"app.css": "css/app.css", "enhance.js": "js/enhance.js"} {
		built, err := os.ReadFile(filepath.Join("..", "internal", "render", "assets", "min", source+".br")) //nolint:gosec // G304: a fixed file of this repository
		assert.NoError(t, err)
		assert.True(t, bytes.Equal(accepting(h, assetURL(t, h, name), "br").Body.Bytes(), built))
	}
}

// What an app makes when it starts has no copy from the build: it is compressed the first time it is asked for.
func TestAnAssetTheAppMakesIsCompressedInBrotliWhenFirstAsked(t *testing.T) {
	t.Parallel()

	h := navApp()
	target := assetURL(t, h, "fontawesome.css")
	rec := accepting(h, target, "br")
	assert.Equal(t, rec.Header().Get("Content-Encoding"), "br")
	assert.Equal(t, unbrotli(t, rec.Body.Bytes()), serve(h, http.MethodGet, target).Body.String())
}

func TestAPageAndASearchAreCompressedInBrotliAsTheyAreBuilt(t *testing.T) {
	t.Parallel()

	h := navApp()
	page := serve(h, http.MethodGet, "/admin/device")
	rec := accepting(h, "/admin/device", "gzip, deflate, br")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Equal(t, rec.Header().Get("Content-Encoding"), "br")
	assert.Contains(t, strings.Join(rec.Header().Values("Vary"), ","), "Accept-Encoding") // a page varies on the leaf header as well
	assert.Equal(t, builtless(unbrotli(t, rec.Body.Bytes())), builtless(page.Body.String()))

	missing := accepting(h, "/admin/nowhere", "br")
	assert.Equal(t, missing.Code, http.StatusNotFound)
	assert.Contains(t, unbrotli(t, missing.Body.Bytes()), "Page not found")

	// A client that takes only gzip is sent gzip, and one that takes neither is sent the page as it is.
	assert.Equal(t, accepting(h, "/admin/device", "gzip").Header().Get("Content-Encoding"), "gzip")
	assert.Equal(t, accepting(h, "/admin/device", "identity").Header().Get("Content-Encoding"), "")

	// An answer that is not a document is left alone: a redirect has no body to compress.
	root := accepting(webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, Body: webui.Stack{}}}}.MustCompile("/admin"), "/admin/", "br")
	assert.Equal(t, root.Header().Get("Content-Encoding"), "")
}

func TestAnExportIsCompressedInBrotliWhenTheBrowserAccepts(t *testing.T) {
	t.Parallel()

	rec := accepting(stockApp(rowsTable(stocks)), "/admin/s?items.export=csv", "br")
	assert.Equal(t, rec.Header().Get("Content-Encoding"), "br")
	assert.Contains(t, unbrotli(t, rec.Body.Bytes()), "bolt")
}

// The bytes sent so far are a Brotli stream that can be read up to there.
func TestAStreamedPageIsCompressedInBrotliSoThatItCanBeReadAsItArrives(t *testing.T) {
	t.Parallel()

	slow := newGated()
	h := streamApp(webui.Stack{quickTable("First"), slow.table("Slow")})
	rec := newStreamRecorder()
	done := make(chan struct{})
	go func() {
		get(h, rec, "/admin/p", func(r *http.Request) { r.Header.Set("Accept-Encoding", "gzip, br") })
		close(done)
	}()
	<-slow.asked
	waitFor(t, "the first flush", func() bool { return rec.flushCount() > 0 })
	assert.Equal(t, rec.header.Get("Content-Encoding"), "br")

	partial, _ := io.ReadAll(brotli.NewReader(bytes.NewReader(rec.sentSoFar()))) // not finished: it reads as far as there is, and then says so
	assert.Contains(t, string(partial), "First-row")
	assert.False(t, strings.Contains(string(partial), "Slow-row"))

	close(slow.release)
	<-done
	full, err := io.ReadAll(brotli.NewReader(bytes.NewReader([]byte(rec.all()))))
	assert.NoError(t, err)
	assert.Contains(t, string(full), "Slow-row")
	assert.True(t, strings.HasSuffix(string(full), "</html>"))
}

// The editor is megabytes, and is where Brotli is worth most: its files are the build's copies, and the copies have no address of their own.
func TestTheEditorsFilesAreSentInBrotliAndTheCopiesAreNotFilesOfTheirOwn(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Editor[snippet]{Title: "Config", Load: loadSnippet, Content: snippetView, Language: lang.JSON})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	var files []string
	for _, m := range monacoLinks.FindAllStringSubmatch(body, -1) {
		files = append(files, m[1])
	}
	assert.True(t, len(files) >= 6) // the entry, the stylesheet, the font and the modules

	for _, file := range files {
		plain := serve(h, http.MethodGet, file)
		assert.Equal(t, plain.Code, http.StatusOK)
		rec := accepting(h, file, "gzip, br")
		assert.Equal(t, rec.Header().Get("Content-Encoding"), "br")
		assert.Equal(t, unbrotli(t, rec.Body.Bytes()), plain.Body.String())
		assert.True(t, rec.Body.Len() < plain.Body.Len())
		assert.Equal(t, serve(h, http.MethodGet, file+".br").Code, http.StatusNotFound)
	}
}

// What is committed is what the build wrote: every .br is its file compressed, and every file that shrinks has one, so that a file that was changed and not compressed again
// is a failure and not a client that is sent the old one.
func TestEveryBrotliFileIsItsFilesCompressedCopyAndEveryFileThatShrinksHasOne(t *testing.T) {
	t.Parallel()

	compressible := map[string]bool{".js": true, ".css": true, ".json": true, ".svg": true, ".ttf": true}
	checked := 0
	for _, dir := range []string{
		filepath.Join("..", "internal", "render", "assets", "min"),
		filepath.Join("..", "pkg", "monaco", "editor"),
		filepath.Join("..", "pkg", "monaco", "css", "worker"), filepath.Join("..", "pkg", "monaco", "html", "worker"),
		filepath.Join("..", "pkg", "monaco", "json", "worker"), filepath.Join("..", "pkg", "monaco", "typescript", "worker"),
	} {
		assert.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Base(path) == "manifest.json" {
				return err
			}
			if source, ok := strings.CutSuffix(path, ".br"); ok {
				raw, err := os.ReadFile(source) //nolint:gosec // G304: the files of this repository
				if err != nil {
					t.Errorf("%s has no file beside it: run make assets, or make monaco", path)
					return nil
				}
				packed, err := os.ReadFile(path) //nolint:gosec // G304: as above
				assert.NoError(t, err)
				if unbrotli(t, packed) != string(raw) {
					t.Errorf("%s is not %s compressed: run make assets, or make monaco", path, source)
				}
				checked++
				return nil
			}
			if !compressible[filepath.Ext(path)] || strings.HasPrefix(filepath.Base(path), "LICENSE") || strings.HasPrefix(filepath.Base(path), "THIRD") {
				return nil
			}
			raw, err := os.ReadFile(path) //nolint:gosec // G304: as above
			assert.NoError(t, err)
			if len(raw) < 64 {
				return nil // too small for the headers of a stream to be worth it
			}
			if _, err := os.Stat(path + ".br"); err != nil {
				var buf bytes.Buffer
				zw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
				_, _ = zw.Write(raw)
				_ = zw.Close()
				if buf.Len() < len(raw) {
					t.Errorf("%s has no Brotli copy, and would shrink: run make assets, or make monaco", path)
				}
			}
			return nil
		}))
	}
	assert.True(t, checked > 90)
}
