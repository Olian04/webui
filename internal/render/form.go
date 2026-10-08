package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// FormView is a form with its model loaded. The runtime fills it.
type FormView struct {
	Node *ir.Form
	Page *ir.Page

	// Path and Query are the page's current arguments, so a submission returns
	// to the address it came from.
	Path, Query map[string]string

	Model any

	// Values echoes what the user typed, by Field.Name, after a rejected
	// submission. A field with no entry shows the model's value. The echo is
	// raw input, not the model: otherwise the bad value disappears and the
	// form looks like it reset.
	Values map[string]string

	// Errors is the message for each rejected field, by label.
	Errors map[string]string

	// SubmitGate is why the viewer may not submit, "" when they may.
	SubmitGate string

	// CancelHref is where Cancel goes, "" for no Cancel.
	CancelHref string

	Failed bool // Load failed: the panel says so, the page stands
}

func (v FormView) title() string { return v.Node.Title }

func (v FormView) status() c.Tone {
	if v.Failed {
		return c.ToneCritical
	}
	return c.ToneNeutral
}

// fieldView is one input with everything the component needs resolved.
type fieldView struct {
	ID          string
	Name        string // empty for a read-only input: it is never submitted
	Field       ir.Field
	Value       string
	Error       string
	Hint        string
	Placeholder string
	ReadOnly    bool
	Type        string
	Rules       c.Rules
}

func (r *Renderer) fieldView(v FormView, f ir.Field) fieldView {
	fv := fieldView{
		ID:          fieldID(v.Node.At, f.Name),
		Name:        f.Name,
		Field:       f,
		Error:       v.Errors[f.Label],
		Hint:        hint(f),
		Placeholder: f.Placeholder,
		ReadOnly:    f.Set == nil,
		Type:        "text",
		Rules:       htmlRules(f),
	}
	if raw, ok := v.Values[f.Name]; ok {
		fv.Value = raw
	} else if f.Get != nil {
		fv.Value = f.Get(v.Model)
	}
	if fv.ReadOnly {
		fv.Name = ""
	}
	// A number input shows its value in the viewer's locale ("0,67"), which is
	// right for typing and wrong for reading. A read-only field is only read.
	if !fv.ReadOnly {
		switch f.Kind {
		case ir.KindInt:
			fv.Type, fv.Rules.Step = "number", "1"
		case ir.KindFloat:
			fv.Type, fv.Rules.Step = "number", "any"
		}
	}
	return fv
}

// fieldID is unique in the document: a page may hold several forms.
func fieldID(at ir.Addr, name string) string {
	return strings.ReplaceAll(LeafID(at), ".", "-") + "-" + name
}

// htmlRules is the one place ir.Rules becomes HTML constraint attributes. A
// read-only field carries none: it is never submitted, so a rule on it could
// only block the form.
func htmlRules(f ir.Field) c.Rules {
	if f.Set == nil {
		return c.Rules{}
	}
	out := c.Rules{Required: f.Rules.Required, MinLen: f.Rules.MinLen, MaxLen: f.Rules.MaxLen, Min: f.Rules.Min, Max: f.Rules.Max}
	if f.Rules.Pattern != nil {
		out.Pattern = f.Rules.Pattern.Source
	}
	return out
}

// hint is the line beneath an input that states the rule. A pattern speaks in
// its own words, because a regular expression cannot explain itself.
func hint(f ir.Field) string {
	r := f.Rules
	switch {
	case f.Set == nil:
		return ""
	case r.Pattern != nil && r.Pattern.Message != "":
		return r.Pattern.Message
	case r.MinLen > 0 && r.MaxLen > 0:
		return fmt.Sprintf("%d–%d characters", r.MinLen, r.MaxLen)
	case r.MinLen > 0:
		return fmt.Sprintf("At least %d characters", r.MinLen)
	case r.MaxLen > 0:
		return fmt.Sprintf("Up to %d characters", r.MaxLen)
	case r.Min != nil && r.Max != nil:
		return fmt.Sprintf("%s–%s", num(*r.Min), num(*r.Max))
	case r.Min != nil:
		return "At least " + num(*r.Min)
	case r.Max != nil:
		return "At most " + num(*r.Max)
	}
	return ""
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func (v FormView) submitVariant() c.Variant {
	switch v.Node.Submit.Role {
	case ir.RoleDestructive:
		return c.VariantDanger
	case ir.RoleSecondary:
		return c.VariantSecondary
	}
	return c.VariantPrimary
}
