package webui

import (
	"errors"
	"net/http"

	"github.com/Olian04/webui/internal/ir"
)

// Refusal is an error that the visitor is told of, with a status and a way on, in place of "Could not load" or a bare 403. A page's
// Guard returns one to say that there is no such record (404), that the visitor must sign in (401), or that they may not (403), and
// the page answers with that status and says it, with a link on from there if the Refusal has one:
//
//	Guard: func(ctx context.Context, a DeviceArgs) error {
//		if _, ok := service.Device(a.ID); !ok {
//			return webui.NotFound("There is no device "+a.ID).WithLink("Back to the devices", webui.Open(ctx, DevicesPath, webui.NoArgs{}))
//		}
//		return nil
//	}
//
// The page's Guard runs before anything is loaded, so its status is the page's. A panel's Load may fail with one too ([Failed] of it), to say
// that what it shows is not there: the panel says it, with its link, and the page stays as it is, since a page that has begun to be sent
// has its status already. Any other error from either is as it was: a Guard's is a 403 with its text, and a Load's [Failed] is "Could not
// load", with the cause logged and not shown. A Refusal is for what a visitor may be told, and is not logged.
type Refusal struct {
	status  int
	message string
	label   string
	to      Target
}

// NotFound is a Refusal that says there is nothing at the address: a 404, with the message for the visitor.
func NotFound(message string) Refusal { return Refusal{status: http.StatusNotFound, message: message} }

// Unauthorized is a Refusal that says the visitor is not signed in, or not as someone who may: a 401, with the message
// for the visitor. A link to where they sign in, or to the page that sets up what is missing, goes with [Refusal.WithLink].
func Unauthorized(message string) Refusal {
	return Refusal{status: http.StatusUnauthorized, message: message}
}

// Forbidden is a Refusal that says the visitor may not: a 403, with the message for the visitor. It is what a Guard that
// returns any error already answers, with its text.
func Forbidden(message string) Refusal {
	return Refusal{status: http.StatusForbidden, message: message}
}

// WithLink adds a link to the Refusal, with the words it says and where it leads, which [Open] builds. The visitor follows it
// if they choose to: unlike [Outcome.Then], which takes the user to another page when an action has ended, nothing here
// redirects. A destination that Open could not resolve leaves the Refusal without a link.
func (r Refusal) WithLink(label string, to Target) Refusal {
	r.label, r.to = label, to
	return r
}

// Error is the message for the visitor.
func (r Refusal) Error() string { return r.message }

// refused is err as the IR holds a Refusal when it is, or err.
func refused(err error) error {
	var r Refusal
	if !errors.As(err, &r) {
		return err
	}
	out := &ir.Refusal{Status: r.status, Message: r.message}
	if r.to.err == nil && r.to.url != "" && r.label != "" {
		out.Label, out.Href = r.label, r.to.url
	}
	return out
}
