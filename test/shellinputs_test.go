package webui_test

import (
	"context"
	"net/http"
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
	home := webui.Page[webui.NoArgs]{Path: "/home", Nav: webui.Nav{Label: "Home"}, Body: webui.Table[Device]{
		Title: "Devices", Columns: []webui.Accessor[Device]{formID},
		Load: func(ctx context.Context, _ webui.Query) (webui.Window[Device], error) {
			meet(ctx, &arrived, 3, &met)
			return webui.Window[Device]{Items: []Device{{Id: "row"}}, Total: 1}, nil
		},
	}}
	guarded := webui.Page[webui.NoArgs]{Path: "/guarded", Nav: webui.Nav{Label: "Guarded"}, Body: webui.Stack{},
		Guard: func(ctx context.Context, _ webui.NoArgs) error { meet(ctx, &arrived, 3, &met); return nil }}
	site := webui.Page[navSiteArgs]{Path: "/site/{name}", Nav: webui.Nav{Section: "Sites"}, Body: webui.Stack{},
		NavEntries: func(ctx context.Context) ([]webui.NavEntry[navSiteArgs], error) {
			meet(ctx, &arrived, 3, &met)
			return []webui.NavEntry[navSiteArgs]{{Args: navSiteArgs{Name: "a"}, Label: "Site A"}}, nil
		}}
	h := webui.App{Pages: webui.Pages{home, guarded, site}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/home")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, arrived.Load() >= 3)
	assert.Equal(t, met.Load(), arrived.Load()) // each of them found the others asked for
	body := rec.Body.String()
	assert.Contains(t, body, "Site A") // and the sidebar has what they answered
	assert.Contains(t, body, "Guarded")
}
