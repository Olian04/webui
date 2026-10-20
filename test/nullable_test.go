package webui_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
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
	limitPaused = webui.Bool[limits]{Label: "Paused", Load: func(l limits) *bool { return l.Paused }, Store: func(l *limits, v *bool) { l.Paused = v }}
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
		Load: func(context.Context) (limits, error) {
			mu.Lock()
			defer mu.Unlock()
			return saved, nil
		},
		Fields: fields,
		Submit: webui.Action[limits]{Run: func(_ context.Context, l limits) (webui.Outcome, error) {
			mu.Lock()
			saved = l
			mu.Unlock()
			return webui.Success("Saved"), nil
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/l", Body: form}}}.MustCompile("/admin")
	return h, func() limits {
		mu.Lock()
		defer mu.Unlock()
		return saved
	}
}

func TestAnAbsentValueIsAnEmptyInputAndAPresentOneIsFilledIn(t *testing.T) {
	t.Parallel()

	none, _ := limitsApp(limits{})
	body := serve(none, http.MethodGet, "/admin/l").Body.String()
	assert.Contains(t, body, `name="f0" type="number" value=""`)
	assert.Contains(t, body, `name="f1" type="number" value=""`)
	assert.Contains(t, body, `name="f2" type="number" value=""`)
	// A choice among Yes and No, with the empty one first and neither of them chosen.
	assert.Contains(t, body, `<option value="">—</option> <option value="true" data-badge="badge-mute">Yes</option><option value="false" data-badge="badge-mute">No</option>`)
	assert.False(t, strings.Contains(body, "selected"))

	some, _ := limitsApp(limits{Rate: ptr(0), Ratio: ptr(1.5), Level: ptr(7.0), Paused: ptr(false)})
	body = serve(some, http.MethodGet, "/admin/l").Body.String()
	assert.Contains(t, body, `name="f0" type="number" value="0"`) // zero is a value
	assert.Contains(t, body, `name="f1" type="number" value="1.50"`)
	assert.Contains(t, body, `name="f2" type="number" value="7"`)
	assert.Contains(t, body, `<option value="false" data-badge="badge-mute" selected>No</option>`)
}

func TestAnEmptiedInputStoresNothingAndATypedOneStoresIt(t *testing.T) {
	t.Parallel()

	h, saved := limitsApp(limits{Rate: ptr(5), Ratio: ptr(2.5), Level: ptr(3.0), Paused: ptr(true)})

	rec := post(h, "/admin/l", formValues("f0", "", "f1", "", "f2", "", "f3", ""))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{})

	rec = post(h, "/admin/l", formValues("f0", "0", "f1", "0.25", "f2", "4", "f3", "false"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{Rate: ptr(0), Ratio: ptr(0.25), Level: ptr(4.0), Paused: ptr(false)})

	rec = post(h, "/admin/l", formValues("f0", "7", "f1", "", "f2", "", "f3", "true"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{Rate: ptr(7), Paused: ptr(true)})
}

func TestAnAbsentValueIsStillHeldToItsRulesWhenThereIsOne(t *testing.T) {
	t.Parallel()

	rate := webui.OptionalInt[limits]{Label: "Rate", Rules: webui.NumberRules[int]{Min: ptr(1), Max: ptr(10), Required: true},
		Load: func(l limits) *int { return l.Rate }, Store: func(l *limits, v *int) { l.Rate = v }}
	paused := webui.Bool[limits]{Label: "Paused", Required: true, Load: func(l limits) *bool { return l.Paused }, Store: func(l *limits, v *bool) { l.Paused = v }}
	h, saved := limitsApp(limits{}, rate, paused)

	// Required is what makes a value that must be there.
	rec := post(h, "/admin/l", formValues("f0", "", "f1", ""))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Rate")
	assert.DeepEqual(t, saved(), limits{})

	rec = post(h, "/admin/l", formValues("f0", "11", "f1", "true"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Must be at most 10.")

	rec = post(h, "/admin/l", formValues("f0", "3", "f1", "true"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), limits{Rate: ptr(3), Paused: ptr(true)})
}

func TestABoolRefusesWhatIsNotYesOrNo(t *testing.T) {
	t.Parallel()

	h, saved := limitsApp(limits{Paused: ptr(true)}, limitPaused)
	rec := post(h, "/admin/l", formValues("f0", "maybe"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Choose from the options offered.")
	assert.DeepEqual(t, saved(), limits{Paused: ptr(true)})

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
		webui.Bool[limits]{Label: "Paused", Load: func(l limits) *bool { return l.Paused }},
	}
	none, _ := limitsApp(limits{}, readOnly...)
	body := serve(none, http.MethodGet, "/admin/l").Body.String()
	assert.Equal(t, strings.Count(body, `<span class="dim" aria-hidden="true">—</span>`), 3)
	assert.False(t, strings.Contains(body, `name="f0"`)) // none of them is an input

	some, _ := limitsApp(limits{Rate: ptr(0), Level: ptr(0.0), Paused: ptr(false)}, readOnly...)
	body = serve(some, http.MethodGet, "/admin/l").Body.String()
	assert.False(t, strings.Contains(body, `aria-hidden="true">—</span>`))
	assert.Contains(t, body, ">No<")
}

func limitsTable(rows ...limits) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: webui.Table[limits]{
		Title: "Items", Export: true,
		Rows: func(context.Context) ([]limits, error) { return rows, nil },
		Columns: []webui.Accessor[limits]{
			webui.String[limits]{Label: "Name", Load: func(l limits) string { return l.Name }},
			limitRate, limitRatio, limitLevel, limitPaused,
		},
	}}}}.MustCompile("/admin")
}

var someLimits = []limits{
	{Name: "a", Rate: ptr(30), Ratio: ptr(0.5), Level: ptr(5.0), Paused: ptr(true)},
	{Name: "b"},
	{Name: "c", Rate: ptr(0), Level: ptr(0.0), Paused: ptr(false)},
}

func TestATableShowsADashWhereThereIsNoValueAndAZeroWhereThereIsOne(t *testing.T) {
	t.Parallel()

	body := serve(limitsTable(someLimits...), http.MethodGet, "/admin/s").Body.String()
	// The row that has nothing has a dash for each of the four, and the others have none.
	assert.Equal(t, strings.Count(body, `<span class="dim" aria-hidden="true">—</span>`), 4+1) // and one for the ratio of c
	assert.Contains(t, body, ">30<")
	assert.Contains(t, body, ">0<")
	assert.Contains(t, body, ">Yes<")
	assert.Contains(t, body, ">No<")
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
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/l", Body: webui.Form[limits]{
			Load:   func(context.Context) (limits, error) { return limits{}, nil },
			Fields: fields,
			Submit: webui.Action[limits]{Run: func(context.Context, limits) (webui.Outcome, error) { return webui.Outcome{}, nil }},
		}}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	assert.Contains(t, problems(webui.OptionalInt[limits]{Label: "Rate", Rules: webui.NumberRules[int]{Required: true}, Load: func(l limits) *int { return l.Rate }}), "is Required but has no Store")
	assert.Contains(t, problems(webui.Bool[limits]{Label: "Paused", Required: true, Load: func(l limits) *bool { return l.Paused }}), "is Required but has no Store")
	assert.Contains(t, problems(webui.OptionalSlider[limits]{Label: "Level", Load: func(l limits) *float64 { return l.Level }}), "has no range")
	assert.Contains(t, problems(webui.OptionalInt[limits]{Label: "Rate"}), "Load")
	assert.Contains(t, problems(webui.Bool[limits]{Label: "Paused"}), "Load")
}
