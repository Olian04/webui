// Package webui is the public library surface for this module.
//
// This package is glue: it composes internal capabilities and exports the
// API other modules import. Consumers cannot import internal/. Keep
// behaviour in internal/<capability>; keep public types here.
package webui

import "github.com/Olian04/webui/internal/echo"

// Echo returns message with surrounding whitespace removed.
func Echo(message string) string {
	return echo.Echo(message)
}
