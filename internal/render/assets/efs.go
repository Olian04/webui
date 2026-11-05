// Package assets embeds the minified stylesheets, scripts and icon font the renderer
// serves.
package assets

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
)

// FS holds min/ (the minified stylesheets and scripts, built from css/ and js/ by make assets), fonts/ and the default favicon. Nothing else is served, and not the
// editor, which is in pkg/monaco, so that a program that has no editor does not carry it.
//
//go:embed min fonts favicon.svg
var FS embed.FS

// iconNames is every Font Awesome Free solid icon name, one per line, so a name
// is looked up without building a map.
//
//go:embed icons.txt
var iconNames string

// HasIcon reports whether name is a Font Awesome Free solid icon the renderer
// ships, such as "house", without the "fa-" prefix.
func HasIcon(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\n ") && strings.Contains(iconNames, "\n"+name+"\n")
}

// Monaco is the editor's entry module, its stylesheet and the plain worker, by name
// under monaco/, as the build wrote them. All of them carry a hash of their contents.
type Monaco struct {
	Version      string `json:"version"`
	Entry        string `json:"entry"`
	Style        string `json:"style"`
	EditorWorker string `json:"editorWorker"`
}

// The editor is the files of pkg/monaco, which a program imports to have it. The package
// registers them here from init, as a language service registers its worker.
var (
	editorMu sync.RWMutex
	editor   fs.FS
)

// RegisterEditor installs the editor: manifest.json and the files it names are at the root of files.
func RegisterEditor(files fs.FS) {
	editorMu.Lock()
	defer editorMu.Unlock()
	editor = files
}

// EditorInstalled is whether the program imported the editor.
func EditorInstalled() bool {
	editorMu.RLock()
	defer editorMu.RUnlock()
	return editor != nil
}

// MonacoFiles reads the manifest of the editor the build produced, or fails when the program did not import the editor.
func MonacoFiles() (Monaco, error) {
	var m Monaco
	editorMu.RLock()
	files := editor
	editorMu.RUnlock()
	if files == nil {
		return m, errors.New("the editor is not installed: import _ \"github.com/Olian04/webui/pkg/monaco\"")
	}
	b, err := fs.ReadFile(files, "manifest.json")
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(b, &m)
}

// The language services are workers, one package each under pkg/monaco, so an
// application that does not import one does not carry it. A package registers its
// worker here when it is imported.
var (
	servicesMu sync.RWMutex
	services   = map[string]fs.FS{}
)

// RegisterService installs the worker of a language service: the files of one are in
// worker/ in files, and there is one script in it.
func RegisterService(id string, files fs.FS) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	services[id] = files
}

// ServiceWorkers is the name under monaco/ of the worker of each installed language
// service, by the service's id.
func ServiceWorkers() map[string]string {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	out := make(map[string]string, len(services))
	for id, files := range services {
		if scripts, _ := fs.Glob(files, "worker/*.js"); len(scripts) == 1 {
			out[id] = "workers/" + path.Base(scripts[0])
		}
	}
	return out
}

// MonacoFile is one file of the editor, by its name under monaco/: from the editor
// itself, or from the worker of an installed language service.
func MonacoFile(name string) ([]byte, bool) {
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || name == "manifest.json" {
		return nil, false
	}
	editorMu.RLock()
	files := editor
	editorMu.RUnlock()
	if files != nil {
		if b, err := fs.ReadFile(files, name); err == nil {
			return b, true
		}
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	ids := make([]string, 0, len(services))
	for id := range services {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if file, ok := strings.CutPrefix(name, "workers/"); ok {
			if b, err := fs.ReadFile(services[id], path.Join("worker", file)); err == nil {
				return b, true
			}
		}
	}
	return nil, false
}
