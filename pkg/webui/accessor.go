package webui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// Accessor is a named, typed projection of M. A table cell and a form input
// share the same accessor.
type Accessor[M any] interface {
	// label is the accessor's label, which names a column and resolves a FieldError. A Group has none.
	label() string
}

// String projects a string field of M.
type String[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model.
	Load func(M) string

	// Store writes the submitted value to the model. Without it the field is
	// read-only.
	Store func(*M, string)

	// Rules are checked in the browser and again on the server.
	Rules StringRules

	// Placeholder is shown in the input while it is empty.
	Placeholder string
}

func (a String[M]) label() string { return a.Label }

// StringRules is declarative validation for a string value.
// Scalar zero means "no constraint". Pattern is a pointer because the
// expression needs a user-facing message.
type StringRules struct {
	// Required refuses an empty value.
	Required bool

	// MinLen is the fewest characters, or zero for no minimum.
	MinLen int

	// MaxLen is the most characters, or zero for no maximum.
	MaxLen int

	// Pattern is a regular expression the whole value must match.
	Pattern *PatternRule
}

// PatternRule is an RE2 expression plus the message shown when it fails.
type PatternRule struct {
	// Expr is the expression, in RE2 syntax, so without lookahead or backreferences.
	// It must match the whole value.
	Expr string

	// Message is shown beneath the field when the value does not match. It says
	// what is wrong with the value, such as "must be a valid IPv4 address", and
	// only then: it is not shown beside a value that is accepted.
	Message string
}

// Int projects an int field of M.
type Int[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model.
	Load func(M) int

	// Store writes the submitted value to the model. Without it the field is
	// read-only.
	Store func(*M, int)

	// Rules are checked in the browser and again on the server.
	Rules NumberRules[int]

	// Placeholder is shown in the input while it is empty.
	Placeholder string

	// Format says how the value reads where it is only read: a table cell, and a form field
	// that has no Store. It is how a size is shown as "1.5 GiB" while the column still sorts,
	// filters and exports on the number: everything but what is shown keeps the value. A field
	// that can be edited holds the value in its input, so that what is typed is what is stored.
	// The search matches what Format shows as well as the value's own text. The filter of
	// a number is typed in the number, not in the text Format makes of it.
	//
	// For a size in bytes: func(n int) string { return humanBytes(n) }, where humanBytes is the
	// application's own: the library has no formats of its own.
	//
	// If Format panics, the panic is logged and the value is shown as it would be without
	// Format, so a bug in a formatter is a plain number and not a page that does not load.
	Format func(int) string
}

func (a Int[M]) label() string { return a.Label }

// NumberRules is declarative validation for a numeric value.
// Min and Max are pointers because a zero bound is a real constraint.
type NumberRules[T int | float64] struct {
	// Required refuses an empty value.
	Required bool

	// Min is the smallest allowed value, or nil for none.
	Min *T

	// Max is the largest allowed value, or nil for none.
	Max *T
}

// Float projects a float64 field of M. Precision is the number of decimals
// shown; zero shows the shortest exact representation.
type Float[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model.
	Load func(M) float64

	// Store writes the submitted value to the model. Without it the field is
	// read-only.
	Store func(*M, float64)

	// Rules are checked in the browser and again on the server.
	Rules NumberRules[float64]

	// Placeholder is shown in the input while it is empty.
	Placeholder string

	// Precision is the number of decimals shown. Zero shows the shortest exact
	// representation. Format, when set, shows the value instead.
	Precision int

	// Format says how the value reads where it is only read: a table cell, and a form field
	// that has no Store. It is how a size is shown as "1.5 GiB" while the column still sorts,
	// filters and exports on the number: everything but what is shown keeps the value. A field
	// that can be edited holds the value in its input, so that what is typed is what is stored.
	// The search matches what Format shows as well as the value's own text. The filter of
	// a number is typed in the number, not in the text Format makes of it.
	//
	// It is handed the value as it is, so a percentage held as 0.42 can read "42%".
	//
	// If Format panics, the panic is logged and the value is shown as it would be without
	// Format, so a bug in a formatter is a plain number and not a page that does not load.
	Format func(float64) string
}

func (a Float[M]) label() string { return a.Label }

// Group composes accessors inside a form. It is not a table column;
// Compile rejects Group listed in Columns.
type Group[M any] []Accessor[M]

func (Group[M]) label() string { return "" }

// Select is a choice from a list of options: the values of a field of M that holds some
// of a set of known words, such as a status, a size or a few tags. Read, in a table or in
// a form that is not editable, each value is a badge, in the [Tone] of its option. With a
// Store it is edited, as a drop-down list, or as a list to tick several of with Multi.
// Either way the badge or the option always carries its words, so the colour is never the only thing
// saying what state something is in.
//
// A Select holds a list of values, since some hold several, so Load returns a slice and Store is
// handed one: of at most one value unless Multi is set. A status that is one word is
// written []string{s.Health}.
//
// A Select does not need to know every value it can hold unless it is edited. How a table filters
// the column depends on where its rows come from:
//
//   - With Rows, the library holds every row, so the filter is a multi-select of the
//     values the rows hold, whether or not Options names them. A field with an open set
//     of values, such as a storage class, can leave Options empty, or name only the
//     values worth a colour.
//   - With Load, the source does the filtering, so the library can only offer what it
//     is told: Options is then the whole set, a multi-select of exactly those values.
//     List a neutral value too, as an Option with no Tone, for it to be filterable. With no
//     Options the column's filter is text.
type Select[M any] struct {
	// Label is the column header and the field's label. In a table it also names
	// the column in the address and in a [Query].
	Label string

	// Options are the values it can hold, in the order they are listed, with what each says
	// and the tone it is drawn in. They are required with a Store, since they are what the
	// visitor chooses from, and a value that is not among them is refused on the server.
	// See Select for what they mean to a table's filter.
	Options []Option

	// Load reads the values from the model. Without Multi it returns at most one; a select
	// that was handed several shows them all when it is read, and the first when it is edited.
	Load func(M) []string

	// Store writes the chosen values to the model, for a field that is edited: none, one, or
	// with Multi any number, each one of the Options, and without repeats. Without it the
	// field is read-only.
	Store func(*M, []string)

	// Multi lets the visitor choose any number of the options, as a list to tick, and not
	// one, as a drop-down list. The zero value is one.
	Multi bool

	// Required refuses a submission with nothing chosen. It is for a field that is edited.
	Required bool
}

func (a Select[M]) label() string { return a.Label }

// Option is one value a [Select] can hold.
type Option struct {
	// Value is what is stored: what Load returns and what Store is handed. It is not empty.
	Value string

	// Label is what the visitor reads, such as "Healthy" for the value "ok". The value
	// is shown when it is empty.
	Label string

	// Tone is how the value is drawn: neutral, in grey, by default.
	Tone Tone
}

// URL projects a string field of M as a link, which the page declares by using it:
// no other accessor ever makes a link of its value. It is read-only, in a table and
// in a form. The link opens in a new tab, with rel="noopener noreferrer", and is
// marked with an arrow, and screen readers are told it opens a new tab.
//
// The address may be a path on this site, or an http, https or mailto address. A
// value that is none of these, such as a script address, is shown as plain text and
// is never a link, so data in a model cannot make one. The first column of a table
// with a RowClick cannot be a URL, since the row's own link is there.
type URL[M any] struct {
	// Label is the column header and the field's label. In a table it also names
	// the column in the address and in a [Query].
	Label string

	// Load reads the address from the model. An empty string is no link.
	Load func(M) string

	// Text is what the link says, the same on every row, such as "Download". Leave
	// it empty to show the address itself.
	Text string
}

func (a URL[M]) label() string { return a.Label }

// Datetime projects a string field of M that holds an ISO 8601 moment, such as
// "2026-10-09T11:27:00Z" or "2026-10-09", as a date and time. It is shown in UTC as
// "2026-10-09 11:27" (a date alone as "2026-10-09"), and a table sorts and filters it
// as a moment and not as the text: the filter takes a start and an end, in UTC. A
// value with no zone is taken as UTC, and one that is not a moment at all is shown as
// written and sorts first.
//
// Without Store it is read-only. With Store a form shows a date and time picker, in
// UTC, and Store is handed the choice as RFC 3339 in UTC, "2026-10-09T11:27:00Z", or
// an empty string when the picker is cleared.
type Datetime[M any] struct {
	// Label is the column header and the field's label. In a table it also names
	// the column in the address and in a [Query].
	Label string

	// Load reads the ISO 8601 string from the model. An empty string is no value.
	Load func(M) string

	// Store writes the chosen moment to the model, as RFC 3339 in UTC. Without it the
	// field is read-only.
	Store func(*M, string)

	// Format says how the moment reads where it is only read, as [Int.Format] does for a
	// number: a relative "3 h ago", or another layout than the default. It is handed the
	// moment in UTC, and a date alone as midnight. The column still sorts and filters by the
	// moment, and its <time> element and the export still hold it. A value that is empty or is
	// not a moment is not handed to Format: it shows as it does without one.
	Format func(time.Time) string
}

func (a Datetime[M]) label() string { return a.Label }

// Timestamp projects an int field of M that holds a Unix time, in seconds, as a date
// and time. It is shown, sorted, filtered and, with a Store, picked as [Datetime] is.
// Zero is "not set": it shows as nothing, and clearing the picker stores it. For a
// time in milliseconds, divide it in Load and multiply it in Store.
type Timestamp[M any] struct {
	// Label is the column header and the field's label. In a table it also names
	// the column in the address and in a [Query].
	Label string

	// Load reads the Unix time, in seconds, from the model.
	Load func(M) int

	// Store writes the chosen moment to the model, as Unix seconds. Without it the
	// field is read-only.
	Store func(*M, int)

	// Format says how the moment reads where it is only read, as [Datetime.Format] does. It
	// is handed the moment in UTC, and is not handed zero, which is "not set" and shows as
	// nothing.
	Format func(time.Time) string
}

func (a Timestamp[M]) label() string { return a.Label }

// Slider projects a float64 field of M on a range from Min to Max (Max must be
// greater). Without Store it is a bar: in a table, and read-only in a form. With
// Store a form shows a range input, and the server rejects a value outside the
// range. Precision is the number of decimals shown; zero shows the shortest
// exact representation.
type Slider[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model.
	Load func(M) float64

	// Store writes the submitted value to the model. Without it the slider is a
	// bar, and read-only.
	Store func(*M, float64)

	// Min is the value at the start of the range.
	Min float64

	// Max is the value at the end of the range, and must be greater than Min.
	Max float64

	// Precision is the number of decimals shown. Zero shows the shortest exact
	// representation.
	Precision int
}

func (a Slider[M]) label() string { return a.Label }

// Tone is the semantic meaning of a badge.
type Tone string

// The tones. There are four because the design has a colour for each.
const (
	// ToneNeutral is a value with no particular meaning, in grey.
	ToneNeutral Tone = ""

	// ToneOK is healthy or done, in green.
	ToneOK Tone = "ok"

	// ToneWarning is degraded or needing a look, in amber.
	ToneWarning Tone = "warn"

	// ToneCritical is failed or urgent, in red.
	ToneCritical Tone = "bad"
)

// ASSERT: accessors implement Accessor
var (
	_ Accessor[struct{}] = String[struct{}]{}
	_ Accessor[struct{}] = Int[struct{}]{}
	_ Accessor[struct{}] = Float[struct{}]{}
	_ Accessor[struct{}] = Group[struct{}](nil)
	_ Accessor[struct{}] = Select[struct{}]{}
	_ Accessor[struct{}] = URL[struct{}]{}
	_ Accessor[struct{}] = Datetime[struct{}]{}
	_ Accessor[struct{}] = Timestamp[struct{}]{}
	_ Accessor[struct{}] = Slider[struct{}]{}
)

// ---------------------------------------------------------------- validate

// accessorSite says where an accessor list sits, since the same Accessor is
// legal in one and not the other.
type accessorSite struct {
	where      string // "Form.Fields" or "Table.Columns"
	allowGroup bool
	uniqueLbl  bool // a FieldError resolves by label, so a form needs them unique
}

func validateAccessors[M any](v *bodyValidator, accs []Accessor[M], site accessorSite) {
	seen := map[string]bool{}
	for i, acc := range accs {
		validateAccessor[M](v, acc, fmt.Sprintf("%s[%d]", site.where, i), site, seen)
	}
}

func validateAccessor[M any](v *bodyValidator, acc Accessor[M], at string, site accessorSite, seen map[string]bool) {
	// common is what every leaf needs: a label, unique where that matters, and a Load. sig is what
	// Load must be, for the message.
	common := func(l string, noLoad bool, sig string) {
		switch {
		case l == "":
			v.add(at+" has no Label", "Give the accessor a Label; it is the column header and the input label.")
		case site.uniqueLbl && seen[l]:
			v.add(fmt.Sprintf("%s: label %q is used twice", site.where, l),
				"Rename one; a FieldError resolves its accessor by Label, so duplicates are ambiguous.")
		default:
			seen[l] = true
		}
		if noLoad {
			v.add(at+" has no Load", "Set Load to "+sig+".")
		}
	}
	switch a := acc.(type) {
	case nil:
		v.add(at+" is nil", "Remove it or declare an accessor.")
	case String[M]:
		common(a.Label, a.Load == nil, "func(M) string")
		if p := a.Rules.Pattern; p != nil {
			if _, err := rules.Compile(p.Expr, p.Message); err != nil {
				fix := "Rewrite the pattern in RE2 syntax."
				var pe *rules.PatternError
				if errors.As(err, &pe) {
					fix = pe.Fix
				}
				v.add(fmt.Sprintf("%s: %v", at, err), fix)
			}
		}
		if a.Rules.MinLen < 0 || a.Rules.MaxLen < 0 || (a.Rules.MaxLen > 0 && a.Rules.MinLen > a.Rules.MaxLen) {
			v.add(at+" has inconsistent length rules", "MinLen and MaxLen must be non-negative with MinLen <= MaxLen.")
		}
	case Int[M]:
		common(a.Label, a.Load == nil, "func(M) int")
		validateBounds(v, at, a.Rules.Min, a.Rules.Max)
	case Float[M]:
		common(a.Label, a.Load == nil, "func(M) float64")
		validatePrecision(v, at, a.Precision)
		validateBounds(v, at, a.Rules.Min, a.Rules.Max)
	case Group[M]:
		if !site.allowGroup {
			v.add(at+" is a Group in "+site.where,
				"Groups compose form fields; a nested group has no meaning in a table cell. List the accessors directly.")
		}
		for i, child := range a {
			if _, nested := child.(Group[M]); nested {
				v.add(fmt.Sprintf("%s[%d] nests a Group in a Group", at, i), "Flatten it; groups are one level deep.")
				continue
			}
			validateAccessor[M](v, child, fmt.Sprintf("%s[%d]", at, i), site, seen)
		}
	case Select[M]:
		common(a.Label, a.Load == nil, "func(M) []string")
		validateSelect(v, at, a)
	case URL[M]:
		common(a.Label, a.Load == nil, "func(M) string, returning the address")
	case Datetime[M]:
		common(a.Label, a.Load == nil, "func(M) string, returning an ISO 8601 moment")
	case Timestamp[M]:
		common(a.Label, a.Load == nil, "func(M) int, returning a Unix time in seconds")
	case Slider[M]:
		common(a.Label, a.Load == nil, "func(M) float64")
		if !(a.Max > a.Min) {
			v.add(at+" has no range", "Set Max greater than Min; the slider is drawn between them.")
		}
		validatePrecision(v, at, a.Precision)
	default:
		v.add(fmt.Sprintf("%s has unsupported accessor type %T", at, acc),
			"Use String, Int, Float, Select, URL, Datetime, Timestamp, Slider or Group.")
	}
}

func validatePrecision(v *bodyValidator, at string, precision int) {
	if precision < 0 {
		v.add(at+" has a negative Precision", "Use 0 for the shortest exact form, or a number of decimals.")
	}
}

// validateSelect checks a select's options: each has a value, none twice, and one that could not be told
// from a list of several; and that one that is edited has some to choose from.
func validateSelect[M any](v *bodyValidator, at string, a Select[M]) {
	seen := map[string]bool{}
	for i, o := range a.Options {
		switch {
		case o.Value == "":
			v.add(fmt.Sprintf("%s.Options[%d] has no Value", at, i), "Set Value to what is stored for this option.")
		case strings.Contains(o.Value, ir.ListSep):
			v.add(fmt.Sprintf("%s.Options[%d] has a control character in its Value", at, i), "A value is a word: remove the character.")
		case seen[o.Value]:
			v.add(fmt.Sprintf("%s.Options[%d], %q, is listed twice", at, i, o.Value), "Remove one: a value is one option.")
		}
		seen[o.Value] = true
	}
	if a.Store != nil && len(a.Options) == 0 {
		v.add(at+" has a Store but no Options", "A select that is edited chooses from Options: list them, or remove Store to make it read-only.")
	}
	if a.Required && a.Store == nil {
		v.add(at+" is Required but has no Store", "Required is about a submission, and a read-only select has none: remove it, or add a Store.")
	}
}

func validateBounds[T int | float64](v *bodyValidator, at string, lo, hi *T) {
	if lo != nil && hi != nil && *lo > *hi {
		v.add(at+" has Min greater than Max", "Swap them, or drop one.")
	}
}

// ------------------------------------------------------------------- lower

func lowerAccessors[M any](accs []Accessor[M], prefix string) []ir.Field {
	out := make([]ir.Field, len(accs))
	for i, acc := range accs {
		out[i] = lowerAccessor[M](acc, prefix+strconv.Itoa(i))
	}
	return out
}

// lowerAccessor is the one place Store becomes Set. validate has run, so a nil
// Load or an unknown accessor cannot reach here.
func lowerAccessor[M any](acc Accessor[M], name string) ir.Field {
	switch a := acc.(type) {
	case String[M]:
		f := head(name, a.Label, ir.KindString)
		f.Rules, f.Placeholder = lowerStringRules(a.Rules), a.Placeholder
		f.Get = func(m any) string { return a.Load(m.(M)) }
		f.Set = setter(a.Store, func(raw string) (string, error) { return raw, nil })
		return f
	case Int[M]:
		f := head(name, a.Label, ir.KindInt)
		f.Rules, f.Placeholder = lowerNumberRules(a.Rules), a.Placeholder
		f.Get = func(m any) string { return strconv.Itoa(a.Load(m.(M))) }
		f.Num = func(m any) float64 { return float64(a.Load(m.(M))) }
		f.Show = shown(a.Label, f.Get, a.Format, func(m any) (int, bool) { return a.Load(m.(M)), true })
		f.Set = setter(a.Store, func(raw string) (int, error) { return strconv.Atoi(strings.TrimSpace(raw)) })
		return f
	case Float[M]:
		f := head(name, a.Label, ir.KindFloat)
		f.Rules, f.Placeholder = lowerNumberRules(a.Rules), a.Placeholder
		f.Get, f.Num = floatReads(a.Load, a.Precision)
		f.Show = shown(a.Label, f.Get, a.Format, func(m any) (float64, bool) { return a.Load(m.(M)), true })
		f.Set = setter(a.Store, parseFloat)
		return f
	case Group[M]:
		return ir.Field{Name: name, Group: lowerAccessors[M](a, name+"_")}
	case Select[M]:
		f := head(name, a.Label, ir.KindString)
		f.Display, f.Multi, f.Rules = ir.DisplaySelect, a.Multi, ir.Rules{Required: a.Required}
		for _, o := range a.Options {
			f.Choices = append(f.Choices, ir.Choice{Value: o.Value, Label: o.Label, Tone: ir.Tone(o.Tone)})
			f.Options = append(f.Options, o.Value)
		}
		f.Get = func(m any) string { return strings.Join(a.Load(m.(M)), ir.ListSep) }
		f.Set = setter(a.Store, func(raw string) ([]string, error) { return ir.SplitValues(raw), nil })
		return f
	case URL[M]:
		f := head(name, a.Label, ir.KindString)
		f.Display = ir.DisplayLink
		f.Get = func(m any) string {
			if a.Text != "" && a.Load(m.(M)) != "" {
				return a.Text
			}
			return a.Load(m.(M))
		}
		f.Link = func(m any) string {
			if u := a.Load(m.(M)); validURL(u) {
				return u
			}
			return ""
		}
		return f
	case Datetime[M]:
		f := head(name, a.Label, ir.KindString)
		f.Display = ir.DisplayTime
		f.Get = func(m any) string { return datetimeText(a.Load(m.(M))) }
		f.Num = func(m any) float64 { return datetimeNum(a.Load(m.(M))) }
		f.Show = shown(a.Label, f.Get, a.Format, func(m any) (time.Time, bool) {
			t, _, ok := readDatetime(a.Load(m.(M))) // not a moment: as written
			return t, ok
		})
		f.Set = setter(a.Store, func(raw string) (string, error) {
			t, blank, err := pickedMoment(raw)
			if err != nil || blank {
				return "", err // cleared: stored as nothing
			}
			return t.Format(time.RFC3339), nil
		})
		return f
	case Timestamp[M]:
		f := head(name, a.Label, ir.KindString)
		f.Display = ir.DisplayTime
		f.Get = func(m any) string { return timestampText(a.Load(m.(M))) }
		f.Num = func(m any) float64 { return timestampNum(a.Load(m.(M))) }
		f.Show = shown(a.Label, f.Get, a.Format, func(m any) (time.Time, bool) {
			seconds := a.Load(m.(M)) // zero is not set: shown as nothing
			return time.Unix(int64(seconds), 0).UTC(), seconds != 0
		})
		f.Set = setter(a.Store, func(raw string) (int, error) {
			t, blank, err := pickedMoment(raw)
			if err != nil || blank {
				return 0, err // cleared: stored as not set
			}
			return int(t.Unix()), nil
		})
		return f
	case Slider[M]:
		lo, hi := a.Min, a.Max
		f := head(name, a.Label, ir.KindFloat)
		f.Display, f.Min, f.Max, f.Rules = ir.DisplaySlider, lo, hi, ir.Rules{Min: &lo, Max: &hi}
		f.Get, f.Num = floatReads(a.Load, a.Precision)
		f.Set = setter(a.Store, parseFloat)
		return f
	}
	return ir.Field{Name: name}
}

// head is what every field starts with: its name, its label, the key the address names it by,
// and its kind.
func head(name, label string, kind ir.ValueKind) ir.Field {
	return ir.Field{Name: name, Label: label, Key: args.Slug(label), Kind: kind}
}

// setter is the Set of a field: the raw text is read as a T, then stored. A nil store makes no
// Set, which is what makes a field read-only. What went wrong in reading is not said: the form
// answers every unreadable value alike.
func setter[M, T any](store func(*M, T), parse func(string) (T, error)) func(any, string) error {
	if store == nil {
		return nil
	}
	return func(m any, raw string) error {
		v, err := parse(raw)
		if err != nil {
			return err
		}
		store(m.(*M), v)
		return nil
	}
}

func parseFloat(raw string) (float64, error) { return strconv.ParseFloat(strings.TrimSpace(raw), 64) }

// floatReads are the Get and Num of a float: the text, with Precision decimals or the shortest
// exact form for zero, and the number.
func floatReads[M any](load func(M) float64, precision int) (get func(any) string, num func(any) float64) {
	prec := -1
	if precision > 0 {
		prec = precision
	}
	return func(m any) string { return strconv.FormatFloat(load(m.(M)), 'f', prec, 64) },
		func(m any) float64 { return load(m.(M)) }
}

func lowerStringRules(r StringRules) ir.Rules {
	out := ir.Rules{Required: r.Required, MinLen: r.MinLen, MaxLen: r.MaxLen}
	if r.Pattern != nil {
		// Recompiled rather than threaded through from validate: it may repeat
		// work, but it never repeats a decision. validate proved it compiles.
		out.Pattern, _ = rules.Compile(r.Pattern.Expr, r.Pattern.Message)
	}
	return out
}

func lowerNumberRules[T int | float64](r NumberRules[T]) ir.Rules {
	out := ir.Rules{Required: r.Required}
	if r.Min != nil {
		f := float64(*r.Min)
		out.Min = &f
	}
	if r.Max != nil {
		f := float64(*r.Max)
		out.Max = &f
	}
	return out
}

// labelOf is the label of an accessor, for a FieldError and a column's name. A nil accessor and a
// Group have none.
func labelOf[M any](acc Accessor[M]) string {
	if acc == nil {
		return ""
	}
	return acc.label()
}

// shown is how a field reads where it is only read, from the application's Format, or nil when it
// has none. value reads the field's value in the form Format takes, and says whether there is
// one: a value that is not one is shown as it is without a Format. A Format that panics is logged,
// to the logger of the app that is answering, and the value is shown that way too, so a bug in a formatter is a plain value and not a page
// that cannot be drawn.
func shown[T any](label string, plain func(any) string, format func(T) string, value func(any) (T, bool)) func(context.Context, any) string {
	if format == nil {
		return nil
	}
	return func(ctx context.Context, m any) (text string) {
		defer func() {
			if r := recover(); r != nil {
				ir.LoggerFrom(ctx).Error("webui: a Format panicked, so the value is shown as it is", "column", label, "panic", r)
				text = plain(m)
			}
		}()
		if v, ok := value(m); ok {
			return format(v)
		}
		return plain(m)
	}
}
