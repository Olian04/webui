package webui_test

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type cancelArgs struct {
	ID string
}

// cancelApp is a list of devices, a detail form reached from its rows, and the
// same form on a page with nothing above it. The pages declare nothing about
// where Cancel goes: the library remembers where the user came from.
func cancelApp(redirect string) http.Handler {
	var details webui.Page[cancelArgs]
	details.Path = "/device/{id}"
	var notes webui.Page[cancelArgs]
	notes.Path = "/notes/{id}"

	rows := func(dest webui.PageRef[cancelArgs]) webui.Table[Device] {
		return webui.Table[Device]{
			Load: func(context.Context, webui.Query) (webui.Rows[Device], error) {
				return webui.Rows[Device]{Items: []Device{{Id: "a"}}, Total: 1}, nil
			},
			RowClick: webui.Link[Device, cancelArgs]{
				Page: dest,
				Args: func(_ context.Context, d Device) cancelArgs { return cancelArgs{ID: d.Id} },
			},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	form := webui.Form[Device]{
		Load:   func(context.Context) (Device, error) { return Device{Id: "a", Ip: "10.0.0.1"}, nil },
		Fields: []webui.Accessor[Device]{formID, formIP},
		Submit: webui.Action[Device]{Run: func(_ context.Context, d Device) (webui.Effect, error) {
			if d.Ip == "10.0.0.9" {
				return webui.Effect{Fields: webui.Fields[Device]{{Field: formIP, Message: "taken"}}}, nil
			}
			return webui.Effect{Redirect: webui.Target{URL: redirect}}, nil
		}},
	}
	details.Body = webui.Stack{form, rows(details)} // and a way on to another form, so forms chain
	notes.Body = webui.Stack{} // a page with no form has nothing to cancel out of

	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: rows(details)}
	other := webui.Page[webui.NoArgs]{Path: "/other", Nav: webui.Nav{Label: "Other"}, Body: rows(details)}
	plain := webui.Page[webui.NoArgs]{Path: "/plain", Body: rows(notes)}
	solo := webui.Page[cancelArgs]{Path: "/solo/{id}", Body: form} // no parent page to cancel to
	return webui.App{Pages: webui.Pages{list, other, plain, details, notes, solo}}.MustCompile("/admin")
}

// firstRowHref is where the first row of a page goes, as the page's HTML says it.
func firstRowHref(t *testing.T, h http.Handler, target string) string {
	t.Helper()

	body := serve(h, http.MethodGet, target).Body.String()
	const marker = `<a class="rowlink" href="`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no row link in %s", target)
	}
	body = body[i+len(marker):]
	return html.UnescapeString(body[:strings.Index(body, `"`)])
}

func cancelHref(t *testing.T, h http.Handler, target string) string {
	t.Helper()

	body := serve(h, http.MethodGet, target).Body.String()
	const marker = `<a class="btn btn-ghost " href="`
	i := strings.Index(body, marker)
	if i < 0 {
		return "" // no Cancel button
	}
	body = body[i+len(marker):]
	return html.UnescapeString(body[:strings.Index(body, `"`)])
}

func TestCancelGoesBackToThePageTheFormWasOpenedFrom(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	for _, origin := range []string{"/admin/device?table.sort=ID&table.desc=true", "/admin/other"} {
		link := firstRowHref(t, h, origin)
		assert.Equal(t, cancelHref(t, h, link), origin) // view state and all
	}
}

func TestCancelOfAFormOpenedDirectlyGoesToTheBreadcrumbParent(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	assert.Equal(t, cancelHref(t, h, "/admin/device/a"), "/admin/device")
}

func TestAFormWithNoBreadcrumbParentHasNoCancelUnlessItWasOpenedFromAPage(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	assert.Equal(t, cancelHref(t, h, "/admin/solo/a"), "")
	assert.Equal(t, cancelHref(t, h, "/admin/solo/a?webui.from=%2Fadmin%2Fother"), "/admin/other")
}

func TestOnlyLinksToPagesWithAFormRememberTheOrigin(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	assert.Equal(t, firstRowHref(t, h, "/admin/plain"), "/admin/notes/a")
	assert.Contains(t, firstRowHref(t, h, "/admin/device"), "webui.from=%2Fadmin%2Fdevice")
}

func TestCancelNeverGoesToAnAddressThatIsNotInTheApp(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	for _, evil := range []string{
		"https://evil.example/", "//evil.example/", `/\evil.example`, "javascript:alert(1)", "evil.example/x",
		"/logout", "/administrator", "/admin/_webui/logo",
	} {
		got := cancelHref(t, h, "/admin/device/a?webui.from="+url.QueryEscape(evil))
		assert.Equal(t, got, "/admin/device") // refused: the breadcrumb stands
	}
}

func TestAChainOfFormsUnwindsOneHopAtATime(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	first := firstRowHref(t, h, "/admin/device")
	// A form page reached from another form page remembers that one, which
	// remembers its own.
	second := "/admin/other?webui.from=" + url.QueryEscape(first)
	assert.Equal(t, cancelHref(t, h, first), "/admin/device")
	assert.Equal(t, cancelHref(t, h, "/admin/solo/a?webui.from="+url.QueryEscape(second)), second)
}

func TestTheOriginIsBoundedSoAChainCannotGrowTheAddressWithoutEnd(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	address := "/admin/device"
	for range 60 {
		address = firstRowHref(t, h, address)
	}
	assert.Equal(t, len(address) <= 2048, true)
}

func TestARejectedSubmissionKeepsTheCancelItHad(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	rec := post(h, "/admin/device/a?webui.from=%2Fadmin%2Fother", url.Values{"_leaf": {"p.0"}, "f1": {"not-an-ip"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), `href="/admin/other">Cancel`)
	// And it still posts to an address that remembers it, so a second try does too.
	assert.Contains(t, rec.Body.String(), `webui.from=%2Fadmin%2Fother`)
}

func TestASavedFormReturnsToThePageItWasOpenedFrom(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	rec := post(h, "/admin/device/a?webui.from=%2Fadmin%2Fother%3Ftable.sort%3DID", url.Values{"_leaf": {"p.0"}, "f1": {"10.0.0.2"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/other?table.sort=ID")

	// Opened directly, it stays where it was, as it always did.
	rec = post(h, "/admin/device/a", url.Values{"_leaf": {"p.0"}, "f1": {"10.0.0.2"}})
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/a")
}

func TestAnExplicitRedirectBeatsTheOrigin(t *testing.T) {
	t.Parallel()

	h := cancelApp("/admin/plain")
	rec := post(h, "/admin/device/a?webui.from=%2Fadmin%2Fother", url.Values{"_leaf": {"p.0"}, "f1": {"10.0.0.2"}})
	assert.Equal(t, rec.Header().Get("Location"), "/admin/plain")
}

func TestASavedFormNeverReturnsToAnAddressThatIsNotInTheApp(t *testing.T) {
	t.Parallel()

	h := cancelApp("")
	rec := post(h, "/admin/device/a?webui.from=https%3A%2F%2Fevil.example%2F", url.Values{"_leaf": {"p.0"}, "f1": {"10.0.0.2"}})
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/a?webui.from=https%3A%2F%2Fevil.example%2F")
}
