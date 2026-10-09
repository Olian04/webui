package render

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path"
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

// asset is one file the app serves under AssetDir, with what it needs to be served
// well: its name, which carries a short hash of its contents, and a compressed copy
// when that is smaller.
type asset struct {
	name string // as served: "app.410f9909.css"
	body []byte
	gz   []byte // compressed, or nil when it does not shrink or is already compressed
	mime string
	etag string
}

// assetSet is every asset of one app, by the name it is served as.
type assetSet struct {
	files map[string]*asset
	names map[string]string // logical name ("app.css") -> served name
}

// add registers a file, named by a short hash of its contents: the address changes
// when, and only when, the file does, so it can be cached for good. The name puts
// the hash before the extension.
func (s *assetSet) add(logical string, body []byte, mime string) {
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:4])
	ext := path.Ext(logical)
	a := &asset{
		name: strings.TrimSuffix(logical, ext) + "." + hash + ext,
		body: body, mime: mime, etag: `"` + hash + `"`,
	}
	if Compressible(mime) {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		_, _ = zw.Write(body)
		_ = zw.Close()
		if buf.Len() < len(body) {
			a.gz = buf.Bytes()
		}
	}
	s.files[a.name] = a
	s.names[logical] = a.name
}

// Compressible reports whether a response of this content type is worth compressing:
// text of any kind, JSON and SVG. Images and fonts are compressed already.
func Compressible(contentType string) bool {
	ct, _, _ := strings.Cut(contentType, ";")
	ct = strings.TrimSpace(strings.ToLower(ct))
	return strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "image/svg+xml"
}

// Assets serves the stylesheets, scripts, icon font, theme overrides and images.
// Everything is read, hashed and compressed once, so a request does no work and the
// handler is safe to share. Each is served under a name with a hash of its contents
// in it, which never changes meaning, so it is cached for a year as immutable; and
// each is compressed when the browser accepts it.
func (r *Renderer) Assets() (http.Handler, error) {
	set := &assetSet{files: map[string]*asset{}, names: map[string]string{}}
	read := func(path string) ([]byte, error) { return assets.FS.ReadFile(path) }

	// The font first: the stylesheet that uses it names it by its hashed name.
	font, err := read("fonts/fa-solid-900.woff2")
	if err != nil {
		return nil, err
	}
	set.add("fa-solid-900.woff2", font, "font/woff2")
	icons, err := read("css/fontawesome.css")
	if err != nil {
		return nil, err
	}
	icons = bytes.ReplaceAll(icons, []byte("url(fa-solid-900.woff2)"), []byte("url("+set.names["fa-solid-900.woff2"]+")"))
	set.add("fontawesome.css", icons, "text/css; charset=utf-8")

	for logical, file := range map[string]string{"app.css": "css/app.css", "prefs.js": "js/prefs.js", "enhance.js": "js/enhance.js"} {
		body, err := read(file)
		if err != nil {
			return nil, err
		}
		mime := "text/css; charset=utf-8"
		if strings.HasSuffix(logical, ".js") {
			mime = "text/javascript; charset=utf-8"
		}
		set.add(logical, body, mime)
	}
	if r.hasCSS {
		set.add("theme.css", themeCSS(r.app.Theme), "text/css; charset=utf-8")
	}
	if r.logo {
		set.add("logo", r.app.Brand.Logo, r.app.Brand.Mime)
	}
	for _, f := range r.app.Brand.Favicons {
		set.add(faviconName(f.Size), f.PNG, "image/png")
	}
	if r.app.Brand.DefaultFavicon {
		body, err := read("favicon.svg")
		if err != nil {
			return nil, err
		}
		set.add("favicon.svg", body, "image/svg+xml")
	}
	r.served = set.names

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		a, ok := set.files[req.PathValue("file")]
		if !ok {
			http.NotFound(w, req)
			return
		}
		body, etag := a.body, a.etag
		h := w.Header()
		h.Set("Content-Type", a.mime)
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		if a.gz != nil {
			h.Add("Vary", "Accept-Encoding")
			if AcceptsGzip(req) {
				h.Set("Content-Encoding", "gzip")
				body, etag = a.gz, strings.TrimSuffix(a.etag, `"`)+`-gzip"`
			}
		}
		h.Set("ETag", etag)
		if req.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(body)
	}), nil
}

// AcceptsGzip reports whether the client accepts a gzip-encoded response: it lists
// gzip, and not with a quality of zero.
func AcceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		coding, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(coding), "gzip") {
			continue
		}
		q := 1.0
		if _, v, ok := strings.Cut(strings.ReplaceAll(params, " ", ""), "q="); ok {
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				q = parsed
			}
		}
		return q > 0
	}
	return false
}

// faviconName is the asset a favicon of one size is served as.
func faviconName(size int) string { return "favicon-" + strconv.Itoa(size) + ".png" }
