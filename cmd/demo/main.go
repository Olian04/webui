// Command demo serves a small admin panel built with webui, over in-memory
// data. It exists to look at the library working; it is not part of its API.
//
// The app is a static config: every page, accessor and action is a top-level
// var, read top to bottom. devices.go is the place to start.
//
//	go run ./cmd/demo                # http://localhost:8080/admin/
//	go run ./cmd/demo -viewer        # guarded controls are disabled, with a reason
//	go run ./cmd/demo -broken        # the failed-to-compile page
package main

import (
	"flag"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

var app = webui.App{
	Brand: webui.Brand{Name: "Collector"},
	Pages: webui.Pages{Devices, Details, Alerts, Ingest, Retention},
}

var broken = flag.Bool("broken", false, "declare an invalid page, to see the compile error page")

// Arguments must be string, bool, int, int64 or float64, so a slice is a mistake
// Compile reports, with the page and a fix, rather than a panic.
type brokenArgs struct{ When []string }

var Broken = webui.Page[brokenArgs]{Path: "/event/{id}", Body: webui.Stack{}}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	a := app
	if *broken {
		a.Pages = append(slices.Clone(a.Pages), Broken)
	}

	// Compile never returns a nil handler: a broken app serves its own errors.
	handler, err := a.Compile("/admin")
	if err != nil {
		log.Println("webui:", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/admin/", handler)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})
	log.Printf("listening on http://localhost%s/admin/", *addr)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
