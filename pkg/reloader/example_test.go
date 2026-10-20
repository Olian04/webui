package reloader_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Olian04/webui/pkg/reloader"
	"github.com/Olian04/webui/pkg/webui"
)

// A program whose pages follow something that changes compiles the app again with a reloader.Handler: the app that was serving is
// replaced when the new one compiles, and kept when it does not.
func ExampleHandler() {
	app := func(path webui.PageID[webui.NoArgs]) webui.App {
		return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: path, Body: webui.Stack{}}}}
	}
	status := func(h http.Handler, target string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Code
	}

	handler := reloader.New() // mounted once, as the handler
	_ = handler.Compile(app("/first"), "/admin")
	fmt.Println(status(handler, "/admin/first"), status(handler, "/admin/second"))

	_ = handler.Compile(app("/second"), "/admin") // the resources changed
	fmt.Println(status(handler, "/admin/first"), status(handler, "/admin/second"))
	// Output:
	// 200 404
	// 404 200
}
