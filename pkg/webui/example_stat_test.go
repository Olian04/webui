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

// A Stat is a few figures of one model. A tile's value is any accessor, shown as it is in a cell; its change is a Delta, which says which way it
// moved and whether that is the good way, and its trend is a Series, a small chart with its values in words.
func ExampleStat() {
	page := webui.Page[webui.NoArgs]{
		Path: "/status",
		Body: webui.Stat[Latency]{
			Title: "Latency",
			Load: func(context.Context) (Latency, webui.Loaded) {
				return Latency{P95: 184, Change: -12, Samples: []float64{140, 150, 190, 184}}, webui.Ready()
			},
			Tiles: []webui.StatTile[Latency]{{
				Label: "95th percentile",
				Value: webui.Float[Latency]{
					Label:  "P95",
					Load:   func(l Latency) float64 { return l.P95 },
					Format: func(v float64) string { return fmt.Sprintf("%.0f ms", v) },
				},
				// A latency that falls is good news, so the fall is drawn as good.
				Change: webui.Delta[Latency]{
					Label: "Since yesterday", LowerIsBetter: true, Unit: " ms",
					Load: func(l Latency) float64 { return l.Change },
				},
				Trend: webui.Series[Latency]{
					Label: "Latency over the day", Unit: " ms",
					Load: func(l Latency) []float64 { return l.Samples },
				},
			}},
		},
	}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/status", nil))
	body := rec.Body.String()
	for _, want := range []string{"184 ms", "−12 ms", "delta-ok", "Latency over the day: 4 values, from 140 ms to 184 ms, lowest 140 ms, highest 190 ms"} {
		fmt.Println(strings.Contains(body, want))
	}

	// Output:
	// true
	// true
	// true
	// true
}
