// Package echo holds echo capability behaviour.
//
// pkg/webui consumes this package as glue. Do not import pkg/.
package echo

import "strings"

// Echo returns message with surrounding whitespace removed.
func Echo(message string) string {
	return strings.TrimSpace(message)
}
