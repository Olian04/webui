// Package css installs the language service of the editor for CSS, SCSS and Less.
//
// Import it for its effect:
//
//	import _ "github.com/Olian04/webui/pkg/monaco/css"
//
// An [webui.Editor] that can be edited, and whose language is LangCSS, LangSCSS and LangLess, then
// offers CSS, SCSS and Less (diagnostics, completion of properties and values, colour hints, formatting), from a worker that the page loads only when it has such an
// editor. Without the import the editor still highlights, and the worker is not
// linked into the program.
//
// A viewer, and a [webui.Diff], never load it.
package css

import (
	"embed"

	"github.com/Olian04/webui/internal/render/assets"
)

//go:embed worker
var files embed.FS

func init() { assets.RegisterService("css", files) }
