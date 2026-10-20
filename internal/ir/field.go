package ir

import (
	"context"
	"fmt"
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

	// NoSort and NoFilter leave a column out of the table's sort and of its filters: its header has no sort link or
	// no filter, and the address cannot ask for one. A form ignores them.
	NoSort, NoFilter bool

	// ChoicesFor, when a select has it, reads Choices (and Options, the values of them) when a request is
	// answered, from whatever the app keeps them in: until it has been read there are none, and Options is
	// the empty set, so that the column is known to have a fixed one. A field is shared by every request, so
	// it is not filled in: see [ResolveChoices].
	ChoicesFor func(ctx context.Context) ([]Choice, error)

	// Secret says the value is never read back: Get returns "", the form never echoes what was typed, and an empty
	// submission leaves what is stored as it is.
	Secret bool

	// Stored says whether a Secret has a value, for a model: the one thing about it that the page is told, so that it can say
	// there is one (as a row of bullets) without saying what it is. Nil for a field that is not a Secret.
	Stored func(model any) bool

	// Multiline says the text is of more than one line: a text area, and a cell that shows the first lines. A line break in
	// what is submitted is "\n", whatever the browser sent.
	Multiline bool

	// DateOnly says a moment is a date: the input picks a day, and not a time, and the day is read, shown and stored as midnight
	// UTC. It is set only for DisplayTime.
	DateOnly bool

	// Nullable says the value may be absent: Get is "" for none, Num is NaN, an emptied input is set to nothing and not left
	// as it was, and where it is only read it says so with a dash. A Bool, which is held as a select, is one.
	Nullable bool

	// Bool says the select holds a bool, which a file of it writes as true, false or null, and not as a list of words.
	Bool bool

	// Unset writes the zero value of the field's type to the model it is given, a pointer, for a field that has a
	// Set: what an input that was emptied means. Nil for a field that is read-only.
	Unset func(model any)

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

	// Step is a number that is one of a step from its minimum, such as a slider's, or 0 for any number.
	Step float64

	// Format is what kind of text a string is, which the browser draws an input of its own for and the server checks, or ""
	// for text of any kind.
	Format Format

	Pattern *Pattern
}

// Format is a kind of text that has an input of its own: its value is the type attribute of the input.
type Format string

// The formats.
const (
	FormatEmail Format = "email"
	FormatURL   Format = "url"
	FormatTel   Format = "tel"
	FormatColor Format = "color"
)

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

// ResolveChoices returns the fields with the choices of each select that reads them, as they are now. It copies what it
// changes, and returns fields itself when no field reads its choices, which is nearly always. A source that fails, or
// that names a value that is empty, holds the separator or is named twice, fails the request: it is the app's.
func ResolveChoices(ctx context.Context, fields []Field) ([]Field, error) {
	var out []Field
	for i, f := range fields {
		r, err := f.resolved(ctx)
		if err != nil {
			return nil, err
		}
		if r == nil {
			continue
		}
		if out == nil {
			out = append([]Field(nil), fields...)
		}
		out[i] = *r
	}
	if out == nil {
		return fields, nil
	}
	return out, nil
}

// resolved is the field with its choices read, or nil when it has nothing to read.
func (f Field) resolved(ctx context.Context) (*Field, error) {
	if len(f.Group) > 0 {
		group, err := ResolveChoices(ctx, f.Group)
		if err != nil || len(group) == 0 || &group[0] == &f.Group[0] {
			return nil, err
		}
		f.Group = group
		return &f, nil
	}
	if f.ChoicesFor == nil {
		return nil, nil
	}
	choices, err := f.ChoicesFor(ctx)
	if err != nil {
		return nil, fmt.Errorf("the options of %q: %w", f.Label, err)
	}
	seen := make(map[string]bool, len(choices))
	f.Choices, f.Options = choices, make([]string, 0, len(choices))
	for i, c := range choices {
		switch {
		case c.Value == "":
			return nil, fmt.Errorf("the options of %q: option %d has no Value", f.Label, i)
		case strings.Contains(c.Value, ListSep):
			return nil, fmt.Errorf("the options of %q: option %d, %q, has a control character in its Value", f.Label, i, c.Value)
		case seen[c.Value]:
			return nil, fmt.Errorf("the options of %q: %q is listed twice", f.Label, c.Value)
		}
		seen[c.Value] = true
		f.Options = append(f.Options, c.Value)
	}
	return &f, nil
}
