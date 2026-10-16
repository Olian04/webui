package webui_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func recovered(t *testing.T, fn func()) (value any) {
	t.Helper()

	defer func() { value = recover() }()
	fn()
	return nil
}

func TestArgsOfPanicsOutsideAPage(t *testing.T) {
	t.Parallel()

	got := recovered(t, func() { webui.ArgsOf[pageArgs](context.Background()) })
	assert.Equal(t, got, any("webui: ArgsOf: no compiled app in this context"))
}

func panicApp(load func(ctx context.Context) (Device, error)) http.Handler {
	page := webui.Page[pageArgs]{
		Path: "/device/{id}",
		Body: webui.Form[Device]{Load: load, Fields: []webui.Accessor[Device]{formID}},
	}
	return webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
}

func TestAWrongArgumentTypeIsA500NotADroppedConnection(t *testing.T) {
	t.Parallel()

	h := panicApp(func(ctx context.Context) (Device, error) {
		webui.ArgsOf[webui.NoArgs](ctx) // the page's arguments are pageArgs
		return Device{}, nil
	})
	rec := serve(h, http.MethodGet, "/admin/device/a")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.Contains(t, rec.Body.String(), "Something went wrong")
	assert.False(t, strings.Contains(rec.Body.String(), "ArgsOf")) // the cause is logged, never shown
}

func TestAnyPanicInAClosureIsRecoveredTheSameWay(t *testing.T) {
	t.Parallel()

	h := panicApp(func(context.Context) (Device, error) { panic("db password is hunter2") })
	rec := serve(h, http.MethodGet, "/admin/device/a")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2"))
}

func TestThePageStillServesAfterAPanic(t *testing.T) {
	t.Parallel()

	calls := 0
	h := panicApp(func(context.Context) (Device, error) {
		calls++
		if calls == 1 {
			panic("once")
		}
		return Device{Id: "a"}, nil
	})
	assert.Equal(t, serve(h, http.MethodGet, "/admin/device/a").Code, http.StatusInternalServerError)
	assert.Equal(t, serve(h, http.MethodGet, "/admin/device/a").Code, http.StatusOK)
}

func brokenGuardApp(log *slog.Logger, guard func(context.Context, webui.NoArgs) error) http.Handler {
	home := webui.Page[webui.NoArgs]{Path: "/home", Nav: webui.Nav{Label: "Home"}, Body: webui.Stack{}}
	broken := webui.Page[webui.NoArgs]{Path: "/broken", Nav: webui.Nav{Label: "Broken"}, Body: webui.Stack{}, Guard: guard}
	return webui.App{Logger: log, Pages: webui.Pages{home, broken}}.MustCompile("/admin")
}

func TestANavigationPageWhoseGuardPanicsIsHiddenAndTheCauseIsLogged(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	h := brokenGuardApp(slog.New(slog.NewTextHandler(&logged, nil)), func(context.Context, webui.NoArgs) error { panic("x") })

	rec := serve(h, http.MethodGet, "/admin/home")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.DeepEqual(t, sidebar(t, h, "/admin/home"), []string{"Home>/admin/home*"})
	assert.Contains(t, logged.String(), "a page's Guard panicked")
	assert.Contains(t, logged.String(), "/broken")
	assert.Contains(t, logged.String(), "panic in Guard: x")

	// The page itself is as broken as its Guard, and the server-error page, which draws the sidebar
	// too, is still drawn.
	rec = serve(h, http.MethodGet, "/admin/broken")
	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	assert.Contains(t, rec.Body.String(), "Something went wrong")
	assert.DeepEqual(t, sidebar(t, h, "/admin/broken"), []string{"Home>/admin/home"})
}

func TestANavigationPageWhoseGuardRefusesIsHiddenAndNothingIsLogged(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	h := brokenGuardApp(slog.New(slog.NewTextHandler(&logged, nil)), func(context.Context, webui.NoArgs) error { return errors.New("no") })

	assert.Equal(t, serve(h, http.MethodGet, "/admin/home").Code, http.StatusOK)
	assert.DeepEqual(t, sidebar(t, h, "/admin/home"), []string{"Home>/admin/home*"})
	assert.Equal(t, serve(h, http.MethodGet, "/admin/broken").Code, http.StatusForbidden)
	assert.Equal(t, logged.String(), "")
}
