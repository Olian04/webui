package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// A Target has no parts a caller can read, so these look inside it. What a caller
// sees of it is what Outcome.Then does with it, which the integration tests cover.

type openArgs struct {
	Id    string
	Debug bool
	Q     string
}

// probeApp is a page at "/device/{id}" whose Guard runs probe with the request's
// context, which is what carries the compiled app Open resolves through.
func probeApp(probe func(ctx context.Context)) (App, Page[openArgs]) {
	details := Page[openArgs]{
		Path: "/device/{id}",
		Guard: func(ctx context.Context, _ openArgs) error {
			probe(ctx)
			return nil
		},
		Body: Stack{},
	}
	return App{Pages: Pages{details}}, details
}

func get(h http.Handler, target string) {
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
}

func TestOpenResolvesThroughTheRuntime(t *testing.T) {
	t.Parallel()

	var mounted, unmounted, mutated, empty Target
	var details Page[openArgs]
	a, d := probeApp(func(ctx context.Context) {
		mounted = Open(ctx, details, openArgs{Id: "abc", Debug: true})
		unmounted = Open(ctx, Page[openArgs]{Path: "/never-mounted/{id}"}, openArgs{Id: "x"})
		changed := details
		changed.Path = "/CHANGED/{id}"
		mutated = Open(ctx, changed, openArgs{Id: "x"})
		empty = Open(ctx, details, openArgs{})
	})
	details = d
	get(a.MustCompile("/admin"), "/admin/device/zzz")

	assert.NoError(t, mounted.err)
	assert.Equal(t, mounted.url, "/admin/device/abc?debug=true")
	assert.Equal(t, unmounted.err.Error(), `webui: Open "/never-mounted/{id}": that page is not mounted in this app`)
	assert.Equal(t, mutated.err.Error(), `webui: Open "/CHANGED/{id}": that page is not mounted in this app`)
	assert.Equal(t, empty.err.Error(), `webui: Open "/device/{id}": path argument "id" is empty`)

	none := Open(context.Background(), details, openArgs{Id: "x"})
	assert.Equal(t, none.err.Error(), `webui: Open "/device/{id}": no compiled app in this context`)
}

func TestOpenEscapesAndOmitsZeroValues(t *testing.T) {
	t.Parallel()

	var target Target
	var details Page[openArgs]
	a, d := probeApp(func(ctx context.Context) {
		target = Open(ctx, details, openArgs{Id: "a b/c", Q: "x&y"})
	})
	details = d
	get(a.MustCompile(""), "/device/zzz")
	assert.Equal(t, target.url, "/device/a%20b%2Fc?q=x%26y")
}

func TestOpenAcceptsAPageIDInPlaceOfAPage(t *testing.T) {
	t.Parallel()

	const detailPath PageID[openArgs] = "/device/{id}"
	var target, gone Target
	a, _ := probeApp(func(ctx context.Context) {
		target = Open(ctx, detailPath, openArgs{Id: "x y"})
		gone = Open(ctx, PageID[openArgs]("/gone/{id}"), openArgs{Id: "1"})
	})
	get(a.MustCompile("/admin"), "/admin/device/zzz")

	assert.NoError(t, target.err)
	assert.Equal(t, target.url, "/admin/device/x%20y")
	assert.Equal(t, gone.err.Error(), `webui: Open "/gone/{id}": that page is not mounted in this app`)
}
