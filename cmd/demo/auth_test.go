package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
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
	assert.Equal(t, rec.Header().Get("Location"), "/login")
}

func TestLoginPageEndsTheSessionAndOffersABrowserBothKindsOfUser(t *testing.T) {
	rec := withCookie(site(t), http.MethodGet, "/login", sign(Editor), nil)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Continue as a viewer")
	assert.Contains(t, rec.Body.String(), "Continue as an editor")
	assert.Contains(t, rec.Body.String(), "It is not part of the library") // the page says so
	assert.Equal(t, rec.Result().Cookies()[0].MaxAge, -1)                  // the session is over
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
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
			t.Errorf("%s: got %d to %q, want a redirect to /login", name, rec.Code, rec.Header().Get("Location"))
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

func TestTheLoginPageFollowsTheThemeTheAppStoresForTheVisitor(t *testing.T) {
	// What the sign-in page does with the choice is in the browser suite: its text is script and style.
	body := withCookie(site(t), http.MethodGet, "/login", "", nil).Body.String()
	assert.True(t, len(page.Parse(t, body).ByRole("heading", "")) > 0)
}

func TestTheAppsExternalLinksLogOutByLinkingToTheChooser(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/device")
	var logout, issues page.Link
	for _, l := range p.Links() {
		switch l.Href {
		case "/login":
			logout = l
		case "https://github.com/Olian04/webui/issues":
			issues = l
		}
	}
	assert.Equal(t, logout.Name, "Log out")
	assert.False(t, logout.NewTab) // a path on this site: the same tab
	assert.True(t, issues.NewTab)  // a web address: a tab of its own
}
