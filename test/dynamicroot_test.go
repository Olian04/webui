package webui_test

import (
	"net/http"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type tenantArgs struct{ Tenant string }
type tenantRestArgs struct {
	Tenant string
	Rest   []string
}

func dynamicRootApp(prefix string) http.Handler {
	return webui.App{Pages: webui.Pages{
		webui.Page[tenantArgs]{Path: "/{tenant}/info", Body: webui.Stack{}},
		webui.Page[tenantArgs]{Path: "/{tenant}/monaco/x", Body: webui.Stack{}},
		webui.Page[tenantRestArgs]{Path: "/{tenant}/files/{rest...}", Body: webui.Stack{}},
		webui.Page[tenantArgs]{Path: "/{tenant}", Body: webui.Stack{}},
	}}.MustCompile(prefix)
}

func TestAPageMayBeginWithAPlaceholderEvenWhereItWouldMeetTheFrameworksOwnRoutes(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{"", "/admin"} {
		h := dynamicRootApp(prefix)
		for _, path := range []string{"/acme/info", "/acme/monaco/x", "/acme/files/a/b", "/acme"} {
			assert.Equal(t, serve(h, http.MethodGet, prefix+path).Code, http.StatusOK)
		}
		// The framework's own routes still answer.
		assert.Equal(t, serve(h, http.MethodGet, prefix+"/_webui/search?q=a").Code, http.StatusOK)
	}
}

func TestTheAssetDirectoryIsNeverAPageWhateverAPlaceholderCouldMatch(t *testing.T) {
	t.Parallel()

	h := dynamicRootApp("/admin")
	for _, path := range []string{"/admin/_webui", "/admin/_webui/", "/admin/_webui/info", "/admin/_webui/monaco/x", "/admin/_webui/files/a", "/admin/_webui/nothing/here"} {
		rec := serve(h, http.MethodGet, path)
		assert.Equal(t, rec.Code, http.StatusNotFound)
	}
	// A wrong method on a route of the framework is a 404 as it always was, not a page's 405.
	assert.Equal(t, post(h, "/admin/_webui/info", nil).Code, http.StatusNotFound)
}

func TestTwoPagesThatMatchTheSameAddressWithNeitherMoreSpecificAreACompileErrorNotACrash(t *testing.T) {
	t.Parallel()

	type a struct{ A string }
	type b struct{ B string }
	app := webui.App{Pages: webui.Pages{
		webui.Page[a]{Path: "/{a}/x", Body: webui.Stack{}},
		webui.Page[b]{Path: "/y/{b}", Body: webui.Stack{}},
	}}
	errs := compileErrors(t, app)
	assert.Contains(t, errs[0].Detail, `"/y/x"`)
	assert.Contains(t, errs[0].Detail, "neither is more specific")
	assert.Contains(t, errs[0].Fix, "strictly more specific")

	// One that is strictly more specific than the other is fine.
	_, err := webui.App{Pages: webui.Pages{
		webui.Page[a]{Path: "/{a}/x", Body: webui.Stack{}},
		webui.Page[webui.NoArgs]{Path: "/y/x", Body: webui.Stack{}},
	}}.Compile("")
	assert.NoError(t, err)
}
