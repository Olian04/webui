package render

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/Olian04/webui/internal/editorctx"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render/assets"
)

// CodeNeeds is what a page's editors and diffs need loaded. The editor is a large
// bundle, so a page that has none does not link it, and a language service is a worker
// of its own, so one is handed to a page only when it has an editor that can be edited
// in that language.
type CodeNeeds struct {
	Used     bool            // the page has an editor or a diff
	Services map[string]bool // the language services an editable editor on it can use

	// Contexts is what the language services of the page's editable editors are given, by
	// the context's id, so editors given one folder are one entry.
	Contexts map[string]*editorctx.Context
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

// AddContext records the context of an editor that can be edited. It is what hands the
// page the addresses of the files, so it is called only for a visitor who may edit.
func (c *CodeNeeds) AddContext(ctx *editorctx.Context) {
	if ctx == nil {
		return
	}
	if c.Contexts == nil {
		c.Contexts = map[string]*editorctx.Context{}
	}
	c.Contexts[ctx.ID] = ctx
}

// codeConfig is what the script that mounts the editors is told: where to find the
// worker of each language service the page may use, and the plain one, and what the
// services are given besides the text.
type codeConfig struct {
	Workers map[string]string `json:"workers"`
	Context *contextConfig    `json:"context,omitempty"`
}

// contextConfig is the files and options of the contexts of a page's editors. The files
// are addresses, which carry the key that unlocks them.
type contextConfig struct {
	// Libs are the declaration files of the TypeScript and JavaScript services, by service
	// language, each service's files being one program for the page.
	Libs map[string][]contextFile `json:"libs,omitempty"`

	// Compiler is the options of those services, by language.
	Compiler map[string]map[string]any `json:"compiler,omitempty"`

	// Schemas are the files of each JSON editor's schema, by the context's id, which the
	// editor's markup names.
	Schemas map[string][]contextFile `json:"schemas,omitempty"`
}

type contextFile struct {
	Path string `json:"path"`
	URL  string `json:"url"`
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
	return codeAssets{Entry: base + files.Entry, Style: base + files.Style, Config: codeConfig{Workers: workers, Context: r.contextConfig(needs, workers)}}
}

// contextConfig is what the page tells its script about the contexts of its editors. A
// context is for a language service, so one whose service the program did not import is
// left out: nothing would read it, and the page should not be handed addresses for it.
func (r *Renderer) contextConfig(needs CodeNeeds, workers map[string]string) *contextConfig {
	out := &contextConfig{}
	files := func(ctx *editorctx.Context) []contextFile {
		list := make([]contextFile, len(ctx.Files))
		for i, f := range ctx.Files {
			list[i] = contextFile{Path: f.Path, URL: r.ContextURL(f.Name)}
		}
		return list
	}
	for _, ctx := range needs.Contexts {
		if _, ok := workers[ctx.Service()]; !ok {
			continue
		}
		switch ctx.Dir {
		case "json":
			if out.Schemas == nil {
				out.Schemas = map[string][]contextFile{}
			}
			out.Schemas[ctx.ID] = files(ctx)
		default:
			if out.Libs == nil {
				out.Libs, out.Compiler = map[string][]contextFile{}, map[string]map[string]any{}
			}
			seen := map[string]bool{}
			for _, f := range out.Libs[ctx.Dir] {
				seen[f.Path] = true
			}
			for _, f := range files(ctx) {
				if !seen[f.Path] { // editors of a page share one program: a path is one file
					out.Libs[ctx.Dir] = append(out.Libs[ctx.Dir], f)
				}
			}
			if ctx.Compiler != nil {
				out.Compiler[ctx.Dir] = ctx.Compiler
			}
		}
	}
	if out.Libs == nil && out.Schemas == nil {
		return nil
	}
	return out
}

// SetSecret keys the addresses of the context files. It is called once, as the program
// is made.
func (r *Renderer) SetSecret(secret []byte) { r.secret = secret }

// contextKey is the key that unlocks one context file: derived from its name, so a key
// opens that file and no other, and nothing has to be remembered per key.
func (r *Renderer) contextKey(name string) string {
	mac := hmac.New(sha256.New, r.secret)
	mac.Write([]byte(name))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// ContextURL is the address of a context file, with the key that unlocks it.
func (r *Renderer) ContextURL(name string) string {
	return r.prefix + AssetDir + "/code/" + name + "?k=" + url.QueryEscape(r.contextKey(name))
}

// ContextHandler serves the context files, to whoever has the key a page gave them. A
// file is private to the visitors a page was drawn for, but the same for all of them,
// so it is kept for good by the browser, and not by a cache that other people share. A
// request without the right key, or for a file that is not there, is a 404 either way,
// so it does not tell whether a file exists.
func (r *Renderer) ContextHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		name := req.PathValue("name")
		body, ok := r.contexts[name]
		key := req.URL.Query().Get("k")
		if !ok || subtle.ConstantTimeCompare([]byte(key), []byte(r.contextKey(name))) != 1 {
			http.NotFound(w, req)
			return
		}
		h := w.Header()
		h.Set("Content-Type", contextType(name))
		h.Set("Cache-Control", "private, max-age=31536000, immutable")
		etag := `"` + name + `"`
		h.Set("ETag", etag)
		if req.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(body) //nolint:gosec // G705: a file the application embedded, never request data.
	})
}

func contextType(name string) string {
	if strings.HasSuffix(name, ".json") {
		return "application/json"
	}
	return "text/plain; charset=utf-8"
}

// collectContexts remembers the text of every context file of the app, by the name it is
// served as, which is the hash of the text.
func collectContexts(app *ir.App) map[string][]byte {
	out := map[string][]byte{}
	for _, p := range app.Pages {
		for _, f := range ir.Editors(p.Body) {
			if f.Editor.Context == nil {
				continue
			}
			for _, file := range f.Editor.Context.Files {
				out[file.Name] = file.Text
			}
		}
	}
	return out
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
