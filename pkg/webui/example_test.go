package webui_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Olian04/webui/pkg/webui"
)

// Device is the model the page lists.
type Device struct{ ID, IP string }

var devices = []Device{
	{ID: "dev_1", IP: "10.0.0.1"},
	{ID: "dev_2", IP: "10.0.0.2"},
}

// An accessor is written once and is a column in a table and an input in a form.
var (
	ID = webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.ID }}
	IP = webui.String[Device]{Label: "IP", Load: func(d Device) string { return d.IP }}
)

// A page is a path, a sidebar entry, and a body. This one is a table whose rows
// are a slice; the library sorts, filters and pages them.
var Devices = webui.Page[webui.NoArgs]{
	Path: "/device",
	Nav:  webui.Nav{Label: "Devices", Icon: "display"},
	Body: webui.Table[Device]{
		Title:   "Devices",
		Rows:    func(context.Context) ([]Device, error) { return devices, nil },
		Columns: []webui.Accessor[Device]{ID, IP},
	},
}

// A page with a table of devices, served under /admin. Run it, and open
// /admin/device.
func Example() {
	app := webui.App{
		Brand: webui.Brand{Name: "Acme"},
		Pages: webui.Pages{Devices},
	}

	// Compile checks the declaration and returns an http.Handler. In a real
	// program, mount it and listen:
	//
	//	http.Handle("/admin/", app.MustCompile("/admin"))
	//	log.Fatal(http.ListenAndServe(":8080", nil))
	handler := app.MustCompile("/admin")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/device", nil))

	fmt.Println(rec.Code)
	fmt.Println(strings.Contains(rec.Body.String(), "10.0.0.2"))
	// Output:
	// 200
	// true
}
