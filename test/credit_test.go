package webui_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

const generatorMeta = `<meta name="generator" content="webui">`

// The page says what built it, in its source: the page script reads this to say it in the console, and an app that does not want either leaves it out.
func TestEveryPageSaysItWasBuiltWithTheLibraryAndAnAppCanLeaveThatOut(t *testing.T) {
	t.Parallel()

	page := func(app webui.App) http.Handler {
		app.Pages = webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, View: webui.Stack{}}}
		return app.MustCompile("/admin")
	}
	on, off := page(webui.App{}), page(webui.App{NoConsoleMessage: true})
	for _, path := range []string{"/admin/a", "/admin/nowhere"} { // a page, and the page for an address that is not one
		assert.Equal(t, strings.Count(serve(on, http.MethodGet, path).Body.String(), generatorMeta), 1)
		assert.False(t, strings.Contains(serve(off, http.MethodGet, path).Body.String(), `name="generator"`))
	}
}
