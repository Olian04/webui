package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// post sends a form straight through the handler.
func post(h http.Handler, target string, form url.Values, mod ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, m := range mod {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// follow answers the redirect that rec is, as a browser does: a GET of its Location, carrying the cookies the
// redirect set. Every POST ends in one, so that a reload does not send it again.
func follow(h http.Handler, rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, rec.Header().Get("Location"), nil)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge >= 0 {
			req.AddCookie(c)
		}
	}
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

// shop is the README's two pages with every action kind, recording what ran.
type shop struct {
	mu       sync.Mutex
	saved    []Device
	deleted  []string
	bulk     [][]string
	pageGate error
	subGate  error
	runErr   error
	redirect func(ctx context.Context) webui.Target
	h        http.Handler
}

func newShop() *shop {
	s := &shop{}
	ip := webui.String[Device]{
		Label: "IP", Load: func(d Device) string { return d.Ip }, Store: func(d *Device, v string) { d.Ip = v },
		Required: true,
		Rules:    webui.StringRules{MinLen: 7, Pattern: &webui.PatternRule{Expr: `\d+(\.\d+){3}`, Message: "must be a valid IPv4 address"}},
	}
	count := webui.Int[Device]{
		Label: "Occurrences", Load: func(d Device) int { return d.Count }, Store: func(d *Device, v int) { d.Count = v },
		Rules: webui.NumberRules[int]{Max: new(100)},
	}

	var list webui.Page[webui.NoArgs]
	details := webui.Page[detailsArgs]{
		Path: "/device/{id}",
		Guard: func(context.Context, detailsArgs) error {
			return s.pageGate
		},
		Body: webui.Form[Device]{
			Title: "Device",
			Load: func(ctx context.Context) (Device, webui.Loaded) {
				a := webui.ArgsOf[detailsArgs](ctx)
				return Device{Id: a.Id, Ip: "10.0.0.1", Count: 3}, webui.Ready()
			},
			Fields: []webui.Accessor[Device]{formID, ip, count},
			Submit: webui.Action[Device]{
				Guard: func(context.Context, Device) error { return s.subGate },
				Run: func(ctx context.Context, d Device) (webui.Outcome, error) {
					if s.runErr != nil {
						return webui.Outcome{}, s.runErr
					}
					if d.Ip == "10.0.4.17" {
						return webui.Reject(webui.Field[Device](ip, "already in use by another device")), nil
					}
					s.mu.Lock()
					s.saved = append(s.saved, d)
					s.mu.Unlock()
					e := webui.Success("Device saved!")
					if s.redirect != nil {
						e = e.Then(s.redirect(ctx))
					}
					return e, nil
				},
			},
		},
	}
	list = webui.Page[webui.NoArgs]{
		Path: "/device", Nav: webui.Nav{Label: "Devices"},
		Body: webui.Table[Device]{
			Title: "Devices",
			Load: func(context.Context, webui.Query) (webui.Window[Device], webui.Loaded) {
				return webui.Window[Device]{Items: []Device{{Id: "a:1"}, {Id: "b"}, {Id: "locked"}}, Total: 3}, webui.Ready()
			},
			Key:     func(d Device) string { return d.Id },
			Columns: []webui.Accessor[Device]{formID},
			Actions: []webui.Action[Device]{{
				Label: "Delete", Role: webui.RoleDestructive,
				Guard: func(_ context.Context, d Device) error {
					if d.Id == "locked" {
						return errors.New("locked by an administrator")
					}
					return nil
				},
				Run: func(_ context.Context, d Device) (webui.Outcome, error) {
					s.mu.Lock()
					s.deleted = append(s.deleted, d.Id)
					s.mu.Unlock()
					return webui.Success("Deleted " + d.Id), nil
				},
			}},
			BulkActions: []webui.Action[[]Device]{{
				Label: "Acknowledge",
				Run: func(_ context.Context, ds []Device) (webui.Outcome, error) {
					var ids []string
					for _, d := range ds {
						ids = append(ids, d.Id)
					}
					s.mu.Lock()
					s.bulk = append(s.bulk, ids)
					s.mu.Unlock()
					return webui.Success("Acknowledged"), nil
				},
			}},
		},
	}
	s.redirect = nil
	s.h = webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")
	return s
}

func formValues(kv ...string) url.Values {
	v := url.Values{"_leaf": {"p"}}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Add(kv[i], kv[i+1])
	}
	return v
}

func TestSubmitBindsFromLoadedModelThenRedirectsWithAFlash(t *testing.T) {
	t.Parallel()

	s := newShop()
	// Field names are positional: ID f0 (read-only), IP f1, Occurrences f2.
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))

	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev1") // a reload must not repeat the POST
	assert.DeepEqual(t, s.saved, []Device{{Id: "dev1", Ip: "10.0.0.9", Count: 5}})

	cookie := rec.Result().Cookies()[0]
	assert.Equal(t, cookie.Name, "webui_flash")
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, cookie.SameSite, http.SameSiteLaxMode)
	assert.Equal(t, cookie.Path, "/admin")

	// The next page shows the toast once and clears the cookie.
	req := httptest.NewRequest(http.MethodGet, "/admin/device/dev1", nil)
	req.AddCookie(cookie)
	got := httptest.NewRecorder()
	s.h.ServeHTTP(got, req)
	assert.Contains(t, got.Body.String(), `<div class="toast`)
	assert.Contains(t, got.Body.String(), "Device saved!")
	assert.Equal(t, got.Result().Cookies()[0].MaxAge, -1)

	again := serve(s.h, http.MethodGet, "/admin/device/dev1")
	assert.False(t, strings.Contains(again.Body.String(), "Device saved!"))
}

func TestRuleFailuresAreCollectedAndTheInputIsEchoed(t *testing.T) {
	t.Parallel()

	s := newShop()
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "not-an-ip", "f2", "500"))

	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity) // the page, with the form drawn again: not a redirect
	assert.Equal(t, len(s.saved), 0)                          // Run never ran
	body := rec.Body.String()
	assert.Contains(t, body, `value="not-an-ip"`) // raw input, not the model
	assert.Contains(t, body, `value="500"`)
	assert.Contains(t, body, "must be a valid IPv4 address")
	assert.Contains(t, body, "Must be at most 100.")
	assert.Contains(t, body, `class="input invalid"`)
	assert.Contains(t, body, "2 fields need attention.")
}

func TestServerSideRejectionLooksLikeARuleFailure(t *testing.T) {
	t.Parallel()

	s := newShop()
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.4.17", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	assert.Contains(t, body, "already in use by another device")
	assert.Contains(t, body, `value="10.0.4.17"`)
	assert.Contains(t, body, "1 field needs attention.")
}

func TestRunErrorIsARedirectThatNeverLeaks(t *testing.T) {
	t.Parallel()

	s := newShop()
	s.runErr = errors.New("connection refused to db.internal:5432")
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusSeeOther) // so that a reload does not run it again
	page := follow(s.h, rec)
	assert.Contains(t, page.Body.String(), "Something went wrong")
	assert.False(t, strings.Contains(rec.Body.String()+rec.Header().Get("Set-Cookie")+page.Body.String(), "db.internal"))
}

func TestGuardsRejectBeforeAnythingRuns(t *testing.T) {
	t.Parallel()

	s := newShop()
	s.pageGate = errors.New("no access to this device")
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusSeeOther) // to the page, which answers the same request with its 403
	page := follow(s.h, rec)
	assert.Equal(t, page.Code, http.StatusForbidden)
	assert.Contains(t, page.Body.String(), "no access to this device")
	assert.Equal(t, len(s.saved), 0)

	s2 := newShop()
	s2.subGate = errors.New("requires the editor role")
	rec = post(s2.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusSeeOther) // back to the page, with the reason in a toast
	assert.Contains(t, follow(s2.h, rec).Body.String(), "requires the editor role")
	assert.Equal(t, len(s2.saved), 0)
}

func TestCrossOriginPostsAreRefused(t *testing.T) {
	t.Parallel()

	s := newShop()
	form := formValues("f1", "10.0.0.9", "f2", "5")

	rec := post(s.h, "/admin/device/dev1", form, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") })
	assert.Equal(t, rec.Code, http.StatusForbidden)
	rec = post(s.h, "/admin/device/dev1", form, func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") })
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Equal(t, len(s.saved), 0)

	rec = post(s.h, "/admin/device/dev1", form, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin") })
	assert.Equal(t, rec.Code, http.StatusSeeOther)
}

func TestUnknownLeafAndOversizedBody(t *testing.T) {
	t.Parallel()

	s := newShop()
	bad := url.Values{"_leaf": {"p.9"}}
	rec := post(s.h, "/admin/device/dev1", bad)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, follow(s.h, rec).Body.String(), "That request was not understood.")
	assert.Equal(t, post(s.h, "/admin/device/dev1", url.Values{}).Code, http.StatusSeeOther)

	big := formValues("f1", strings.Repeat("x", 2<<20))
	rec = post(s.h, "/admin/device/dev1", big)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, follow(s.h, rec).Body.String(), "That submission was too large.")
	assert.Equal(t, len(s.saved), 0)
}

func TestWrongMethodAdvertisesPost(t *testing.T) {
	t.Parallel()

	s := newShop()
	rec := serve(s.h, http.MethodPut, "/admin/device/dev1")
	assert.Equal(t, rec.Code, http.StatusMethodNotAllowed)
	assert.Equal(t, rec.Header().Get("Allow"), "GET, HEAD, POST")
	// The list has actions too; a page with none would say GET, HEAD only.
	assert.Equal(t, serve(s.h, http.MethodPut, "/admin/device").Header().Get("Allow"), "GET, HEAD, POST")
}

func TestRedirectGoesWhereOpenSays(t *testing.T) {
	t.Parallel()

	s := newShop()
	var list webui.Page[webui.NoArgs]
	list.Path = "/device"
	s.redirect = func(ctx context.Context) webui.Target { return webui.Open(ctx, list, webui.NoArgs{}) }
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device")

	// A page that is not mounted cannot be redirected to: that is a mistake in Run.
	const gone webui.PageID[webui.NoArgs] = "/gone"
	s.redirect = func(ctx context.Context) webui.Target { return webui.Open(ctx, gone, webui.NoArgs{}) }
	rec = post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusSeeOther) // the cause is logged, and the page says something went wrong
	assert.Contains(t, follow(s.h, rec).Body.String(), "Something went wrong")
}

func TestTableRendersSelectionBarAndOmitsTheRowActionsItsGuardRefuses(t *testing.T) {
	t.Parallel()

	s := newShop()
	body := serve(s.h, http.MethodGet, "/admin/device").Body.String()

	assert.Contains(t, body, `<th scope="col" class="pick">`) // the checkbox column exists because bulk is declared
	assert.Equal(t, strings.Count(body, `name="_sel"`), 3)
	assert.Contains(t, body, `<div class="actionbar"`)
	assert.Contains(t, body, `name="_act" value="bulk:0">Acknowledge</button>`)
	assert.Contains(t, body, `name="_act" value="row:0:a:1"`) // keys may contain colons
	assert.Contains(t, body, `class="btn btn-danger btn-sm"`)

	// The locked row's Guard returns an error that says nothing of how to draw it: the action is not there, and its row
	// has an empty cell in the column that the other rows' actions are in.
	assert.Equal(t, strings.Count(body, `>Delete <span class="sr-only">`), 2)
	assert.False(t, strings.Contains(body, `value="row:0:locked"`))
	assert.Contains(t, body, `<td class="actions"></td>`)
	assert.False(t, strings.Contains(body, `class="gate"`))
}

func TestTableWithoutActionsHasNoFormNoCheckboxesNoBar(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	for _, absent := range []string{`class="pick"`, `actionbar`, `_sel`, `<form action="/admin/device"`} {
		assert.False(t, strings.Contains(body, absent))
	}
}

func TestBulkActionRunsOnTheSelectionInTableOrder(t *testing.T) {
	t.Parallel()

	s := newShop()
	form := formValues("_act", "bulk:0", "_sel", "b", "_sel", "a:1")
	rec := post(s.h, "/admin/device", form)
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, s.bulk, [][]string{{"a:1", "b"}}) // table order, not arrival order
}

func TestBulkActionRefusesAStaleSelection(t *testing.T) {
	t.Parallel()

	s := newShop()
	rec := post(s.h, "/admin/device", formValues("_act", "bulk:0", "_sel", "a:1", "_sel", "gone"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, len(s.bulk), 0)                         // nothing ran on a different set than the one the user saw
	assert.Contains(t, rec.Result().Cookies()[0].Value, "") // an error flash was set

	rec = post(s.h, "/admin/device", formValues("_act", "bulk:0"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, len(s.bulk), 0)
}

func TestRowActionFindsTheRowByKeyAndHonoursItsGuard(t *testing.T) {
	t.Parallel()

	s := newShop()
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "row:0:a:1")).Code, http.StatusSeeOther)
	assert.DeepEqual(t, s.deleted, []string{"a:1"})

	// The guard that disabled the button also rejects the request.
	rec := post(s.h, "/admin/device", formValues("_act", "row:0:locked"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, follow(s.h, rec).Body.String(), "toast err")
	assert.DeepEqual(t, s.deleted, []string{"a:1"})

	// A row that vanished, an action that does not exist, a malformed value.
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "row:0:ghost")).Code, http.StatusSeeOther)
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "row:7:a:1")).Code, http.StatusSeeOther)
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "nonsense")).Code, http.StatusSeeOther)
	assert.DeepEqual(t, s.deleted, []string{"a:1"})
}

func TestForgedFlashCookiesAreInertAndCleared(t *testing.T) {
	t.Parallel()

	s := newShop()
	for name, value := range map[string]string{
		"not base64":  "%%%",
		"too short":   "dA", // "t"
		"not utf-8":   "dP8",
		"markup":      "dDxzY3JpcHQ-YWxlcnQoMSk8L3NjcmlwdD4", // t<script>alert(1)</script>
		"unknown tag": "eDo",
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin/device/dev1", nil)
		req.Header.Set("Cookie", "webui_flash="+value)
		rec := httptest.NewRecorder()
		s.h.ServeHTTP(rec, req)

		assert.Equal(t, rec.Code, http.StatusOK)
		assert.False(t, strings.Contains(rec.Body.String(), "<script>alert"))
		assert.Equal(t, rec.Result().Cookies()[0].MaxAge, -1)
		_ = name
	}

	// Markup in a toast is text.
	req := httptest.NewRequest(http.MethodGet, "/admin/device/dev1", nil)
	req.Header.Set("Cookie", "webui_flash=dDxzY3JpcHQ-YWxlcnQoMSk8L3NjcmlwdD4")
	rec := httptest.NewRecorder()
	s.h.ServeHTTP(rec, req)
	assert.Contains(t, rec.Body.String(), "&lt;script&gt;alert(1)&lt;/script&gt;")
}

func TestHeadDoesNotConsumeTheFlash(t *testing.T) {
	t.Parallel()

	s := newShop()
	req := httptest.NewRequest(http.MethodHead, "/admin/device/dev1", nil)
	req.Header.Set("Cookie", "webui_flash=dGhpcw")
	rec := httptest.NewRecorder()
	s.h.ServeHTTP(rec, req)
	assert.Equal(t, len(rec.Result().Cookies()), 0)
}
