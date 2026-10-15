package webui_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
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

// Every answer to a POST is a redirect, so that a reload of what the visitor is left on does not send it again;
// and what is on that page does not run the action again either, however often it is loaded.
func TestEveryAnswerToAPostIsARedirectAndLoadingWhatItLeadsToRunsNothing(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		ip   string
		runs int32 // how often the action ran for the one POST
		seen string
	}{
		"rules refuse the input":      {"not-an-ip", 0, "must be a valid IPv4 address"},
		"accepted":                    {"10.0.0.1", 1, "Saved"},
		"the action fails":            {"10.0.0.3", 1, "Could not reach the device"},
		"the action rejects":          {"10.0.0.4", 1, "taken"},
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

// hasToast is whether a page shows a toast: the region that holds them is always there.
func hasToast(body string) bool {
	return strings.Contains(body, `class="toast"`) || strings.Contains(body, `class="toast `)
}

// A submission is kept for one load of the page that draws it again, and for no one else.
func TestAKeptSubmissionIsGivenOnceToTheVisitorWhoseCookieNamesIt(t *testing.T) {
	t.Parallel()

	var runs atomic.Int32
	h := counted(&runs)
	submit := func() *http.Cookie {
		rec := post(h, "/admin/device/a", formValues("f1", "not-an-ip", "f2", "5"))
		var cookie *http.Cookie
		for _, c := range rec.Result().Cookies() {
			cookie = c
		}
		return cookie
	}
	get := func(path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		out := httptest.NewRecorder()
		h.ServeHTTP(out, req)
		return out
	}

	cookie := submit()
	assert.Equal(t, cookie.Name, "webui_flash")
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, cookie.MaxAge, 60)
	raw, _ := base64.RawURLEncoding.DecodeString(cookie.Value)
	assert.False(t, strings.Contains(string(raw), "not-an-ip")) // the cookie holds a token, not what was typed

	// The visitor's own load gets it, once, and the cookie is cleared.
	mine := get("/admin/device/a", cookie)
	assert.Contains(t, mine.Body.String(), `value="not-an-ip"`)
	assert.Equal(t, mine.Result().Cookies()[0].MaxAge, -1)

	// Sending the cookie again finds nothing, and says so.
	again := get("/admin/device/a", cookie)
	assert.False(t, strings.Contains(again.Body.String(), "not-an-ip"))
	assert.Contains(t, again.Body.String(), "Your changes could not be kept")

	// Another address, even of the same page, is not the one it was kept for: it is shown none of it, and the
	// cookie it clears is the only way left to it.
	other := get("/admin/device/b", submit())
	assert.False(t, strings.Contains(other.Body.String(), "not-an-ip"))
	assert.False(t, hasToast(other.Body.String()))
}

func TestAForgedTokenKeepsNothingAndSaysSo(t *testing.T) {
	t.Parallel()

	var runs atomic.Int32
	h := counted(&runs)
	req := httptest.NewRequest(http.MethodGet, "/admin/device/a", nil)
	//nolint:gosec // G124: a request the test makes, not a cookie the app sets.
	req.AddCookie(&http.Cookie{Name: "webui_flash", Value: base64.RawURLEncoding.EncodeToString([]byte("sNOTATOKEN"))})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Your changes could not be kept")
	assert.Equal(t, rec.Result().Cookies()[0].MaxAge, -1)
}

// What was typed is kept whole, up to what a POST may carry.
func TestALargeRefusedSubmissionComesBackAsTyped(t *testing.T) {
	t.Parallel()

	var runs atomic.Int32
	h := counted(&runs)
	big := strings.Repeat("x", 900<<10) // under the 1 MiB a page without an editor takes
	rec := post(h, "/admin/device/a", formValues("f1", big, "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	body := follow(h, rec).Body.String()
	assert.True(t, strings.Count(body, "x") >= 900<<10)
	assert.Equal(t, runs.Load(), int32(0))
}
