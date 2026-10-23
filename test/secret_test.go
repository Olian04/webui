package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type credential struct {
	Name string
	Key  string
}

// credentialApp is a form of a credential that holds a key: what it saved is what its Run was given.
func credentialApp(start credential) (http.Handler, func() credential) {
	var (
		mu    sync.Mutex
		saved = start
	)
	form := webui.Form[credential]{
		Load: func(context.Context) (credential, webui.Loaded) {
			mu.Lock()
			defer mu.Unlock()
			return saved, webui.Ready()
		},
		Fields: []webui.Accessor[credential]{
			webui.String[credential]{
				Label: "Name", Rules: webui.StringRules{Required: true, MaxLen: 20},
				Load: func(c credential) string { return c.Name }, Store: func(c *credential, v string) { c.Name = v },
			},
			webui.String[credential]{
				Label: "API key", Secret: true, Rules: webui.StringRules{MinLen: 8},
				Load: func(c credential) string { return c.Key }, Store: func(c *credential, v string) { c.Key = v },
			},
		},
		Submit: webui.Action[credential]{Run: func(_ context.Context, c credential) (webui.Outcome, error) {
			mu.Lock()
			saved = c
			mu.Unlock()
			return webui.Success("Saved"), nil
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/c", Body: form}}}.MustCompile("/admin")
	return h, func() credential {
		mu.Lock()
		defer mu.Unlock()
		return saved
	}
}

func TestASecretIsAPasswordInputThatNeverHoldsTheStoredValue(t *testing.T) {
	t.Parallel()

	h, _ := credentialApp(credential{Name: "prod", Key: "sk-live-1234567890"})
	body := serve(h, http.MethodGet, "/admin/c").Body.String()

	assert.Contains(t, body, `type="password"`)
	assert.Contains(t, body, `autocomplete="new-password"`)
	// That there is one is shown, as the same bullets whatever it is, and not what it is.
	assert.Contains(t, body, `placeholder="••••••••"`)
	assert.False(t, strings.Contains(body, "sk-live-1234567890"))
	assert.Contains(t, body, `<span class="tip-item">Leave empty to keep the current value</span>`)
	assert.Contains(t, body, `minlength="8"`) // its rules are the browser's too
	// The one that is not a secret is as it was.
	assert.Contains(t, body, `name="f0" type="text" value="prod"`)
}

// With nothing stored there is nothing to show but the field's own placeholder, and the bullets are the same for a short value and a long one.
func TestASecretShowsBulletsOnlyWhenThereIsOneAndTheSameWhateverItsLength(t *testing.T) {
	t.Parallel()

	none, _ := credentialApp(credential{Name: "prod"})
	assert.False(t, strings.Contains(serve(none, http.MethodGet, "/admin/c").Body.String(), "••••"))

	short, _ := credentialApp(credential{Name: "prod", Key: "k"})
	long, _ := credentialApp(credential{Name: "prod", Key: strings.Repeat("sk-live-", 20)})
	for _, h := range []http.Handler{short, long} {
		body := serve(h, http.MethodGet, "/admin/c").Body.String()
		assert.Equal(t, strings.Count(body, "••••••••"), 1)
		assert.False(t, strings.Contains(body, "••••••••••"))
	}

	// After a refusal the form is drawn again, and still says that there is one.
	h, _ := credentialApp(credential{Name: "prod", Key: "sk-live-1234567890"})
	rec := post(h, "/admin/c", formValues("f0", "a name that is far too long to be accepted", "f1", ""))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), `placeholder="••••••••"`)
}

func TestAnEmptySubmissionLeavesASecretAndATypedOneStoresIt(t *testing.T) {
	t.Parallel()

	h, saved := credentialApp(credential{Name: "prod", Key: "sk-live-1234567890"})

	rec := post(h, "/admin/c", formValues("f0", "staging", "f1", ""))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, saved(), credential{Name: "staging", Key: "sk-live-1234567890"})

	rec = post(h, "/admin/c", formValues("f0", "staging", "f1", "sk-test-abcdefgh"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, saved().Key, "sk-test-abcdefgh")

	// What is typed is held to its rules, as anything is; what is not typed is not.
	rec = post(h, "/admin/c", formValues("f0", "staging", "f1", "short"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Must be at least 8 characters.")
	assert.Equal(t, saved().Key, "sk-test-abcdefgh")
}

// A form that is answered because something else was refused is drawn again with what was typed, which is not a secret.
func TestASecretIsNotEchoedInTheFormThatAnswersARefusal(t *testing.T) {
	t.Parallel()

	h, saved := credentialApp(credential{Name: "prod", Key: "sk-live-1234567890"})
	rec := post(h, "/admin/c", formValues("f0", "a name that is far too long to be accepted", "f1", "sk-typed-secret-0001"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	assert.Contains(t, body, "Must be at most 20 characters.")
	assert.Contains(t, body, `value="a name that is far too long to be accepted"`) // the one that is not a secret is kept
	assert.False(t, strings.Contains(body, "sk-typed-secret-0001"))
	assert.False(t, strings.Contains(body, "sk-live-1234567890"))
	assert.Equal(t, saved().Key, "sk-live-1234567890") // and nothing was stored
}

// Nothing reads a secret through anything but Get, so a search finds nothing in it.
func TestASecretIsNotSearchable(t *testing.T) {
	t.Parallel()

	secret := webui.String[Device]{Label: "Key", Secret: true, Load: func(d Device) string { return d.Ip }, Store: func(*Device, string) {}}
	h := searchApp(webui.SearchRows(
		func(context.Context) ([]Device, webui.Loaded) { return searchDevices(), webui.Ready() },
		deviceHit, secret,
	), nil)
	assert.Equal(t, len(search(t, h, "10.0.0.1").Results), 0)
}

func TestASecretIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	load := func(c credential) string { return c.Key }
	store := func(*credential, string) {}
	problems := func(body webui.PageBody) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/c", Body: body}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	form := func(s webui.String[credential]) webui.PageBody {
		return webui.Form[credential]{
			Load:   func(context.Context) (credential, webui.Loaded) { return credential{}, webui.Ready() },
			Fields: []webui.Accessor[credential]{s},
			Submit: webui.Action[credential]{Run: func(context.Context, credential) (webui.Outcome, error) { return webui.Outcome{}, nil }},
		}
	}
	assert.Contains(t, problems(form(webui.String[credential]{Label: "Key", Secret: true, Load: load})), "is a Secret but has no Store")
	assert.Contains(t, problems(form(webui.String[credential]{Label: "Key", Secret: true, Load: load, Store: store, Rules: webui.StringRules{Required: true}})), "is a Secret and Required")
	assert.Contains(t, problems(webui.Table[credential]{
		Rows:    func(context.Context) ([]credential, webui.Loaded) { return nil, webui.Ready() },
		Columns: []webui.Accessor[credential]{webui.String[credential]{Label: "Key", Secret: true, Load: load, Store: store}},
	}), "is a Secret in a table")
	assert.Contains(t, problems(webui.Editor[credential]{
		Load:    func(context.Context) (credential, webui.Loaded) { return credential{}, webui.Ready() },
		Content: webui.String[credential]{Label: "Key", Secret: true, Load: load, Store: store},
	}), "is a Secret in an editor")
	assert.Contains(t, problems(webui.Toolbar[webui.NoArgs]{Left: []webui.ToolbarItem[webui.NoArgs]{
		webui.String[webui.NoArgs]{Label: "Key", Secret: true, Load: func(webui.NoArgs) string { return "" }, Store: func(*webui.NoArgs, string) {}},
	}}), "is a Secret in a toolbar")
}
