package webui_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"

	"github.com/Olian04/webui/pkg/webui"
)

// A table whose rows are a slice, that links each row to a page of its own, which
// finds them from the search box.
func ExampleTable() {
	type device struct{ ID, IP string }
	type deviceArgs struct{ ID string }

	id := webui.String[device]{Label: "ID", Load: func(d device) string { return d.ID }}
	ip := webui.String[device]{Label: "IP", Load: func(d device) string { return d.IP }}
	devices := []device{{"dev_1", "10.0.0.1"}, {"dev_2", "192.168.1.2"}}

	// The page a row leads to. Its path argument is the device's ID. Its Search is what makes
	// the devices findable from the search box: SearchRows looks in the fields it is given,
	// and says what a device is as a result.
	details := webui.Page[deviceArgs]{
		Path: "/device/{id}",
		Search: webui.SearchRows(
			func(context.Context) ([]device, error) { return devices, nil },
			func(d device) webui.Hit[deviceArgs] {
				return webui.Hit[deviceArgs]{Args: deviceArgs{ID: d.ID}, Title: d.ID, Desc: d.IP}
			},
			id, ip,
		),
		Body: webui.Form[device]{
			Load: func(ctx context.Context) (device, error) {
				want := webui.ArgsOf[deviceArgs](ctx).ID
				for _, d := range devices {
					if d.ID == want {
						return d, nil
					}
				}
				return device{}, fmt.Errorf("no device %q", want)
			},
			Fields: []webui.Accessor[device]{id, ip},
		},
	}

	list := webui.Page[webui.NoArgs]{
		Path: "/device",
		Nav:  webui.Nav{Label: "Devices"},
		Body: webui.Table[device]{
			Title: "Devices",
			Rows:  func(context.Context) ([]device, error) { return devices, nil },
			RowClick: webui.Link[device, deviceArgs]{
				Page: details,
				Args: func(_ context.Context, d device) deviceArgs { return deviceArgs{ID: d.ID} },
			},
			Columns: []webui.Accessor[device]{id, ip},
		},
	}

	handler := webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")

	// Typing "192.168" in the search box finds the second device, by its address,
	// and links to its page. The box lists the answer, which is HTML: a heading for the page
	// that found it, and a link.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/_webui/search?fragment=1&q=192.168", nil))

	group := regexp.MustCompile(`<div class="pal-label">(.*?)</div>`).FindStringSubmatch(rec.Body.String())
	link := regexp.MustCompile(`<a class="pal-item" role="option" href="(.*?)">\s*<span>(.*?)</span>`).FindStringSubmatch(rec.Body.String())
	fmt.Println(group[1], link[2], link[1])
	// Output:
	// Devices dev_2 /admin/device/dev_2
}
