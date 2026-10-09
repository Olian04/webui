// Package json installs the language service of the editor for JSON.
//
// Import it for its effect:
//
//	import _ "github.com/Olian04/webui/pkg/monaco/json"
//
// An [webui.Editor] that can be edited, and whose language is LangJSON, then
// offers JSON (diagnostics against the syntax, completion, formatting, folding), from a worker that the page loads only when it has such an
// editor. Without the import the editor still highlights, and the worker is not
// linked into the program.
//
// A viewer, and a [webui.Diff], never load it.
package json

import (
	"embed"

	"github.com/Olian04/webui/internal/render/assets"
)

//go:embed worker
var files embed.FS

func init() { assets.RegisterService("json", files) }
