// Package monaco installs the code editor: the files that an [webui.Editor] and a [webui.Diff] draw with, which are most of what the library carries, a few megabytes. Import
// it for its effect, and only a program that has an editor or a diff needs to:
//
//	import _ "github.com/Olian04/webui/pkg/monaco"
//
// A program that does not import it does not link the files, and an app that has an Editor or a Diff and was not given them is refused when it is compiled, with this import as the
// fix. The language services (pkg/monaco/css, html, json and typescript) import this package themselves, so a program that imports one has the editor already.
package monaco

import (
	"embed"
	"io/fs"

	"github.com/Olian04/webui/internal/render/assets"
)

//go:embed editor
var files embed.FS

func init() {
	editor, err := fs.Sub(files, "editor")
	if err != nil {
		panic(err) // the directory is embedded by the line above
	}
	assets.RegisterEditor(editor)
}
