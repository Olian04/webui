package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type layoutArgs struct {
	Id string
	Q  string
}

// layoutApp is the README's nested body: a Split of a form and a table, then
// Tabs. It counts loads so a test can prove which leaves were read.
func layoutApp(guard func(context.Context, layoutArgs) error) (http.Handler, *[4]atomic.Int32) {
	var loads [4]atomic.Int32
	table := func(i int, title string) webui.Table[Device] {
		return webui.Table[Device]{
			Title: title,
			Load: func(context.Context, webui.Query) (webui.Rows[Device], error) {
				loads[i].Add(1)
				return webui.Rows[Device]{Items: []Device{{Id: title + "-row"}}, Total: 1}, nil
			},
			Columns: []webui.Accessor[Device]{formID},
		}
	}
	page := webui.Page[layoutArgs]{
		Path:  "/device/{id}",
		Guard: guard,
		Body: webui.Stack{
			webui.Split{
				webui.Form[Device]{
					Title:  "Form",
					Load:   func(context.Context) (Device, error) { loads[0].Add(1); return Device{Id: "x"}, nil },
					Fields: []webui.Accessor[Device]{formID},
				},
				table(1, "Events"),
			},
			webui.Tabs{ID: "view", Panels: []webui.Tab{
				{Label: "Overview", Body: table(2, "Overview")},
				{Label: "Raw", Body: table(3, "Raw")},
			}},
		},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin"), &loads
}

func TestTabsLoadOnlyTheSelectedPanel(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)

	body := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	assert.Contains(t, body, "Overview-row")
	assert.False(t, strings.Contains(body, "Raw-row"))
	assert.Equal(t, loads[2].Load(), int32(1))
	assert.Equal(t, loads[3].Load(), int32(0)) // the hidden tab was never read

	// The strip is links; the first needs no argument, the others carry theirs.
	assert.Contains(t, body, `<a class="tab active" href="/admin/device/d1" aria-current="page">Overview</a>`)
	assert.Contains(t, body, `<a class="tab" href="/admin/device/d1?view.tab=Raw">Raw</a>`)

	raw := serve(h, http.MethodGet, "/admin/device/d1?view.tab=Raw").Body.String()
	assert.Contains(t, raw, "Raw-row")
	assert.False(t, strings.Contains(raw, "Overview-row"))
	assert.Equal(t, loads[3].Load(), int32(1))
	assert.Contains(t, raw, `<a class="tab active" href="/admin/device/d1?view.tab=Raw" aria-current="page">Raw</a>`)

	// An unknown tab lands on the first rather than on nothing.
	stale := serve(h, http.MethodGet, "/admin/device/d1?view.tab=Gone").Body.String()
	assert.Contains(t, stale, "Overview-row")
}

func TestTabStateIsNotAToolbarPill(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	body := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	assert.Equal(t, strings.Count(body, `class="var"`), 1) // Q only
	assert.False(t, strings.Contains(body, `aria-label="Tab"`))
}

func TestLayoutsGiveLeavesPositionalIDs(t *testing.T) {
	t.Parallel()

	h, _ := layoutApp(nil)
	body := serve(h, http.MethodGet, "/admin/device/d1").Body.String()
	for _, id := range []string{`data-leaf="p.0.0"`, `data-leaf="p.0.1"`, `data-leaf="p.1.0"`} {
		assert.Contains(t, body, id)
	}
	assert.Contains(t, body, `class="split"`)
	assert.Contains(t, body, `class="stack"`)
}

func TestLeafRequestReturnsOnlyThatPanel(t *testing.T) {
	t.Parallel()

	h, loads := layoutApp(nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/device/d1", nil)
	req.Header.Set("X-Webui-Leaf", "p.0.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.True(t, strings.HasPrefix(body, `<div class="panel" data-leaf="p.0.1">`))
	assert.False(t, strings.Contains(body, "<html"))
	assert.Contains(t, body, "Events-row")
	assert.Contains(t, rec.Header().Get("Vary"), "X-Webui-Leaf")
	assert.Equal(t, rec.Header().Get("Cache-Control"), "no-store")
	// Only that leaf was read; the rest of the screen holds still.
	assert.Equal(t, loads[0].Load(), int32(0))
	assert.Equal(t, loads[1].Load(), int32(1))
	assert.Equal(t, loads[2].Load(), int32(0))

	// A full page says it varies on the header too, so no cache serves one for the other.
	assert.Contains(t, serve(h, http.MethodGet, "/admin/device/d1").Header().Get("Vary"), "X-Webui-Leaf")
}

func TestLeafRequestStillRunsThePageGuardAndRejectsUnknownIDs(t *testing.T) {
	t.Parallel()

	var denied bool
	h, loads := layoutApp(func(context.Context, layoutArgs) error {
		if denied {
			return errors.New("not for you")
		}
		return nil
	})

	ask := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/admin/device/d1", nil)
		req.Header.Set("X-Webui-Leaf", id)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, ask("p.9").Code, http.StatusNotFound)
	assert.Equal(t, ask("p.0").Code, http.StatusNotFound) // a layout is not a leaf
	assert.Equal(t, ask("../../etc").Code, http.StatusNotFound)

	denied = true
	rec := ask("p.0.1") // the leaf URL is the page URL: the same Guard runs first
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), "not for you")
	assert.Equal(t, loads[1].Load(), int32(0))
}
