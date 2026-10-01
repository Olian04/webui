// Package components is the webui design language, expressed as templ
// components. It implements docs/design.md and nothing else.
//
// Conventions, which are the whole of the API design:
//
//   - A component takes one props struct, named <Component>Props, whose ZERO
//     VALUE is the default. Go has no optional arguments; a props struct is the
//     closest thing, and it lets fields be added without touching call sites.
//   - Choices that will grow are typed string constants, never booleans. Tone
//     can gain a sixth meaning; `isWarning bool` cannot.
//   - The main content of a component is { children... }. Anything else that
//     takes markup is a templ.Component field — a slot. Make the common
//     configurable; make the uncommon composable.
//   - Every component carries Attrs templ.Attributes. That escape hatch is why
//     this package does not need to grow a prop every time a caller needs an
//     id, a data- attribute or an hx- attribute.
//   - Components render. They do not load, decide policy, or know what a page
//     is. Nothing here imports internal/ir: the mapping from IR to props
//     belongs to internal/render, so the design language stays usable and
//     testable on its own.
//   - Controls that navigate are links. Button takes an Href and renders <a>.
//     This is not a convenience; it is what makes the UI work without script.
package components

// Tone is the semantic meaning a component carries. There are five, matching
// the five in the design language. A sixth would mean the design has grown a
// meaning it has no colour for.
type Tone string

const (
	ToneNeutral  Tone = ""
	TonePrimary  Tone = "primary"
	ToneOK       Tone = "ok"
	ToneWarning  Tone = "warn"
	ToneCritical Tone = "bad"
)

func (t Tone) badge() string {
	switch t {
	case TonePrimary, ToneOK:
		return "badge-ok"
	case ToneWarning:
		return "badge-warn"
	case ToneCritical:
		return "badge-bad"
	case ToneNeutral:
		return "badge-mute"
	}
	return "badge-mute"
}

// Size is the control height. Medium is 32px, Small is 28px.
type Size string

const (
	SizeMedium Size = ""
	SizeSmall  Size = "sm"
)

func (s Size) btn() string {
	if s == SizeSmall {
		return "btn-sm"
	}
	return ""
}

// Variant is a button's role. Secondary is the zero value because it is the
// one that is always safe: a page may have no primary action, but it may not
// have two.
type Variant string

const (
	VariantSecondary Variant = ""
	VariantPrimary   Variant = "primary"
	VariantGhost     Variant = "ghost"
	VariantDanger    Variant = "danger"
)

func (v Variant) btn() string {
	switch v {
	case VariantPrimary:
		return "btn-primary"
	case VariantGhost:
		return "btn-ghost"
	case VariantDanger:
		return "btn-danger"
	case VariantSecondary:
		return "btn-secondary"
	}
	return "btn-secondary"
}

// Align is a table column's alignment. Numbers go to the end, with tabular
// figures; everything else goes to the start.
type Align string

const (
	AlignStart Align = ""
	AlignEnd   Align = "num"
)

func (a Align) cell() string {
	if a == AlignEnd {
		return "num"
	}
	return ""
}

// SortDir is set on the one column a table is sorted by.
type SortDir string

const (
	SortNone SortDir = ""
	SortAsc  SortDir = "asc"
	SortDesc SortDir = "desc"
)
