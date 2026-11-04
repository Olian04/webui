package webui_test

import (
	"context"
	"math"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// reading is a figure with a change, as a percent, and the points it was made from.
type reading struct {
	Name    string
	Devices int
	Change  float64
	Latency float64
	Points  []float64
	Hour    int
}

var (
	rDevices = webui.Int[reading]{Label: "Devices", Load: func(r reading) int { return r.Devices }}
	// a change with no points, and a chart with no change: either part of a Trend may be absent
	rChange  = webui.Trend[reading]{Label: "Change", Load: func(r reading) ([]float64, *float64) { return nil, &r.Change }}
	rLatency = webui.Trend[reading]{Label: "Latency", LowerIsBetter: true, Load: func(r reading) ([]float64, *float64) { return nil, &r.Latency }}
	rTrend   = webui.Trend[reading]{Label: "Devices over a day", Load: func(r reading) ([]float64, *float64) { return r.Points, nil }}
)

func loadReading(r reading) func(context.Context) (reading, webui.Loaded) {
	return func(context.Context) (reading, webui.Loaded) { return r, webui.Ready() }
}

// readingApp is a table of the one row that r is, with the columns that are given: the way a page shows a few figures side by side.
func readingApp(r reading, columns ...webui.Accessor[reading]) http.Handler {
	return editorApp(webui.Table[reading]{
		Title: "System", Columns: columns,
		Rows: func(context.Context) ([]reading, webui.Loaded) { return []reading{r}, webui.Ready() },
	})
}

func TestAChangeIsItsSignAnArrowAndAColourForWhetherThatWayIsGood(t *testing.T) {
	t.Parallel()

	tile := func(r reading) string {
		return serve(readingApp(r, rDevices, rChange), http.MethodGet, "/admin/e").Body.String()
	}

	up := tile(reading{Change: 4.35})
	assert.Contains(t, up, `<span class="delta tone-ok">`)
	assert.Contains(t, up, `fa-arrow-up`)
	assert.Contains(t, up, `<span>+4.35%</span>`)
	assert.Contains(t, up, `<span class="sr-only"> (better)</span>`) // what the colour means, for a reader who does not see it
	assert.False(t, strings.Contains(up, "<svg"))                    // no points, no chart

	down := tile(reading{Change: -4.8})
	assert.Contains(t, down, `<span class="delta tone-bad">`)
	assert.Contains(t, down, `fa-arrow-down`)
	assert.Contains(t, down, `<span>−4.80%</span>`) // the minus sign, which is as wide as the plus
	assert.Contains(t, down, `<span class="sr-only"> (worse)</span>`)
}

// A change is written in three digits whatever its size, so that a column of them has one width.
func TestAChangeIsAlwaysWrittenInThreeDigits(t *testing.T) {
	t.Parallel()

	for change, want := range map[float64]string{100: "+100%", 35.4: "+35.4%", 0.35: "+0.35%", 99.96: "+100%", 9.996: "+10.0%", -35.44: "−35.4%", 1234: "+1234%"} {
		body := serve(readingApp(reading{Change: change}, rDevices, rChange), http.MethodGet, "/admin/e").Body.String()
		assert.Contains(t, body, `<span>`+want+`</span>`)
	}
}

// A response time that falls is good news, and the field says so: the arrow still points the way it moved.
func TestALowerIsBetterChangeIsGoodWhenItFalls(t *testing.T) {
	t.Parallel()

	tile := func(r reading) string {
		return serve(readingApp(r, rDevices, rLatency), http.MethodGet, "/admin/e").Body.String()
	}
	fell := tile(reading{Latency: -12})
	assert.Contains(t, fell, `<span class="delta tone-ok">`)
	assert.Contains(t, fell, `fa-arrow-down`)
	assert.Contains(t, fell, `<span>−12.0%</span>`)

	rose := tile(reading{Latency: 30})
	assert.Contains(t, rose, `<span class="delta tone-bad">`)
	assert.Contains(t, rose, `fa-arrow-up`)
}

// Each trend has its own direction, so one row can hold a figure that is better up and one that is better down: the same rise is good in one and bad in the other.
func TestEachTrendOfARowHasItsOwnDirection(t *testing.T) {
	t.Parallel()

	both := webui.Trend[reading]{Label: "Both", Load: func(r reading) ([]float64, *float64) { return nil, &r.Change }}
	slower := webui.Trend[reading]{Label: "Slower", LowerIsBetter: true, Load: func(r reading) ([]float64, *float64) { return nil, &r.Change }}
	body := serve(readingApp(reading{Change: 5}, both, slower), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `<span class="delta tone-ok">`), 1)
	assert.Equal(t, strings.Count(body, `<span class="delta tone-bad">`), 1)
}

// What the text says and what the colour says are one thing: a change that reads zero as it is shown has no sign, no arrow and no colour,
// though the number behind it is not quite zero.
func TestAChangeThatReadsZeroHasNoDirection(t *testing.T) {
	t.Parallel()

	for _, v := range []float64{0, 0.001, -0.004} {
		body := serve(readingApp(reading{Change: v}, rDevices, rChange), http.MethodGet, "/admin/e").Body.String()
		assert.Contains(t, body, `<span class="delta"><span>0%</span>`)
		assert.False(t, strings.Contains(body, "fa-arrow"))
		assert.False(t, strings.Contains(body, "tone-ok") || strings.Contains(body, "tone-bad"))
	}
}

// A change that is not a number is a dash, with no direction.
func TestAChangeThatIsNotANumberIsADash(t *testing.T) {
	t.Parallel()

	nan := math.NaN()
	body := serve(readingApp(reading{Change: nan}, rDevices, rChange), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<span class="delta"><span>—</span>`)
	assert.False(t, strings.Contains(body, "fa-arrow"))
}

func TestATrendIsAChartWithItsValuesInWords(t *testing.T) {
	t.Parallel()

	r := reading{Devices: 9, Points: []float64{3, 5, 4, 9}}
	body := serve(readingApp(r, rDevices, rTrend), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<svg class="spark" viewBox="0 0 96 20" role="img" aria-label="Devices over a day: 4 values, from 3.00 to 9.00, lowest 3.00, highest 9.00" focusable="false">`)
	assert.Contains(t, body, `<polyline class="spark-line" points="3.0,`)
	assert.Contains(t, body, `<circle class="spark-dot"`) // a line marks its last point when nothing else is
	assert.False(t, strings.Contains(body, "<rect"))
}

// A point that is not a number is left out of the line and of its words.
func TestAPointThatIsNotANumberIsLeftOut(t *testing.T) {
	t.Parallel()

	body := serve(readingApp(reading{Devices: 1, Points: []float64{1, math.NaN(), 3, math.Inf(1)}}, rDevices, rTrend), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `aria-label="Devices over a day: 2 values, from 1.00 to 3.00, lowest 1.00, highest 3.00"`)
	assert.Equal(t, strings.Count(body, `<circle class="spark-dot"`), 1)
}

func TestAnEmptyAndAOnePointTrendAreDrawnWithoutFailing(t *testing.T) {
	t.Parallel()

	// No points draw no chart, and with no change either there is nothing to say: a dash.
	empty := serve(readingApp(reading{Devices: 1}, rDevices, rTrend), http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(empty, "<svg"))
	assert.Contains(t, empty, `<span class="dim" aria-hidden="true">—</span>`)

	one := serve(readingApp(reading{Devices: 1, Points: []float64{7}}, rDevices, rTrend), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, one, `aria-label="Devices over a day: 1 value, 7.00"`)
	assert.Contains(t, one, "<polyline")

	flat := serve(readingApp(reading{Devices: 1, Points: []float64{4, 4, 4}}, rDevices, rTrend), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, flat, `points="3.0,10.0 48.0,10.0 93.0,10.0"`) // level, in the middle of the box
}

// A change and a chart are two parts of one trend, side by side, and either may be left out for a model.
func TestATrendHasAChangeAndAChartSideBySideAndEitherMayBeAbsent(t *testing.T) {
	t.Parallel()

	both := webui.Trend[reading]{Label: "Devices trend", Load: func(r reading) ([]float64, *float64) { return r.Points, &r.Change }}
	body := serve(readingApp(reading{Points: []float64{1, 2}, Change: 5}, rDevices, both), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<span class="trend"><svg class="spark"`)
	assert.True(t, strings.Index(body, `<svg class="spark"`) < strings.Index(body, `<span>+5.00%</span>`)) // the chart, then the change at its right

	only := serve(readingApp(reading{Points: []float64{1, 2}}, rDevices, rTrend), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, only, `<svg class="spark"`)
	assert.False(t, strings.Contains(only, `class="delta`))
}

type listed struct {
	Name     string
	Change   float64
	NoChange bool // no figure to set it against: the trend has a chart and no change
	Points   []float64
}

var (
	lName  = webui.String[listed]{Label: "Name", Load: func(l listed) string { return l.Name }}
	lTrend = webui.Trend[listed]{Label: "Change", Load: func(l listed) ([]float64, *float64) {
		if l.NoChange {
			return l.Points, nil
		}
		return l.Points, &l.Change
	}}
)

func listedApp(columns ...webui.Accessor[listed]) http.Handler {
	rows := []listed{
		{Name: "AMD", Change: -4.27, Points: []float64{3, 2, 1}},
		{Name: "NVDA", Change: -4.12, Points: []float64{1, 2, 3}},
		{Name: "BTC", Change: 4.8, Points: []float64{5, 5, 6}},
		{Name: "XYZ", NoChange: true, Points: []float64{7, 7}},
	}
	if columns == nil {
		columns = []webui.Accessor[listed]{lName, lTrend}
	}
	return editorApp(webui.Table[listed]{
		Title: "Markets", Columns: columns, Export: true,
		Rows: func(context.Context) ([]listed, webui.Loaded) { return rows, webui.Ready() },
	})
}

// The figures of a table are what a row of markets is: a name, and a trend, a change with its chart. The change is a number, so it sorts and filters as one,
// and not the points, whose order says nothing.
func TestATrendIsAColumnThatSortsAndFiltersByItsChange(t *testing.T) {
	t.Parallel()

	h := listedApp()
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<span>−4.27%</span>`)
	assert.Contains(t, body, `<span>+4.80%</span>`)
	assert.Equal(t, strings.Count(body, `<svg class="spark"`), 4)

	order := func(target string) []string {
		body := serve(h, http.MethodGet, target).Body.String()
		type at struct {
			name string
			i    int
		}
		var found []at
		for _, name := range []string{"AMD", "NVDA", "BTC", "XYZ"} {
			if i := strings.Index(body, ">"+name+"<"); i >= 0 {
				found = append(found, at{name, i})
			}
		}
		slices.SortFunc(found, func(a, b at) int { return a.i - b.i })
		var names []string
		for _, f := range found {
			names = append(names, f.name)
		}
		return names
	}
	// A trend with no change sorts first, as an empty number does.
	assert.DeepEqual(t, order("/admin/e?markets.sort=change"), []string{"XYZ", "AMD", "NVDA", "BTC"})
	assert.DeepEqual(t, order("/admin/e?markets.sort=change&markets.desc=true"), []string{"BTC", "NVDA", "AMD", "XYZ"})

	// A range is of the change, and leaves out a row that has none.
	assert.DeepEqual(t, order("/admin/e?markets.min.change=-4.2"), []string{"NVDA", "BTC"})
	assert.DeepEqual(t, order("/admin/e?markets.max.change=-4.2"), []string{"AMD"})
}

// A trend that has no change to sort by is declared with NoSort and NoFilter, which leave the links off its header; asking for them is ignored.
func TestATrendColumnCanBeLeftOutOfSortAndFilter(t *testing.T) {
	t.Parallel()

	chart := webui.Trend[listed]{Label: "Chart", NoSort: true, NoFilter: true, Load: func(l listed) ([]float64, *float64) { return l.Points, nil }}
	h := listedApp(lName, chart)
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `?markets.sort=name`)
	assert.False(t, strings.Contains(body, "markets.sort=chart"))
	assert.False(t, strings.Contains(body, "markets.min.chart"))
	asked := serve(h, http.MethodGet, "/admin/e?markets.sort=chart")
	assert.Equal(t, asked.Code, http.StatusOK)
}

// A file of a table holds the change as a number, as a column of numbers does, and not the points.
func TestAnExportHoldsTheChangeAsANumber(t *testing.T) {
	t.Parallel()

	got := records(t, serve(listedApp(), http.MethodGet, "/admin/e?markets.export=csv").Body.String())
	assert.DeepEqual(t, got[0], []string{"Name", "Change"})
	assert.DeepEqual(t, got[1], []string{"AMD", "-4.27"}) // in three digits, with a hyphen that a spreadsheet reads
	assert.DeepEqual(t, got[3], []string{"BTC", "4.80"})
	assert.DeepEqual(t, got[4], []string{"XYZ", ""}) // no change, no number

	json := serve(listedApp(), http.MethodGet, "/admin/e?markets.export=json").Body.String()
	assert.Contains(t, json, `"Change": -4.27`)
	assert.Contains(t, json, `"Change": null`)
}

// A trend is a field like any other where a form only reads it.
func TestATrendIsAReadOnlyFieldOfAForm(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Form[reading]{
		Title: "Reading", Load: loadReading(reading{Change: 2, Points: []float64{1, 2}}),
		Fields: []webui.Accessor[reading]{rChange, rTrend},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<span>+2.00%</span>`)
	assert.Contains(t, body, `<svg class="spark"`)
	assert.False(t, strings.Contains(body, `name="f0"`)) // nothing to type into
}

func TestATrendColumnIsCheckedLikeAnyColumn(t *testing.T) {
	t.Parallel()

	got := editorErrors(t, webui.Table[listed]{
		Title: "T", Rows: func(context.Context) ([]listed, webui.Loaded) { return nil, webui.Ready() },
		Columns: []webui.Accessor[listed]{webui.Trend[listed]{}, webui.Trend[listed]{Label: "S"}},
	})
	assert.Contains(t, got, "Table.Columns[0] has no Label")
	assert.Contains(t, got, "Table.Columns[0] has no Load")
	assert.Contains(t, got, "Table.Columns[1] has no Load")
}
