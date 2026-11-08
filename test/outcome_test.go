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
	"github.com/Olian04/webui/test/util/page"
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
		View: webui.Form[Device]{
			Load:   func(context.Context) (Device, webui.Loaded) { return Device{Id: "a", Ip: "10.0.0.1"}, webui.Ready() },
			Fields: []webui.Accessor[Device]{formID, formIP, formCount},
			Submit: webui.Action[Device]{Run: run},
		},
	}
	return webui.App{Pages: webui.Pages{page, webui.Page[webui.NoArgs]{Path: elsewhere, View: webui.Stack{}}}}.MustCompile("/admin")
}

func submitIP(h http.Handler, ip string) (code int, location, flash, body string) {
	rec := post(h, "/admin/device/a", url.Values{"_view": {"p"}, "f1": {ip}, "f2": {"5"}})
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

	code, location, flash, body := submitIP(outcomeApp(), "10.0.0.3")
	assert.Equal(t, code, http.StatusUnprocessableEntity) // the page, with the form drawn again: not a redirect
	assert.Equal(t, location, "")
	assert.Equal(t, flash, "") // and no cookie: nothing is kept anywhere but the page
	p := page.Parse(t, body)
	ip, _ := p.Field("f1")
	count, _ := p.Field("f2")
	assert.DeepEqual(t, []string{ip.Value, count.Value}, []string{"10.0.0.3", "5"})
	assert.DeepEqual(t, toasts(t, body), []toast{{"alert", "Could not reach the device"}})
	assert.False(t, strings.Contains(p.Root().Text(), "This field")) // it is not about a field: no field message
	assert.Equal(t, len(p.ByAttr("aria-invalid", "true")), 0)
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
	assert.Contains(t, body, `value="10.0.0.4"`) // as it was typed, like a Failure
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
	code, location, flash, _ := submitIP(outcomeApp(), "10.0.0.3") // Failure: the form again, as the answer
	assert.Equal(t, code, http.StatusUnprocessableEntity)
	assert.Equal(t, location, "")
	assert.Equal(t, flash, "")
}

func TestATableActionsFailureAndRejectAreAnErrorToastAndASuccessIsNot(t *testing.T) {
	t.Parallel()

	run := func(o webui.Outcome) http.Handler {
		page := webui.Page[webui.NoArgs]{
			Path: "/device",
			View: webui.Table[Device]{
				Loader:  webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) { return []Device{{Id: "a"}}, webui.Ready() }},
				Key:     func(d Device) string { return d.Id },
				Columns: []webui.Accessor[Device]{formID},
				Actions: []webui.Control[Device]{webui.Action[Device]{Label: "Go", Run: func(context.Context, Device) (webui.Outcome, error) { return o, nil }}},
			},
		}
		return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	}
	flashOf := func(h http.Handler) string {
		rec := post(h, "/admin/device", url.Values{"_view": {"p"}, "_act": {"row:0:a"}})
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
	// A failure is an alert, which a screen reader says at once; the others wait their turn.
	assert.DeepEqual(t, toasts(t, get("wCareful")), []toast{{"status", "Careful"}})
	assert.DeepEqual(t, toasts(t, get("oFine")), []toast{{"status", "Fine"}})
	assert.DeepEqual(t, toasts(t, get("eBad")), []toast{{"alert", "Bad"}})
}

// A toast can be dismissed without script: the button is a label for a radio, in a group of its own, that is named.
func TestAToastHasADismissControlThatNeedsNoScript(t *testing.T) {
	t.Parallel()

	h := outcomeApp()
	_, _, _, body := submitIP(h, "10.0.0.3")
	dismiss := page.One(t, "the dismiss control", page.Parse(t, body).ByRole("radio", "Dismiss"))
	assert.True(t, dismiss.Attr("name") != "") // in a group of its own
}
