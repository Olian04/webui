package webui_test

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// The server declares which links replace their panel in place (data-in-place): the sort and pager links of a table, its way on from an empty state, and a failed panel's
// retry. The script follows those and no other, and does not guess from where a link leads.
func TestTheLinksThatReplaceTheirPanelAreDeclaredByTheServer(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device?devices.offset=10").Body.String()
	for _, link := range regexp.MustCompile(`<a [^>]*>`).FindAllString(body, -1) {
		inPlace := strings.Contains(link, "data-in-place")
		switch {
		case strings.Contains(link, "data-refresh"):
			// the page's own Refresh button, which is a navigation
		case strings.Contains(link, "devices.sort") || strings.Contains(link, "devices.offset") || strings.Contains(link, `href="/admin/device"`) && strings.Contains(link, "btn"):
			// a sort, the pager, or the first page: the panel's own
			if !inPlace {
				t.Errorf("a link that replaces its panel is not declared: %s", link)
			}
		case strings.Contains(link, `class="rowlink"`):
			assert.False(t, inPlace) // the row leads to another page
		}
	}
	assert.Contains(t, body, `data-in-place>Previous</a>`)

	// An empty table's ways on, and a failed panel's retry, are declared too.
	empty, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Ready()
	})
	assert.Contains(t, serve(empty, http.MethodGet, "/admin/device?devices.filter.id=zzz").Body.String(), `data-in-place>Clear the filters`)
	failed, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Failed(errors.New("down"))
	})
	assert.Contains(t, serve(failed, http.MethodGet, "/admin/device").Body.String(), `data-in-place>Try again</a>`)
}

// The box's list is asked for with the header that a request for one panel carries, named for the box, and not with a parameter of the address: the answer varies by it,
// and says so. Without the header the same address is the page of results.
func TestTheSearchBoxIsAskedForByTheHeaderAndNotByAParameter(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	list := serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=eth")
	assert.Equal(t, list.Code, http.StatusOK)
	assert.False(t, strings.Contains(list.Body.String(), "<html"))
	assert.Contains(t, strings.Join(list.Header().Values("Vary"), ","), "X-Webui-View")

	page := serve(h, http.MethodGet, "/admin/_webui/search?q=eth")
	assert.Contains(t, page.Body.String(), "<html")
	assert.Contains(t, strings.Join(page.Header().Values("Vary"), ","), "X-Webui-View")

	// The parameter that used to say it is a parameter like any other: the page.
	old := serve(h, http.MethodGet, "/admin/_webui/search?fragment=1&q=eth")
	assert.Contains(t, old.Body.String(), "<html")
}
