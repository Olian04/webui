package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type navSiteArgs struct{ Name string }

// A page's panels, the Guard of another navigation page, and the entries that a page lists in the sidebar are
// asked for at the same time: each waits until the others have started, which one after another they never would.
func TestThePanelsTheGuardsAndTheNavEntriesAreAskedForAtTheSameTime(t *testing.T) {
	t.Parallel()

	var arrived, met atomic.Int32
	home := webui.Page[webui.NoArgs]{Path: "/home", View: webui.Table[Device]{
		Title: "Devices", Columns: []webui.Accessor[Device]{formID},
		Loader: webui.PagedLoader[Device]{Load: func(ctx context.Context, _ webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
			meet(ctx, &arrived, 3, &met)
			return webui.PagedWindow[Device]{Items: []Device{{Id: "row"}}, Total: 1}, webui.Ready()
		}},
	}}
	guarded := webui.Page[webui.NoArgs]{Path: "/guarded", View: webui.Stack{},
		Guard: func(ctx context.Context, _ webui.NoArgs) error { meet(ctx, &arrived, 3, &met); return nil }}
	site := webui.Page[navSiteArgs]{Path: "/site/{name}", View: webui.Stack{}}
	sites := func(ctx context.Context) ([]navSiteArgs, webui.Loaded) {
		meet(ctx, &arrived, 3, &met)
		return []navSiteArgs{{Name: "a"}}, webui.Ready()
	}
	h := webui.App{Pages: webui.Pages{home, guarded, site}, Navigation: []webui.NavItem{
		webui.NavPage{Page: home, Label: "Home"},
		webui.NavPage{Page: guarded, Label: "Guarded"},
		webui.NavGroup{Label: "Sites", Items: []webui.NavItem{webui.NavPages[navSiteArgs]{Page: site, Load: sites, Label: func(a navSiteArgs) string { return "Site " + strings.ToUpper(a.Name) }}}},
	}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/home")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, arrived.Load() >= 3)
	assert.Equal(t, met.Load(), arrived.Load()) // each of them found the others asked for
	body := rec.Body.String()
	assert.Contains(t, body, "Site A") // and the sidebar has what they answered
	assert.Contains(t, body, "Guarded")
}
