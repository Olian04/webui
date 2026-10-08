// Package assets embeds the stylesheet and scripts the renderer serves.
package assets

import (
	"embed"
)

// FS holds css/ and js/. Nothing else is served.
//
//go:embed css js
var FS embed.FS
