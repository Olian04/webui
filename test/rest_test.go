package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type bucketArgs struct{ Name string }
type objectArgs struct {
	Name string
	Key  []string
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

func TestARestPlaceholderMustBeLastAndAListOfStrings(t *testing.T) {
	t.Parallel()

	middle := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}/more", Body: webui.Stack{}}
	errs := compileErrors(t, webui.App{Pages: webui.Pages{middle}})
	assert.Contains(t, errs[0].Detail, "takes the rest of the path but is not the last")

	type text struct {
		Name string
		Key  string
	}
	wrong := webui.Page[text]{Path: "/bucket/{name}/{key...}", Body: webui.Stack{}}
	errs = compileErrors(t, webui.App{Pages: webui.Pages{wrong}})
	assert.Contains(t, errs[0].Detail, "is not a []string")
	assert.Contains(t, errs[0].Fix, "one element for each")

	type tagged struct {
		Name string
		Tags []string
	}
	list := webui.Page[tagged]{Path: "/bucket/{name}", Body: webui.Stack{}}
	errs = compileErrors(t, webui.App{Pages: webui.Pages{list}})
	assert.Contains(t, errs[0].Detail, "is a []string, but the path has no {tags...}")
}

func TestARestPlaceholderArrivesAsAListOfSegments(t *testing.T) {
	t.Parallel()

	var seen []string
	object := webui.Page[objectArgs]{Path: "/bucket/{name}/{key...}", Body: webui.Form[Device]{
		Load: func(ctx context.Context) (Device, error) {
			seen = webui.ArgsOf[objectArgs](ctx).Key
			return Device{Id: "a"}, nil
		},
		Fields: []webui.Accessor[Device]{formID},
	}}
	h := webui.App{Pages: webui.Pages{object}}.MustCompile("/admin")

	assert.Equal(t, serve(h, http.MethodGet, "/admin/bucket/photos/2026/summer/img%20one.png").Code, http.StatusOK)
	assert.DeepEqual(t, seen, []string{"2026", "summer", "img one.png"})

	// An empty segment is not an address of this page.
	assert.Equal(t, serve(h, http.MethodGet, "/admin/bucket/photos/2026/").Code, http.StatusBadRequest)
}

func TestALinkToARestPlaceholderBuildsTheAddressFromTheList(t *testing.T) {
	t.Parallel()

	var object webui.Page[objectArgs]
	object.Path = "/bucket/{name}/{key...}"
	object.Body = webui.Stack{}
	from := webui.Page[webui.NoArgs]{Path: "/from", Body: webui.Table[Device]{
		Title:   "Objects",
		Rows:    func(context.Context) ([]Device, error) { return []Device{{Id: "a"}}, nil },
		Columns: []webui.Accessor[Device]{formID},
		RowClick: webui.Link[Device, objectArgs]{
			Page: object,
			Args: func(_ context.Context, d Device) objectArgs {
				return objectArgs{Name: "photos", Key: []string{"2026", "a b", d.Id}}
			},
		},
	}}
	h := webui.App{Pages: webui.Pages{from, object}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/from").Body.String()
	assert.Contains(t, body, `href="/admin/bucket/photos/2026/a%20b/a"`) // each element escaped on its own

	// An element with a slash would be read back as two, so no link is made.
	bad := webui.Page[webui.NoArgs]{Path: "/bad", Body: webui.Table[Device]{
		Title:   "Objects",
		Rows:    func(context.Context) ([]Device, error) { return []Device{{Id: "a"}}, nil },
		Columns: []webui.Accessor[Device]{formID},
		RowClick: webui.Link[Device, objectArgs]{
			Page: object,
			Args: func(context.Context, Device) objectArgs { return objectArgs{Name: "p", Key: []string{"a/b"}} },
		},
	}}
	h = webui.App{Pages: webui.Pages{bad, object}}.MustCompile("/admin")
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/bad").Body.String(), `rowlink`))
}
