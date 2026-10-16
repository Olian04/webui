package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// counted is a form whose Run counts how often it ran and answers with what the IP asks for.
func counted(runs *atomic.Int32) http.Handler {
	run := func(_ context.Context, d Device) (webui.Outcome, error) {
		runs.Add(1)
		switch d.Ip {
		case "10.0.0.3":
			return webui.Failure("Could not reach the device"), nil
		case "10.0.0.4":
			return webui.Reject(webui.Field[Device](formIP, "taken")), nil
		case "10.0.0.9":
			return webui.Outcome{}, errors.New("connection refused to db.internal:5432")
		}
		return webui.Success("Saved"), nil
	}
	page := webui.Page[detailsArgs]{
		Path: "/device/{id}",
		Body: webui.Form[Device]{
			Load:   func(context.Context) (Device, error) { return Device{Id: "a", Ip: "10.0.0.1"}, nil },
			Fields: []webui.Accessor[Device]{formID, formIP, formCount},
			Submit: webui.Action[Device]{Run: run},
		},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
}

// An accepted action, and an error in Run, are answered with a redirect, so that a reload of what the visitor is left on
// does not send the POST again; and what is on that page does not run the action again either, however often it is loaded.
func TestAnAcceptedActionAndAnErrorAreAnsweredWithARedirectAndLoadingWhatItLeadsToRunsNothing(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		ip   string
		runs int32 // how often the action ran for the one POST
		seen string
	}{
		"accepted":                    {"10.0.0.1", 1, "Saved"},
		"the action returns an error": {"10.0.0.9", 1, "Something went wrong"},
	} {
		var runs atomic.Int32
		h := counted(&runs)
		rec := post(h, "/admin/device/a", formValues("f1", c.ip, "f2", "5"))
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.Equal(t, rec.Header().Get("Location"), "/admin/device/a")

		first := follow(h, rec)
		assert.Equal(t, first.Code, http.StatusOK)
		assert.Contains(t, first.Body.String(), c.seen)
		assert.False(t, strings.Contains(first.Body.String()+rec.Header().Get("Set-Cookie"), "db.internal"))

		// The browser has cleared the cookie, so a reload is the page as it is.
		reload := serve(h, http.MethodGet, "/admin/device/a")
		assert.False(t, hasToast(reload.Body.String()))
		assert.Equal(t, runs.Load(), c.runs)
		_ = name
	}
}

// A refused submission is answered with the page: the form drawn again with what was typed and what was refused, and the
// status 422 (Unprocessable Content). It is not redirected, since a redirect exists so that a reload does not do again what
// was done and a refusal did nothing: the browser's own question about sending the form again is the right one. Nothing is
// kept on the server and nothing is put in a cookie.
func TestARefusedSubmissionIsAnsweredWithTheFormAgainAndKeepsNothing(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		ip   string
		runs int32 // how often the action ran for the one POST
		seen string
	}{
		"rules refuse the input": {"not-an-ip", 0, "must be a valid IPv4 address"},
		"the action fails":       {"10.0.0.3", 1, "Could not reach the device"},
		"the action rejects":     {"10.0.0.4", 1, "taken"},
	} {
		var runs atomic.Int32
		h := counted(&runs)
		rec := post(h, "/admin/device/a", formValues("f1", c.ip, "f2", "5"))
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.Equal(t, rec.Header().Get("Location"), "")
		assert.Equal(t, rec.Header().Get("Set-Cookie"), "") // nothing is put in a cookie, and nothing is kept for one to name
		assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
		assert.Contains(t, rec.Body.String(), c.seen)
		assert.Contains(t, rec.Body.String(), `value="`+c.ip+`"`) // what was typed, as it was typed
		assert.True(t, hasToast(rec.Body.String()))
		assert.Equal(t, runs.Load(), c.runs)

		// A load of the page afterwards is the page as it is, with none of it: there is nothing to give.
		reload := serve(h, http.MethodGet, "/admin/device/a")
		assert.Equal(t, reload.Code, http.StatusOK)
		assert.False(t, strings.Contains(reload.Body.String(), c.ip))
		assert.False(t, hasToast(reload.Body.String()))
		_ = name
	}
}

// hasToast is whether a page shows a toast: the region that holds them is always there.
func hasToast(body string) bool {
	return strings.Contains(body, `class="toast"`) || strings.Contains(body, `class="toast `)
}

// What was typed comes back whole, up to what a POST may carry, since it is the answer and not kept.
func TestALargeRefusedSubmissionComesBackAsTyped(t *testing.T) {
	t.Parallel()

	var runs atomic.Int32
	h := counted(&runs)
	big := strings.Repeat("x", 900<<10) // under the 1 MiB a page without an editor takes
	rec := post(h, "/admin/device/a", formValues("f1", big, "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.True(t, strings.Count(rec.Body.String(), "x") >= 900<<10)
	assert.Equal(t, runs.Load(), int32(0))
}
