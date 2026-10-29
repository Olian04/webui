package webui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

// OptionalInt projects an *int field of M: a whole number that may be absent. Absent is not zero: a nil value reads as empty (a dash
// in a table), an input that is emptied stores nil, and a column sorts it before every number and leaves it out of a filter
// by range. Otherwise it is an [Int], and has what an Int has.
type OptionalInt[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model: nil when there is none.
	Load func(M) *int

	// Store writes the submitted value to the model: nil when the input was left empty. Without it the field
	// is read-only.
	Store func(*M, *int)

	// Rules are checked in the browser and again on the server. Required refuses an empty input, which is what makes
	// a value that must be there.
	Rules NumberRules[int]

	// Placeholder is shown in the input while it is empty.
	Placeholder string

	// Format says how a value reads where it is only read, as [Int.Format] does. It is not called for an absent one.
	Format func(int) string

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and
	// its rules cannot: it is in the popover beside the label that lists the rules, under a heading of its own and above
	// them, on hover and on keyboard focus, and is part of the icon's accessible name. It is shown where the field is, in a
	// form or a toolbar, read-only too. In a table it is not shown, as a placeholder is not. It is text, not markup.
	Help string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a OptionalInt[M]) label() string { return a.Label }

func (a OptionalInt[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// OptionalFloat projects a *float64 field of M: a number that may be absent, as [OptionalInt] is a whole number that may.
type OptionalFloat[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model: nil when there is none.
	Load func(M) *float64

	// Store writes the submitted value to the model: nil when the input was left empty. Without it the field
	// is read-only.
	Store func(*M, *float64)

	// Rules are checked in the browser and again on the server.
	Rules NumberRules[float64]

	// Precision is the number of decimals shown, as for a [Float].
	Precision int

	// Placeholder is shown in the input while it is empty.
	Placeholder string

	// Format says how a value reads where it is only read, as [Float.Format] does. It is not called for an absent one.
	Format func(float64) string

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and
	// its rules cannot: it is in the popover beside the label that lists the rules, under a heading of its own and above
	// them, on hover and on keyboard focus, and is part of the icon's accessible name. It is shown where the field is, in a
	// form or a toolbar, read-only too. In a table it is not shown, as a placeholder is not. It is text, not markup.
	Help string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a OptionalFloat[M]) label() string { return a.Label }

func (a OptionalFloat[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// OptionalSlider projects a *float64 field of M on a range, as a [Slider] does, for a value that may be absent: a bar that is not
// drawn, a dash in its place, and a number input that is empty and stores nil when it is left so.
type OptionalSlider[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model: nil when there is none.
	Load func(M) *float64

	// Store writes the submitted value to the model: nil when it was left empty. Without it the slider is a bar,
	// and read-only.
	Store func(*M, *float64)

	// Min is the value at the start of the range.
	Min float64

	// Max is the value at the end of the range, and must be greater than Min.
	Max float64

	// StepSize is the distance between the values the slider holds, as for a [Slider].
	StepSize float64

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and
	// its rules cannot: it is in the popover beside the label that lists the rules, under a heading of its own and above
	// them, on hover and on keyboard focus, and is part of the icon's accessible name. It is shown where the field is, in a
	// form or a toolbar, read-only too. In a table it is not shown, as a placeholder is not. It is text, not markup.
	Help string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a OptionalSlider[M]) label() string { return a.Label }

func (a OptionalSlider[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// OptionalBool projects a *bool field of M: yes, no, or not set, as [OptionalInt] is a whole number that may be absent. A checkbox cannot say "not set", and its unchecked
// state is not sent at all, so it is a choice of Yes and No with an empty one, as a [Select] is, and read it is a Yes or a No in a badge. Like a
// Select, its filter in a table is a choice among the values. A value that is always one or the other is a [Bool].
type OptionalBool[M any] struct {
	// Label is the column header and the input's label. In a table it also names the
	// column in the address and in a [Query].
	Label string

	// Load reads the value from the model: nil when it is not set.
	Load func(M) *bool

	// Store writes the choice to the model: nil when none was made. Without it the field is read-only.
	Store func(*M, *bool)

	// Required refuses a submission with none chosen.
	Required bool

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and
	// its rules cannot: it is in the popover beside the label that lists the rules, under a heading of its own and above
	// them, on hover and on keyboard focus, and is part of the icon's accessible name. It is shown where the field is, in a
	// form or a toolbar, read-only too. In a table it is not shown, as a placeholder is not. It is text, not markup.
	Help string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a OptionalBool[M]) label() string { return a.Label }

func (a OptionalBool[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// validateOptional checks what an optional number needs besides what every accessor does: Required needs a Store, since an
// input that cannot be filled in cannot be required.
func validateOptional(v *bodyValidator, at string, required bool, store bool) {
	if required && !store {
		v.add(at+" is Required but has no Store", "Required is about a submission, and a read-only field has none: remove it, or add a Store.")
	}
}

// lowerOptionalInt is an OptionalInt as the IR holds it.
func lowerOptionalInt[M any](a OptionalInt[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindInt)
	f.Nullable = true
	f.Rules, f.Placeholder = lowerNumberRules(a.Rules), a.Placeholder
	f.Get = func(m any) string {
		if p := a.Load(m.(M)); p != nil {
			return strconv.Itoa(*p)
		}
		return ""
	}
	f.Num = func(m any) float64 {
		if p := a.Load(m.(M)); p != nil {
			return float64(*p)
		}
		return math.NaN() // before every number, and in no range
	}
	f.Show = shown(a.Label, f.Get, a.Format, func(m any) (int, bool) {
		if p := a.Load(m.(M)); p != nil {
			return *p, true
		}
		return 0, false
	})
	f.Set = setter(a.Store, func(raw string) (*int, error) {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		return &n, nil
	})
	f.Unset = unsetter(a.Store)
	return f
}

// lowerOptionalFloat is an OptionalFloat as the IR holds it.
func lowerOptionalFloat[M any](a OptionalFloat[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindFloat)
	f.Nullable = true
	f.Rules, f.Placeholder = lowerNumberRules(a.Rules), a.Placeholder
	prec := -1
	if a.Precision > 0 {
		prec = a.Precision
	}
	f.Get = func(m any) string {
		if p := a.Load(m.(M)); p != nil {
			return strconv.FormatFloat(*p, 'f', prec, 64)
		}
		return ""
	}
	f.Num = func(m any) float64 {
		if p := a.Load(m.(M)); p != nil {
			return *p
		}
		return math.NaN()
	}
	f.Show = shown(a.Label, f.Get, a.Format, func(m any) (float64, bool) {
		if p := a.Load(m.(M)); p != nil {
			return *p, true
		}
		return 0, false
	})
	f.Set = setter(a.Store, func(raw string) (*float64, error) {
		x, err := parseFloat(raw)
		if err != nil {
			return nil, err
		}
		return &x, nil
	})
	f.Unset = unsetter(a.Store)
	return f
}

// lowerOptionalSlider is an OptionalSlider as the IR holds it: a Slider whose value may be absent.
func lowerOptionalSlider[M any](a OptionalSlider[M], name string) ir.Field {
	lo, hi := a.Min, a.Max
	f := head(name, a.Label, ir.KindFloat)
	f.Nullable = true
	f.Placeholder = "—" // what the number says while it is empty, and what the range is dimmed by
	f.Display, f.Min, f.Max, f.Rules = ir.DisplaySlider, lo, hi, ir.Rules{Min: &lo, Max: &hi}
	step := a.StepSize
	if step == 0 && a.Store != nil {
		step = defaultStep(lo, hi)
	}
	// What a slider with a value reads is what the Slider's own reads, and an absent one reads as nothing.
	present, num := sliderReads(func(m M) float64 {
		if p := a.Load(m); p != nil {
			return *p
		}
		return 0
	}, lo, step)
	f.Get = func(m any) string {
		if a.Load(m.(M)) == nil {
			return ""
		}
		return present(m)
	}
	f.Num = func(m any) float64 {
		if a.Load(m.(M)) == nil {
			return math.NaN()
		}
		return num(m)
	}
	if a.Store != nil {
		f.Rules.Step = step
	}
	f.Set = setter(a.Store, func(raw string) (*float64, error) {
		x, err := parseFloat(raw)
		if err != nil {
			return nil, err
		}
		return &x, nil
	})
	f.Unset = unsetter(a.Store)
	return f
}

// The two values of a Bool, as a select holds them.
const (
	boolYes = "true"
	boolNo  = "false"
)

// lowerBool is a Bool as the IR holds it: the same Yes and No that an OptionalBool has, read, filtered and written to a file as it is, and drawn as a checkbox where it is edited, whose
// absence is no.
func lowerBool[M any](a Bool[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindString)
	f.Display, f.Bool, f.Checkbox = ir.DisplaySelect, true, true
	f.Choices = []ir.Choice{{Value: boolYes, Label: "Yes"}, {Value: boolNo, Label: "No"}}
	f.Options = []string{boolYes, boolNo}
	f.Get = func(m any) string {
		if a.Load(m.(M)) {
			return boolYes
		}
		return boolNo
	}
	f.Set = setter(a.Store, func(raw string) (bool, error) {
		switch raw {
		case boolYes:
			return true, nil
		case "", boolNo:
			return false, nil // an unchecked checkbox is not sent
		}
		return false, fmt.Errorf("not a choice: %q", raw)
	})
	return f
}

// lowerOptionalBool is an OptionalBool as the IR holds it: a select of Yes and No, whose value is "true", "false" or none.
func lowerOptionalBool[M any](a OptionalBool[M], name string) ir.Field {
	f := head(name, a.Label, ir.KindString)
	f.Display, f.Rules, f.Nullable, f.Bool = ir.DisplaySelect, ir.Rules{Required: a.Required}, true, true
	f.Choices = []ir.Choice{{Value: boolYes, Label: "Yes"}, {Value: boolNo, Label: "No"}}
	f.Options = []string{boolYes, boolNo}
	f.Get = func(m any) string {
		switch p := a.Load(m.(M)); {
		case p == nil:
			return ""
		case *p:
			return boolYes
		}
		return boolNo
	}
	f.Set = setter(a.Store, func(raw string) (*bool, error) {
		switch raw {
		case "":
			return nil, nil
		case boolYes:
			t := true
			return &t, nil
		case boolNo:
			no := false
			return &no, nil
		}
		return nil, fmt.Errorf("not a choice: %q", raw)
	})
	f.Unset = unsetter(a.Store)
	return f
}
