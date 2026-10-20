package render

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
	"github.com/Olian04/webui/internal/rules"
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

	// Refusal is what the Load said the visitor may be told, when it was a refusal and not a failure.
	Refusal *ir.Refusal
}

// Editable is whether the visitor can change the text of a code editor: it has a Store,
// and they may submit. A viewer, or an editor whose guard refuses them, is read.
func (v FormView) Editable() bool {
	return len(v.Node.Fields) > 0 && v.editable(v.Node.Fields[0])
}

// storedBullets is what a secret that has a value shows in its empty input: the same row of bullets whatever the value is, so
// that neither the value nor its length is in the page.
const storedBullets = "••••••••"

// fieldView is one input with everything the component needs resolved.
type fieldView struct {
	ID        string
	Name      string // empty for a read-only input: it is never submitted
	Field     ir.Field
	Value     string
	Error     string
	RuleLines []string
	Model     any // what the form loaded, which a link or a <time> element of the field reads
	ReadOnly  bool
	Type      string
	Rules     c.Rules

	// FormID is the form the control belongs to, when it is not the one it is inside of.
	FormID string

	// Placeholder is what the input shows while it is empty: the field's own, or for a secret that has a value the bullets.
	Placeholder string

	// Autocomplete is what the browser may fill the control in with: "new-password" for a secret, so it does not offer a
	// password that it has saved.
	Autocomplete string

	// Validity, Parts and Title are what an editable field tells the browser to say when it is refused: see [c.InputProps].
	Validity string
	Parts    string
	Title    string
}

// editable is whether the visitor can change a field: it has a Store, and the
// form can be submitted by them. A form with no Submit, or whose Submit is refused
// to the visitor, is only read.
func (v FormView) editable(f ir.Field) bool {
	return f.Set != nil && v.Node.Submit != nil && v.SubmitGate == ""
}

func (r *Renderer) fieldView(ctx context.Context, v FormView, f ir.Field) fieldView {
	editable := v.editable(f)
	fv := fieldView{
		ID:        fieldID(v.Node.At, f.Name),
		Name:      f.Name,
		Field:     f,
		Error:     v.Errors[f.Label],
		RuleLines: ruleLines(f, editable),
		Model:     v.Model,
		ReadOnly:  !editable,
		Type:      "text",
		Rules:     htmlRules(f, editable),
	}
	fv.Validity, fv.Parts, fv.Title = validity(f, editable)
	if raw, ok := v.Values[f.Name]; ok {
		fv.Value = raw
	} else if f.Get != nil {
		fv.Value = f.Get(v.Model)
		if !editable && f.Show != nil {
			fv.Value = f.Shown(ctx, v.Model) // only read: as a person reads it, and not as it is held
		}
	}
	if fv.ReadOnly {
		fv.Name = ""
	}
	fv.Placeholder = f.Placeholder
	if f.Secret {
		fv.Value = "" // not even what was typed, in the form that answers a refusal
		if f.Stored != nil && f.Stored(v.Model) {
			fv.Placeholder = storedBullets // that there is one, and not what it is, nor how long
		}
	}
	switch {
	case fv.ReadOnly:
		// A number input shows its value in the viewer's locale ("0,67"), which is
		// right for typing and wrong for reading. A read-only field is only read.
	case f.Display == ir.DisplayTime:
		// A date and time picker, whose value is the moment in UTC to the second, not
		// the text a table shows. What was typed, after a rejection, stays as it was.
		fv.Type, fv.Rules.Step = "datetime-local", "1"
		if f.DateOnly {
			fv.Type, fv.Rules.Step = "date", ""
		}
		if _, typed := v.Values[f.Name]; !typed {
			fv.Value = momentInput(f, v.Model)
		}
	case f.Secret:
		fv.Type, fv.Autocomplete = "password", "new-password"
	case f.Rules.Format != "":
		fv.Type = string(f.Rules.Format)
		if f.Rules.Format == ir.FormatColor && fv.Value == "" {
			fv.Value = "#000000" // a picker cannot be empty: a field with no color is black
		}
	case f.Kind == ir.KindInt:
		fv.Type, fv.Rules.Step = "number", "1"
	case f.Display == ir.DisplaySlider:
		// What the range beside it moves in, and what the server holds a value to.
		fv.Type, fv.Rules.Step = "number", num(f.Rules.Step)
	case f.Kind == ir.KindFloat:
		fv.Type, fv.Rules.Step = "number", "any"
	}
	return fv
}

// choices are the options of an editable select, each with whether the field holds it: from what was
// typed, after a rejection, or from the model. A value the field holds that is not an option
// is not offered, and is dropped when it is saved.
func (fv fieldView) choices() []c.SelectChoice {
	held := ir.SplitValues(fv.Value)
	if !fv.Field.Multi && len(held) > 1 {
		held = held[:1] // a single select shows the first
	}
	out := make([]c.SelectChoice, len(fv.Field.Choices))
	for i, ch := range fv.Field.Choices {
		out[i] = c.SelectChoice{Value: ch.Value, Label: fv.Field.LabelOf(ch.Value), Selected: slices.Contains(held, ch.Value), Tone: c.Tone(fv.Field.ToneOf(ch.Value))}
	}
	return out
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
	if f.DateOnly {
		return time.Unix(int64(seconds), 0).UTC().Format("2006-01-02")
	}
	return time.Unix(int64(seconds), 0).UTC().Format("2006-01-02T15:04:05")
}

// fieldID is unique in the document: a page may hold several forms.
func fieldID(at ir.Addr, name string) string {
	return strings.ReplaceAll(LeafID(at), ".", "-") + "-" + name
}

// validity is the words that a field is refused in, for the browser to say, and what a matching value looks like: the
// same as the server says beneath it (see [rules.Validity]), so a value that the browser refuses reads as one that the
// server would. A read-only field has none, and a moment is only ever missing.
//
// A field that has more than one condition on what the value is, the text rules and the pattern, has them as parts too: a
// pattern attribute is one expression, so the browser cannot say which of them a value fails, and script, given each
// with the words that the server says for it, finds the first that fails and says that one.
func validity(f ir.Field, editable bool) (messages, parts, title string) {
	if !editable {
		return "", "", ""
	}
	kind := f.Kind
	if f.Display == ir.DisplayTime || f.Display == ir.DisplaySelect {
		kind = ir.KindString
	}
	m := rules.Validity(f.Rules, kind)
	if f.Display == ir.DisplayTime || f.Display == ir.DisplaySelect {
		m = map[string]string{"valueMissing": m["valueMissing"]}
		if m["valueMissing"] == "" {
			m = nil
		}
	}
	if len(m) > 0 {
		if b, err := json.Marshal(m); err == nil {
			messages = string(b)
		}
	}
	text := kind == ir.KindString && f.Display != ir.DisplayTime && f.Display != ir.DisplaySelect
	if ps := rules.Parts(f.Rules); text && rules.HasText(f.Rules) {
		msgs := make([]string, len(ps))
		for i, p := range ps {
			msgs[i] = p.Message
		}
		title = strings.Join(msgs, " ")
		if len(ps) > 1 {
			pairs := make([][2]string, len(ps))
			for i, p := range ps {
				pairs[i] = [2]string{p.Expr, p.Message}
			}
			if b, err := json.Marshal(pairs); err == nil {
				parts = string(b)
			}
		}
	} else if p := f.Rules.Pattern; p != nil {
		title = p.Message
	}
	return messages, parts, title
}

// htmlRules is the one place ir.Rules becomes HTML constraint attributes. A
// read-only field carries none: it is never submitted, so a rule on it could
// only block the form.
func htmlRules(f ir.Field, editable bool) c.Rules {
	if !editable {
		return c.Rules{}
	}
	out := c.Rules{Required: f.Rules.Required, MinLen: f.Rules.MinLen, MaxLen: f.Rules.MaxLen, Min: f.Rules.Min, Max: f.Rules.Max}
	out.Pattern = rules.BrowserPattern(f.Rules)
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
		if f.DateOnly {
			out[len(out)-1] = "Date in UTC"
		}
	}
	if f.Rules.Format == ir.FormatURL {
		out = append(out, "A web address, http or https")
	}
	if f.Display == ir.DisplaySelect && f.Multi {
		out = append(out, "Hold Ctrl or ⌘ to choose more than one")
	}
	if f.Secret {
		out = append(out, "Leave empty to keep the current value")
	}
	for _, s := range []string{lengthRule(f.Rules), boundsRule(f.Rules), stepRule(f)} {
		if s != "" {
			out = append(out, s)
		}
	}
	out = append(out, rules.TextLines(f.Rules)...)
	if p := f.Rules.Pattern; p != nil && p.Message != "" {
		out = append(out, p.Message)
	}
	return out
}

// stepRule is what a slider moves in, when that is not already clear from the bounds.
func stepRule(f ir.Field) string {
	if f.Display != ir.DisplaySlider || f.Rules.Step <= 0 {
		return ""
	}
	return "In steps of " + num(f.Rules.Step)
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
