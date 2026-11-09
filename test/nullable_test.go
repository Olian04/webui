package webui_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// limits is what the nullable fields hold: each may be absent, and absent is not zero.
type limits struct {
	Name   string
	Rate   *int
	Ratio  *float64
	Level  *float64
	Paused *bool
}

var (
	limitRate   = webui.OptionalInt[limits]{Label: "Rate", Load: func(l limits) *int { return l.Rate }, Store: func(l *limits, v *int) { l.Rate = v }}
	limitRatio  = webui.OptionalFloat[limits]{Label: "Ratio", Precision: 2, Load: func(l limits) *float64 { return l.Ratio }, Store: func(l *limits, v *float64) { l.Ratio = v }}
	limitLevel  = webui.OptionalSlider[limits]{Label: "Level", Min: 0, Max: 10, StepSize: 1, Load: func(l limits) *float64 { return l.Level }, Store: func(l *limits, v *float64) { l.Level = v }}
	limitPaused = webui.OptionalBool[limits]{Label: "Paused", Load: func(l limits) *bool { return l.Paused }, Store: func(l *limits, v *bool) { l.Paused = v }}
)

// limitsApp is a form of limits: what it saved is what its Run was given.
func limitsApp(start limits, fields ...webui.Accessor[limits]) (http.Handler, func() limits) {
	var (
		mu    sync.Mutex
		saved = start
	)
	if len(fields) == 0 {
		fields = []webui.Accessor[limits]{limitRate, limitRatio, limitLevel, limitPaused}
	}
	form := webui.Form[limits]{
		Load: func(context.Context) (limits, webui.Loaded) {
			mu.Lock()
			defer mu.Unlock()
			return saved, webui.Ready()
		},
		Fields: fields,
		Submit: webui.Action[limits]{Run: func(_ context.Context, l limits) (webui.Outcome, error) {
			mu.Lock()
			saved = l
			mu.Unlock()
			return webui.Success("Saved"), nil
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/l", View: form}}}.MustCompile("/admin")
	return h, func() limits {
		mu.Lock()
		defer mu.Unlock()
		return saved
	}
}

func TestAnAbsentValueIsAnEmptyInputAndAPresentOneIsFilledIn(t *testing.T) {
	t.Parallel()

	none, _ := limitsApp(limits{})
	p := pageOf(t, none, "/admin/l")
	for _, name := range []string{"f0", "f1", "f2"} {
		f, ok := p.Field(name)
		assert.True(t, ok)
		assert.Equal(t, f.Type, "number")
		assert.Equal(t, f.Value, "")
	}
	// A choice among Yes and No, with the empty one first and neither of them chosen.
	paused, _ := p.Field("f3")
	assert.DeepEqual(t, paused.Options, []page.Option{{Value: "", Text: "—"}, {Value: "true", Text: "Yes"}, {Value: "false", Text: "No"}})
	assert.Equal(t, paused.Value, "")

	some, _ := limitsApp(limits{Rate: new(0), Ratio: new(1.5), Level: new(7.0), Paused: new(false)})
	p = pageOf(t, some, "/admin/l")
	for name, want := range map[string]string{"f0": "0", "f1": "1.50", "f2": "7", "f3": "false"} { // zero is a value
		f, _ := p.Field(name)
		assert.Equal(t, f.Value, want)
	}
}

func TestAnEmptiedInputStoresNothingAndATypedOneStoresIt(t *testing.T) {
	t.Parallel()

	h, saved := limitsApp(limits{Rate: new(5), Ratio: new(2.5), Level: new(3.0), Paused: new(true)})

	rec := post(h, "/admin/l", formValues("f0", "", "f1", "", "f2", "", "f3", ""))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{})

	rec = post(h, "/admin/l", formValues("f0", "0", "f1", "0.25", "f2", "4", "f3", "false"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{Rate: new(0), Ratio: new(0.25), Level: new(4.0), Paused: new(false)})

	rec = post(h, "/admin/l", formValues("f0", "7", "f1", "", "f2", "", "f3", "true"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{Rate: new(7), Paused: new(true)})
}

func TestAnAbsentValueIsStillHeldToItsRulesWhenThereIsOne(t *testing.T) {
	t.Parallel()

	rate := webui.OptionalInt[limits]{Label: "Rate", Rules: webui.NumberRules[int]{Min: new(1), Max: new(10)},
		Load: func(l limits) *int { return l.Rate }, Store: func(l *limits, v *int) { l.Rate = v }}
	paused := webui.OptionalBool[limits]{Label: "Paused", Load: func(l limits) *bool { return l.Paused }, Store: func(l *limits, v *bool) { l.Paused = v }}
	h, saved := limitsApp(limits{Rate: new(4), Paused: new(true)}, rate, paused)

	// An optional value may be left empty, and is then stored as nil: there is no Required to say otherwise.
	rec := post(h, "/admin/l", formValues("f0", "", "f1", ""))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{})

	rec = post(h, "/admin/l", formValues("f0", "11", "f1", "true"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Must be at most 10.")

	rec = post(h, "/admin/l", formValues("f0", "3", "f1", "true"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{Rate: new(3), Paused: new(true)})
}

func TestAnOptionalBoolRefusesWhatIsNotYesOrNo(t *testing.T) {
	t.Parallel()

	h, saved := limitsApp(limits{Paused: new(true)}, limitPaused)
	rec := post(h, "/admin/l", formValues("f0", "maybe"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose from the options offered.")
	assert.DeepEqual(t, saved(), limits{Paused: new(true)})

	// A number of a slider is held to its range and its steps, an absent one or not.
	h, saved = limitsApp(limits{}, limitLevel)
	rec = post(h, "/admin/l", formValues("f0", "11"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.DeepEqual(t, saved(), limits{})
}

func TestAReadOnlyNullableFieldShowsADashForNothingAndTheValueOtherwise(t *testing.T) {
	t.Parallel()

	readOnly := []webui.Accessor[limits]{
		webui.OptionalInt[limits]{Label: "Rate", Load: func(l limits) *int { return l.Rate }},
		webui.OptionalSlider[limits]{Label: "Level", Min: 0, Max: 10, Load: func(l limits) *float64 { return l.Level }},
		webui.OptionalBool[limits]{Label: "Paused", Load: func(l limits) *bool { return l.Paused }},
	}
	none, _ := limitsApp(limits{}, readOnly...)
	p := pageOf(t, none, "/admin/l")
	// Each reads as empty, and none of them is an input.
	assert.Equal(t, len(p.ByText("empty")), 3)
	_, isInput := p.Field("f0")
	assert.False(t, isInput)

	some, _ := limitsApp(limits{Rate: new(0), Level: new(0.0), Paused: new(false)}, readOnly...)
	p = pageOf(t, some, "/admin/l")
	assert.Equal(t, len(p.ByText("empty")), 0)
	assert.Equal(t, len(p.ByText("No")), 1)
}

func limitsTable(rows ...limits) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", View: webui.Table[limits]{
		Title: "Items", Export: true,
		Loader: webui.RowsLoader[limits]{Load: func(context.Context) ([]limits, webui.Loaded) { return rows, webui.Ready() }},
		Columns: []webui.Accessor[limits]{
			webui.String[limits]{Label: "Name", Load: func(l limits) string { return l.Name }},
			limitRate, limitRatio, limitLevel, limitPaused,
		},
	}}}}.MustCompile("/admin")
}

var someLimits = []limits{
	{Name: "a", Rate: new(30), Ratio: new(0.5), Level: new(5.0), Paused: new(true)},
	{Name: "b"},
	{Name: "c", Rate: new(0), Level: new(0.0), Paused: new(false)},
}

func TestATableShowsADashWhereThereIsNoValueAndAZeroWhereThereIsOne(t *testing.T) {
	t.Parallel()

	table := page.One(t, "the table", pageOf(t, limitsTable(someLimits...), "/admin/s").ByRole("table", ""))
	// The row that has nothing is empty in each of the four, and c is empty in its ratio only.
	assert.DeepEqual(t, table.Rows(), [][]string{
		{"a", "30", "0.50", "5", "Yes"},
		{"b", "empty", "empty", "empty", "empty"},
		{"c", "0", "empty", "0", "No"},
	})
}

func TestAnAbsentNumberSortsBeforeEveryNumberAndIsInNoRange(t *testing.T) {
	t.Parallel()

	names := func(target string) []string {
		var got []string
		for _, line := range records(t, serve(limitsTable(someLimits...), http.MethodGet, target).Body.String())[1:] {
			got = append(got, line[0])
		}
		return got
	}
	assert.DeepEqual(t, names("/admin/s?items.export=csv&items.sort=rate"), []string{"b", "c", "a"})
	assert.DeepEqual(t, names("/admin/s?items.export=csv&items.sort=rate&items.desc=true"), []string{"a", "c", "b"})
	assert.DeepEqual(t, names("/admin/s?items.export=csv&items.min.rate=0"), []string{"a", "c"})
}

func TestAnExportOfNullableFieldsIsEmptyInCSVAndNullInJSON(t *testing.T) {
	t.Parallel()

	h := limitsTable(someLimits...)
	rows := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv").Body.String())
	assert.DeepEqual(t, rows[1], []string{"a", "30", "0.50", "5", "Yes"})
	assert.DeepEqual(t, rows[2], []string{"b", "", "", "", ""})
	assert.DeepEqual(t, rows[3], []string{"c", "0", "", "0", "No"})

	var got []map[string]any
	assert.NoError(t, json.Unmarshal(serve(h, http.MethodGet, "/admin/s?items.export=json").Body.Bytes(), &got))
	assert.DeepEqual(t, got[0], map[string]any{"Name": "a", "Rate": float64(30), "Ratio": 0.5, "Level": float64(5), "Paused": true})
	assert.DeepEqual(t, got[1], map[string]any{"Name": "b", "Rate": nil, "Ratio": nil, "Level": nil, "Paused": nil})
	assert.DeepEqual(t, got[2], map[string]any{"Name": "c", "Rate": float64(0), "Ratio": nil, "Level": float64(0), "Paused": false})
}

func TestABoolFiltersByItsChoicesInATable(t *testing.T) {
	t.Parallel()

	h := limitsTable(someLimits...)
	got := records(t, serve(h, http.MethodGet, "/admin/s?items.export=csv&items.filter.paused=false").Body.String())
	assert.Equal(t, len(got), 2)
	assert.Equal(t, got[1][0], "c")
}

func TestNullableFieldsAreCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(fields ...webui.Accessor[limits]) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/l", View: webui.Form[limits]{
			Load:   func(context.Context) (limits, webui.Loaded) { return limits{}, webui.Ready() },
			Fields: fields,
			Submit: webui.Action[limits]{Run: func(context.Context, limits) (webui.Outcome, error) { return webui.Outcome{}, nil }},
		}}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	refuses(t, problems(webui.OptionalSlider[limits]{Label: "Level", Load: func(l limits) *float64 { return l.Level }}), "range")
	refuses(t, problems(webui.OptionalInt[limits]{Label: "Rate"}), "Load")
	refuses(t, problems(webui.OptionalBool[limits]{Label: "Paused"}), "Load")
}
