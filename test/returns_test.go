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

// returnsApp has a form whose Run builds the Outcome its IP names: "10.0.K.M", where K is the
// kind (1 Success, 2 Warning, 3 Failure, 4 Reject) and M what it says about where to go
// (0 nothing, 1 Back, 2 Stay, 3 Then). The form is on a page below a list, so it has a
// breadcrumb parent, and can be opened from a link or directly.
func returnsApp() http.Handler {
	run := func(ctx context.Context, d Device) (webui.Outcome, error) {
		var o webui.Outcome
		switch d.Ip[5] {
		case '1':
			o = webui.Success("Saved")
		case '2':
			o = webui.Warning("Careful")
		case '3':
			o = webui.Failure("Nope")
		case '4':
			o = webui.Reject(webui.Field[Device](formIP, "Taken"))
		}
		switch d.Ip[7] {
		case '1':
			o = o.Back()
		case '2':
			o = o.Stay()
		case '3':
			o = o.Then(webui.Open(ctx, elsewhere, webui.NoArgs{}))
		}
		return o, nil
	}
	details := webui.Page[detailsArgs]{
		Path: "/device/{id}",
		Body: webui.Form[Device]{
			Load:   func(context.Context) (Device, webui.Loaded) { return Device{Id: "a", Ip: "10.0.1.0"}, webui.Ready() },
			Fields: []webui.Accessor[Device]{formID, formIP},
			Submit: webui.Action[Device]{Run: run},
		},
	}
	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: webui.Stack{}}
	return webui.App{Pages: webui.Pages{list, details, webui.Page[webui.NoArgs]{Path: elsewhere, Body: webui.Stack{}}}}.MustCompile("/admin")
}

// sent is what the user is left with after a submit: the status, where they go, and the
// toast that goes with them, as the flash cookie (tone letter and text) or, when the form is
// shown again, as the page's own toast.
type sent struct {
	code     int
	location string
	flash    string
	rerender bool
}

func submitTo(h http.Handler, target, ip string) sent {
	rec := post(h, target, url.Values{"_leaf": {"p"}, "f1": {"10.0." + ip}})
	out := sent{code: rec.Code, location: rec.Header().Get("Location")}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "webui_flash" {
			raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
			out.flash = string(raw)
		}
	}
	out.rerender = rec.Code == http.StatusUnprocessableEntity // the form drawn again, as the answer
	return out
}

const (
	openedDirectly = "/admin/device/a"
	openedFromLink = "/admin/device/a?webui.from=%2Fadmin%2Felsewhere"
)

func TestAnOutcomeThatSaysNothingDoesWhatItsKindDoes(t *testing.T) {
	t.Parallel()

	h := returnsApp()
	// Accepted: back to where the form was opened from, or where it is.
	assert.Equal(t, submitTo(h, openedFromLink, "1.0"), sent{code: 303, location: "/admin/elsewhere", flash: "oSaved"})
	assert.Equal(t, submitTo(h, openedDirectly, "2.0"), sent{code: 303, location: openedDirectly, flash: "wCareful"})
	// Not accepted: the form again, with what was typed.
	assert.True(t, submitTo(h, openedFromLink, "3.0").rerender)
	assert.True(t, submitTo(h, openedFromLink, "4.0").rerender)
}

func TestBackLeavesForWhereTheUserCameFromWhateverTheKindOfOutcome(t *testing.T) {
	t.Parallel()

	h := returnsApp()
	for ip, flash := range map[string]string{"1.1": "oSaved", "2.1": "wCareful", "3.1": "eNope", "4.1": "eTaken"} {
		// Opened from a link: back to that page.
		assert.Equal(t, submitTo(h, openedFromLink, ip), sent{code: 303, location: "/admin/elsewhere", flash: flash})
		// Opened directly: to the parent in the breadcrumb.
		assert.Equal(t, submitTo(h, openedDirectly, ip), sent{code: 303, location: "/admin/device", flash: flash})
	}
}

func TestAFailureThatGoesBackDoesNotShowTheFormAgain(t *testing.T) {
	t.Parallel()

	rec := post(returnsApp(), openedFromLink, url.Values{"_leaf": {"p"}, "f1": {"10.0.3.1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Body.Len(), 0) // a redirect, not a page: what was typed is not kept
}

func TestBackWithNowhereToGoStaysWhereTheUserIs(t *testing.T) {
	t.Parallel()

	// A form on a page with no breadcrumb parent, and opened directly.
	page := webui.Page[webui.NoArgs]{Path: "/solo", Body: webui.Form[Device]{
		Load:   func(context.Context) (Device, webui.Loaded) { return Device{Id: "a"}, webui.Ready() },
		Fields: []webui.Accessor[Device]{formID, formIP},
		Submit: webui.Action[Device]{Run: func(context.Context, Device) (webui.Outcome, error) {
			return webui.Failure("Nope").Back(), nil
		}},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	got := submitTo(h, "/admin/solo", "1.0")
	assert.Equal(t, got, sent{code: 303, location: "/admin/solo", flash: "eNope"})
}

func TestBackNeverLeavesForAnAddressThatIsNotInTheApp(t *testing.T) {
	t.Parallel()

	got := submitTo(returnsApp(), "/admin/device/a?webui.from=https%3A%2F%2Fevil.example%2F", "1.1")
	assert.Equal(t, got.location, "/admin/device") // the unsafe origin is dropped: the parent
}

func TestStayKeepsASuccessOnTheFormEvenWhenItWasOpenedFromALink(t *testing.T) {
	t.Parallel()

	h := returnsApp()
	// The address still has where it came from, so saving again, or Cancel, still returns.
	assert.Equal(t, submitTo(h, openedFromLink, "1.2"), sent{code: 303, location: openedFromLink, flash: "oSaved"})
	assert.Equal(t, submitTo(h, openedFromLink, "2.2"), sent{code: 303, location: openedFromLink, flash: "wCareful"})
	assert.Equal(t, submitTo(h, openedDirectly, "1.2"), sent{code: 303, location: openedDirectly, flash: "oSaved"})
}

func TestStayChangesNothingForAnOutcomeThatAlreadyStays(t *testing.T) {
	t.Parallel()

	h := returnsApp()
	for _, ip := range []string{"3.2", "4.2"} {
		got := submitTo(h, openedFromLink, ip)
		assert.True(t, got.rerender)      // the form again, with what was typed and the reasons
		assert.Equal(t, got.location, "") // as the answer, with no redirect
		page := post(h, openedFromLink, url.Values{"_leaf": {"p"}, "f1": {"10.0." + ip}}).Body.String()
		assert.Contains(t, page, "webui.from=%2Fadmin%2Felsewhere") // and it still posts to the address that has where it came from
	}
	body := post(h, openedFromLink, url.Values{"_leaf": {"p"}, "f1": {"10.0.4.2"}}).Body.String()
	assert.Contains(t, body, "Taken") // a Reject still says it beside the field
}

func TestThenStillBeatsTheDefaultForEveryKind(t *testing.T) {
	t.Parallel()

	h := returnsApp()
	for ip, flash := range map[string]string{"1.3": "oSaved", "2.3": "wCareful", "3.3": "eNope", "4.3": "eTaken"} {
		assert.Equal(t, submitTo(h, openedFromLink, ip), sent{code: 303, location: "/admin/elsewhere", flash: flash})
	}
}

func TestAnOutcomeCannotSayMoreThanOneThingAboutWhereToGo(t *testing.T) {
	t.Parallel()

	for name, outcome := range map[string]func(context.Context) webui.Outcome{
		"Back and Stay": func(context.Context) webui.Outcome { return webui.Success("x").Back().Stay() },
		"Back and Then": func(ctx context.Context) webui.Outcome {
			return webui.Success("x").Back().Then(webui.Open(ctx, elsewhere, webui.NoArgs{}))
		},
		"Stay and Then": func(ctx context.Context) webui.Outcome {
			return webui.Success("x").Stay().Then(webui.Open(ctx, elsewhere, webui.NoArgs{}))
		},
	} {
		page := webui.Page[webui.NoArgs]{Path: "/solo", Body: webui.Form[Device]{
			Load:   func(context.Context) (Device, webui.Loaded) { return Device{Id: "a"}, webui.Ready() },
			Fields: []webui.Accessor[Device]{formID, formIP},
			Submit: webui.Action[Device]{Run: func(ctx context.Context, _ Device) (webui.Outcome, error) { return outcome(ctx), nil }},
		}}
		h := webui.App{Pages: webui.Pages{page, webui.Page[webui.NoArgs]{Path: elsewhere, Body: webui.Stack{}}}}.MustCompile("/admin")
		rec := post(h, "/admin/solo", url.Values{"_leaf": {"p"}, "f1": {"10.0.0.1"}})
		if rec.Code != http.StatusSeeOther || !strings.Contains(follow(h, rec).Body.String(), "Something went wrong") {
			t.Errorf("%s: status %d, want a redirect that says something went wrong, a mistake in the action's Run", name, rec.Code)
		}
	}
}

func TestBackAndStayOnATableActionLeaveForTheParentOrStay(t *testing.T) {
	t.Parallel()

	act := func(o webui.Outcome) http.Handler {
		table := webui.Table[Device]{
			Title: "Devices",
			Load: func(context.Context, webui.Query) (webui.Window[Device], webui.Loaded) {
				return webui.Window[Device]{Items: []Device{{Id: "a"}}, Total: 1}, webui.Ready()
			},
			Key:     func(d Device) string { return d.Id },
			Columns: []webui.Accessor[Device]{formID},
			Actions: []webui.Action[Device]{{Label: "Go", Run: func(context.Context, Device) (webui.Outcome, error) { return o, nil }}},
		}
		parent := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: webui.Stack{}}
		child := webui.Page[webui.NoArgs]{Path: "/device/log", Body: table}
		return webui.App{Pages: webui.Pages{parent, child}}.MustCompile("/admin")
	}
	post1 := func(h http.Handler) *httptest.ResponseRecorder {
		return post(h, "/admin/device/log", url.Values{"_leaf": {"p"}, "_act": {"row:0:a"}})
	}
	// Back leaves for the breadcrumb parent, even for a Failure; Stay and the default stay.
	rec := post1(act(webui.Failure("Nope").Back()))
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device")
	rec = post1(act(webui.Success("Done").Stay()))
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/log")
	rec = post1(act(webui.Success("Done")))
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/log")
}
