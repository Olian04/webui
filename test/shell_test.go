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
	"github.com/Olian04/webui/test/util/page"
)

// get serves one request straight through the handler.
func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestCompileFailureServesEveryPathAndMethod(t *testing.T) {
	t.Parallel()

	type bad struct{ When map[string]int }
	app := webui.App{Pages: webui.Pages{
		webui.Page[bad]{Path: "/event/{id}", View: webui.Stack{}},
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
		answer := page.Parse(t, rec.Body.String())
		text := answer.Root().Text()
		assert.Contains(t, text, "Failed to compile")
		assert.Contains(t, text, `page "/event/{id}" (bad)`)
		assert.Contains(t, text, "field When has unsupported type map[string]int")
		assert.Contains(t, text, "the path declares {id} but bad has no field for it")
		assert.Equal(t, len(answer.ByAttr("rel", "stylesheet")), 0) // self-contained: no asset route to depend on
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
	assert.Equal(t, serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Code, http.StatusOK)
}

func TestThemeAndLogoAreServedAndLinked(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	h := webui.App{
		Brand: webui.Brand{Name: "Acme", Logo: img},
		Theme: webui.Theme{Accent: "#c2410c"},
	}.MustCompile("/admin")

	home := pageOf(t, h, "/admin/")
	// The app does not choose light or dark: the document carries no data-theme
	// attribute, and the stylesheet follows the viewer's system until they pick one.
	assert.Equal(t, page.One(t, "the document", home.ByAttr("lang", "en")).Tag(), "html")
	assert.Equal(t, len(home.ByAttr("data-theme", "")), 0)
	assetURL(t, h, "theme.css") // linked, with its hash
	assetURL(t, h, "logo")

	css := serve(h, http.MethodGet, assetURL(t, h, "theme.css"))
	assert.Contains(t, css.Body.String(), "--blue: #c2410c;")

	logo := serve(h, http.MethodGet, assetURL(t, h, "logo"))
	assert.Equal(t, logo.Header().Get("Content-Type"), "image/png")
	assert.True(t, strings.HasPrefix(logo.Body.String(), "\x89PNG"))

	// No tokens, no logo: nothing to link, nothing to serve.
	plain := webui.App{}.MustCompile("")
	plainPage := serve(plain, http.MethodGet, "/").Body.String()
	assert.False(t, strings.Contains(plainPage, "theme."))
	assert.False(t, strings.Contains(plainPage, "/_webui/logo"))
}

// A label that is wider than the sidebar is the whole of it in the title, in the entry and in the caption.
func TestASidebarLabelWiderThanTheSidebarIsWholeInItsTitle(t *testing.T) {
	t.Parallel()

	h := webui.App{Brand: webui.Brand{Name: "A name that is far too long for the sidebar to hold"}, Pages: webui.Pages{
		webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "An entry that is far too long for the sidebar to hold", Section: "A caption that is far too long for the sidebar to hold"}, View: webui.Stack{}},
	}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/a").Body.String()
	assert.Contains(t, body, `title="An entry that is far too long for the sidebar to hold"`)
	assert.Contains(t, body, `title="A caption that is far too long for the sidebar to hold"`)
}
