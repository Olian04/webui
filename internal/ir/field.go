package ir

import (
	"regexp"
)

// Field is one accessor, lowered. The same Field can render as a table cell
// or a form input. Decorators from the declaration are flattened into it.
type Field struct {
	Label string
	Kind  ValueKind
	Group []Field // non-empty means a group; Get and Set are then nil

	// Get formats the value for display. Set parses and assigns, and is nil
	// for a read-only accessor, which is what a missing Store lowers to.
	Get func(model any) string
	Set func(model any, raw string) error

	Rules Rules

	SortKey     string // from Sortable; empty means not sortable
	Placeholder string // from Placeholder
}

// Rules is one struct for every value kind. pkg/webui guarantees only the
// applicable parts are set, so downstream renders unconditionally.
type Rules struct {
	Required bool

	MinLen int // strings; 0 means no minimum
	MaxLen int // strings; 0 means no maximum

	Min *float64 // numbers; nil means unbounded
	Max *float64 // numbers; nil means unbounded

	Pattern *Pattern
}

// Pattern is an RE2 expression with the message shown when it fails. RE2 is
// the intersection of Go's regexp and JavaScript's, so the same expression
// runs on both sides. pkg/webui compiled it while lowering; it is valid here.
type Pattern struct {
	Expr    *regexp.Regexp
	Message string
}
