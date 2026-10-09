package webui_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func logo() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 102, A: 255})
		}
	}
	return img
}

func TestTheFaviconIsMadeFromTheLogoAndServed(t *testing.T) {
	t.Parallel()

	h := webui.App{Brand: webui.Brand{Name: "Acme", Logo: logo()}}.MustCompile("/admin")
	page := serve(h, http.MethodGet, "/admin/").Body.String()
	assert.Contains(t, page, `<link rel="icon" type="image/png" sizes="32x32" href="`+assetURL(t, h, "favicon-32.png")+`">`)
	assert.Contains(t, page, `<link rel="apple-touch-icon" sizes="180x180" href="`+assetURL(t, h, "favicon-180.png")+`">`)

	for name, size := range map[string]int{"favicon-32.png": 32, "favicon-180.png": 180} {
		rec := serve(h, http.MethodGet, assetURL(t, h, name))
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Equal(t, rec.Header().Get("Content-Type"), "image/png")
		img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		assert.NoError(t, err)
		assert.Equal(t, img.Bounds().Dx(), size)
		assert.Equal(t, img.Bounds().Dy(), size)
	}
}

func TestWithoutALogoTheFaviconIsTheLibrarysMark(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	assert.Contains(t, serve(h, http.MethodGet, "/admin/").Body.String(),
		`<link rel="icon" type="image/svg+xml" href="`+assetURL(t, h, "favicon.svg")+`">`)

	rec := serve(h, http.MethodGet, assetURL(t, h, "favicon.svg"))
	assert.Equal(t, rec.Header().Get("Content-Type"), "image/svg+xml")
	assert.True(t, strings.HasPrefix(rec.Body.String(), "<svg"))
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/").Body.String(), "favicon-32"))
}

func TestNoFaviconGeneratesAndServesNothing(t *testing.T) {
	t.Parallel()

	for name, brand := range map[string]webui.Brand{
		"with a logo": {Logo: logo(), NoFavicon: true},
		"no logo":     {NoFavicon: true},
	} {
		h := webui.App{Brand: brand}.MustCompile("/admin")
		page := serve(h, http.MethodGet, "/admin/").Body.String()
		assert.False(t, strings.Contains(page, `rel="icon"`))
		assert.False(t, strings.Contains(page, "apple-touch-icon"))
		for _, file := range []string{"favicon.svg", "favicon-32", "favicon-180"} {
			if strings.Contains(page, file) {
				t.Errorf("%s: the page links %s", name, file)
			}
		}
	}
}
