package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func settingsApp(allow func(context.Context) error, items ...webui.MenuItem) http.Handler {
	page := func(path string, guard func(context.Context, webui.NoArgs) error) webui.Page[webui.NoArgs] {
		return webui.Page[webui.NoArgs]{Path: webui.PageID[webui.NoArgs](path), Nav: webui.Nav{Label: path[1:]}, Guard: guard, Body: webui.Stack{}}
	}
	if allow == nil {
		allow = func(context.Context) error { return nil }
	}
	return webui.App{
		Settings: items,
		Pages: webui.Pages{
			page("/device", nil),
			page("/account", func(ctx context.Context, _ webui.NoArgs) error { return allow(ctx) }),
		},
	}.MustCompile("/admin")
}

func TestThereIsNoCogWithoutSettings(t *testing.T) {
	t.Parallel()

	body := serve(settingsApp(nil), http.MethodGet, "/admin/device").Body.String()
	assert.False(t, strings.Contains(body, `aria-label="Settings"`))
}

func TestTheCogOpensAListOfTheAppsSettings(t *testing.T) {
	t.Parallel()

	h := settingsApp(func(context.Context) error { return nil },
		webui.MenuItem{Label: "Account", Icon: "user", URL: "/account"},
		webui.MenuItem{Label: "Sign out", URL: "/logout"},
		webui.MenuItem{Label: "Help", Icon: "book", URL: "https://example.com/help"},
	)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()

	assert.Contains(t, body, `<summary class="iconbtn" title="Settings" aria-label="Settings">`)
	// A page of the app gets the mount prefix, any other path is used as written, and
	// so is an address elsewhere.
	assert.Contains(t, body, `<a class="menu-item" href="/admin/account"><i class="fa-solid fa-user menu-icon" aria-hidden="true"></i><span>Account</span></a>`)
	assert.Contains(t, body, `<a class="menu-item" href="/logout"><span>Sign out</span></a>`)
	assert.Contains(t, body, `href="https://example.com/help"`)

	// It sits after the page's Refresh, at the right of the top bar.
	assert.True(t, strings.Index(body, "data-refresh") < strings.Index(body, `aria-label="Settings"`))
}

func TestAnEntryForAPageTheVisitorMayNotOpenIsLeftOut(t *testing.T) {
	t.Parallel()

	h := settingsApp(func(context.Context) error { return errors.New("requires the admin role") },
		webui.MenuItem{Label: "Account", URL: "/account"},
		webui.MenuItem{Label: "Help", URL: "https://example.com/help"},
	)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.False(t, strings.Contains(body, `<span>Account</span>`))
	assert.Contains(t, body, `<span>Help</span>`) // not a page of the app: nothing to refuse

	// With nothing left to show there is no cog.
	only := settingsApp(func(context.Context) error { return errors.New("no") }, webui.MenuItem{Label: "Account", URL: "/account"})
	assert.False(t, strings.Contains(serve(only, http.MethodGet, "/admin/device").Body.String(), `aria-label="Settings"`))
}

func TestACogEntryMustHaveALabelAndAnAddressThatIsNotAScript(t *testing.T) {
	t.Parallel()

	app := func(item webui.MenuItem) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Settings: []webui.MenuItem{item}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	assert.Contains(t, app(webui.MenuItem{URL: "/x"}), "App.Settings[0] has no Label")
	assert.Contains(t, app(webui.MenuItem{Label: "x", Icon: "rocket-ship", URL: "/x"}), `"rocket-ship", which is not a Font Awesome`)
	for _, evil := range []string{"", "javascript:alert(1)", "//evil.example/", `/\evil.example`, "data:text/html,x", "ftp://example.com", "relative/path", "https://"} {
		assert.Contains(t, app(webui.MenuItem{Label: "x", URL: evil}), "which is not a path on this site")
	}
	for _, ok := range []string{"/x", "/x?y=1", "http://example.com", "https://example.com/a?b=c", "mailto:ops@example.com"} {
		_, err := webui.App{Settings: []webui.MenuItem{{Label: "x", URL: ok}}}.Compile("")
		assert.NoError(t, err)
	}
}
