package webui

import (
	"fmt"
	"slices"
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
	isAccessor()
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

func (String[M]) isAccessor() {}

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
}

func (Int[M]) isAccessor() {}

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
	// representation.
	Precision int
}

func (Float[M]) isAccessor() {}

// Group composes accessors inside a form. It is not a table column;
// Compile rejects Group listed in Columns.
type Group[M any] []Accessor[M]

func (Group[M]) isAccessor() {}

// Badge projects a string field of M as a badge: the value in a coloured pill.
// It is read-only, in a table and in a form. Kinds gives the tone of each value it
// knows; any other value is drawn neutral. The badge always carries its word, so the
// colour is never the only thing saying what state something is in.
//
// A badge does not need to know every value it can hold. How a table filters the
// column depends on where its rows come from:
//
//   - With Rows, the library holds every row, so the filter is a multi-select of the
//     values the rows hold, whether or not Kinds names them. A field with an open set
//     of values, such as a storage class, can leave Kinds empty, or name only the
//     values worth a colour.
//   - With Load, the source does the filtering, so the library can only offer what it
//     is told: Kinds is then the whole set, a multi-select of exactly those values.
//     List a neutral value too, as ToneNeutral, for it to be filterable. With no Kinds
//     the column's filter is text.
type Badge[M any] struct {
	// Label is the column header and the field's label. In a table it also names
	// the column in the address and in a [Query].
	Label string

	// Load reads the value from the model.
	Load func(M) string

	// Kinds gives the [Tone] each known value is drawn in. See Badge for what it
	// means to a table's filter.
	Kinds map[string]Tone
}

func (Badge[M]) isAccessor() {}

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
}

func (Datetime[M]) isAccessor() {}

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
}

func (Timestamp[M]) isAccessor() {}

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

func (Slider[M]) isAccessor() {}

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
	_ Accessor[struct{}] = Badge[struct{}]{}
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

func validateAccessors[M any](v *bodyValidator, accs []Accessor[M], site accessorSite, seen map[string]bool) {
	for i, acc := range accs {
		validateAccessor[M](v, acc, fmt.Sprintf("%s[%d]", site.where, i), site, seen)
	}
}

func validateAccessor[M any](v *bodyValidator, acc Accessor[M], at string, site accessorSite, seen map[string]bool) {
	label := func(l string) {
		switch {
		case l == "":
			v.add(at+" has no Label", "Give the accessor a Label; it is the column header and the input label.")
		case site.uniqueLbl && seen[l]:
			v.add(fmt.Sprintf("%s: label %q is used twice", site.where, l),
				"Rename one; a FieldError resolves its accessor by Label, so duplicates are ambiguous.")
		default:
			seen[l] = true
		}
	}
	switch a := acc.(type) {
	case nil:
		v.add(at+" is nil", "Remove it or declare an accessor.")
	case String[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) string.")
		}
		if p := a.Rules.Pattern; p != nil {
			if _, err := rules.Compile(p.Expr, p.Message); err != nil {
				v.add(fmt.Sprintf("%s: %v", at, err),
					"Rewrite the pattern in RE2 syntax; Go does not support lookahead or backreferences.")
			}
		}
		if a.Rules.MinLen < 0 || a.Rules.MaxLen < 0 || (a.Rules.MaxLen > 0 && a.Rules.MinLen > a.Rules.MaxLen) {
			v.add(at+" has inconsistent length rules", "MinLen and MaxLen must be non-negative with MinLen <= MaxLen.")
		}
	case Int[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) int.")
		}
		validateBounds(v, at, a.Rules.Min, a.Rules.Max)
	case Float[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) float64.")
		}
		if a.Precision < 0 {
			v.add(at+" has a negative Precision", "Use 0 for the shortest exact form, or a number of decimals.")
		}
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
	case Badge[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) string.")
		}
	case Datetime[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) string, returning an ISO 8601 moment.")
		}
	case Timestamp[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) int, returning a Unix time in seconds.")
		}
	case Slider[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) float64.")
		}
		if !(a.Max > a.Min) {
			v.add(at+" has no range", "Set Max greater than Min; the slider is drawn between them.")
		}
		if a.Precision < 0 {
			v.add(at+" has a negative Precision", "Use 0 for the shortest exact form, or a number of decimals.")
		}
	default:
		v.add(fmt.Sprintf("%s has unsupported accessor type %T", at, acc),
			"Use String, Int, Float, Badge, Datetime, Timestamp, Slider or Group.")
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
		f := ir.Field{
			Name: name, Label: a.Label, Key: args.Slug(a.Label), Kind: ir.KindString, Rules: lowerStringRules(a.Rules), Placeholder: a.Placeholder,
			Get: func(m any) string { return a.Load(m.(M)) },
		}
		if a.Store != nil {
			f.Set = func(m any, raw string) error { a.Store(m.(*M), raw); return nil }
		}
		return f
	case Int[M]:
		f := ir.Field{
			Name: name, Label: a.Label, Key: args.Slug(a.Label), Kind: ir.KindInt, Rules: lowerNumberRules(a.Rules), Placeholder: a.Placeholder,
			Get: func(m any) string { return strconv.Itoa(a.Load(m.(M))) },
			Num: func(m any) float64 { return float64(a.Load(m.(M))) },
		}
		if a.Store != nil {
			f.Set = func(m any, raw string) error {
				n, err := strconv.Atoi(strings.TrimSpace(raw))
				if err != nil {
					return fmt.Errorf("%w", err)
				}
				a.Store(m.(*M), n)
				return nil
			}
		}
		return f
	case Float[M]:
		prec := -1
		if a.Precision > 0 {
			prec = a.Precision
		}
		f := ir.Field{
			Name: name, Label: a.Label, Key: args.Slug(a.Label), Kind: ir.KindFloat, Rules: lowerNumberRules(a.Rules), Placeholder: a.Placeholder,
			Get: func(m any) string { return strconv.FormatFloat(a.Load(m.(M)), 'f', prec, 64) },
			Num: func(m any) float64 { return a.Load(m.(M)) },
		}
		if a.Store != nil {
			f.Set = func(m any, raw string) error {
				x, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
				a.Store(m.(*M), x)
				return nil
			}
		}
		return f
	case Group[M]:
		return ir.Field{Name: name, Group: lowerAccessors[M](a, name+"_")}
	case Badge[M]:
		kinds := make(map[string]ir.Tone, len(a.Kinds))
		var options []string
		for k, t := range a.Kinds {
			kinds[k] = ir.Tone(t)
			options = append(options, k)
		}
		slices.Sort(options) // the filter lists them in a fixed order
		return ir.Field{
			Name: name, Label: a.Label, Key: args.Slug(a.Label), Kind: ir.KindString, Display: ir.DisplayBadge, Kinds: kinds, Options: options,
			Get: func(m any) string { return a.Load(m.(M)) },
		}
	case Datetime[M]:
		f := ir.Field{
			Name: name, Label: a.Label, Key: args.Slug(a.Label), Kind: ir.KindString, Display: ir.DisplayTime,
			Get: func(m any) string { return datetimeText(a.Load(m.(M))) },
			Num: func(m any) float64 { return datetimeNum(a.Load(m.(M))) },
		}
		if a.Store != nil {
			f.Set = func(m any, raw string) error {
				t, blank, err := pickedMoment(raw)
				if err != nil {
					return err
				}
				if blank {
					a.Store(m.(*M), "")
					return nil
				}
				a.Store(m.(*M), t.Format(time.RFC3339))
				return nil
			}
		}
		return f
	case Timestamp[M]:
		f := ir.Field{
			Name: name, Label: a.Label, Key: args.Slug(a.Label), Kind: ir.KindString, Display: ir.DisplayTime,
			Get: func(m any) string { return timestampText(a.Load(m.(M))) },
			Num: func(m any) float64 { return float64(a.Load(m.(M))) },
		}
		if a.Store != nil {
			f.Set = func(m any, raw string) error {
				t, blank, err := pickedMoment(raw)
				if err != nil {
					return err
				}
				if blank {
					a.Store(m.(*M), 0)
					return nil
				}
				a.Store(m.(*M), int(t.Unix()))
				return nil
			}
		}
		return f
	case Slider[M]:
		prec := -1
		if a.Precision > 0 {
			prec = a.Precision
		}
		lo, hi := a.Min, a.Max
		f := ir.Field{
			Name: name, Label: a.Label, Key: args.Slug(a.Label), Kind: ir.KindFloat, Display: ir.DisplaySlider, Min: lo, Max: hi,
			Rules: ir.Rules{Min: &lo, Max: &hi},
			Get:   func(m any) string { return strconv.FormatFloat(a.Load(m.(M)), 'f', prec, 64) },
			Num:   func(m any) float64 { return a.Load(m.(M)) },
		}
		if a.Store != nil {
			f.Set = func(m any, raw string) error {
				x, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
				a.Store(m.(*M), x)
				return nil
			}
		}
		return f
	}
	return ir.Field{Name: name}
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

// accessorLabel resolves an accessor to its label, for FieldError. A Group has no
// label of its own.
func accessorLabel[M any](acc Accessor[M]) string {
	switch a := acc.(type) {
	case String[M]:
		return a.Label
	case Int[M]:
		return a.Label
	case Float[M]:
		return a.Label
	case Badge[M]:
		return a.Label
	case Datetime[M]:
		return a.Label
	case Timestamp[M]:
		return a.Label
	case Slider[M]:
		return a.Label
	}
	return ""
}
