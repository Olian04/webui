package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// The compare page takes two lists in its address, a parameter that repeats, and shows the devices they name: each device by
// its id and every device at each site, once. A sort or a pager on the page keeps every element of both.
func TestTheComparePageTakesLists(t *testing.T) {
	h := handler(t)

	two := pageAt(t, h, "/admin/compare?device=dev_27c38b&device=dev_27c75c")
	assert.True(t, shown(two, "dev_27c38b"))
	assert.True(t, shown(two, "dev_27c75c"))
	assert.False(t, shown(two, "dev_27cb2d"))

	// A site is every device in it, and a device that is also in a site is not shown twice.
	mixed := pageAt(t, h, "/admin/compare?site=Malm%C3%B6&device=dev_27c75c")
	count := 0
	for _, row := range page.One(t, "the devices", mixed.ByRole("table", "Devices")).Rows() {
		if row[0] == "dev_27c75c" {
			count++
		}
	}
	assert.Equal(t, count, 1)
	assert.True(t, shown(mixed, "Malmö"))

	// The page's own links keep both lists whole, in order.
	whole := pageAt(t, h, "/admin/compare?site=Malm%C3%B6&site=G%C3%B6teborg&device=dev_27c38b&device=dev_27c75c")
	kept := false
	for _, l := range mainOf(t, whole).Links() {
		kept = kept || strings.Contains(l.Href, "device=dev_27c38b&device=dev_27c75c&devices.sort=id&site=Malm%C3%B6&site=G%C3%B6teborg")
	}
	assert.True(t, kept)

	// No lists, no devices: an empty list is no parameter, and not an error.
	assert.Equal(t, get(h, "/admin/compare").Code, http.StatusOK)
	assert.Equal(t, get(h, "/admin/compare?device=").Code, http.StatusOK)
}
