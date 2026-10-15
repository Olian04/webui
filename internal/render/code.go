package render

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"mime"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/Olian04/webui/internal/assetserve"
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

// Merge records what another set of needs, built for other panels of the page, has.
func (c *CodeNeeds) Merge(other CodeNeeds) {
	c.Used = c.Used || other.Used
	for service := range other.Services {
		c.Add(service)
	}
	for _, ctx := range other.Contexts {
		c.AddContext(ctx)
	}
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
			list[i] = contextFile{Path: f.Path, URL: r.contexts[f.Name].url}
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

// contextEntry is one context file as the app serves it, made once: the key that unlocks it, the address
// a page gives for it with that key, and the file itself.
type contextEntry struct {
	key  string
	url  string
	blob *assetserve.Blob
}

// contextKey is the key that unlocks one context file: derived from its name, so a key
// opens that file and no other, and nothing has to be remembered per key.
func contextKey(secret []byte, name string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(name))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// ContextFile is a context file, for whoever has the key a page gave them. A file is private to the
// visitors a page was drawn for, but the same for all of them, so it is kept for good by the browser,
// and not by a cache that other people share. A wrong key, or a file that is not there, is the same
// answer, so it does not tell whether a file exists.
func (r *Renderer) ContextFile(name, key string) (*assetserve.Blob, bool) {
	e, ok := r.contexts[name]
	if !ok || subtle.ConstantTimeCompare([]byte(key), []byte(e.key)) != 1 {
		return nil, false
	}
	return e.blob, true
}

func contextType(name string) string {
	if strings.HasSuffix(name, ".json") {
		return "application/json"
	}
	return "text/plain; charset=utf-8"
}

// buildContexts makes the entry of every context file of the app, by the name it is served as, which
// is the hash of the text. They are the same for every request, so the key, the address and the file
// are made once, here.
func (r *Renderer) buildContexts() {
	r.contexts = map[string]*contextEntry{}
	for _, p := range r.app.Pages {
		for _, f := range ir.Editors(p.Body) {
			if f.Editor.Context == nil {
				continue
			}
			for _, file := range f.Editor.Context.Files {
				key := contextKey(r.secret, file.Name)
				r.contexts[file.Name] = &contextEntry{
					key: key,
					url: r.prefix + AssetDir + "/code/" + file.Name + "?k=" + url.QueryEscape(key),
					blob: &assetserve.Blob{
						Mime: contextType(file.Name), Cache: "private, max-age=31536000, immutable",
						ETag: `"` + file.Name + `"`, Body: file.Text,
					},
				}
			}
		}
	}
}

// monacoBlobs remembers each editor file as it is served, made when it is first asked for: the
// bundle is several megabytes and most programs never serve it. A file is read, and compressed, once.
var monacoBlobs sync.Map // name → *assetserve.Blob

// MonacoFile is a file of the editor, or the worker of a language service the application imported,
// by its name under monaco/. Every name carries a hash of its contents, so a browser keeps a file for
// good, and a script or stylesheet is offered compressed, made when it is first asked for.
func (r *Renderer) MonacoFile(name string) (*assetserve.Blob, bool) {
	if b, ok := monacoBlobs.Load(name); ok {
		return b.(*assetserve.Blob), true
	}
	body, ok := assets.MonacoFile(name)
	if !ok {
		return nil, false
	}
	b := &assetserve.Blob{Mime: monacoType(name), Cache: "public, max-age=31536000, immutable", ETag: `"` + path.Base(name) + `"`, Body: body}
	if strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") {
		b.Gzip = sync.OnceValue(func() []byte { return assetserve.Gzip(body) })
	}
	shared, _ := monacoBlobs.LoadOrStore(name, b) // two first requests may race: one blob wins
	return shared.(*assetserve.Blob), true
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
