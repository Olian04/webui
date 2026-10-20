package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// The compare page takes two lists in its address, a parameter that repeats, and shows the devices they name: each device by
// its id and every device at each site, once. A sort or a pager on the page keeps every element of both.
func TestTheComparePageTakesLists(t *testing.T) {
	h := handler(t)

	two := get(h, "/admin/compare?device=dev_27c38b&device=dev_27c75c").Body.String()
	assert.Contains(t, two, "dev_27c38b")
	assert.Contains(t, two, "dev_27c75c")
	assert.False(t, strings.Contains(two, "dev_27cb2d"))

	// A site is every device in it, and a device that is also in a site is not shown twice.
	mixed := get(h, "/admin/compare?site=Malm%C3%B6&device=dev_27c75c").Body.String()
	assert.Equal(t, strings.Count(mixed, `>dev_27c75c<`), 1)
	assert.Contains(t, mixed, "Malmö")

	// The page's own links keep both lists whole, in order.
	body := get(h, "/admin/compare?site=Malm%C3%B6&site=G%C3%B6teborg&device=dev_27c38b&device=dev_27c75c").Body.String()
	assert.Contains(t, body, `device=dev_27c38b&amp;device=dev_27c75c&amp;devices.sort=id&amp;site=Malm%C3%B6&amp;site=G%C3%B6teborg`)

	// No lists, no devices: an empty list is no parameter, and not an error.
	assert.Equal(t, get(h, "/admin/compare").Code, http.StatusOK)
	assert.Equal(t, get(h, "/admin/compare?device=").Code, http.StatusOK)
}
