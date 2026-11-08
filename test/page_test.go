package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type pageArgs struct {
	Id    string
	Debug bool
	Q     string
	N     int
}

// app builds the README's two-page shape with empty bodies: a list that owns
// a nav entry and a detail page below it, which has none of its own.
func app(guard func(context.Context, pageArgs) error) (webui.App, webui.Page[pageArgs], webui.Page[webui.NoArgs]) {
	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices", Section: "Platform"}, View: webui.Stack{}}
	details := webui.Page[pageArgs]{Path: "/device/{id}", Guard: guard, View: webui.Stack{}}
	return webui.App{Brand: webui.Brand{Name: "Demo"}, Pages: webui.Pages{list, details}}, details, list
}

func TestPageRendersNavBreadcrumbAndTheAncestorsEntry(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	rec := serve(a.MustCompile("/admin"), http.MethodGet, "/admin/device/abc")
	assert.Equal(t, rec.Code, http.StatusOK)
	p := page.Parse(t, rec.Body.String())

	// The detail page has no entry; the list's lights up, as its ancestor.
	assert.DeepEqual(t, outline(t, p), []string{"# Platform", "Devices"})
	assert.DeepEqual(t, sidebarNav(t, p).Links(), []page.Link{{Name: "Devices", Href: "/admin/device", Current: true}})

	// Breadcrumbs: brand › the list page's label (a link) › the path value.
	assert.DeepEqual(t, trail(t, p), []string{"Demo>/admin/device", "Devices>/admin/device", "abc"})
	assert.Equal(t, p.Title(), "abc — Demo")
	assert.Equal(t, page.One(t, "the address in the status bar", p.ByAttr("data-url", "")).Text(), "/admin/device/abc")
}

func TestThereIsNoArgumentsRowAndRefreshLivesInTheTopBar(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	h := a.MustCompile("/admin")
	p := pageOf(t, h, "/admin/device/abc?debug=true&q=x&n=5")

	// Page arguments are read from the address and have no controls of their own.
	assert.Equal(t, len(p.ByAttr("data-toolbar", "")), 0)
	for _, absent := range []string{"Clear all", "declares no arguments"} {
		assert.False(t, strings.Contains(p.Root().AllText(), absent))
	}

	// Refresh sits in the top bar, a link to the address the browser is on.
	top := page.One(t, "top bar", p.ByRole("banner", ""))
	refresh := page.One(t, "the Refresh link", top.ByRole("link", "Refresh"))
	assert.Equal(t, refresh.Attr("href"), "/admin/device/abc?debug=true&q=x&n=5")
	assert.True(t, refresh.Has("data-refresh"))
	assert.Equal(t, len(top.ByAttr("data-palette-input", "")), 0) // no page has a Search, so there is no search box
	assert.Equal(t, len(p.ByAttr("data-refresh", "")), 1)
}

func TestGuardRunsBeforeAnythingAndRejectsWith403(t *testing.T) {
	t.Parallel()

	var seen pageArgs
	a, _, _ := app(func(ctx context.Context, args pageArgs) error {
		assert.Equal(t, webui.ArgsOf[pageArgs](ctx), args)
		seen = args
		if args.Id == "locked" {
			return errors.New("requires the editor role")
		}
		return nil
	})
	h := a.MustCompile("/admin")

	assert.Equal(t, serve(h, http.MethodGet, "/admin/device/open?debug=true&n=3").Code, http.StatusOK)
	assert.Equal(t, seen, pageArgs{Id: "open", Debug: true, N: 3})

	rec := serve(h, http.MethodGet, "/admin/device/locked")
	assert.Equal(t, rec.Code, http.StatusForbidden)
	assert.Contains(t, rec.Body.String(), "Not permitted")
	assert.Contains(t, rec.Body.String(), "requires the editor role")
	assert.False(t, strings.Contains(rec.Body.String(), "ran before anything was loaded")) // how the check was made is not for the visitor
}

func TestBadArgumentIs400(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	rec := serve(a.MustCompile("/admin"), http.MethodGet, "/admin/device/abc?n=lots")
	assert.Equal(t, rec.Code, http.StatusBadRequest)
	assert.Contains(t, rec.Body.String(), "That address is not valid")
}

func TestRootRedirectsToFirstNavEntryOrServesRootPage(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	rec := serve(a.MustCompile("/admin"), http.MethodGet, "/admin/")
	assert.Equal(t, rec.Code, http.StatusFound)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/device")

	home := webui.Page[webui.NoArgs]{Path: "/", Nav: webui.Nav{Label: "Home"}, View: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{home}}.MustCompile("/admin")
	for _, target := range []string{"/admin", "/admin/"} {
		rec := serve(h, http.MethodGet, target)
		assert.Equal(t, rec.Code, http.StatusOK)
		got := trail(t, page.Parse(t, rec.Body.String()))
		assert.Equal(t, got[len(got)-1], "Home") // the landing page is where the reader is
	}
}

func TestHeadAndWrongMethod(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	h := a.MustCompile("/admin")
	assert.Equal(t, serve(h, http.MethodHead, "/admin/device").Code, http.StatusOK)
	assert.Equal(t, serve(h, http.MethodDelete, "/admin/device").Code, http.StatusMethodNotAllowed)
}
