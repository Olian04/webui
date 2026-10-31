package webui

import (
	"context"
	"strings"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// Form is a leaf that loads one model M, shows it as fields, and submits it through
// an [Action]. After a submit that was accepted the user returns to the page they came
// from; one that was not shows the form again with what they typed.
//
// A Form with no Submit is the library's detail view: a panel of labelled values,
// each quiet label over its value, with no inputs and no buttons. Use it to show one
// thing, such as an object's properties; its fields can be any accessor, including a
// [Select], a [Datetime] and a [Timestamp], and a [Link] or an [Action] elsewhere on the
// page leads on from it.
type Form[M any] struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside
	// the title.
	Desc string

	// Load returns the model the form shows. On submit it is loaded again, and the
	// submitted values are applied to it, so a field with no Store keeps its value.
	//
	// If Load returns [Failed] the panel says "Could not load", the cause is logged
	// and the rest of the page stands. The error's text is not shown to the visitor,
	// since it may name things they should not see. A form that shows something that is
	// still changing returns [Loaded.RefreshAfter].
	Load func(ctx context.Context) (M, Loaded)

	// Submit is what the form's button does. Without a Run the form is read-only, and
	// so it is for a visitor its Guard refuses: every field is shown but not editable,
	// and a Back button replaces Save and Cancel.
	// Its Label defaults to "Save".
	Submit Action[M]

	// Fields are the form's inputs, in order. An accessor with a Store is an
	// input, and without one a read-only value. A [Group] puts fields side by side.
	Fields []Accessor[M]
}

// ASSERT: Form implements PageBody
var _ PageBody = Form[struct{}]{}

func (f Form[M]) validateBody(v *bodyValidator) {
	needLoad(v, "a Form", f.Load != nil)
	validateAccessors(v, f.Fields, accessorSite{where: "Form.Fields", allowGroup: true, uniqueLbl: true})
	if f.Submit.Run != nil {
		validateAction(v, "Form.Submit", f.Submit, false)
	} else if f.Submit.Guard != nil || f.Submit.Label != "" {
		v.add("Form.Submit has a Guard or Label but no Run", "Set Run, or remove the Submit.")
	}
}

func (f Form[M]) lowerBody(at ir.Addr) ir.Node {
	fields := lowerAccessors(f.Fields, "f")
	out := &ir.Form{
		At: at, Title: f.Title, Desc: f.Desc, Fields: fields,
		Load: eraseLoad(f.Load),
		Bind: bindFunc[M],
	}
	if f.Submit.Run != nil {
		a := f.Submit
		if a.Label == "" {
			a.Label = "Save"
		}
		if a.Role == RoleSecondary {
			a.Role = RolePrimary // the one primary action of a form, by position
		}
		out.Submit = lowerAction(a)
	}
	return out
}

// bindFunc applies a submission to the fields as they are for the request, with the choices of its selects read. It
// starts from a copy of the loaded model, checks every writable field's rules
// (parse → rules), applies each value that passed, and collects every failure.
func bindFunc[M any](fields []ir.Field, base any, values map[string]string, files *ir.Files) (any, []ir.FieldError) {
	m := base.(M)
	var errs []ir.FieldError
	var stores []func() // the files, which are stored when nothing in the submission is refused
	for f := range ir.Leaves(fields) {
		if f.File != nil {
			if f.File.Store != nil {
				if msg, store := bindFile(f, base, files); msg != "" {
					errs = append(errs, ir.FieldError{Label: f.Label, Message: msg})
				} else if store != nil {
					stores = append(stores, func() { store(&m) })
				}
			}
			continue
		}
		if f.Set == nil {
			continue
		}
		raw := values[f.Name]
		if f.Multiline {
			// A browser sends a line break of a text area as CRLF, which counts as two characters where its own
			// maxlength counts one: the text is what it reads as, and is stored so.
			raw = strings.ReplaceAll(raw, "\r\n", "\n")
		}
		if f.Display == ir.DisplaySelect {
			if msg := rules.CheckSelect(f, raw); msg != "" {
				errs = append(errs, ir.FieldError{Label: f.Label, Message: msg})
				continue
			}
		}
		if msg := rules.Check(f.Rules, f.Kind, raw); msg != "" {
			errs = append(errs, ir.FieldError{Label: f.Label, Message: msg})
			continue
		}
		if raw == "" && f.Kind != ir.KindString {
			if f.Unset != nil {
				f.Unset(&m) // an input that was emptied is the default of its type: nil for a field that may have none, the zero value for any other
			}
			continue // empty: there is nothing to parse
		}
		if err := f.Set(&m, raw); err != nil {
			errs = append(errs, ir.FieldError{Label: f.Label, Message: "Could not read this value."})
		}
	}
	if len(errs) == 0 {
		for _, store := range stores {
			store()
		}
	}
	return m, errs
}

// bindFile is what a submission does to a file field: the words for a file that was refused as it arrived, or what to do to the model, which
// is nothing when the visitor chose no file and asked for nothing. A file that is chosen replaces one that is asked to be removed.
func bindFile(f ir.Field, base any, files *ir.Files) (message string, store func(model any)) {
	var up *ir.Upload
	removed := false
	if files != nil {
		if msg := files.Refused[f.Name]; msg != "" {
			return msg, nil
		}
		up, removed = files.Uploads[f.Name], files.Removed[f.Name] && f.File.Removable
	}
	switch {
	case up != nil:
		return "", func(m any) { f.File.Store(m, up) }
	case removed:
		if f.Rules.Required {
			return rules.FileRequired(), nil
		}
		return "", func(m any) { f.File.Store(m, nil) }
	case f.Rules.Required && f.File.Stored(base) == nil:
		return rules.FileRequired(), nil
	}
	return "", nil
}
