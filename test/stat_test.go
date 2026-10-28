package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// reading is a figure with a change and the points it was made from.
type reading struct {
	Name    string
	Devices int
	Health  []string
	Disk    float64
	Change  float64
	Latency float64
	Points  []float64
	Hour    int
}

var (
	rDevices = webui.Int[reading]{Label: "Devices", Load: func(r reading) int { return r.Devices }}
	rHealth  = webui.Select[reading]{Label: "Health", Load: func(r reading) []string { return r.Health }, Options: []webui.Option{{Value: "ok", Label: "healthy", Tone: webui.ToneOK}}}
	rDisk    = webui.Slider[reading]{Label: "Disk", Min: 0, Max: 100, Load: func(r reading) float64 { return r.Disk }}
	rChange  = webui.Delta[reading]{Label: "Change", Load: func(r reading) float64 { return r.Change }, Precision: 2, Unit: "%"}
	rLatency = webui.Delta[reading]{Label: "Latency", Load: func(r reading) float64 { return r.Latency }, Better: webui.Lower, Unit: " ms"}
	rTrend   = webui.Series[reading]{Label: "Devices over a day", Load: func(r reading) []float64 { return r.Points }}
)

func loadReading(r reading) func(context.Context) (reading, webui.Loaded) {
	return func(context.Context) (reading, webui.Loaded) { return r, webui.Ready() }
}

func statApp(r reading, tiles ...webui.StatTile[reading]) http.Handler {
	return editorApp(webui.Stat[reading]{Title: "System", Load: loadReading(r), Tiles: tiles})
}

func TestAStatIsAListOfFiguresInAPanelThatNeedsNoScript(t *testing.T) {
	t.Parallel()

	h := statApp(reading{Devices: 37, Health: []string{"ok"}, Disk: 71},
		webui.StatTile[reading]{Value: rDevices, Help: "Devices that have reported today"},
		webui.StatTile[reading]{Label: "Collector", Value: rHealth},
		webui.StatTile[reading]{Value: rDisk},
	)
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()

	assert.Contains(t, body, `<h2 class="panel-title">System</h2>`) // the panel's heading
	assert.Contains(t, body, `<dl class="stats">`)                  // terms and values, to a reader who goes by structure
	assert.Equal(t, strings.Count(body, `<div class="stat">`), 3)
	assert.Contains(t, body, `<dt class="stat-label"><span>Devices</span>`) // a tile with no Label is called what its value is
	assert.Contains(t, body, `<dt class="stat-label"><span>Collector</span>`)
	assert.Contains(t, body, `<dd class="stat-value">37</dd>`)
	assert.Contains(t, body, "Devices that have reported today") // the help, in the popover beside the label
	assert.Contains(t, body, `class="badge badge-ok"`)           // a select is the badge it is in a table
	assert.Contains(t, body, `class="gauge"`)                    // and a slider is its bar
	assert.False(t, strings.Contains(body, "<script>"))
}

// Every value is shown as it is in a cell, so a Format of its own is kept: a tile has no way of showing a value that a column cannot.
func TestATilesValueIsShownAsItsAccessorFormatsIt(t *testing.T) {
	t.Parallel()

	size := webui.Int[reading]{Label: "Stored", Load: func(r reading) int { return r.Devices }, Format: func(int) string { return "1.5 GiB" }}
	body := serve(statApp(reading{Devices: 5}, webui.StatTile[reading]{Value: size}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<dd class="stat-value">1.5 GiB</dd>`)
}

func TestAChangeIsItsSignAnArrowAndAColourForWhetherThatWayIsGood(t *testing.T) {
	t.Parallel()

	tile := func(r reading) string {
		return serve(statApp(r, webui.StatTile[reading]{Value: rDevices, Change: rChange}), http.MethodGet, "/admin/e").Body.String()
	}

	up := tile(reading{Change: 4.35})
	assert.Contains(t, up, `<dd class="stat-change">`)
	assert.Contains(t, up, `<span class="delta delta-ok">`)
	assert.Contains(t, up, `fa-arrow-up`)
	assert.Contains(t, up, `<span>+4.35%</span>`)
	assert.Contains(t, up, `<span class="sr-only"> (better)</span>`) // what the colour means, for a reader who does not see it

	down := tile(reading{Change: -4.8})
	assert.Contains(t, down, `<span class="delta delta-bad">`)
	assert.Contains(t, down, `fa-arrow-down`)
	assert.Contains(t, down, `<span>−4.80%</span>`) // the minus sign, which is as wide as the plus
	assert.Contains(t, down, `<span class="sr-only"> (worse)</span>`)
}

// A response time that falls is good news, and the field says so: the arrow still points the way it moved.
func TestALowerIsBetterChangeIsGoodWhenItFalls(t *testing.T) {
	t.Parallel()

	tile := func(r reading) string {
		return serve(statApp(r, webui.StatTile[reading]{Value: rDevices, Change: rLatency}), http.MethodGet, "/admin/e").Body.String()
	}
	fell := tile(reading{Latency: -12})
	assert.Contains(t, fell, `<span class="delta delta-ok">`)
	assert.Contains(t, fell, `fa-arrow-down`)
	assert.Contains(t, fell, `<span>−12 ms</span>`)

	rose := tile(reading{Latency: 30})
	assert.Contains(t, rose, `<span class="delta delta-bad">`)
	assert.Contains(t, rose, `fa-arrow-up`)
}

// What the text says and what the colour says are one thing: a change that reads zero as it is shown has no sign, no arrow and no colour,
// though the number behind it is not quite zero.
func TestAChangeThatReadsZeroHasNoDirection(t *testing.T) {
	t.Parallel()

	for _, v := range []float64{0, 0.001, -0.004} {
		body := serve(statApp(reading{Change: v}, webui.StatTile[reading]{Value: rDevices, Change: rChange}), http.MethodGet, "/admin/e").Body.String()
		assert.Contains(t, body, `<span class="delta"><span>0.00%</span>`)
		assert.False(t, strings.Contains(body, "fa-arrow"))
		assert.False(t, strings.Contains(body, "delta-ok") || strings.Contains(body, "delta-bad"))
	}
}

func TestATrendIsAChartWithItsValuesInWords(t *testing.T) {
	t.Parallel()

	r := reading{Devices: 9, Points: []float64{3, 5, 4, 9}}
	body := serve(statApp(r, webui.StatTile[reading]{Value: rDevices, Trend: rTrend}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<dd class="stat-trend">`)
	assert.Contains(t, body, `<svg class="spark" viewBox="0 0 96 28" role="img" aria-label="Devices over a day: 4 values, from 3 to 9, lowest 3, highest 9" focusable="false">`)
	assert.Contains(t, body, `<polyline class="spark-line" points="3.0,`)
	assert.Contains(t, body, `<circle class="spark-dot"`) // a line marks its last point when nothing else is
	assert.False(t, strings.Contains(body, "<rect"))
}

func TestBarsStandOnZeroAndTheHighlightedOneIsMarked(t *testing.T) {
	t.Parallel()

	bars := webui.Series[reading]{
		Label: "Forecast", Style: webui.SeriesBars, Unit: "°",
		Load:      func(r reading) []float64 { return r.Points },
		Highlight: func(r reading) int { return r.Hour },
	}
	body := serve(statApp(reading{Devices: 1, Points: []float64{6, 8, 12, 9}, Hour: 2}, webui.StatTile[reading]{Value: rDevices, Trend: bars}), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `<rect class="spark-bar`), 4)
	assert.Equal(t, strings.Count(body, `class="spark-bar spark-mark"`), 1)
	assert.Contains(t, body, `aria-label="Forecast: 4 values, from 6° to 9°, lowest 6°, highest 12°"`)
	assert.False(t, strings.Contains(body, "<polyline"))
	// The tallest bar, which is 12, reaches the top of its box, and the others are shorter in proportion: heights are the values.
	assert.Contains(t, body, `y="3.0"`)

	// An index outside the series marks none.
	none := webui.Series[reading]{Label: "F", Style: webui.SeriesBars, Load: func(r reading) []float64 { return r.Points }, Highlight: func(reading) int { return -1 }}
	body = serve(statApp(reading{Devices: 1, Points: []float64{1, 2}}, webui.StatTile[reading]{Value: rDevices, Trend: none}), http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, "spark-mark"))
}

// A point that is not a number is left out, and what a Highlight says is the index of the points as they were loaded.
func TestAPointThatIsNotANumberIsLeftOutAndTheHighlightKeepsItsIndex(t *testing.T) {
	t.Parallel()

	nan := 0.0
	nan /= nan
	bars := webui.Series[reading]{
		Label: "F", Style: webui.SeriesBars, Load: func(r reading) []float64 { return r.Points },
		Highlight: func(reading) int { return 2 },
	}
	body := serve(statApp(reading{Devices: 1, Points: []float64{1, nan, 3}}, webui.StatTile[reading]{Value: rDevices, Trend: bars}), http.MethodGet, "/admin/e").Body.String()
	assert.Equal(t, strings.Count(body, `<rect class="spark-bar`), 2)
	assert.Contains(t, body, `aria-label="F: 2 values, from 1 to 3, lowest 1, highest 3"`)
	// It is the second bar that is marked: the third point as loaded, which is the second that is a number.
	assert.Equal(t, strings.Count(body, `class="spark-bar spark-mark"`), 1)
	assert.True(t, strings.Index(body, `spark-mark`) > strings.Index(body, `<rect class="spark-bar"`))
	assert.True(t, strings.HasSuffix(strings.SplitN(body, "</svg>", 2)[0], `rx="1"></rect>`))
	last := strings.LastIndex(strings.SplitN(body, "</svg>", 2)[0], "<rect")
	assert.Contains(t, strings.SplitN(body, "</svg>", 2)[0][last:], "spark-mark")
}

func TestAnEmptyAndAOnePointSeriesAreDrawnWithoutFailing(t *testing.T) {
	t.Parallel()

	empty := serve(statApp(reading{Devices: 1}, webui.StatTile[reading]{Value: rDevices, Trend: rTrend}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, empty, `aria-label="Devices over a day: no values"`)
	assert.False(t, strings.Contains(empty, "<polyline"))

	one := serve(statApp(reading{Devices: 1, Points: []float64{7}}, webui.StatTile[reading]{Value: rDevices, Trend: rTrend}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, one, `aria-label="Devices over a day: 1 value, 7"`)
	assert.Contains(t, one, "<polyline")

	flat := serve(statApp(reading{Devices: 1, Points: []float64{4, 4, 4}}, webui.StatTile[reading]{Value: rDevices, Trend: rTrend}), http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, flat, `points="3.0,14.0 48.0,14.0 93.0,14.0"`) // level, in the middle of the box
}

func TestATileHasNoChangeOrTrendUnlessItsLoadIsSet(t *testing.T) {
	t.Parallel()

	body := serve(statApp(reading{Devices: 1}, webui.StatTile[reading]{Value: rDevices}), http.MethodGet, "/admin/e").Body.String()
	assert.False(t, strings.Contains(body, "stat-change"))
	assert.False(t, strings.Contains(body, "stat-trend"))
}

func TestAStatRefreshesAsALeafOfItsOwn(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Stack{
		webui.Stat[reading]{Title: "First", Load: loadReading(reading{Devices: 11}), Tiles: []webui.StatTile[reading]{{Value: rDevices}}},
		webui.Stat[reading]{Title: "Second", Load: loadReading(reading{Devices: 22}), Tiles: []webui.StatTile[reading]{{Value: rDevices}}},
	})
	req := httptest.NewRequest(http.MethodGet, "/admin/e", nil)
	req.Header.Set("X-Webui-Leaf", "p.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), `<dd class="stat-value">22</dd>`)
	assert.False(t, strings.Contains(rec.Body.String(), ">11<"))
	assert.False(t, strings.Contains(rec.Body.String(), "<html"))
}

// A figure that is still being worked out is asked for again, as any panel is.
func TestAStatThatIsNotFinalIsAskedForAgain(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Stat[reading]{
		Title: "S", Tiles: []webui.StatTile[reading]{{Value: rDevices}},
		Load: func(context.Context) (reading, webui.Loaded) {
			return reading{}, webui.Ready().RefreshAfter(5 * time.Second)
		},
	})
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `data-refresh-after="5"`)
}

func TestAFailingStatLoadFailsThePanelAndNotThePage(t *testing.T) {
	t.Parallel()

	h := editorApp(webui.Stat[reading]{
		Title: "System", Tiles: []webui.StatTile[reading]{{Value: rDevices}},
		Load: func(context.Context) (reading, webui.Loaded) {
			return reading{}, webui.Failed(errors.New("store secret-bucket is gone"))
		},
	})
	rec := serve(h, http.MethodGet, "/admin/e")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "secret-bucket"))
	assert.False(t, strings.Contains(rec.Body.String(), "stat-value"))
}

// ---------------------------------------------------------------- in a table

type listed struct {
	Name   string
	Change float64
	Points []float64
}

var (
	lName   = webui.String[listed]{Label: "Name", Load: func(l listed) string { return l.Name }}
	lChange = webui.Delta[listed]{Label: "Change", Load: func(l listed) float64 { return l.Change }, Precision: 1, Unit: "%"}
	lTrend  = webui.Series[listed]{Label: "Trend", Load: func(l listed) []float64 { return l.Points }}
)

func listedApp() http.Handler {
	rows := []listed{
		{Name: "AMD", Change: -4.27, Points: []float64{3, 2, 1}},
		{Name: "NVDA", Change: -4.12, Points: []float64{1, 2, 3}},
		{Name: "BTC", Change: 4.8, Points: []float64{5, 5, 6}},
	}
	return editorApp(webui.Table[listed]{
		Title: "Markets", Columns: []webui.Accessor[listed]{lName, lChange, lTrend}, Export: true,
		Rows: func(context.Context) ([]listed, webui.Loaded) { return rows, webui.Ready() },
	})
}

// The figures of a table are what a row of markets is: a name, a change and a chart. The change is a number, so it sorts and filters as one.
func TestAChangeAndATrendAreColumnsAndTheChangeSortsAsANumber(t *testing.T) {
	t.Parallel()

	h := listedApp()
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `<span>−4.3%</span>`)
	assert.Contains(t, body, `<span>+4.8%</span>`)
	assert.Equal(t, strings.Count(body, `<svg class="spark"`), 3)

	asc := serve(h, http.MethodGet, "/admin/e?markets.sort=change").Body.String()
	assert.True(t, strings.Index(asc, "AMD") < strings.Index(asc, "NVDA") && strings.Index(asc, "NVDA") < strings.Index(asc, "BTC"))
	desc := serve(h, http.MethodGet, "/admin/e?markets.sort=change&markets.desc=true").Body.String()
	assert.True(t, strings.Index(desc, "BTC") < strings.Index(desc, "NVDA"))
}

// A list of numbers has no order and no one value to filter by, so its header has no link for either.
func TestATrendColumnIsNeitherSortedNorFilteredBy(t *testing.T) {
	t.Parallel()

	h := listedApp()
	body := serve(h, http.MethodGet, "/admin/e").Body.String()
	assert.Contains(t, body, `?markets.sort=change`)
	assert.False(t, strings.Contains(body, "markets.sort=trend"))
	assert.False(t, strings.Contains(body, "markets.filter.trend"))
	// And asking for it in the address is ignored.
	asked := serve(h, http.MethodGet, "/admin/e?markets.sort=trend")
	assert.Equal(t, asked.Code, http.StatusOK)
}

// A file of a table holds the numbers, as a column of numbers does, and the points of a trend, separated by semicolons.
func TestAnExportHoldsTheChangeAsANumberAndTheTrendAsItsPoints(t *testing.T) {
	t.Parallel()

	got := records(t, serve(listedApp(), http.MethodGet, "/admin/e?markets.export=csv").Body.String())
	assert.DeepEqual(t, got[0], []string{"Name", "Change", "Trend"})
	assert.DeepEqual(t, got[1], []string{"AMD", "-4.3", "3;2;1"}) // the number as it is shown to its precision, with a hyphen that a spreadsheet reads
	assert.DeepEqual(t, got[3], []string{"BTC", "4.8", "5;5;6"})
}

// A change is a field like any other where a form only reads it.
func TestAChangeAndATrendAreReadOnlyFieldsOfAForm(t *testing.T) {
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

// ----------------------------------------------------------------- validate

func TestAStatThatCannotBeDrawnIsACompileError(t *testing.T) {
	t.Parallel()

	load := loadReading(reading{})
	for name, tc := range map[string]struct {
		body webui.PageBody
		want string
	}{
		"no load":          {webui.Stat[reading]{Tiles: []webui.StatTile[reading]{{Value: rDevices}}}, "a Stat has no Load"},
		"no tiles":         {webui.Stat[reading]{Load: load}, "a Stat has no Tiles"},
		"nil value":        {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{}}}, "Stat.Tiles[0].Value is nil"},
		"a group":          {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{Value: webui.Group[reading]{rDevices}}}}, "Stat.Tiles[0].Value is a Group"},
		"a value no load":  {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{Value: webui.Int[reading]{Label: "X"}}}}, "Stat.Tiles[0].Value has no Load"},
		"a change no load": {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{Value: rDevices, Change: webui.Delta[reading]{Unit: "%"}}}}, "Stat.Tiles[0].Change has no Load"},
		"a trend no load":  {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{Value: rDevices, Trend: webui.Series[reading]{Label: "T"}}}}, "Stat.Tiles[0].Trend has no Load"},
		"unknown better": {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{Value: rDevices, Change: webui.Delta[reading]{
			Load: func(reading) float64 { return 1 }, Better: "sideways"}}}}, `Stat.Tiles[0].Change has the unknown Better "sideways"`},
		"unknown style": {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{Value: rDevices, Trend: webui.Series[reading]{
			Load: func(reading) []float64 { return nil }, Style: "pie"}}}}, `Stat.Tiles[0].Trend has the unknown Style "pie"`},
		"negative precision": {webui.Stat[reading]{Load: load, Tiles: []webui.StatTile[reading]{{Value: rDevices, Change: webui.Delta[reading]{
			Load: func(reading) float64 { return 1 }, Precision: -1}}}}, "Stat.Tiles[0].Change has a negative Precision"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, editorErrors(t, tc.body), tc.want)
		})
	}
}

func TestAColumnOfChangesAndTrendsIsCheckedLikeAnyColumn(t *testing.T) {
	t.Parallel()

	got := editorErrors(t, webui.Table[listed]{
		Title: "T", Rows: func(context.Context) ([]listed, webui.Loaded) { return nil, webui.Ready() },
		Columns: []webui.Accessor[listed]{webui.Delta[listed]{Unit: "%"}, webui.Series[listed]{Label: "S"}},
	})
	assert.Contains(t, got, "Table.Columns[0] has no Label")
	assert.Contains(t, got, "Table.Columns[0] has no Load")
	assert.Contains(t, got, "Table.Columns[1] has no Load")
}
