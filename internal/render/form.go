package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

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

	// SubmitGate is why the viewer may not submit, "" when they may. A viewer who
	// may not submit is offered a Back button in place of Save and Cancel.
	SubmitGate string

	// CancelHref is where Cancel, or Back, goes, "" for neither.
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
	RuleLines   []string
	Placeholder string
	Href        string // the address of a link field, "" for any other
	ReadOnly    bool
	Type        string
	Rules       c.Rules
}

// editable is whether the visitor can change a field: it has a Store, and the
// form can be submitted by them. A form with no Submit, or whose Submit is refused
// to the visitor, is only read.
func (v FormView) editable(f ir.Field) bool {
	return f.Set != nil && v.Node.Submit != nil && v.SubmitGate == ""
}

func (r *Renderer) fieldView(v FormView, f ir.Field) fieldView {
	editable := v.editable(f)
	fv := fieldView{
		ID:          fieldID(v.Node.At, f.Name),
		Name:        f.Name,
		Field:       f,
		Error:       v.Errors[f.Label],
		RuleLines:   ruleLines(f, editable),
		Placeholder: f.Placeholder,
		ReadOnly:    !editable,
		Type:        "text",
		Rules:       htmlRules(f, editable),
	}
	if raw, ok := v.Values[f.Name]; ok {
		fv.Value = raw
	} else if f.Get != nil {
		fv.Value = f.Get(v.Model)
	}
	if fv.ReadOnly {
		fv.Name = ""
	}
	fv.Href = linkOf(f, v.Model)
	// A number input shows its value in the viewer's locale ("0,67"), which is
	// right for typing and wrong for reading. A read-only field is only read.
	if !fv.ReadOnly && f.Display == ir.DisplayTime {
		// A date and time picker, whose value is the moment in UTC to the second, not
		// the text a table shows. What was typed, after a rejection, stays as it was.
		fv.Type, fv.Rules.Step = "datetime-local", "1"
		if _, typed := v.Values[f.Name]; !typed {
			fv.Value = momentInput(f, v.Model)
		}
	} else if !fv.ReadOnly {
		switch f.Kind {
		case ir.KindInt:
			fv.Type, fv.Rules.Step = "number", "1"
		case ir.KindFloat:
			fv.Type, fv.Rules.Step = "number", "any"
		}
	}
	return fv
}

// momentInput is a moment as a date and time input holds it, in UTC, or "" when the
// field has none.
func momentInput(f ir.Field, model any) string {
	if f.Get(model) == "" {
		return ""
	}
	seconds := f.Num(model)
	if math.IsInf(seconds, 0) || math.IsNaN(seconds) {
		return ""
	}
	return time.Unix(int64(seconds), 0).UTC().Format("2006-01-02T15:04:05")
}

// fieldID is unique in the document: a page may hold several forms.
func fieldID(at ir.Addr, name string) string {
	return strings.ReplaceAll(LeafID(at), ".", "-") + "-" + name
}

// htmlRules is the one place ir.Rules becomes HTML constraint attributes. A
// read-only field carries none: it is never submitted, so a rule on it could
// only block the form.
func htmlRules(f ir.Field, editable bool) c.Rules {
	if !editable {
		return c.Rules{}
	}
	out := c.Rules{Required: f.Rules.Required, MinLen: f.Rules.MinLen, MaxLen: f.Rules.MaxLen, Min: f.Rules.Min, Max: f.Rules.Max}
	if f.Rules.Pattern != nil {
		out.Pattern = f.Rules.Pattern.Source
	}
	return out
}

// ruleLines is every constraint on an editable field, in words: what the
// information icon beside its label lists. A pattern is there by its message, which
// says what a matching value is, and which the field shows otherwise only when a
// value is refused. Required is not among them:
// the asterisk beside the label says it.
func ruleLines(f ir.Field, editable bool) []string {
	if !editable {
		return nil
	}
	var out []string
	if f.Display == ir.DisplayTime {
		out = append(out, "Date and time in UTC")
	}
	for _, s := range []string{lengthRule(f.Rules), boundsRule(f.Rules)} {
		if s != "" {
			out = append(out, s)
		}
	}
	if p := f.Rules.Pattern; p != nil && p.Message != "" {
		out = append(out, p.Message)
	}
	return out
}

func lengthRule(r ir.Rules) string {
	switch {
	case r.MinLen > 0 && r.MaxLen > 0:
		return fmt.Sprintf("%d–%d characters", r.MinLen, r.MaxLen)
	case r.MinLen > 0:
		return fmt.Sprintf("At least %d characters", r.MinLen)
	case r.MaxLen > 0:
		return fmt.Sprintf("Up to %d characters", r.MaxLen)
	}
	return ""
}

func boundsRule(r ir.Rules) string {
	switch {
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
