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
		Rules: webui.StringRules{Required: true, MinLen: 7, Pattern: &webui.PatternRule{Expr: `\d+(\.\d+){3}`, Message: "must be a valid IPv4 address"}},
	}
	count := webui.Int[Device]{
		Label: "Occurrences", Load: func(d Device) int { return d.Count }, Store: func(d *Device, v int) { d.Count = v },
		Rules: webui.NumberRules[int]{Max: ptr(100)},
	}

	var list webui.Page[webui.NoArgs]
	details := webui.Page[detailsArgs]{
		Path: "/device/{id}",
		Guard: func(context.Context, detailsArgs) error {
			return s.pageGate
		},
		Body: webui.Form[Device]{
			Title: "Device",
			Load: func(ctx context.Context) (Device, error) {
				a := webui.ArgsOf[detailsArgs](ctx)
				return Device{Id: a.Id, Ip: "10.0.0.1", Count: 3}, nil
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
			Load: func(context.Context, webui.Query) (webui.Rows[Device], error) {
				return webui.Rows[Device]{Items: []Device{{Id: "a:1"}, {Id: "b"}, {Id: "locked"}}, Total: 3}, nil
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

	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Equal(t, len(s.saved), 0) // Run never ran
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
	assert.Contains(t, rec.Body.String(), "already in use by another device")
	assert.Contains(t, rec.Body.String(), `value="10.0.4.17"`)
	assert.Contains(t, rec.Body.String(), "1 field needs attention.")
}

func TestRunErrorIs500AndNeverLeaks(t *testing.T) {
	t.Parallel()

	s := newShop()
	s.runErr = errors.New("connection refused to db.internal:5432")
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.False(t, strings.Contains(rec.Body.String(), "db.internal"))
}

func TestGuardsRejectBeforeAnythingRuns(t *testing.T) {
	t.Parallel()

	s := newShop()
	s.pageGate = errors.New("no access to this device")
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), "no access to this device")
	assert.Equal(t, len(s.saved), 0)

	s2 := newShop()
	s2.subGate = errors.New("requires the editor role")
	rec = post(s2.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Code, http.StatusForbidden)
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
	assert.Equal(t, post(s.h, "/admin/device/dev1", bad).Code, http.StatusBadRequest)
	assert.Equal(t, post(s.h, "/admin/device/dev1", url.Values{}).Code, http.StatusBadRequest)

	big := formValues("f1", strings.Repeat("x", 2<<20))
	assert.Equal(t, post(s.h, "/admin/device/dev1", big).Code, http.StatusRequestEntityTooLarge)
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

func TestRedirectGoesWhereOpenSaysAndNowhereElse(t *testing.T) {
	t.Parallel()

	s := newShop()
	var list webui.Page[webui.NoArgs]
	list.Path = "/device"
	s.redirect = func(ctx context.Context) webui.Target { return webui.Open(ctx, list, webui.NoArgs{}) }
	rec := post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device")

	for _, evil := range []string{"https://evil.example/", "//evil.example/", `/\evil.example`} {
		s.redirect = func(context.Context) webui.Target { return webui.Target{URL: evil} }
		rec = post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5"))
		assert.Equal(t, rec.Header().Get("Location"), "/admin/device/dev1") // stays on this host
	}

	s.redirect = func(context.Context) webui.Target { return webui.Target{Err: errors.New("nope")} }
	assert.Equal(t, post(s.h, "/admin/device/dev1", formValues("f1", "10.0.0.9", "f2", "5")).Code, http.StatusInternalServerError)
}

func TestTableRendersSelectionBarAndGatedRowActions(t *testing.T) {
	t.Parallel()

	s := newShop()
	body := serve(s.h, http.MethodGet, "/admin/device").Body.String()

	assert.Contains(t, body, `<th class="pick">`) // the checkbox column exists because bulk is declared
	assert.Equal(t, strings.Count(body, `name="_sel"`), 3)
	assert.Contains(t, body, `<div class="actionbar"`)
	assert.Contains(t, body, `name="_act" value="bulk:0">Acknowledge</button>`)
	assert.Contains(t, body, `name="_act" value="row:0:a:1"`) // keys may contain colons
	assert.Contains(t, body, `class="btn btn-danger btn-sm"`)

	// The locked row's action is disabled, with the reason in a tooltip.
	assert.Contains(t, body, `<span class="gate" data-guard="locked by an administrator">`)
	assert.Equal(t, strings.Count(body, `class="gate"`), 1)
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
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "row:0:locked")).Code, http.StatusForbidden)
	assert.DeepEqual(t, s.deleted, []string{"a:1"})

	// A row that vanished, an action that does not exist, a malformed value.
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "row:0:ghost")).Code, http.StatusSeeOther)
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "row:7:a:1")).Code, http.StatusBadRequest)
	assert.Equal(t, post(s.h, "/admin/device", formValues("_act", "nonsense")).Code, http.StatusBadRequest)
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
