package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The server declares which links replace their panel in place (data-in-place): the sort and pager links of a table, its way on from an empty state, and a failed panel's
// retry. The script follows those and no other, and does not guess from where a link leads.
func TestTheLinksThatReplaceTheirPanelAreDeclaredByTheServer(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device?devices.offset=10")
	for _, link := range mainOf(t, p).ByRole("link", "") {
		href := link.Attr("href")
		inPlace := link.Has("data-in-place")
		switch {
		case strings.Contains(href, "devices.sort") || strings.Contains(href, "devices.offset"):
			// a sort or the pager: the panel's own
			if !inPlace {
				t.Errorf("a link that replaces its panel is not declared: %s (%s)", link.Name(), href)
			}
		case strings.HasPrefix(href, "/admin/device/"):
			assert.False(t, inPlace) // the row leads to another page
		}
	}
	previous := page.One(t, "the link to the previous page", mainOf(t, p).ByRole("link", "Previous"))
	assert.True(t, previous.Has("data-in-place"))

	// An empty table's ways on, and a failed panel's retry, are declared too.
	empty, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Ready()
	})
	clearing := page.One(t, "the link that clears the filters", pageOf(t, empty, "/admin/device?devices.filter.id=zzz").ByRoleContaining("link", "Clear the filters"))
	assert.True(t, clearing.Has("data-in-place"))
	failed, _ := tableApp(func(context.Context, webui.PagedQuery) (webui.PagedWindow[Device], webui.Loaded) {
		return webui.PagedWindow[Device]{}, webui.Failed(errors.New("down"))
	})
	retry := page.One(t, "the link that tries again", pageOf(t, failed, "/admin/device").ByRole("link", "Try again"))
	assert.True(t, retry.Has("data-in-place"))
}

// The box's list is asked for with the header that a request for one panel carries, named for the box, and not with a parameter of the address: the answer varies by it,
// and says so. Without the header the same address is the page of results.
func TestTheSearchBoxIsAskedForByTheHeaderAndNotByAParameter(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	list := serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=eth")
	assert.Equal(t, list.Code, http.StatusOK)
	assert.True(t, page.IsFragment(list.Body.String()))
	assert.Contains(t, strings.Join(list.Header().Values("Vary"), ","), "X-Webui-View")

	whole := serve(h, http.MethodGet, "/admin/_webui/search?q=eth")
	assert.False(t, page.IsFragment(whole.Body.String()))
	assert.Contains(t, strings.Join(whole.Header().Values("Vary"), ","), "X-Webui-View")

	// The parameter that used to say it is a parameter like any other: the page.
	old := serve(h, http.MethodGet, "/admin/_webui/search?fragment=1&q=eth")
	assert.False(t, page.IsFragment(old.Body.String()))
}
