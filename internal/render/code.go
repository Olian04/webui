package render

import (
	"bytes"
	"compress/gzip"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/Olian04/webui/internal/render/assets"
)

// CodeNeeds is what a page's editors and diffs need loaded. The editor is a large
// bundle, so a page that has none does not link it, and a language service is a worker
// of its own, so one is handed to a page only when it has an editor that can be edited
// in that language.
type CodeNeeds struct {
	Used     bool            // the page has an editor or a diff
	Services map[string]bool // the language services an editable editor on it can use
}

// Add records an editor or a diff, and the language service an editor that can be
// edited could use ("" for none).
func (c *CodeNeeds) Add(service string) {
	c.Used = true
	if service == "" {
		return
	}
	if c.Services == nil {
		c.Services = map[string]bool{}
	}
	c.Services[service] = true
}

// codeConfig is what the script that mounts the editors is told: where to find the
// worker of each language service the page may use, and the plain one.
type codeConfig struct {
	Workers map[string]string `json:"workers"`
}

// codeAssets is the addresses of the editor's entry script and stylesheet, and its
// configuration for one page.
type codeAssets struct {
	Entry  string
	Style  string
	Config codeConfig
}

func (r *Renderer) code(needs CodeNeeds) codeAssets {
	files := r.monaco
	base := r.prefix + AssetDir + "/monaco/"
	workers := map[string]string{"editor": base + files.EditorWorker}
	installed := assets.ServiceWorkers()
	for service := range needs.Services {
		if file, ok := installed[service]; ok {
			workers[service] = base + file
		}
	}
	return codeAssets{Entry: base + files.Entry, Style: base + files.Style, Config: codeConfig{Workers: workers}}
}

// gzipped remembers each editor file compressed, made when it is first asked for: the
// bundle is several megabytes and most programs never serve it.
var gzipped sync.Map // name → []byte

// MonacoHandler serves the editor's files, and the workers of the language services the
// application imported. Every name carries a hash of its contents, so a browser keeps a
// file for good.
func (r *Renderer) MonacoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		name := req.PathValue("path")
		body, ok := assets.MonacoFile(name)
		if !ok {
			http.NotFound(w, req)
			return
		}
		h := w.Header()
		h.Set("Content-Type", monacoType(name))
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		etag := `"` + path.Base(name) + `"`
		if strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") {
			h.Add("Vary", "Accept-Encoding")
			if AcceptsGzip(req) {
				h.Set("Content-Encoding", "gzip")
				etag = strings.TrimSuffix(etag, `"`) + `-gzip"`
				body = compress(name, body)
			}
		}
		h.Set("ETag", etag)
		if req.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(body) //nolint:gosec // G705: an embedded file, never request data.
	})
}

func compress(name string, body []byte) []byte {
	if cached, ok := gzipped.Load(name); ok {
		return cached.([]byte)
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(body)
	_ = zw.Close()
	gzipped.Store(name, buf.Bytes())
	return buf.Bytes()
}

func monacoType(name string) string {
	switch path.Ext(name) {
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".ttf":
		return "font/ttf"
	}
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}
