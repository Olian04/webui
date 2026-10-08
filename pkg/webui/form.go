package webui

import (
	"context"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// Form is a leaf that loads one M and submits through Action. A Form whose
// Submit has no Run is read-only: it has no submit button.
type Form[M any] struct {
	Title  string
	Desc   string
	Load   func(ctx context.Context) (M, error)
	Submit Action[M]
	Fields []Accessor[M]
}

func (Form[M]) isPageBody() {}

// ASSERT: Form implements PageBody
var _ PageBody = Form[struct{}]{}

func (f Form[M]) validateBody(v *bodyValidator) {
	if f.Load == nil {
		v.add("a Form has no Load", "Set Load to func(ctx) (M, error).")
	}
	validateAccessors(v, f.Fields, accessorSite{where: "Form.Fields", allowGroup: true, uniqueLbl: true}, map[string]bool{})
	if f.Submit.Run != nil {
		validateAction(v, "Form.Submit", f.Submit.Label, true, false)
	} else if f.Submit.Guard != nil || f.Submit.Label != "" {
		v.add("Form.Submit has a Guard or Label but no Run", "Set Run, or remove the Submit.")
	}
}

func (f Form[M]) lowerBody(at ir.Addr) ir.Node {
	fields := lowerAccessors(f.Fields, "f")
	out := &ir.Form{
		At: at, Title: f.Title, Desc: f.Desc, Fields: fields,
		Load: func(ctx context.Context) (any, error) { return f.Load(ctx) },
		Bind: bindFunc[M](fields),
	}
	if f.Submit.Run != nil {
		label := f.Submit.Label
		if label == "" {
			label = "Save"
		}
		role := f.Submit.Role
		if role == RoleSecondary {
			role = RolePrimary // the one primary action of a form, by position
		}
		a := f.Submit
		a.Role = role
		out.Submit = lowerAction(a, label)
	}
	return out
}

// bindFunc builds, once, the function that applies a submission. It starts
// from a copy of the loaded model, checks every writable field's rules
// (parse → rules), applies each value that passed, and collects every failure.
func bindFunc[M any](fields []ir.Field) func(any, map[string]string) (any, []ir.FieldError) {
	var writable []ir.Field
	var walk func([]ir.Field)
	walk = func(fs []ir.Field) {
		for _, f := range fs {
			if len(f.Group) > 0 {
				walk(f.Group)
			} else if f.Set != nil {
				writable = append(writable, f)
			}
		}
	}
	walk(fields)

	return func(base any, values map[string]string) (any, []ir.FieldError) {
		m := base.(M)
		var errs []ir.FieldError
		for _, f := range writable {
			raw := values[f.Name]
			if v := rules.Check(f.Rules, f.Kind, raw); v != nil {
				errs = append(errs, ir.FieldError{Label: f.Label, Message: v.Message})
				continue
			}
			if raw == "" && f.Kind != ir.KindString {
				continue // optional and empty: leave the loaded value alone
			}
			if err := f.Set(&m, raw); err != nil {
				errs = append(errs, ir.FieldError{Label: f.Label, Message: "Could not read this value."})
			}
		}
		return m, errs
	}
}
