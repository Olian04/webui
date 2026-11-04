package webui_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Olian04/webui/pkg/webui"
)

// Latency is the figures of one moment of a service.
type Latency struct {
	P95     float64
	Change  float64
	Samples []float64
}

// A few figures of one model are a Table of one row: a column for the value and a Trend column beside it, which says which way it moved and whether that is
// the good way, with a small chart that has its values in words. Each Trend has a direction of its own.
func ExampleTrend() {
	page := webui.Page[webui.NoArgs]{
		Path: "/status",
		Body: webui.Table[Latency]{
			Title: "Latency",
			Rows: func(context.Context) ([]Latency, webui.Loaded) {
				return []Latency{{P95: 184, Change: -12, Samples: []float64{140, 150, 190, 184}}}, webui.Ready()
			},
			Columns: []webui.Accessor[Latency]{
				webui.Float[Latency]{
					Label:  "95th percentile",
					Load:   func(l Latency) float64 { return l.P95 },
					Format: func(v float64) string { return fmt.Sprintf("%.0f ms", v) },
				},
				// A latency that falls is good news, so the fall is drawn as good.
				webui.Trend[Latency]{
					Label: "Latency over the day", LowerIsBetter: true, NoSort: true, NoFilter: true,
					Load: func(l Latency) ([]float64, *float64) { return l.Samples, &l.Change },
				},
			},
		},
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/status", nil))
	body := rec.Body.String()
	for _, want := range []string{"184 ms", "−12.0%", "tone-ok", "Latency over the day: 4 values, from 140 to 184, lowest 140, highest 190"} {
		fmt.Println(strings.Contains(body, want))
	}

	// Output:
	// true
	// true
	// true
	// true
}
