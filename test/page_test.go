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

type pageArgs struct {
	Id    string
	Debug bool
	Q     string
	N     int
}

// app builds the README's two-page shape with empty bodies: a list that owns
// a nav entry and a detail page that borrows it.
func app(guard func(context.Context, pageArgs) error) (webui.App, webui.Page[pageArgs], webui.Page[webui.NoArgs]) {
	devices := webui.Nav{Label: "Devices", Section: "Platform"}
	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: devices, Body: webui.Stack{}}
	details := webui.Page[pageArgs]{Path: "/device/{id}", Nav: webui.Nav{Shadow: &devices}, Guard: guard, Body: webui.Stack{}}
	return webui.App{Brand: webui.Brand{Name: "Demo"}, Pages: webui.Pages{list, details}}, details, list
}

func TestPageRendersNavBreadcrumbAndShadow(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	rec := serve(a.MustCompile("/admin"), http.MethodGet, "/admin/device/abc")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()

	// The detail page has no entry; the list's lights up through Shadow.
	assert.Contains(t, body, `<div class="nav-section">Platform</div>`)
	assert.Contains(t, body, `<a class="nav-item active" href="/admin/device"`)
	assert.Equal(t, strings.Count(body, `class="nav-item`), 1)

	// Breadcrumbs: brand › the list page's label (a link) › the path value.
	assert.Contains(t, body, `<a href="/admin/">Demo</a>`)
	assert.Contains(t, body, `<a href="/admin/device">Devices</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">abc</span>`)
	assert.Contains(t, body, "<title>abc — Demo</title>")
	assert.Contains(t, body, `<span class="url" data-url>/admin/device/abc</span>`)
}

func TestThereIsNoArgumentsRowAndRefreshLivesInTheTopBar(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	h := a.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/device/abc?debug=true&q=x&n=5").Body.String()

	// Page arguments are read from the address and have no controls of their own.
	for _, absent := range []string{`data-toolbar`, `class="toolbar"`, `class="var"`, "Clear all", "declares no arguments"} {
		assert.False(t, strings.Contains(body, absent))
	}

	// Refresh sits in the top bar, a link to the address the browser is on.
	top := body[strings.Index(body, `<header class="topbar">`):strings.Index(body, "</header>")]
	assert.Contains(t, top, `class="iconbtn" href="/admin/device/abc?debug=true&amp;q=x&amp;n=5" title="Refresh" aria-label="Refresh" data-refresh`)
	assert.Contains(t, top, `data-palette-input`) // next to the global search
	assert.Equal(t, strings.Count(body, "data-refresh"), 1)
}

func TestGuardRunsBeforeAnythingAndRejectsWith403(t *testing.T) {
	t.Parallel()

	var seen pageArgs
	a, _, _ := app(func(ctx context.Context, args pageArgs) error {
		got, err := webui.ArgsOf[pageArgs](ctx)
		assert.NoError(t, err)
		assert.Equal(t, got, args)
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
	assert.Contains(t, rec.Body.String(), "ran before anything was loaded")
	assert.Contains(t, rec.Body.String(), "requires the editor role")
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

	home := webui.Page[webui.NoArgs]{Path: "/", Nav: webui.Nav{Label: "Home"}, Body: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{home}}.MustCompile("/admin")
	for _, target := range []string{"/admin", "/admin/"} {
		rec := serve(h, http.MethodGet, target)
		assert.Equal(t, rec.Code, http.StatusOK)
		assert.Contains(t, rec.Body.String(), `<span class="cur" aria-current="page">Home</span>`)
	}
}

func TestHeadAndWrongMethod(t *testing.T) {
	t.Parallel()

	a, _, _ := app(nil)
	h := a.MustCompile("/admin")
	assert.Equal(t, serve(h, http.MethodHead, "/admin/device").Code, http.StatusOK)
	assert.Equal(t, serve(h, http.MethodDelete, "/admin/device").Code, http.StatusMethodNotAllowed)
}

func TestOpenResolvesThroughTheRuntime(t *testing.T) {
	t.Parallel()

	var (
		mounted, unmounted, mutated, empty webui.Target
		details                            webui.Page[pageArgs]
	)
	a, d, _ := app(func(ctx context.Context, _ pageArgs) error {
		mounted = webui.Open(ctx, details, pageArgs{Id: "abc", Debug: true})
		unmounted = webui.Open(ctx, webui.Page[pageArgs]{Path: "/never-mounted/{id}"}, pageArgs{Id: "x"})
		changed := details
		changed.Path = "/CHANGED/{id}"
		mutated = webui.Open(ctx, changed, pageArgs{Id: "x"})
		empty = webui.Open(ctx, details, pageArgs{})
		return nil
	})
	details = d
	serve(a.MustCompile("/admin"), http.MethodGet, "/admin/device/zzz")

	assert.NoError(t, mounted.Err)
	assert.Equal(t, mounted.URL, "/admin/device/abc?debug=true")
	assert.Equal(t, unmounted.Err.Error(), `webui: Open "/never-mounted/{id}": that page is not mounted in this app`)
	assert.Equal(t, mutated.Err.Error(), `webui: Open "/CHANGED/{id}": that page is not mounted in this app`)
	assert.Equal(t, empty.Err.Error(), `webui: Open "/device/{id}": path argument "id" is empty`)

	none := webui.Open(context.Background(), details, pageArgs{Id: "x"})
	assert.Equal(t, none.Err.Error(), `webui: Open "/device/{id}": no compiled app in this context`)
	_, err := webui.ArgsOf[pageArgs](context.Background())
	assert.Error(t, err)
}

func TestOpenEscapesAndOmitsZeroValues(t *testing.T) {
	t.Parallel()

	var (
		target  webui.Target
		details webui.Page[pageArgs]
	)
	a, d, _ := app(func(ctx context.Context, _ pageArgs) error {
		target = webui.Open(ctx, details, pageArgs{Id: "a b/c", Q: "x&y"})
		return nil
	})
	details = d
	serve(a.MustCompile(""), http.MethodGet, "/device/zzz")
	assert.Equal(t, target.URL, "/device/a%20b%2Fc?q=x%26y")
}
