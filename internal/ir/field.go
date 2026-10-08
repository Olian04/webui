package ir

import (
	"regexp"
)

// Field is one accessor, lowered. The same Field can render as a table cell
// or a form input. Decorators from the declaration are flattened into it.
type Field struct {
	// Name is the stable form-control name, derived from position ("f0", "f0_1"
	// inside a group; "c0" for a table column). It never comes from user input.
	Name  string
	Label string
	Kind  ValueKind // KindString, KindInt or KindFloat: what an accessor holds
	Group []Field   // non-empty means a group; Get and Set are then nil

	// Get formats the value for display and for echoing into an input. Set
	// parses and assigns to a *model, and is nil for a read-only accessor,
	// which is what a missing Store lowers to.
	Get func(model any) string
	Set func(model any, raw string) error

	// Num is the value as a number, for a numeric column, which a table sorts and
	// filters as a number and not as the text Get shows. Nil for any other kind.
	Num func(model any) float64

	Rules Rules

	Key         string // the label as the address names it, lower-cased with dashes: what Query.Sort carries
	Placeholder string // from Placeholder

	// Options is the fixed set of values a column can hold, when it has one: a
	// table's filter offers exactly these, as a multi-select. Nil means free
	// text, which a table's filter takes as typed.
	Options []string

	// Display is how the value is shown. Text is the default.
	Display Display
	Kinds   map[string]Tone // DisplayBadge: the values it can hold, each with its tone
	Min     float64         // DisplaySlider: the range, also in Rules for a writable one
	Max     float64
}

// Display is how a field shows its value, in a table cell and in a form.
type Display uint8

// The displays.
const (
	DisplayText Display = iota
	DisplayBadge
	DisplaySlider // a bar when read-only, a range input when writable
)

// Tone is the semantic meaning of a badge.
type Tone string

// The tones, matching the design's five meanings (neutral is the zero value).
const (
	ToneNeutral  Tone = ""
	ToneOK       Tone = "ok"
	ToneWarning  Tone = "warn"
	ToneCritical Tone = "bad"
)

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
//
// Expr is anchored: the whole value must match, which is what an HTML pattern
// attribute does, so the browser and the server agree. Source is the
// expression as the user wrote it, for the attribute.
type Pattern struct {
	Expr    *regexp.Regexp
	Source  string
	Message string
}
