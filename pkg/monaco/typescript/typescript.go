// Package typescript installs the language service of the editor for TypeScript and JavaScript.
//
// Import it for its effect:
//
//	import _ "github.com/Olian04/webui/pkg/monaco/typescript"
//
// An [webui.Editor] that can be edited, and whose language is LangTypeScript and LangJavaScript, then
// offers TypeScript and JavaScript (type checking, completion, signature help, quick info), from a worker that the page loads only when it has such an
// editor. Without the import the editor still highlights, and the worker is not
// linked into the program.
//
// A viewer, and a [webui.Diff], never load it.
package typescript

import (
	"embed"

	"github.com/Olian04/webui/internal/render/assets"
)

//go:embed worker
var files embed.FS

func init() { assets.RegisterService("ts", files) }
