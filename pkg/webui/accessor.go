package webui

import (
	"fmt"
	"strconv"
	"strings"

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
	Label string
	Load  func(M) string
	Store func(*M, string)
	Rules StringRules
}

func (String[M]) isAccessor() {}

// StringRules is declarative validation for a string value.
// Scalar zero means "no constraint". Pattern is a pointer because the
// expression needs a user-facing message.
type StringRules struct {
	Required bool
	MinLen   int
	MaxLen   int
	Pattern  *PatternRule
}

// PatternRule is an RE2 expression plus the message shown when it fails.
type PatternRule struct {
	Expr    string
	Message string
}

// Int projects an int field of M.
type Int[M any] struct {
	Label string
	Load  func(M) int
	Store func(*M, int)
	Rules NumberRules[int]
}

func (Int[M]) isAccessor() {}

// NumberRules is declarative validation for a numeric value.
// Min and Max are pointers because a zero bound is a real constraint.
type NumberRules[T int | float64] struct {
	Required bool
	Min      *T
	Max      *T
}

// Float projects a float64 field of M. Precision is the number of decimals
// shown; zero shows the shortest exact representation.
type Float[M any] struct {
	Label     string
	Load      func(M) float64
	Store     func(*M, float64)
	Rules     NumberRules[float64]
	Precision int
}

func (Float[M]) isAccessor() {}

// Group composes accessors inside a form. It is not a table column;
// Compile rejects Group listed in Columns.
type Group[M any] []Accessor[M]

func (Group[M]) isAccessor() {}

// Sortable decorates an accessor with the query key used for sorting.
type Sortable[M any] struct {
	Accessor Accessor[M]
	Key      string
}

func (Sortable[M]) isAccessor() {}

// Placeholder decorates an accessor with placeholder text.
type Placeholder[M any] struct {
	Accessor Accessor[M]
	Text     string
}

func (Placeholder[M]) isAccessor() {}

// Badge projects a string field of M as a badge: the value in a coloured pill.
// It is read-only, in a table and in a form. Tones maps a value to its tone;
// a value not in the map is neutral. The badge always carries its word, so the
// colour is never the only thing saying what state something is in.
type Badge[M any] struct {
	Label string
	Load  func(M) string
	Tones map[string]Tone
}

func (Badge[M]) isAccessor() {}

// Slider projects a float64 field of M on a range from Min to Max (Max must be
// greater). Without Store it is a bar: in a table, and read-only in a form. With
// Store a form shows a range input, and the server rejects a value outside the
// range. Precision is the number of decimals shown; zero shows the shortest
// exact representation.
type Slider[M any] struct {
	Label     string
	Load      func(M) float64
	Store     func(*M, float64)
	Min, Max  float64
	Precision int
}

func (Slider[M]) isAccessor() {}

// Tone is the semantic meaning of a badge.
type Tone string

// The tones. There are four because the design has a colour for each.
const (
	ToneNeutral  Tone = ""
	ToneOK       Tone = "ok"
	ToneWarning  Tone = "warn"
	ToneCritical Tone = "bad"
)

// ASSERT: accessors implement Accessor
var (
	_ Accessor[struct{}] = String[struct{}]{}
	_ Accessor[struct{}] = Int[struct{}]{}
	_ Accessor[struct{}] = Float[struct{}]{}
	_ Accessor[struct{}] = Group[struct{}](nil)
	_ Accessor[struct{}] = Sortable[struct{}]{}
	_ Accessor[struct{}] = Placeholder[struct{}]{}
	_ Accessor[struct{}] = Badge[struct{}]{}
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
		validateAccessor[M](v, acc, fmt.Sprintf("%s[%d]", site.where, i), site, seen, false)
	}
}

func validateAccessor[M any](v *bodyValidator, acc Accessor[M], at string, site accessorSite, seen map[string]bool, decorated bool) {
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
		switch {
		case !site.allowGroup:
			v.add(at+" is a Group in "+site.where,
				"Groups compose form fields; a nested group has no meaning in a table cell. List the accessors directly.")
		case decorated:
			v.add(at+" decorates a Group", "Decorate the accessors inside the Group instead.")
		}
		for i, child := range a {
			if _, nested := child.(Group[M]); nested {
				v.add(fmt.Sprintf("%s[%d] nests a Group in a Group", at, i), "Flatten it; groups are one level deep.")
				continue
			}
			validateAccessor[M](v, child, fmt.Sprintf("%s[%d]", at, i), site, seen, false)
		}
	case Sortable[M]:
		if a.Key == "" {
			v.add(at+" Sortable has no Key", "Set Key to the value the Load func receives in Query.Sort.")
		}
		validateAccessor[M](v, a.Accessor, at, site, seen, true)
	case Placeholder[M]:
		validateAccessor[M](v, a.Accessor, at, site, seen, true)
	case Badge[M]:
		label(a.Label)
		if a.Load == nil {
			v.add(at+" has no Load", "Set Load to func(M) string.")
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
			"Use String, Int, Float, Badge, Slider, Group, Sortable or Placeholder.")
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

// lowerAccessor is the one place Store becomes Set. Decorators flatten into
// the inner field; they have no runtime existence. validate has run, so a nil
// Load or an unknown accessor cannot reach here.
func lowerAccessor[M any](acc Accessor[M], name string) ir.Field {
	switch a := acc.(type) {
	case String[M]:
		f := ir.Field{
			Name: name, Label: a.Label, Kind: ir.KindString, Rules: lowerStringRules(a.Rules),
			Get: func(m any) string { return a.Load(m.(M)) },
		}
		if a.Store != nil {
			f.Set = func(m any, raw string) error { a.Store(m.(*M), raw); return nil }
		}
		return f
	case Int[M]:
		f := ir.Field{
			Name: name, Label: a.Label, Kind: ir.KindInt, Rules: lowerNumberRules(a.Rules),
			Get: func(m any) string { return strconv.Itoa(a.Load(m.(M))) },
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
			Name: name, Label: a.Label, Kind: ir.KindFloat, Rules: lowerNumberRules(a.Rules),
			Get: func(m any) string { return strconv.FormatFloat(a.Load(m.(M)), 'f', prec, 64) },
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
	case Sortable[M]:
		f := lowerAccessor[M](a.Accessor, name)
		f.SortKey = a.Key
		return f
	case Placeholder[M]:
		f := lowerAccessor[M](a.Accessor, name)
		f.Placeholder = a.Text
		return f
	case Badge[M]:
		tones := make(map[string]ir.Tone, len(a.Tones))
		for k, t := range a.Tones {
			tones[k] = ir.Tone(t)
		}
		return ir.Field{
			Name: name, Label: a.Label, Kind: ir.KindString, Display: ir.DisplayBadge, Tones: tones,
			Get: func(m any) string { return a.Load(m.(M)) },
		}
	case Slider[M]:
		prec := -1
		if a.Precision > 0 {
			prec = a.Precision
		}
		lo, hi := a.Min, a.Max
		f := ir.Field{
			Name: name, Label: a.Label, Kind: ir.KindFloat, Display: ir.DisplaySlider, Min: lo, Max: hi,
			Rules: ir.Rules{Min: &lo, Max: &hi},
			Get:   func(m any) string { return strconv.FormatFloat(a.Load(m.(M)), 'f', prec, 64) },
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

// accessorLabel resolves an accessor to its label through decorators, for
// FieldError. A Group has no label of its own.
func accessorLabel[M any](acc Accessor[M]) string {
	switch a := acc.(type) {
	case String[M]:
		return a.Label
	case Int[M]:
		return a.Label
	case Float[M]:
		return a.Label
	case Sortable[M]:
		return accessorLabel[M](a.Accessor)
	case Placeholder[M]:
		return accessorLabel[M](a.Accessor)
	case Badge[M]:
		return a.Label
	case Slider[M]:
		return a.Label
	}
	return ""
}
