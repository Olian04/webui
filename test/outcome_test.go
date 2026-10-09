package webui_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

const elsewhere webui.PageID[webui.NoArgs] = "/elsewhere"

// outcomeApp is a form whose Run answers with whatever the IP asks for, so each
// kind of Outcome can be seen by what the user is left with.
func outcomeApp() http.Handler {
	run := func(ctx context.Context, d Device) (webui.Outcome, error) {
		there := webui.Open(ctx, elsewhere, webui.NoArgs{})
		switch d.Ip {
		case "10.0.0.2":
			return webui.Warning("Saved, but the old address is still in use elsewhere"), nil
		case "10.0.0.3":
			return webui.Failure("Could not reach the device"), nil
		case "10.0.0.4":
			return webui.Reject(
				webui.Field[Device](formIP, "taken"),
				webui.Field[Device](formCount, "too many"),
			), nil
		case "10.0.0.5":
			return webui.Failure(""), nil
		case "10.0.0.6":
			return webui.Success("Gone").Then(there), nil
		case "10.0.0.7":
			return webui.Failure("no").Then(there), nil
		case "10.0.0.8":
			return webui.Reject(
				webui.Field[Device](formIP, "taken"),
				webui.Field[Device](formCount, "too many"),
			).Then(there), nil
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
	return webui.App{Pages: webui.Pages{page, webui.Page[webui.NoArgs]{Path: elsewhere, Body: webui.Stack{}}}}.MustCompile("/admin")
}

func submitIP(h http.Handler, ip string) (code int, location, flash, body string) {
	rec := post(h, "/admin/device/a", url.Values{"_leaf": {"p"}, "f1": {ip}, "f2": {"5"}})
	for _, c := range rec.Result().Cookies() {
		if c.Name == "webui_flash" {
			raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
			flash = string(raw)
		}
	}
	return rec.Code, rec.Header().Get("Location"), flash, rec.Body.String()
}

func TestSuccessIsAcceptedAndConfirmsWithAnOKToast(t *testing.T) {
	t.Parallel()

	code, location, flash, _ := submitIP(outcomeApp(), "10.0.0.1")
	assert.Equal(t, code, http.StatusSeeOther)
	assert.Equal(t, location, "/admin/device/a")
	assert.Equal(t, flash, "oSaved")
}

func TestWarningIsAcceptedLikeSuccessButSaysSomethingToBeAwareOf(t *testing.T) {
	t.Parallel()

	code, location, flash, _ := submitIP(outcomeApp(), "10.0.0.2")
	assert.Equal(t, code, http.StatusSeeOther) // accepted: sent back, nothing shown again
	assert.Equal(t, location, "/admin/device/a")
	assert.Equal(t, flash, "wSaved, but the old address is still in use elsewhere")
}

func TestFailureShowsTheFormAgainWithWhatWasTypedAndAnErrorToast(t *testing.T) {
	t.Parallel()

	code, _, flash, body := submitIP(outcomeApp(), "10.0.0.3")
	assert.Equal(t, code, http.StatusUnprocessableEntity)
	assert.Equal(t, flash, "") // a toast on this response, not a cookie for the next
	assert.Contains(t, body, `value="10.0.0.3"`)
	assert.Contains(t, body, `value="5"`)
	assert.Contains(t, body, "Could not reach the device")
	assert.Contains(t, body, `class="toast err"`)
	assert.False(t, strings.Contains(body, `class="err"`)) // it is not about a field: no field message
}

func TestFailureWithNoMessageStillSaysNothingWasSaved(t *testing.T) {
	t.Parallel()

	_, _, _, body := submitIP(outcomeApp(), "10.0.0.5")
	assert.Contains(t, body, "Not saved")
}

func TestRejectShowsTheFormAgainWithWhatWasTypedAndAMessageBesideEachField(t *testing.T) {
	t.Parallel()

	code, _, _, body := submitIP(outcomeApp(), "10.0.0.4")
	assert.Equal(t, code, http.StatusUnprocessableEntity)
	assert.Contains(t, body, `value="10.0.0.4"`) // kept, like a Failure
	assert.Contains(t, body, "taken")
	assert.Contains(t, body, "too many")
	assert.Contains(t, body, "2 fields need attention.")
}

func TestThenTakesTheUserThereWhateverTheOutcome(t *testing.T) {
	t.Parallel()

	// Accepted, and told where: there, with the message as a toast.
	code, location, flash, _ := submitIP(outcomeApp(), "10.0.0.6")
	assert.Equal(t, code, http.StatusSeeOther)
	assert.Equal(t, location, "/admin/elsewhere")
	assert.Equal(t, flash, "oGone")

	// Not accepted, and told where: there too, instead of showing the form again,
	// with the failure as an error toast.
	code, location, flash, _ = submitIP(outcomeApp(), "10.0.0.7")
	assert.Equal(t, code, http.StatusSeeOther)
	assert.Equal(t, location, "/admin/elsewhere")
	assert.Equal(t, flash, "eno")

	// A rejection leaves the form for a page with no fields: its messages are the toast.
	code, location, flash, _ = submitIP(outcomeApp(), "10.0.0.8")
	assert.Equal(t, code, http.StatusSeeOther)
	assert.Equal(t, location, "/admin/elsewhere")
	assert.Equal(t, flash, "etaken; too many")
}

func TestWithoutThenEachOutcomeDoesItsDefault(t *testing.T) {
	t.Parallel()

	code, location, _, _ := submitIP(outcomeApp(), "10.0.0.1") // Success: back where it was
	assert.Equal(t, code, http.StatusSeeOther)
	assert.Equal(t, location, "/admin/device/a")
	code, _, _, _ = submitIP(outcomeApp(), "10.0.0.3") // Failure: the form again
	assert.Equal(t, code, http.StatusUnprocessableEntity)
}

func TestATableActionsFailureAndRejectAreAnErrorToastAndASuccessIsNot(t *testing.T) {
	t.Parallel()

	run := func(o webui.Outcome) http.Handler {
		page := webui.Page[webui.NoArgs]{
			Path: "/device",
			Body: webui.Table[Device]{
				Rows:    func(context.Context) ([]Device, error) { return []Device{{Id: "a"}}, nil },
				Key:     func(d Device) string { return d.Id },
				Columns: []webui.Accessor[Device]{formID},
				Actions: []webui.Action[Device]{{Label: "Go", Run: func(context.Context, Device) (webui.Outcome, error) { return o, nil }}},
			},
		}
		return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	}
	flashOf := func(h http.Handler) string {
		rec := post(h, "/admin/device", url.Values{"_leaf": {"p"}, "_act": {"row:0:a"}})
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		for _, c := range rec.Result().Cookies() {
			raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
			return string(raw)
		}
		return ""
	}
	assert.Equal(t, flashOf(run(webui.Failure("Could not"))), "eCould not")
	assert.Equal(t, flashOf(run(webui.Reject(webui.Field[Device](formID, "a"), webui.Field[Device](formID, "b")))), "ea; b")
	assert.Equal(t, flashOf(run(webui.Warning("Careful"))), "wCareful")
	assert.Equal(t, flashOf(run(webui.Outcome{})), "") // a quiet success shows nothing
}

func TestAWarningToastIsDrawnAsAWarning(t *testing.T) {
	t.Parallel()

	h := outcomeApp()
	get := func(cookie string) string {
		r, _ := http.NewRequest(http.MethodGet, "/admin/device/a", nil)
		r.Header.Set("Cookie", "webui_flash="+base64.RawURLEncoding.EncodeToString([]byte(cookie)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Body.String()
	}
	assert.Contains(t, get("wCareful"), `class="toast warn"`)
	assert.Contains(t, get("oFine"), `class="toast"`)
	assert.Contains(t, get("eBad"), `class="toast err"`)
}

func TestAToastCanBeDismissedWithoutScript(t *testing.T) {
	t.Parallel()

	h := outcomeApp()
	_, _, _, body := submitIP(h, "10.0.0.3")
	// The button is a label for a hidden radio, in a group of its own.
	assert.Contains(t, body, `<label class="toast-close" title="Dismiss"><input class="sr-only" type="radio" name="toast-0" aria-label="Dismiss">`)

	// The stylesheet hides the toast whose radio is chosen: the markup and the rule
	// are one mechanism, so a change to either has to keep both.
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".toast:has(.toast-close input:checked) {\n  display: none;")

	// A browser that cannot do :has() would show a button that does nothing, so the
	// button is shown only where the rule above works.
	assert.Contains(t, css, "display: none; /* until the browser can act on the radio, below */")
	assert.Contains(t, css, "@supports selector(:has(*)) {\n  .toast-close {\n    display: grid;")
}
