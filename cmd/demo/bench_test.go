package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// These benchmarks are the cost of making a page, which is what the runtime and the renderer do between a request arriving and the HTML leaving: the Guards and the
// Loads run, the panels are built, and the markup is drawn. They are on the demo because it is the one app that has every kind of page, with the work of its own
// functions as small as an in-memory service makes it, so what is measured is the library's.
//
//	go test ./cmd/demo -run '^$' -bench 'BenchmarkPage' -benchmem -count=6
//
// and benchstat to compare two runs. A change to the runtime or the renderer that is made for speed quotes these, before and after.

// benchPages is a page of each kind: the overview (tables, a form, toolbars), a table, a form, an editor, several editors and a diff, markdown, a calendar with lists, and a
// table that pages.
var benchPages = []string{
	"/admin/", "/admin/device", "/admin/alert", "/admin/settings", "/admin/rule",
	"/admin/layout/review", "/admin/handbook", "/admin/activity", "/admin/audit",
}

// benchSite is the whole site, for an editor, who may do everything and so is drawn every control.
func benchSite(tb testing.TB) http.Handler {
	tb.Helper()

	service = newService()
	compiled, err := app.Compile("/admin")
	if err != nil {
		tb.Fatal(err)
	}
	site := routes(compiled)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//nolint:gosec // G124: a request the benchmark makes, not a cookie the app sets.
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: sign(Editor)})
		site.ServeHTTP(w, r)
	})
}

// BenchmarkPage is the cost of building one page and not of sending it: a client that takes no compression.
func BenchmarkPage(b *testing.B) {
	h := benchSite(b)
	for _, path := range benchPages {
		b.Run(path, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				if rec.Code != http.StatusOK {
					b.Fatalf("%s answered %d", path, rec.Code)
				}
			}
		})
	}
}

// BenchmarkPageCompressed is BenchmarkPage for a client that takes gzip, or Brotli: what a page costs to send as well as to build. Compression of what is built for one
// request is a cost of the same size as building it.
func BenchmarkPageCompressed(b *testing.B) {
	h := benchSite(b)
	for _, coding := range []string{"gzip", "br"} {
		for _, path := range []string{"/admin/", "/admin/device"} {
			b.Run(coding+path, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					req.Header.Set("Accept-Encoding", coding)
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						b.Fatalf("%s answered %d", path, rec.Code)
					}
					if rec.Header().Get("Content-Encoding") != coding {
						b.Fatalf("%s was not sent in %s", path, coding)
					}
				}
			})
		}
	}
}

// BenchmarkPageInParallel is BenchmarkPage for requests that arrive together, as they do on a server: it shows what the work for one request costs the others, in the
// allocator, the garbage collector and anything shared.
func BenchmarkPageInParallel(b *testing.B) {
	h := benchSite(b)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := benchPages[i%len(benchPages)]
			i++
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				b.Fatalf("%s answered %d", path, rec.Code)
			}
		}
	})
}
