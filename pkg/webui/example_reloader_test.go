package webui_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Olian04/webui/pkg/webui"
)

// A program whose pages follow something that changes compiles the app again with a Reloader: the app that was serving is
// replaced when the new one compiles, and kept when it does not.
func ExampleReloader() {
	app := func(path webui.PageID[webui.NoArgs]) webui.App {
		return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: path, Body: webui.Stack{}}}}
	}
	status := func(h http.Handler, target string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Code
	}

	reloader := webui.NewReloader() // mounted once, as the handler
	_ = reloader.Compile(app("/first"), "/admin")
	fmt.Println(status(reloader, "/admin/first"), status(reloader, "/admin/second"))

	_ = reloader.Compile(app("/second"), "/admin") // the resources changed
	fmt.Println(status(reloader, "/admin/first"), status(reloader, "/admin/second"))
	// Output:
	// 200 404
	// 404 200
}
