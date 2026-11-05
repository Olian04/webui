package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"path"
	"slices"
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
	css, err := assets.FS.ReadFile("min/css/app.css")
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

	if err := r.addIcons(add, read); err != nil {
		return err
	}

	for logical, file := range map[string]string{"app.css": "min/css/app.css", "prefs.js": "min/js/prefs.js", "enhance.js": "min/js/enhance.js"} {
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
	fileJS, err := read("min/js/file.js")
	if err != nil {
		return err
	}
	add("file.js", fileJS, "text/javascript; charset=utf-8")
	uploadJS, err := read("min/js/upload.js")
	if err != nil {
		return err
	}
	add("upload.js", uploadJS, "text/javascript; charset=utf-8")
	phoneJS, err := read("min/js/phone.js")
	if err != nil {
		return err
	}
	add("phone.js", phoneJS, "text/javascript; charset=utf-8")
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

// addIcons makes the icon stylesheet of this app, and serves its font. An app can name an icon in two places only, a page's Nav and an external link, and both are
// checked against the list of icons when it compiles, so the icons it can draw are known now: those, and the ones the library draws itself. The font is cut to
// them, and the stylesheet has a rule for each of them and no other: the stylesheet of every icon was 83 KB of which a page used a few lines, and the font 120 KB of
// which it drew a few glyphs.
func (r *Renderer) addIcons(add func(logical string, body []byte, mime string), read func(string) ([]byte, error)) error {
	names := map[string]bool{}
	for _, name := range assets.CoreIcons() {
		names[name] = true
	}
	for _, p := range r.app.Pages {
		if p.Nav.Icon != "" {
			names[p.Nav.Icon] = true
		}
	}
	for _, l := range r.app.ExternalLinks {
		if l.Icon != "" {
			names[l.Icon] = true
		}
	}
	sorted := slices.Sorted(maps.Keys(names))

	font, err := assets.IconFont(sorted)
	if err != nil {
		return err // Compile checks every name, so this is icons.txt being out of step with the font
	}
	add("fa-solid.woff", font, "font/woff")

	base, err := read("min/css/icons.css")
	if err != nil {
		return err
	}
	var css bytes.Buffer
	css.Write(base)
	fmt.Fprintf(&css, "@font-face{font-family:'Font Awesome Solid';font-style:normal;font-weight:900;font-display:block;src:url(%s) format('woff')}", r.served["fa-solid.woff"])
	for _, name := range sorted {
		code, _ := assets.IconCode(name)
		fmt.Fprintf(&css, ".fa-%s:before{content:\"\\%x\"}", name, code)
	}
	add("fontawesome.css", css.Bytes(), "text/css; charset=utf-8")
	return nil
}
