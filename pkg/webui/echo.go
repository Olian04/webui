// Package webui is the public library surface for this module.
//
// This is the library mode's IO adapter: a thin facade over
// internal/domain/echo. Consumers outside this module cannot import `internal/`,
// so the exported API lives here and delegates inward. Keep the translation in
// this package — plain Go types out, domain types in — so the domain stays free
// of compatibility concerns and the public surface can evolve separately.
package webui

import "github.com/Olian04/webui/internal/domain/echo"

// Echo returns message with surrounding whitespace removed.
func Echo(message string) string {
	return echo.NewService().Echo(echo.Request{Message: message}).Message
}
