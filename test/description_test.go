package webui_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func TestTheDescriptionIsTheMetaDescriptionOfEveryPage(t *testing.T) {
	t.Parallel()

	h := webui.App{Description: `Devices & "alerts" of <one> collector.`}.MustCompile("/admin")
	for _, path := range []string{"/admin/", "/admin/nowhere"} { // the page that says there are none, and one that is not there
		page := serve(h, http.MethodGet, path).Body.String()
		assert.Contains(t, page, `<meta name="description" content="Devices &amp; &#34;alerts&#34; of &lt;one&gt; collector.">`)
	}
}

func TestWithoutADescriptionThePagesHaveNone(t *testing.T) {
	t.Parallel()

	h := webui.App{}.MustCompile("/admin")
	assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/").Body.String(), `name="description"`))
}
