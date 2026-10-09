package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// site is the demo as a server runs it, with no session unless a test adds one.
func site(t *testing.T) http.Handler {
	t.Helper()

	compiled, err := app.Compile("/admin")
	assert.NoError(t, err)
	return routes(compiled)
}

func withCookie(h http.Handler, method, target, cookie string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != "" {
		req.Header.Set("Cookie", sessionCookie+"="+cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWithoutASessionTheAppSendsYouToChooseOne(t *testing.T) {
	rec := withCookie(site(t), http.MethodGet, "/admin/device", "", nil)
	assert.Equal(t, rec.Code, http.StatusFound)
	assert.Equal(t, rec.Header().Get("Location"), "/logout")
}

func TestLogoutEndsTheSessionAndOffersABrowserBothKindsOfUser(t *testing.T) {
	rec := withCookie(site(t), http.MethodGet, "/logout", sign(Editor), nil)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Continue as a viewer")
	assert.Contains(t, rec.Body.String(), "Continue as an editor")
	assert.Equal(t, rec.Result().Cookies()[0].MaxAge, -1) // the session is over
	assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
}

func TestChoosingARoleStartsASignedSessionThatTheGuardsSee(t *testing.T) {
	h := site(t)
	for role, auditCode := range map[Role]int{Viewer: http.StatusForbidden, Editor: http.StatusOK} {
		rec := withCookie(h, http.MethodPost, "/login", "", url.Values{"role": {string(role)}})
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.Equal(t, rec.Header().Get("Location"), "/admin/")
		cookie := rec.Result().Cookies()[0]
		assert.Equal(t, cookie.Name, sessionCookie)
		assert.True(t, cookie.HttpOnly)
		got, ok := verify(cookie.Value)
		assert.True(t, ok)
		assert.Equal(t, got, role)

		// With that session, the app's Guards see the role: the audit log is for editors.
		assert.Equal(t, withCookie(h, http.MethodGet, "/admin/audit", cookie.Value, nil).Code, auditCode)
	}
}

func TestASessionThatIsNotOursIsNotASession(t *testing.T) {
	h := site(t)
	viewerSig := strings.TrimPrefix(sign(Viewer), "viewer.")
	for name, forged := range map[string]string{
		"an unsigned role":                  "editor",
		"a made-up signature":               "editor.00ff",
		"a role we do not have":             "admin." + viewerSig,
		"a viewer's signature on an editor": "editor." + viewerSig,
		"not hex":                           "editor.zz",
	} {
		rec := withCookie(h, http.MethodGet, "/admin/device", forged, nil)
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/logout" {
			t.Errorf("%s: got %d to %q, want a redirect to /logout", name, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestLoginRefusesARoleThatIsNeitherAndACrossSitePost(t *testing.T) {
	h := site(t)
	assert.Equal(t, withCookie(h, http.MethodPost, "/login", "", url.Values{"role": {"admin"}}).Code, http.StatusBadRequest)
	assert.Equal(t, withCookie(h, http.MethodPost, "/login", "", nil).Code, http.StatusBadRequest)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("role=editor"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusForbidden) // another site cannot sign you in
}

func TestTheAppsMenuLogsOutByLinkingToTheChooser(t *testing.T) {
	body := get(handler(t), "/admin/device").Body.String()
	assert.Contains(t, body, `<a class="menu-item" href="/logout">`)
}
