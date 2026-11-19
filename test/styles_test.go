package webui_test

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// linkedStyles are the stylesheets that a page of the app links, in order, but for the icon stylesheet, found in the markup as a browser finds them.
func linkedStyles(t *testing.T, h http.Handler, prefix string) []string {
	t.Helper()

	rec := serve(h, http.MethodGet, prefix+"/")
	var out []string
	for _, m := range regexp.MustCompile(`href="([^"]*_webui/(?:app|theme)\.[0-9a-f]{8}\.css)"`).FindAllStringSubmatch(rec.Body.String(), -1) {
		out = append(out, m[1])
	}
	return out
}

// A page that the app makes itself links the same stylesheets as the app's own pages do, and they are found without a compiled app: a file's name is a hash of its contents, which is the same in every
// app of this version of the library, so that only the mount prefix and the theme are the app's.
func TestStylesAreTheAddressesThatThePagesOfTheAppLink(t *testing.T) {
	t.Parallel()

	home := webui.Page[webui.NoArgs]{Path: "/", View: webui.Markdown[handbook]{Title: "Home", Load: func(context.Context) (handbook, webui.Loaded) { return handbook{Text: "hi"}, webui.Ready() }, Content: handbookText}}
	for _, theme := range []webui.Theme{{}, {Accent: "#1f7a6d"}, {Accent: "#1f7a6d", Light: webui.ModeColors{Canvas: "#f3f6f2"}}} {
		app := webui.App{Theme: theme, Pages: webui.Pages{home}}
		for _, prefix := range []string{"", "/ui", "/admin"} {
			h := app.MustCompile(prefix)
			got, want := app.Styles(prefix), linkedStyles(t, h, prefix)
			assert.DeepEqual(t, got, want)
			if theme == (webui.Theme{}) {
				assert.Equal(t, len(got), 1) // the library's, and no theme to link
			} else {
				assert.Equal(t, len(got), 2) // and the theme, after it
			}
			// The page that the library serves answers for them.
			for _, href := range got {
				assert.Equal(t, serve(h, http.MethodGet, href).Code, http.StatusOK)
			}
		}
	}

	// A theme is named for its contents: another theme is another address, the same is the same.
	a := webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}
	b := webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}
	c := webui.App{Theme: webui.Theme{Accent: "#7a1f6d"}}
	assert.DeepEqual(t, a.Styles("/admin"), b.Styles("/admin"))
	assert.True(t, a.Styles("/admin")[1] != c.Styles("/admin")[1])
	assert.Equal(t, a.Styles("/admin")[0], c.Styles("/admin")[0]) // the library's is the same for every theme

	// A prefix that Compile would refuse gives nothing.
	assert.Equal(t, len(webui.App{}.Styles("admin")), 0)
	assert.Equal(t, len(webui.App{}.Styles("/admin/")), 0)
}
