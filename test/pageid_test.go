package webui_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// Two pages that link to each other, declared as package-level vars. The list
// links forward to the detail page by its variable; the detail page's action
// goes back to the list by its PageID, a constant. If shopSave named shopList
// instead, this file would not build: "initialization cycle for shopList".
// That this compiles is the point of PageID.

type shopItemArgs struct {
	ID string `webui:"id"`
}

const (
	shopListPath   webui.PageID[webui.NoArgs] = "/shop"
	shopDetailPath webui.PageID[shopItemArgs] = "/shop/{id}"
)

var shopList = webui.Page[webui.NoArgs]{
	Path: shopListPath,
	Nav:  webui.Nav{Label: "Shop"},
	Body: webui.Table[Device]{
		Load: func(context.Context, webui.Query) (webui.Rows[Device], error) {
			return webui.Rows[Device]{Items: []Device{{Id: "a"}, {Id: "b"}}, Total: 2}, nil
		},
		RowClick: webui.Link[Device, shopItemArgs]{
			Page: shopDetail,
			Args: func(_ context.Context, d Device) shopItemArgs { return shopItemArgs{ID: d.Id} },
		},
		Columns: []webui.Accessor[Device]{formID},
	},
}

var shopDetail = webui.Page[shopItemArgs]{
	Path: shopDetailPath,
	Body: webui.Form[Device]{
		Load:   func(context.Context) (Device, error) { return Device{Id: "a", Ip: "10.0.0.1"}, nil },
		Fields: []webui.Accessor[Device]{formID, formIP},
		Submit: shopSave,
	},
}

var shopSave = webui.Action[Device]{
	Run: func(ctx context.Context, _ Device) (webui.Effect, error) {
		return webui.Effect{
			Toast:    "Saved",
			Redirect: webui.Open(ctx, shopListPath, webui.NoArgs{}), // back, by constant
		}, nil
	},
}

var shopApp = webui.App{Pages: webui.Pages{shopList, shopDetail}}

func TestPagesThatLinkBothWaysAreStaticVars(t *testing.T) {
	t.Parallel()

	h := shopApp.MustCompile("/admin")

	// Forward: a row of the list links to the detail page.
	list := serve(h, http.MethodGet, "/admin/shop").Body.String()
	assert.Contains(t, list, `<a class="rowlink" href="/admin/shop/a?webui.from=%2Fadmin%2Fshop">a</a>`)

	// Back: the detail page's action redirects to the list.
	rec := post(h, "/admin/shop/a", url.Values{"_leaf": {"p"}, "f1": {"10.0.0.2"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/shop")
}

func TestLinkAndOpenAcceptAPageIDInPlaceOfAPage(t *testing.T) {
	t.Parallel()

	var opened webui.Target
	detail := webui.Page[shopItemArgs]{Path: shopDetailPath, Body: webui.Stack{}}
	list := webui.Page[webui.NoArgs]{
		Path: shopListPath,
		Guard: func(ctx context.Context, _ webui.NoArgs) error {
			opened = webui.Open(ctx, shopDetailPath, shopItemArgs{ID: "x y"})
			return nil
		},
		Body: webui.Table[Device]{
			Load: func(context.Context, webui.Query) (webui.Rows[Device], error) {
				return webui.Rows[Device]{Items: []Device{{Id: "a"}}, Total: 1}, nil
			},
			// The destination named by its ID, not by the page.
			RowClick: webui.Link[Device, shopItemArgs]{
				Page: shopDetailPath,
				Args: func(_ context.Context, d Device) shopItemArgs { return shopItemArgs{ID: d.Id} },
			},
			Columns: []webui.Accessor[Device]{formID},
		},
	}
	h := webui.App{Pages: webui.Pages{list, detail}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/shop").Body.String()
	assert.Contains(t, body, `href="/admin/shop/a"`)
	assert.NoError(t, opened.Err)
	assert.Equal(t, opened.URL, "/admin/shop/x%20y")
}

func TestAPageIDForAPageThatIsNotMountedIsReported(t *testing.T) {
	t.Parallel()

	const gone webui.PageID[shopItemArgs] = "/gone/{id}"

	// At Compile, for a Link.
	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/x", Body: webui.Table[Device]{
		Load: okRows,
		RowClick: webui.Link[Device, shopItemArgs]{
			Page: gone,
			Args: func(context.Context, Device) shopItemArgs { return shopItemArgs{} },
		},
	}}}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 1)
	assert.Contains(t, errs[0].Detail, `a link targets "/gone/{id}", which is not mounted in this app`)

	// At run time, for an Open.
	var got webui.Target
	page := webui.Page[webui.NoArgs]{Path: "/x", Body: webui.Stack{}, Guard: func(ctx context.Context, _ webui.NoArgs) error {
		got = webui.Open(ctx, gone, shopItemArgs{ID: "1"})
		return nil
	}}
	serve(webui.App{Pages: webui.Pages{page}}.MustCompile(""), http.MethodGet, "/x")
	assert.Equal(t, got.Err.Error(), `webui: Open "/gone/{id}": that page is not mounted in this app`)
}

func TestALinkWithoutAPageIsACompileError(t *testing.T) {
	t.Parallel()

	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/x", Body: webui.Table[Device]{
		Load: okRows,
		RowClick: webui.Link[Device, shopItemArgs]{
			Args: func(context.Context, Device) shopItemArgs { return shopItemArgs{} },
		},
	}}}}
	errs := compileErrors(t, app)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Detail, "a Link has no Page")
}

func TestAPlainStringStillWorksAsAPath(t *testing.T) {
	t.Parallel()

	const untyped = "/plain" // an untyped constant converts to PageID[A]
	a := webui.Page[webui.NoArgs]{Path: "/literal", Body: webui.Stack{}}
	b := webui.Page[webui.NoArgs]{Path: untyped, Body: webui.Stack{}}
	h := webui.App{Pages: webui.Pages{a, b}}.MustCompile("")
	assert.Equal(t, serve(h, http.MethodGet, "/literal").Code, http.StatusOK)
	assert.Equal(t, serve(h, http.MethodGet, "/plain").Code, http.StatusOK)
}
