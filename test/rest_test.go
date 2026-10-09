package webui_test

import (
	"net/http"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type bucketArgs struct{ Name string }
type objectArgs struct {
	Name string
	Key  string
}

// bucketApp is a bucket, and the objects in it at any depth: the key is the rest of
// the path, so a nested folder is a path of its own.
func bucketApp() http.Handler {
	buckets := webui.Page[webui.NoArgs]{Path: "/bucket", Nav: webui.Nav{Label: "Buckets"}, Body: webui.Stack{}}
	bucket := webui.Page[bucketArgs]{Path: "/bucket/{name}", Body: webui.Stack{}}
	object := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}", Body: webui.Stack{}}
	return webui.App{Pages: webui.Pages{buckets, bucket, object}}.MustCompile("/admin")
}

func TestARestPlaceholderTakesTheRestOfThePathAndEachSegmentIsACrumb(t *testing.T) {
	t.Parallel()

	body := serve(bucketApp(), http.MethodGet, "/admin/bucket/photos/2026/summer/img%20one.png").Body.String()

	assert.Contains(t, body, `<a href="/admin/bucket">Buckets</a>`)
	assert.Contains(t, body, `<a href="/admin/bucket/photos">photos</a>`)
	// Each folder on the way is a link to this page at that depth.
	assert.Contains(t, body, `<a href="/admin/bucket/photos/2026">2026</a>`)
	assert.Contains(t, body, `<a href="/admin/bucket/photos/2026/summer">summer</a>`)
	assert.Contains(t, body, `<span class="cur" aria-current="page">img one.png</span>`)
	assert.Contains(t, body, "<title>img one.png — ")
}

func TestARestPlaceholderIsEscapedSegmentBySegmentWhenALinkIsBuilt(t *testing.T) {
	t.Parallel()

	body := serve(bucketApp(), http.MethodGet, "/admin/bucket/b/a%20b/c%3Fd/e").Body.String()
	assert.Contains(t, body, `<a href="/admin/bucket/b/a%20b">a b</a>`)
	assert.Contains(t, body, `<a href="/admin/bucket/b/a%20b/c%3Fd">c?d</a>`)
}

func TestARestPlaceholderIsRequiredLikeAnyPathArgument(t *testing.T) {
	t.Parallel()

	assert.Equal(t, serve(bucketApp(), http.MethodGet, "/admin/bucket/photos").Code, http.StatusOK) // the bucket page
	assert.Equal(t, serve(bucketApp(), http.MethodGet, "/admin/bucket/photos/").Code, http.StatusBadRequest)
}

func TestARestPlaceholderMustBeLastAndAString(t *testing.T) {
	t.Parallel()

	middle := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}/more", Body: webui.Stack{}}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{middle}})
	assert.Contains(t, errs[0].Detail, "takes the rest of the path but is not the last")

	type numeric struct {
		Name string
		Key  int
	}
	wrong := webui.Page[numeric]{Path: "/bucket/{name}/{key...}", Body: webui.Stack{}}
	errs = compileErrors(t, webui.App{Pages: webui.Pages{wrong}})
	assert.Contains(t, errs[0].Detail, "is not a string")
}
