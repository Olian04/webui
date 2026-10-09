package webui_test

import (
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// get serves one request straight through the handler.
func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestCompileFailureServesEveryPathAndMethod(t *testing.T) {
	t.Parallel()

	type bad struct{ When []string }
	app := webui.App{Pages: webui.Pages{
		webui.Page[bad]{Path: "/event/{id}", Body: webui.Stack{}},
	}}
	h, err := app.Compile("/admin")
	assert.Error(t, err)
	assert.NotNil(t, h) // never nil

	for _, tt := range []struct{ method, target string }{
		{http.MethodGet, "/admin/"},
		{http.MethodGet, "/admin/event/1"},
		{http.MethodPost, "/admin/anything/else"},
	} {
		rec := serve(h, tt.method, tt.target)
		assert.Equal(t, rec.Code, http.StatusInternalServerError)
		body := rec.Body.String()
		assert.Contains(t, body, "Failed to compile")
		assert.Contains(t, body, `page &#34;/event/{id}&#34; (bad)`)
		assert.Contains(t, body, "field When has unsupported type []string")
		assert.Contains(t, body, "the path declares {id} but bad has no field for it")
		assert.Contains(t, body, "<style>") // self-contained: no asset route to depend on
		assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
	}
}

func TestEmptyAppServesShellAtBothMountForms(t *testing.T) {
	t.Parallel()

	h := webui.App{Brand: webui.Brand{Name: "Acme"}}.MustCompile("/admin")
	for _, target := range []string{"/admin", "/admin/"} {
		rec := serve(h, http.MethodGet, target)
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Contains(t, rec.Body.String(), "Acme")
		assert.Contains(t, rec.Body.String(), "No pages declared")
	}
}

func TestUnknownPathIs404InsideTheShell(t *testing.T) {
	t.Parallel()

	rec := serve(webui.App{Brand: webui.Brand{Name: "Acme"}}.MustCompile("/admin"), http.MethodGet, "/admin/nope")
	assert.Equal(t, rec.Code, http.StatusNotFound)
	assert.Contains(t, rec.Body.String(), "Page not found")
	assert.Contains(t, rec.Body.String(), "Acme")
}

func TestResponsesCarrySecurityHeaders(t *testing.T) {
	t.Parallel()

	rec := serve(webui.App{}.MustCompile(""), http.MethodGet, "/")
	h := rec.Header()
	assert.Equal(t, h.Get("X-Content-Type-Options"), "nosniff")
	assert.Equal(t, h.Get("Referrer-Policy"), "same-origin")
	assert.Contains(t, h.Get("Content-Security-Policy"), "default-src 'self'")
	assert.Contains(t, h.Get("Content-Security-Policy"), "frame-ancestors 'none'")
	assert.Equal(t, h.Get("Cache-Control"), "no-store")
}

func TestAssetsAreServedWithValidators(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, assetURL(t, h, "app.css"))
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/css")
	assert.Contains(t, rec.Body.String(), "--canvas")
	etag := rec.Header().Get("ETag")
	assert.True(t, etag != "")

	cond := httptest.NewRequest(http.MethodGet, assetURL(t, h, "app.css"), nil)
	cond.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, cond)
	assert.Equal(t, rec2.Code, http.StatusNotModified)

	assert.Equal(t, serve(h, http.MethodGet, "/admin/_webui/missing.css").Code, http.StatusNotFound)
	assert.Equal(t, serve(h, http.MethodGet, assetURL(t, h, "prefs.js")).Code, http.StatusOK)
}

func TestThemeAndLogoAreServedAndLinked(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	h := webui.App{
		Brand: webui.Brand{Name: "Acme", Logo: img},
		Theme: webui.Theme{Accent: "#ff6600"},
	}.MustCompile("/admin")

	page := serve(h, http.MethodGet, "/admin/").Body.String()
	// The app does not choose light or dark: the document carries no data-theme
	// attribute, and the stylesheet follows the viewer's system until they pick one.
	assert.Contains(t, page, `<html lang="en">`)
	assetURL(t, h, "theme.css") // linked, with its hash
	assetURL(t, h, "logo")

	css := serve(h, http.MethodGet, assetURL(t, h, "theme.css"))
	assert.Contains(t, css.Body.String(), "--blue: #ff6600;")

	base := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, base, "@media (prefers-color-scheme: light)")
	assert.Contains(t, base, ":root:not([data-theme])")

	logo := serve(h, http.MethodGet, assetURL(t, h, "logo"))
	assert.Equal(t, logo.Header().Get("Content-Type"), "image/png")
	assert.True(t, strings.HasPrefix(logo.Body.String(), "\x89PNG"))

	// No tokens, no logo: nothing to link, nothing to serve.
	plain := webui.App{}.MustCompile("")
	plainPage := serve(plain, http.MethodGet, "/").Body.String()
	assert.False(t, strings.Contains(plainPage, "theme."))
	assert.False(t, strings.Contains(plainPage, "/_webui/logo"))
}
