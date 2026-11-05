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
		app.Pages = webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Nav: webui.Nav{Label: "A"}, Body: webui.Stack{}}}
		return app.MustCompile("/admin")
	}
	on, off := page(webui.App{}), page(webui.App{NoConsoleMessage: true})
	for _, path := range []string{"/admin/a", "/admin/nowhere"} { // a page, and the page for an address that is not one
		assert.Equal(t, strings.Count(serve(on, http.MethodGet, path).Body.String(), generatorMeta), 1)
		assert.False(t, strings.Contains(serve(off, http.MethodGet, path).Body.String(), `name="generator"`))
	}
}

// The line in the console is the page script's, and has what a developer who opens the console is told: what built the page, and where it is. It is `info`, since a page's own
// errors and warnings are counted against it by the tools that look, it is said once for a tab and not for each page, and it is said only where the page says it was built so.
func TestThePageScriptSaysInTheConsoleWhatBuiltThePageAndWhereItIs(t *testing.T) {
	t.Parallel()

	js := assetSource(t, "enhance.js")
	for _, want := range []string{
		`meta[name="generator"][content="webui"]`, "console.info(", "https://github.com/Olian04/webui", "Built with webui",
		"sessionStorage.getItem('webui.credit')", "sessionStorage.setItem('webui.credit', '1')",
	} {
		assert.Contains(t, js, want)
	}
	assert.False(t, strings.Contains(js, "console.error"))
	assert.False(t, strings.Contains(js, "console.warn"))
	assert.False(t, strings.Contains(js, "console.log")) // and no other line in the console, which the app's own are in
}
