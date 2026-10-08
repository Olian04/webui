// Package assets embeds the stylesheet, scripts and icon font the renderer
// serves.
package assets

import (
	"embed"
	"strings"
)

// FS holds css/, js/, fonts/ and the default favicon. Nothing else is served.
//
//go:embed css js fonts favicon.svg
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
