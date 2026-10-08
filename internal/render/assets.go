package render

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/render/assets"
)

// AssetDir is where framework assets are served, under the mount prefix. Page
// paths may not start with it; validate enforces that.
const AssetDir = "/_webui"

// failureHead inlines the stylesheet, because the failure page is served at
// every address and cannot assume any asset route exists.
var failureHead = sync.OnceValue(func() templ.Component {
	css, err := assets.FS.ReadFile("css/app.css")
	if err != nil {
		return templ.NopComponent
	}
	return templ.Raw("<style>" + string(css) + "</style>")
})

type asset struct {
	body []byte
	mime string
	etag string
}

func newAsset(body []byte, mime string) *asset {
	sum := sha256.Sum256(body)
	return &asset{body: body, mime: mime, etag: `"` + hex.EncodeToString(sum[:8]) + `"`}
}

// Assets serves the stylesheet, scripts, theme overrides and logo. Everything
// is read once, so a request does no I/O and the handler is safe to share.
func (r *Renderer) Assets() (http.Handler, error) {
	files := map[string]*asset{}
	for name, path := range map[string]string{
		"app.css": "css/app.css", "prefs.js": "js/prefs.js", "enhance.js": "js/enhance.js",
		"fontawesome.css": "css/fontawesome.css", "fa-solid-900.woff2": "fonts/fa-solid-900.woff2",
	} {
		body, err := assets.FS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		mime := "text/css; charset=utf-8"
		switch {
		case strings.HasSuffix(name, ".js"):
			mime = "text/javascript; charset=utf-8"
		case strings.HasSuffix(name, ".woff2"):
			mime = "font/woff2"
		}
		files[name] = newAsset(body, mime)
	}
	if r.hasCSS {
		files["theme.css"] = newAsset(themeCSS(r.app.Theme), "text/css; charset=utf-8")
	}
	if r.logo {
		files["logo"] = newAsset(r.app.Brand.Logo, r.app.Brand.Mime)
	}
	for _, f := range r.app.Brand.Favicons {
		files[faviconName(f.Size)] = newAsset(f.PNG, "image/png")
	}
	if r.app.Brand.DefaultFavicon {
		body, err := assets.FS.ReadFile("favicon.svg")
		if err != nil {
			return nil, err
		}
		files["favicon.svg"] = newAsset(body, "image/svg+xml")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		a, ok := files[req.PathValue("file")]
		if !ok {
			http.NotFound(w, req)
			return
		}
		h := w.Header()
		h.Set("Content-Type", a.mime)
		h.Set("ETag", a.etag)
		h.Set("Cache-Control", "no-cache") // revalidate: cheap, and never stale
		if req.Header.Get("If-None-Match") == a.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(a.body)
	}), nil
}

// faviconName is the asset a favicon of one size is served as.
func faviconName(size int) string { return "favicon-" + strconv.Itoa(size) + ".png" }
