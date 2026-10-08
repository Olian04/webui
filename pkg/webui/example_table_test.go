package webui_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Olian04/webui/pkg/webui"
)

// A table whose rows are a slice, that links each row to a page of its own and
// offers its rows to the search box.
func ExampleTable() {
	type device struct{ ID, IP string }
	type deviceArgs struct{ ID string }

	id := webui.String[device]{Label: "ID", Load: func(d device) string { return d.ID }}
	ip := webui.String[device]{Label: "IP", Load: func(d device) string { return d.IP }}
	devices := []device{{"dev_1", "10.0.0.1"}, {"dev_2", "192.168.1.2"}}

	// The page a row leads to. Its path argument is the device's ID.
	details := webui.Page[deviceArgs]{
		Path: "/device/{id}",
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
			Title:    "Devices",
			Rows:     func(context.Context) ([]device, error) { return devices, nil },
			Search:   true, // every column of every row is searched
			PageSize: 25,
			RowClick: webui.Link[device, deviceArgs]{
				Page: details,
				Args: func(_ context.Context, d device) deviceArgs { return deviceArgs{ID: d.ID} },
			},
			Columns: []webui.Accessor[device]{id, ip},
		},
	}

	handler := webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")

	// Typing "192.168" in the search box finds the second device, by its address,
	// and links to its page.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/_webui/search?q=192.168", nil))

	var found struct {
		Results []struct{ Group, Title, Href string }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &found)
	for _, r := range found.Results {
		fmt.Println(r.Group, r.Title, r.Href)
	}
	// Output:
	// Devices dev_2 /admin/device/dev_2
}
