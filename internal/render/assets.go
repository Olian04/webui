package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/assetserve"
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

// buildAssets reads, hashes and compresses every file the app serves under AssetDir, once, so that
// a request does no work. Each is named for a short hash of its contents before the extension: the
// address changes when, and only when, the file does, so it is cached for a year as immutable. It
// sets the files by the name they are served as, and that name by the one they are known by here
// ("app.css"), which is what a page links to.
func (r *Renderer) buildAssets() error {
	r.files, r.served = map[string]*assetserve.Blob{}, map[string]string{}
	add := func(logical string, body []byte, mime string) {
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:4])
		ext := path.Ext(logical)
		name := strings.TrimSuffix(logical, ext) + "." + hash + ext
		b := &assetserve.Blob{Mime: mime, Cache: "public, max-age=31536000, immutable", ETag: `"` + hash + `"`, Body: body}
		if assetserve.Compressible(mime) {
			if gz := assetserve.Gzip(body); len(gz) < len(body) {
				b.Gzip = func() []byte { return gz } // only a file that shrinks is offered compressed
			}
		}
		r.files[name], r.served[logical] = b, name
	}
	read := func(path string) ([]byte, error) { return assets.FS.ReadFile(path) }

	// The font first: the stylesheet that uses it names it by its hashed name.
	font, err := read("fonts/fa-solid-900.woff2")
	if err != nil {
		return err
	}
	add("fa-solid-900.woff2", font, "font/woff2")
	icons, err := read("css/fontawesome.css")
	if err != nil {
		return err
	}
	icons = bytes.ReplaceAll(icons, []byte("url(fa-solid-900.woff2)"), []byte("url("+r.served["fa-solid-900.woff2"]+")"))
	add("fontawesome.css", icons, "text/css; charset=utf-8")

	for logical, file := range map[string]string{"app.css": "css/app.css", "prefs.js": "js/prefs.js", "enhance.js": "js/enhance.js"} {
		body, err := read(file)
		if err != nil {
			return err
		}
		mime := "text/css; charset=utf-8"
		if strings.HasSuffix(logical, ".js") {
			mime = "text/javascript; charset=utf-8"
		}
		add(logical, body, mime)
	}
	if r.hasTheme() {
		add("theme.css", themeCSS(r.app.Theme), "text/css; charset=utf-8")
	}
	if r.hasLogo() {
		add("logo", r.app.Brand.Logo, r.app.Brand.Mime)
	}
	for _, f := range r.app.Brand.Favicons {
		add(faviconName(f.Size), f.PNG, "image/png")
	}
	if r.app.Brand.DefaultFavicon {
		body, err := read("favicon.svg")
		if err != nil {
			return err
		}
		add("favicon.svg", body, "image/svg+xml")
	}
	return nil
}

// Asset is the file served under the name, or false for a name no file has.
func (r *Renderer) Asset(name string) (*assetserve.Blob, bool) {
	b, ok := r.files[name]
	return b, ok
}

// faviconName is the asset a favicon of one size is served as.
func faviconName(size int) string { return "favicon-" + strconv.Itoa(size) + ".png" }
