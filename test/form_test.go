package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

var (
	formID = webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}
	formIP = webui.String[Device]{
		Label:    "IP",
		Load:     func(d Device) string { return d.Ip },
		Store:    func(d *Device, v string) { d.Ip = v },
		Required: true,
		Rules: webui.StringRules{
			MinLen: 7, MaxLen: 15,
			Pattern: &webui.PatternRule{Expr: `\d{1,3}(\.\d{1,3}){3}`, Message: "must be a valid IPv4 address"},
		},
	}
	formCount = webui.Int[Device]{
		Label: "Occurrences", Load: func(d Device) int { return d.Count }, Store: func(d *Device, v int) { d.Count = v },
		Rules: webui.NumberRules[int]{Min: new(0), Max: new(100)},
	}
)

func formApp(form webui.Form[Device]) http.Handler {
	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: webui.Stack{}}
	details := webui.Page[detailsArgs]{Path: "/device/{id}", Body: form}
	return webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")
}

func okForm() webui.Form[Device] {
	return webui.Form[Device]{
		Title: "Configuration",
		Desc:  "One device.",
		Load: func(ctx context.Context) (Device, webui.Loaded) {
			a := webui.ArgsOf[detailsArgs](ctx)
			return Device{Id: a.Id, Ip: "10.0.0.1", Count: 7}, webui.Ready()
		},
		Fields: []webui.Accessor[Device]{webui.Group[Device]{formID, formIP}, formCount},
		Submit: webui.Action[Device]{Run: func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }},
	}
}

func TestFormRendersFieldsRulesAndReadOnly(t *testing.T) {
	t.Parallel()

	body := serve(formApp(okForm()), http.MethodGet, "/admin/device/dev1").Body.String()

	assert.Contains(t, body, `<h2 class="panel-title">Configuration</h2>`)
	assert.Contains(t, body, `<form action="/admin/device/dev1" method="post"`)
	assert.Contains(t, body, `name="_leaf" value="p"`)

	// ID has no Store: shown as output, not as an input that is refused, and never
	// submitted (no name).
	assert.Contains(t, body, `role="group" aria-labelledby="p-f0_0-label">dev1</div>`)
	assert.False(t, strings.Contains(body, `value="dev1"`))
	assert.False(t, strings.Contains(body, `name="f0_0"`))

	// IP carries its rules as HTML constraint attributes, and states them.
	assert.Contains(t, body, `name="f0_1" type="text" value="10.0.0.1"`)
	assert.Contains(t, body, `required minlength="7" maxlength="15" pattern="\d{1,3}(\.\d{1,3}){3}"`)
	assert.Contains(t, body, `<span class="req" aria-hidden="true">*</span>`)
	// Nothing is printed beneath the input while its value is good: the rules, the
	// pattern's message among them, are in the popover beside the label.
	assert.False(t, strings.Contains(body, `class="hint"`))
	for _, rule := range []string{"7–15 characters", "must be a valid IPv4 address"} {
		assert.Contains(t, body, `<span class="tip-item">`+rule+`</span>`)
	}
	assert.Equal(t, strings.Count(body, "must be a valid IPv4 address"), 4)           // the popover, its accessible name, the title, and what the browser says
	assert.False(t, strings.Contains(body, `<span class="tip-item">Required</span>`)) // the asterisk says it

	// A number is a number input with its bounds; zero is a real bound.
	assert.Contains(t, body, `name="f1" type="number" value="7" min="0" max="100" step="1"`)
	assert.Contains(t, body, `<span class="tip-item">0–100</span>`) // its bounds, in the popover

	// A group is layout: its fields side by side, with no frame of its own.
	assert.Contains(t, body, `class="field-row"`)
	assert.False(t, strings.Contains(body, `class="group"`))
	assert.False(t, strings.Contains(body, "group-legend"))

	// Submit is the primary button; Cancel goes to the parent in the breadcrumb.
	assert.Contains(t, body, `class="btn btn-primary btn-sm" type="submit">Save</button>`)
	assert.Contains(t, body, `class="btn btn-ghost btn-sm" href="/admin/device">Cancel</a>`)
}

// A field tells the browser what to say when it refuses a value, in the words the server uses beneath the field, by the
// names of the properties of the control's ValidityState; and what a matching value looks like, as its title, which a
// browser says after its own words when there is no script. A field that is only read says nothing.
func TestAFieldTellsTheBrowserWhatToSayWhenItRefusesAValue(t *testing.T) {
	t.Parallel()

	body := serve(formApp(okForm()), http.MethodGet, "/admin/device/dev1").Body.String()

	// One attribute for each message, named for the ValidityState property in kebab case.
	for _, want := range []string{
		`data-msg-pattern-mismatch="must be a valid IPv4 address"`, `data-msg-too-long="Must be at most 15 characters."`,
		`data-msg-too-short="Must be at least 7 characters."`, `data-msg-value-missing="This field is required."`,
		`data-msg-bad-input="Must be a whole number."`, `data-msg-range-overflow="Must be at most 100."`,
		`data-msg-range-underflow="Must be at least 0."`, `data-msg-step-mismatch="Must be a whole number."`,
	} {
		assert.Contains(t, body, want)
	}
	assert.Contains(t, body, `title="must be a valid IPv4 address"`)
	assert.False(t, strings.Contains(body, "data-validity")) // there is no JSON of them

	// The ID has no Store: it is not a control, and has nothing to say.
	assert.Equal(t, strings.Count(body, "data-msg-value-missing="), 1)
	assert.Equal(t, strings.Count(body, "data-msg-range-overflow="), 1)

	// A form that cannot be submitted has no controls to refuse anything.
	f := okForm()
	f.Submit.Guard = func(context.Context, Device) error { return errors.New("requires the editor role") }
	assert.False(t, strings.Contains(serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String(), "data-msg-"))
}

func TestAFormTheVisitorMayNotSubmitOffersBackInPlaceOfSaveAndCancel(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Submit.Guard = func(context.Context, Device) error { return errors.New("requires the editor role") }
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()

	assert.False(t, strings.Contains(body, ">Save</button>"))
	assert.False(t, strings.Contains(body, ">Cancel</a>"))
	assert.Contains(t, body, `class="btn btn-ghost btn-sm" href="/admin/device">Back</a>`)

	// And nothing in it can be changed: every field is read-only, and none is submitted.
	assert.False(t, strings.Contains(body, `name="f`))
	assert.False(t, strings.Contains(body, "<input class=\"input"))
	assert.Contains(t, body, `role="group" aria-labelledby="p-f0_1-label">10.0.0.1</div>`)
	assert.False(t, strings.Contains(body, "must be a valid"))
}

func TestAFormTheVisitorMayNotSubmitAndCannotLeaveHasNoFooter(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Submit.Guard = func(context.Context, Device) error { return errors.New("requires the editor role") }
	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/solo", Body: f}}} // no parent, no origin
	body := serve(app.MustCompile("/admin"), http.MethodGet, "/admin/solo").Body.String()
	assert.False(t, strings.Contains(body, "panel-foot"))
	assert.False(t, strings.Contains(body, ">Save</button>"))
}

func TestAReadOnlyFieldReadsAsALabelAndItsValueAndAnEmptyOneShowsADash(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Load = func(context.Context) (Device, webui.Loaded) { return Device{Ip: "10.0.0.1"}, webui.Ready() } // an empty ID
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()

	// The ID has no Store: its field is the quiet kind, and says it is empty.
	assert.Contains(t, body, `<div class="field field-static">`)
	assert.Contains(t, body, `role="group" aria-labelledby="p-f0_0-label"><span class="dim" aria-hidden="true">—</span> <span class="sr-only">empty</span></div>`)
	// The IP, which can be edited, is an input in an ordinary field.
	assert.Contains(t, body, `name="f0_1"`)
	assert.Equal(t, strings.Count(body, `field-static`), 1)
}

func TestAPlaceholderIsShownInAnEmptyInputOfAnyKind(t *testing.T) {
	t.Parallel()

	f := webui.Form[Device]{
		Load: func(context.Context) (Device, webui.Loaded) { return Device{}, webui.Ready() },
		Fields: []webui.Accessor[Device]{
			webui.String[Device]{Label: "IP", Load: func(d Device) string { return d.Ip }, Store: func(d *Device, v string) { d.Ip = v }, Placeholder: "10.0.0.1"},
			webui.Int[Device]{Label: "Count", Load: func(d Device) int { return d.Count }, Store: func(d *Device, v int) { d.Count = v }, Placeholder: "how many"},
			webui.Float[Device]{Label: "Rate", Load: func(Device) float64 { return 0 }, Store: func(*Device, float64) {}, Placeholder: "per second"},
		},
		Submit: webui.Action[Device]{Run: func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }},
	}
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()
	for _, p := range []string{"10.0.0.1", "how many", "per second"} {
		assert.Contains(t, body, `placeholder="`+p+`"`)
	}
}

func TestFormWithoutSubmitIsReadOnly(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Submit = webui.Action[Device]{}
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()
	assert.False(t, strings.Contains(body, `type="submit"`))
	assert.False(t, strings.Contains(body, "panel-foot"))
	assert.False(t, strings.Contains(body, `name="f`)) // nothing to submit, so nothing to edit
}

func TestFormLoadFailureFailsThePanel(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Load = func(context.Context) (Device, webui.Loaded) {
		return Device{}, webui.Failed(errors.New("db down: hunter2"))
	}
	rec := serve(formApp(f), http.MethodGet, "/admin/device/dev1")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2"))
}

var (
	formStatus = webui.Select[Device]{
		Label: "Status",
		Load: func(d Device) []string {
			return []string{map[bool]string{true: "healthy", false: "degraded"}[d.Count > 5]}
		},
		Options: []webui.Option{{Value: "healthy", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}},
	}
	formLoad = webui.Slider[Device]{
		Label: "Load", Min: 0, Max: 100, StepSize: 0.1,
		Load:  func(d Device) float64 { return float64(d.Count) * 10 },
		Store: func(d *Device, v float64) { d.Count = int(v / 10) },
	}
	formCap = webui.Slider[Device]{Label: "Cap", Max: 8, Load: func(d Device) float64 { return float64(d.Count) }}
)

func sliderForm() webui.Form[Device] {
	f := okForm()
	f.Fields = []webui.Accessor[Device]{formStatus, formLoad, formCap}
	return f
}

func TestFormShowsBadgesAndSlidersByWhetherTheyCanBeChanged(t *testing.T) {
	t.Parallel()

	body := serve(formApp(sliderForm()), http.MethodGet, "/admin/device/dev1").Body.String()

	// A badge is shown, never typed into: its word, its tone, and no input.
	assert.Contains(t, body, `<span class="badge badge-ok"><span class="dot" aria-hidden="true"></span>healthy</span>`)
	assert.False(t, strings.Contains(body, `name="f0"`))

	// A slider with a Store is a number input over its range, submitting its value, with a range beside it to drag.
	assert.Contains(t, body, `name="f1" type="number" value="70.0" min="0" max="100" step="0.1"`)
	assert.Contains(t, body, `<input class="range-input" type="range" value="70.0" min="0" max="100" step="0.1" tabindex="-1" aria-hidden="true">`)

	// One without is a bar, and submits nothing.
	assert.Contains(t, body, `class="gauge-fill" style="width:88%;"`)
	assert.False(t, strings.Contains(body, `name="f2"`))
}

// A range input with no step submits whatever the pointer was over, 81.6719540426991, and a slider showed that as it was. A
// writable slider has a step, which is the distance between the values it holds, and shows a value with the decimals of it.
func TestAWritableSliderMovesInStepsAndShowsNoMoreDecimalsThanThat(t *testing.T) {
	t.Parallel()

	ratio := 0.6719540426991
	for name, tc := range map[string]struct {
		slider      webui.Slider[Device]
		value, step string
	}{
		"a hundred":      {webui.Slider[Device]{Label: "Load", Max: 100}, "67", "1"},
		"one":            {webui.Slider[Device]{Label: "Load", Max: 1}, "0.67", "0.01"},
		"a thousand":     {webui.Slider[Device]{Label: "Load", Max: 1000}, "670", "10"},
		"whole numbers":  {webui.Slider[Device]{Label: "Load", Max: 100, StepSize: 1}, "67", "1"},
		"tenths":         {webui.Slider[Device]{Label: "Load", Max: 100, StepSize: 0.1}, "67.2", "0.1"},
		"quarters":       {webui.Slider[Device]{Label: "Load", Max: 100, StepSize: 0.25}, "67.25", "0.25"},
		"every fifth":    {webui.Slider[Device]{Label: "Load", Max: 100, StepSize: 5}, "65", "5"},
		"from a minimum": {webui.Slider[Device]{Label: "Load", Min: 2, Max: 102, StepSize: 5}, "67", "5"},
	} {
		s := tc.slider
		high := s.Max
		s.Load = func(Device) float64 { return ratio * high }
		s.Store = func(*Device, float64) {}
		f := okForm()
		f.Fields = []webui.Accessor[Device]{s}
		body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()
		lowest := strconv.FormatFloat(s.Min, 'f', -1, 64)
		highest := strconv.FormatFloat(s.Max, 'f', -1, 64)
		// The number is the control, with the rules and the step; the range beside it is for script, and is not submitted.
		assert.Contains(t, body, `name="f0" type="number" value="`+tc.value+`" min="`+lowest+`" max="`+highest+`" step="`+tc.step+`"`)
		assert.Contains(t, body, `<input class="range-input" type="range" value="`+tc.value+`" min="`+lowest+`" max="`+highest+`" step="`+tc.step+`" tabindex="-1" aria-hidden="true">`)
		assert.Equal(t, strings.Count(body, `name="f0"`), 1)
		if name == "" {
			t.Fatal("unnamed")
		}
	}
}

// A value that is not a step from the minimum is refused, in the words that the browser says it in.
func TestAWritableSliderHoldsOnlyTheValuesAStepApart(t *testing.T) {
	t.Parallel()

	s := webui.Slider[Device]{Label: "Load", Min: 2, Max: 102, StepSize: 5, Load: func(Device) float64 { return 7 }, Store: func(*Device, float64) {}}
	f := okForm()
	f.Fields = []webui.Accessor[Device]{s}
	h := formApp(f)

	body := serve(h, http.MethodGet, "/admin/device/dev1").Body.String()
	assert.Contains(t, body, `data-msg-step-mismatch="Must be 2 plus a multiple of 5."`)
	assert.Contains(t, body, `<span class="tip-item">In steps of 5</span>`)

	for value, want := range map[string]int{"7": http.StatusSeeOther, "102": http.StatusSeeOther, "2": http.StatusSeeOther, "8": http.StatusUnprocessableEntity, "7.5": http.StatusUnprocessableEntity, "0": http.StatusUnprocessableEntity} {
		rec := post(h, "/admin/device/dev1", formValues("f0", value))
		assert.Equal(t, rec.Code, want)
		if want != http.StatusSeeOther {
			assert.Contains(t, rec.Body.String(), "Must be 2 plus a multiple of 5.")
		}
	}

	// Whole numbers say so, and not "a multiple of 1".
	whole := webui.Slider[Device]{Label: "Load", Max: 100, StepSize: 1, Load: func(Device) float64 { return 0 }, Store: func(*Device, float64) {}}
	f.Fields = []webui.Accessor[Device]{whole}
	assert.Contains(t, post(formApp(f), "/admin/device/dev1", formValues("f0", "4.5")).Body.String(), "Must be a whole number.")

	// Tenths are three steps of a tenth, whatever floating point says.
	tenths := webui.Slider[Device]{Label: "Load", Max: 1, StepSize: 0.1, Load: func(Device) float64 { return 0 }, Store: func(*Device, float64) {}}
	f.Fields = []webui.Accessor[Device]{tenths}
	h = formApp(f)
	assert.Equal(t, post(h, "/admin/device/dev1", formValues("f0", "0.3")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/device/dev1", formValues("f0", "0.7")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/device/dev1", formValues("f0", "0.35")).Code, http.StatusUnprocessableEntity)
}

func TestWritableSliderIsHeldToItsRangeOnTheServerAndEchoesInput(t *testing.T) {
	t.Parallel()

	f := sliderForm()
	f.Submit = webui.Action[Device]{Run: func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }}
	h := formApp(f)

	rec := post(h, "/admin/device/dev1", formValues("f1", "250"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	page := rec
	assert.Contains(t, page.Body.String(), "Must be at most 100.")
	assert.Contains(t, page.Body.String(), `value="250"`)
	assert.Contains(t, page.Body.String(), `class="range-input invalid"`)

	assert.Equal(t, post(h, "/admin/device/dev1", formValues("f1", "40")).Code, http.StatusSeeOther)
}

// A form's fields are inset from the panel's edge: its markup says so, and the
// stylesheet has the rule, so the two cannot drift apart unnoticed.
func TestAFormsFieldsAreInsetFromThePanelEdge(t *testing.T) {
	t.Parallel()

	h := formApp(okForm())
	assert.Contains(t, serve(h, http.MethodGet, "/admin/device/dev1").Body.String(), `<div class="panel-body pad">`)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".panel-body.pad {\n  padding: 4px 10px 14px;")
}

// With script a slider's number is a narrow box beside the range; without it there is no range, and the number is the whole
// control and fills the field like any input. The stylesheet says so for a page that script has not marked.
func TestASlidersNumberFillsTheFieldWhenThereIsNoScript(t *testing.T) {
	t.Parallel()

	h := formApp(sliderForm())
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, "html:not(.js) .range-number {\n  flex: 1;\n  width: auto;\n  text-align: left;\n}")
	assert.Contains(t, css, ".range-number {\n  flex: none;\n  width: 92px;")
}
