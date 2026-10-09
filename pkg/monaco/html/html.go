// Package html installs the language service of the editor for HTML, Handlebars and Razor.
//
// Import it for its effect:
//
//	import _ "github.com/Olian04/webui/pkg/monaco/html"
//
// An [webui.Editor] that can be edited, and whose language is LangHTML, LangHandlebars and LangRazor, then
// offers HTML, Handlebars and Razor (tag and attribute completion, auto-closing tags, formatting), from a worker that the page loads only when it has such an
// editor. Without the import the editor still highlights, and the worker is not
// linked into the program.
//
// A viewer, and a [webui.Diff], never load it.
package html

import (
	"embed"

	"github.com/Olian04/webui/internal/render/assets"
)

//go:embed worker
var files embed.FS

func init() { assets.RegisterService("html", files) }
