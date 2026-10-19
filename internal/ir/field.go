package ir

import (
	"context"
	"iter"
	"regexp"
	"strings"
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

	// Show is the text a person reads where the value is only read: a table cell, a field that
	// cannot be edited, and what the search looks in. Nil means Get. It is how an application
	// says a size reads "1.5 GiB" while the column sorts, filters and exports on the number:
	// nothing but what is shown reads it. It is given the context of the request, which carries the app's logger
	// (see [LoggerFrom]) for a Format that panics.
	Show func(ctx context.Context, model any) string

	Rules Rules

	Key         string // the label as the address names it, lower-cased with dashes: what Query.Sort carries
	Placeholder string // from the accessor

	// Options is the fixed set of values a column can hold, when it has one: a
	// table's filter offers exactly these, as a multi-select. Nil means free
	// text, which a table's filter takes as typed. For a DisplaySelect it is the
	// values of Choices, in their order.
	Options []string

	// Link is the address of a DisplayLink field, "" when the value is not one that
	// may be a link, which is then shown as plain text.
	Link func(model any) string

	// OpenSet says the column's values are not declared but are whatever its rows
	// hold, for a table whose rows the library holds in full: its filter is a
	// multi-select of the values present, whatever they are, and matches the rows
	// holding any of those chosen.
	OpenSet bool

	// Display is how the value is shown. Text is the default.
	Display Display

	// Choices is, for a DisplaySelect, the values it can hold in the order the author gave
	// them, each with what it says and its tone. It may be empty for a read-only select whose
	// values are whatever its rows hold. Multi says it holds any number of them: Get then
	// returns the values joined by ListSep, and Set is handed them joined the same way.
	Choices []Choice
	Multi   bool

	Min float64 // DisplaySlider: the range, also in Rules for a writable one
	Max float64
}

// ListSep joins the values of a field that holds several, and of a parameter that may repeat,
// into one string. It is a control character, so it cannot be typed into a field and no value
// contains it.
const ListSep = "\x1f"

// Choice is one value a select can hold: what is stored, what it says, and how it is drawn.
type Choice struct {
	Value string
	Label string // what it says; the value when it is empty
	Tone  Tone
}

// Leaves are the fields of fs that are not groups, in order, with the fields of a group in the
// place of the group.
func Leaves(fs []Field) iter.Seq[Field] {
	return func(yield func(Field) bool) {
		for _, f := range fs {
			if len(f.Group) == 0 {
				if !yield(f) {
					return
				}
				continue
			}
			for leaf := range Leaves(f.Group) {
				if !yield(leaf) {
					return
				}
			}
		}
	}
}

// Shown is the text that a value is read as: Show, else Get.
func (f Field) Shown(ctx context.Context, row any) string {
	if f.Show != nil {
		return f.Show(ctx, row)
	}
	return f.Get(row)
}

// Values are the values a DisplaySelect field holds in a row: what Get returns, split.
func (f Field) Values(row any) []string { return SplitValues(f.Get(row)) }

// SplitValues is a select's values, as Get holds them joined by ListSep. An empty one is none.
func SplitValues(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, ListSep)
}

// Choice is the choice a value is, if the field declares one.
func (f Field) Choice(value string) (Choice, bool) {
	for _, c := range f.Choices {
		if c.Value == value {
			return c, true
		}
	}
	return Choice{}, false
}

// ToneOf is the tone a value is drawn in: the one its choice has, or neutral for a value the field does
// not declare.
func (f Field) ToneOf(value string) Tone {
	c, _ := f.Choice(value)
	return c.Tone
}

// LabelOf is what a value says: its choice's label, or the value itself.
func (f Field) LabelOf(value string) string {
	if c, ok := f.Choice(value); ok && c.Label != "" {
		return c.Label
	}
	return value
}

// Display is how a field shows its value, in a table cell and in a form.
type Display uint8

// The displays.
const (
	DisplayText   Display = iota
	DisplaySelect         // badges when read, a choice from a list when writable
	DisplaySlider         // a bar when read-only, a range input when writable
	DisplayLink           // a link: Get is the text it shows, and Link its address
	DisplayTime           // a moment: Get is "2006-01-02 15:04" in UTC, or a date alone, and Num is Unix seconds
)

// Ranged is whether a table filters the column by a minimum and a maximum, as it
// does a number, and sorts it by Num: a numeric column, or a moment.
func (f Field) Ranged() bool {
	return f.Kind == KindInt || f.Kind == KindFloat || f.Display == DisplayTime
}

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

	// What the text of a string must, or must not, start with, end with and contain; "" means no constraint. A value
	// holds them all, and case matters.
	StartsWith, EndsWith, Contains          string
	NotStartsWith, NotEndsWith, NotContains string

	Min *float64 // numbers; nil means unbounded
	Max *float64 // numbers; nil means unbounded

	Pattern *Pattern
}

// Pattern is an expression with the message shown when it fails. It runs twice, in Go on the server and in the
// browser as the pattern attribute, and Go's regexp and JavaScript's differ, so it is written in the syntax that both
// read alike (see rules.Compile). pkg/webui compiled it while lowering; it is valid here.
//
// Expr is for Go: anchored, because the whole value must match, which is what an HTML pattern attribute does, so that
// the browser and the server agree, and with what a browser reads differently written as the browser reads it. Source is
// the expression as the user wrote it, for the attribute.
type Pattern struct {
	Expr    *regexp.Regexp
	Source  string
	Message string
}
