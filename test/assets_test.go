package webui_test

import (
	"bytes"
	"compress/gzip"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// assetURL is the address a page of the app links an asset at, such as "app.css":
// the name has a short hash of the file's contents in it. It is found the way a
// browser finds it, in the markup.
func assetURL(t *testing.T, h http.Handler, logical string) string {
	t.Helper()

	if logical == "fa-solid-900.woff2" {
		// The font is named by the icon stylesheet, not by the page: it is found there,
		// beside it, as the browser would.
		sheet := assetURL(t, h, "fontawesome.css")
		css := serve(h, http.MethodGet, sheet).Body.String()
		m := regexp.MustCompile(`url\((fa-solid-900\.[0-9a-f]{8}\.woff2)\)`).FindStringSubmatch(css)
		if m == nil {
			t.Fatal("the icon stylesheet names no font")
		}
		return sheet[:strings.LastIndex(sheet, "/")+1] + m[1]
	}
	dot := strings.LastIndex(logical, ".")
	base, ext := logical, ""
	if dot > 0 {
		base, ext = logical[:dot], logical[dot:]
	}
	pattern := regexp.MustCompile(`(/[^"' ]*_webui/` + regexp.QuoteMeta(base) + `\.[0-9a-f]{8}` + regexp.QuoteMeta(ext) + `)["' >]`)

	rec := serve(h, http.MethodGet, "/admin/")
	if rec.Code == http.StatusFound { // the root goes to the first page
		rec = serve(h, http.MethodGet, rec.Header().Get("Location"))
	}
	m := pattern.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatalf("no link to %s in the page", logical)
	}
	return m[1]
}

func gzipped(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func unzip(t *testing.T, b []byte) string {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(b))
	assert.NoError(t, err)
	out, err := io.ReadAll(zr)
	assert.NoError(t, err)
	return string(out)
}

func TestAnAssetsAddressCarriesAShortHashOfItsContents(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	css := assetURL(t, h, "app.css")
	assert.True(t, regexp.MustCompile(`^/admin/_webui/app\.[0-9a-f]{8}\.css$`).MatchString(css))

	// The same contents are the same address, and a different file has a different one.
	assert.Equal(t, assetURL(t, webui.App{}.MustCompile("/admin"), "app.css"), css)
	assert.True(t, assetURL(t, h, "enhance.js") != assetURL(t, h, "prefs.js"))

	// An address with no hash, or the wrong hash, is not an asset.
	assert.Equal(t, serve(h, http.MethodGet, "/admin/_webui/app.css").Code, http.StatusNotFound)
	assert.Equal(t, serve(h, http.MethodGet, "/admin/_webui/app.00000000.css").Code, http.StatusNotFound)
}

func TestAnAssetIsCachedForGoodBecauseItsAddressChangesWithIt(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	for _, logical := range []string{"app.css", "enhance.js", "fa-solid-900.woff2", "favicon.svg"} {
		rec := serve(h, http.MethodGet, assetURL(t, h, logical))
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Equal(t, rec.Header().Get("Cache-Control"), "public, max-age=31536000, immutable")
	}
}

func TestTheIconStylesheetNamesTheFontByItsHashedAddress(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	font := assetURL(t, h, "fa-solid-900.woff2")
	css := serve(h, http.MethodGet, assetURL(t, h, "fontawesome.css")).Body.String()
	assert.Contains(t, css, "url("+font[strings.LastIndex(font, "/")+1:]+")") // beside it, by name
	assert.False(t, strings.Contains(css, "url(fa-solid-900.woff2)"))
}

func TestTextAssetsAreCompressedForAClientThatAcceptsIt(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	target := assetURL(t, h, "app.css")
	plain := serve(h, http.MethodGet, target)
	assert.Equal(t, plain.Header().Get("Content-Encoding"), "")
	assert.Equal(t, plain.Header().Get("Vary"), "Accept-Encoding")

	rec := gzipped(t, h, target)
	assert.Equal(t, rec.Header().Get("Content-Encoding"), "gzip")
	assert.Equal(t, rec.Header().Get("Vary"), "Accept-Encoding")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/css")
	assert.True(t, rec.Body.Len() < plain.Body.Len()/2) // it shrinks
	assert.Equal(t, unzip(t, rec.Body.Bytes()), plain.Body.String())

	// Each form has its own validator, and revalidates to a 304.
	assert.True(t, rec.Header().Get("ETag") != plain.Header().Get("ETag"))
	cond := httptest.NewRequest(http.MethodGet, target, nil)
	cond.Header.Set("Accept-Encoding", "gzip")
	cond.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	again := httptest.NewRecorder()
	h.ServeHTTP(again, cond)
	assert.Equal(t, again.Code, http.StatusNotModified)

	// A quality of zero refuses it.
	refuse := httptest.NewRequest(http.MethodGet, target, nil)
	refuse.Header.Set("Accept-Encoding", "gzip;q=0")
	none := httptest.NewRecorder()
	h.ServeHTTP(none, refuse)
	assert.Equal(t, none.Header().Get("Content-Encoding"), "")
}

func TestImagesAndFontsAreNotCompressedAgain(t *testing.T) {
	t.Parallel()

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	img.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	h := webui.App{Brand: webui.Brand{Logo: img}}.MustCompile("/admin")
	for _, logical := range []string{"fa-solid-900.woff2", "logo", "favicon-32.png"} {
		rec := gzipped(t, h, assetURL(t, h, logical))
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Equal(t, rec.Header().Get("Content-Encoding"), "")
	}
}

func TestPagesAndTheSearchAreCompressedToo(t *testing.T) {
	t.Parallel()

	h := searchApp(withRows, nil)

	page := serve(h, http.MethodGet, "/admin/device")
	zipped := gzipped(t, h, "/admin/device")
	assert.Equal(t, zipped.Header().Get("Content-Encoding"), "gzip")
	vary := strings.Join(zipped.Header().Values("Vary"), ",")
	assert.Contains(t, vary, "Accept-Encoding")
	assert.Contains(t, vary, "X-Webui-Leaf") // the page's own is kept
	assert.Equal(t, zipped.Header().Get("Content-Length"), "")
	assert.Equal(t, unzip(t, zipped.Body.Bytes()), page.Body.String())
	assert.True(t, zipped.Body.Len() < page.Body.Len()/2)

	found := gzipped(t, h, "/admin/_webui/search?q=dev")
	assert.Equal(t, found.Header().Get("Content-Encoding"), "gzip")
	assert.Contains(t, unzip(t, found.Body.Bytes()), `"results"`)

	// A client that does not accept it gets the bytes as before.
	assert.Equal(t, page.Header().Get("Content-Encoding"), "")
}

func TestAnErrorPageIsCompressedAndARedirectIsNot(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	missing := gzipped(t, h, "/admin/nowhere")
	assert.Equal(t, missing.Code, http.StatusNotFound)
	assert.Contains(t, unzip(t, missing.Body.Bytes()), "Page not found")

	root := gzipped(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, Body: webui.Stack{}}}}.MustCompile("/admin"), "/admin/")
	assert.Equal(t, root.Code, http.StatusFound)
	assert.Equal(t, root.Header().Get("Content-Encoding"), "")
}
