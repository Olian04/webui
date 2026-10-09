package webui_test

import (
	"context"
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
