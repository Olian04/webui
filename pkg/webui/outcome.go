package webui

import (
	"fmt"

	"github.com/Olian04/webui/internal/ir"
)

// Outcome is how an action ended, in terms of what the user is told and where
// they are left. It is what Action.Run returns beside an error, and it is built
// with one of Success, Warning, Failure and Reject: say what happened, and the
// library decides the rest (the toast and its colour, whether the form is shown
// again, the status code). The zero Outcome is a quiet success: accepted, and no
// message.
//
// An Outcome is either accepted or not.
//
//   - Success and Warning are accepted. The action was submitted and done, and the
//     user is sent back to where they were (or on, with Then). They differ only in
//     the message: a Warning says something the user should be aware of.
//   - Failure and Reject are not accepted. Nothing was done, and the form is shown
//     again with what the user typed kept, so they can fix it and submit again.
//     They differ in what they say: a Failure says it for the whole form, and a
//     Reject says it for particular fields.
//
// Run's error return is for something nobody can fix by editing the form, such as
// a database being down: it is logged and the user sees only that something went
// wrong. Anything the user can act on is an Outcome.
type Outcome struct {
	kind     outcomeKind
	message  string
	fields   rejection
	redirect Target
}

type outcomeKind uint8

const (
	kindSuccess outcomeKind = iota
	kindWarning
	kindFailure
	kindReject
)

// rejection is the field messages of a Reject with the model erased, so Outcome
// stays non-generic.
type rejection interface {
	isRejection()
}

// Success reports that the action was submitted and accepted. message is shown to
// the user once, in a confirming toast; empty shows nothing.
//
//	return webui.Success("Device saved"), nil
func Success(message string) Outcome {
	return Outcome{kind: kindSuccess, message: message}
}

// Warning reports that the action was submitted and accepted, like Success, but
// that the user should be aware of something: it was only partly done, or what it
// did has a consequence they may not expect. message is shown in a warning toast,
// and says what to be aware of.
//
//	return webui.Warning("Saved, but 3 devices still use the old address"), nil
func Warning(message string) Outcome {
	return Outcome{kind: kindWarning, message: message}
}

// Failure reports that the action could not be done, for a reason the user can
// act on and that does not belong to one field: the device could not be reached,
// the limit was reached, the change conflicts with another. Nothing was done. The
// form is shown again with what the user typed kept, and message is shown in an
// error toast. For a table's row or bulk action, which has no form to show again,
// the page is shown again with the error toast.
//
// A Failure and a Reject are both not accepted, and both keep what was typed. A
// Failure is about the whole form; a Reject is about particular fields.
//
//	return webui.Failure("Could not reach the device, try again"), nil
func Failure(message string) Outcome {
	return Outcome{kind: kindFailure, message: message}
}

// Reject reports that the action was refused because of particular fields, and
// says what is wrong with each, with Field. Nothing was done. The form is shown
// again with what the user typed kept, and each message is shown beside its field,
// as if the browser's own rules had caught it. Use it for what a rule cannot know,
// such as a value another record already has.
//
// A Reject and a Failure are both not accepted, and both keep what was typed. A
// Reject is about particular fields; a Failure is about the whole form. A field
// that the form does not show is a mistake in the declaration and is reported as
// an error. Reject is for forms: a table action has no fields, so there it is shown
// as a Failure with the messages joined.
//
//	return webui.Reject(
//	    webui.Field[Device](IP, "already in use by another device"),
//	    webui.Field[Device](Site, "no such site"),
//	), nil
func Reject[M any](errs ...FieldError[M]) Outcome {
	return Outcome{kind: kindReject, fields: rejected[M](errs)}
}

// FieldError is one accessor's message in a Reject. Build one with Field.
type FieldError[M any] struct {
	Field   Accessor[M]
	Message string
}

// Field is a message about one accessor, for Reject. The accessor must be one of
// the form's fields, found by its Label, and is given as the type argument because
// Go cannot infer the model from an accessor.
func Field[M any](field Accessor[M], message string) FieldError[M] {
	return FieldError[M]{Field: field, Message: message}
}

// Then sends the user on after an accepted outcome, to a page built with Open, in
// place of leaving them where they were. It applies to Success and Warning: a
// Failure or a Reject shows the form again, so there is nowhere to send them, and
// Then leaves those unchanged.
//
//	return webui.Success("Device deleted").Then(webui.Open(ctx, DevicesPath, webui.NoArgs{})), nil
func (o Outcome) Then(to Target) Outcome {
	if o.kind == kindSuccess || o.kind == kindWarning {
		o.redirect = to
	}
	return o
}

type rejected[M any] []FieldError[M]

func (rejected[M]) isRejection() {}

// ASSERT: rejected implements rejection
var _ rejection = rejected[struct{}](nil)

// lowerOutcome converts the request-time Outcome. A redirect whose Open failed,
// or a Reject about a different model, are bugs in the user's Run, reported as an
// error rather than silently dropped.
func lowerOutcome[M any](o Outcome) (ir.Outcome, error) {
	out := ir.Outcome{Kind: ir.OutcomeKind(o.kind), Message: o.message}
	if o.redirect.Err != nil {
		return ir.Outcome{}, fmt.Errorf("webui: Outcome.Then: %w", o.redirect.Err)
	}
	out.Redirect = o.redirect.URL
	switch f := o.fields.(type) {
	case nil:
	case rejected[M]:
		for _, fe := range f {
			out.Fields = append(out.Fields, ir.FieldError{Label: accessorLabel[M](fe.Field), Message: fe.Message})
		}
	default:
		return ir.Outcome{}, fmt.Errorf("webui: Reject is about %T, not the action's model", o.fields)
	}
	return out, nil
}
