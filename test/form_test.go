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
	"github.com/Olian04/webui/test/util/page"
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
	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, View: webui.Stack{}}
	details := webui.Page[detailsArgs]{Path: "/device/{id}", View: form}
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

	p := pageOf(t, formApp(okForm()), "/admin/device/dev1")
	main := page.One(t, "main", p.ByRole("main", ""))

	assert.Equal(t, len(p.ByRole("heading", "Configuration")), 1)
	post := page.One(t, "the form", p.ByAttr("action", "/admin/device/dev1"))
	assert.Equal(t, post.Attr("method"), "post")
	view, _ := p.Field("_view")
	assert.Equal(t, view.Value, "p")

	// ID has no Store: shown as output, not as an input that is refused, and never submitted (no name).
	assert.Equal(t, page.One(t, "the ID", main.ByRole("group", "ID")).Text(), "dev1")
	assert.Equal(t, len(p.ByAttr("value", "dev1")), 0)
	_, isInput := p.Field("f0_0")
	assert.False(t, isInput)

	// IP carries its rules as HTML constraint attributes, and states them.
	ip, _ := p.Field("f0_1")
	assert.Equal(t, ip.Type, "text")
	assert.Equal(t, ip.Value, "10.0.0.1")
	assert.True(t, ip.Required)
	ipBox := page.One(t, "the IP input", p.ByAttr("name", "f0_1"))
	assert.Equal(t, ipBox.Attr("minlength"), "7")
	assert.Equal(t, ipBox.Attr("maxlength"), "15")
	assert.Equal(t, ipBox.Attr("pattern"), `\d{1,3}(\.\d{1,3}){3}`)
	// The rules, the pattern's message among them, are in the popover beside the label, and not printed beneath the input while its value is good.
	for _, rule := range []string{"7–15 characters", "must be a valid IPv4 address"} {
		assert.True(t, len(p.ByAllText(rule)) > 0)
		assert.False(t, strings.Contains(main.Text(), rule))
	}
	assert.Equal(t, ipBox.Attr("title"), "must be a valid IPv4 address") // what a browser says after its own words when there is no script
	assert.False(t, strings.Contains(main.AllText(), "Required"))        // the asterisk says it

	// A number is a number input with its bounds, and required, since it always has a value; zero is a real bound.
	count, _ := p.Field("f1")
	assert.Equal(t, count.Type, "number")
	assert.Equal(t, count.Value, "7")
	assert.True(t, count.Required)
	box := page.One(t, "the number input", p.ByAttr("name", "f1"))
	assert.DeepEqual(t, []string{box.Attr("min"), box.Attr("max"), box.Attr("step")}, []string{"0", "100", "1"})
	assert.True(t, len(p.ByAllText("0–100")) > 0) // its bounds, in the popover

	// A group is layout: the fields of the group are all there.
	assert.Equal(t, len(main.ByRole("group", "ID")), 1)
	assert.Equal(t, len(p.ByLabel("IP")), 1)

	// Submit is the button; Cancel goes to the parent in the breadcrumb.
	assert.Equal(t, len(main.ByRole("button", "Save")), 1)
	assert.True(t, hasLink(p, "Cancel", "/admin/device"))
}

// A field tells the browser what to say when it refuses a value, in the words the server uses beneath the field, by the
// names of the properties of the control's ValidityState; and what a matching value looks like, as its title, which a
// browser says after its own words when there is no script. A field that is only read says nothing.
func TestAFieldTellsTheBrowserWhatToSayWhenItRefusesAValue(t *testing.T) {
	t.Parallel()

	p := pageOf(t, formApp(okForm()), "/admin/device/dev1")
	ip := page.One(t, "the IP input", p.ByAttr("name", "f0_1"))
	count := page.One(t, "the number input", p.ByAttr("name", "f1"))

	// One attribute for each message, named for the ValidityState property in kebab case.
	for _, want := range []struct {
		control page.El
		attr    string
		message string
	}{
		{ip, "data-msg-pattern-mismatch", "must be a valid IPv4 address"}, {ip, "data-msg-too-long", "Must be at most 15 characters."},
		{ip, "data-msg-too-short", "Must be at least 7 characters."}, {ip, "data-msg-value-missing", "This field is required."},
		{count, "data-msg-bad-input", "Must be a whole number."}, {count, "data-msg-range-overflow", "Must be at most 100."},
		{count, "data-msg-range-underflow", "Must be at least 0."}, {count, "data-msg-step-mismatch", "Must be a whole number."},
		{count, "data-msg-value-missing", "This field is required."},
	} {
		assert.Equal(t, want.control.Attr(want.attr), want.message)
	}
	assert.False(t, ip.Has("data-msg-range-overflow")) // a message is only for a rule the control has

	// The ID has no Store: it is not a control, and has nothing to say.
	assert.Equal(t, len(p.ByAttr("data-msg-value-missing", "")), 2) // the required text, and the number, which is required by its type

	// A form that cannot be submitted has no controls to refuse anything.
	f := okForm()
	f.Submit.Guard = func(context.Context, Device) error { return errors.New("requires the editor role") }
	assert.Equal(t, len(pageOf(t, formApp(f), "/admin/device/dev1").ByAttr("data-msg-value-missing", "")), 0)
}

func TestAFormTheVisitorMayNotSubmitOffersBackInPlaceOfSaveAndCancel(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Submit.Guard = func(context.Context, Device) error { return errors.New("requires the editor role") }
	p := pageOf(t, formApp(f), "/admin/device/dev1")
	main := page.One(t, "main", p.ByRole("main", ""))

	assert.Equal(t, len(main.ByRole("button", "Save")), 0)
	assert.False(t, hasLink(p, "Cancel", "/admin/device"))
	assert.True(t, hasLink(p, "Back", "/admin/device"))

	// And nothing in it can be changed: every field is read-only, and none is submitted.
	for _, f := range p.Fields() {
		assert.Equal(t, f.Type, "hidden")
	}
	assert.Equal(t, page.One(t, "the IP", main.ByRole("group", "IP")).Text(), "10.0.0.1")
	assert.False(t, strings.Contains(main.AllText(), "must be a valid"))
}

func TestAFormTheVisitorMayNotSubmitAndCannotLeaveHasNoFooter(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Load = func(context.Context) (Device, webui.Loaded) { return Device{Id: "dev1", Ip: "10.0.0.1"}, webui.Ready() } // a page of no arguments has none to read
	f.Submit.Guard = func(context.Context, Device) error { return errors.New("requires the editor role") }
	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/solo", View: f}}} // no parent, no origin
	p := pageOf(t, app.MustCompile("/admin"), "/admin/solo")
	main := page.One(t, "main", p.ByRole("main", ""))
	assert.Equal(t, page.One(t, "the IP", main.ByRole("group", "IP")).Text(), "10.0.0.1") // the form is there
	assert.Equal(t, len(main.ByRole("button", "")), 0)
	assert.Equal(t, len(main.Links()), 0)
}

func TestAReadOnlyFieldReadsAsALabelAndItsValueAndAnEmptyOneShowsADash(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Load = func(context.Context) (Device, webui.Loaded) { return Device{Ip: "10.0.0.1"}, webui.Ready() } // an empty ID
	p := pageOf(t, formApp(f), "/admin/device/dev1")

	// The ID has no Store: its field is the quiet kind, and says it is empty.
	assert.Equal(t, page.One(t, "the ID", p.ByRole("group", "ID")).Text(), "empty")
	// The IP, which can be edited, is an input in an ordinary field.
	_, isInput := p.Field("f0_1")
	assert.True(t, isInput)
	assert.Equal(t, len(p.ByRole("group", "IP")), 0)
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
	p := pageOf(t, formApp(f), "/admin/device/dev1")
	for name, want := range map[string]string{"f0": "10.0.0.1", "f1": "how many", "f2": "per second"} {
		assert.Equal(t, page.One(t, name, p.ByAttr("name", name)).Attr("placeholder"), want)
	}
}

func TestFormWithoutSubmitIsReadOnly(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Submit = webui.Action[Device]{}
	p := pageOf(t, formApp(f), "/admin/device/dev1")
	assert.Equal(t, len(p.ByRole("main", "")[0].ByRole("button", "")), 0)
	for _, f := range p.Fields() {
		assert.Equal(t, f.Type, "hidden") // nothing to submit, so nothing to edit
	}
}

func TestFormLoadFailureFailsThePanel(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Load = func(context.Context) (Device, webui.Loaded) {
		return Device{}, webui.Failed(errors.New("db down: hunter2"))
	}
	rec := serve(formApp(f), http.MethodGet, "/admin/device/dev1")
	assert.Equal(t, rec.Code, http.StatusOK)
	failed := page.Parse(t, rec.Body.String())
	assert.True(t, strings.Contains(failed.Root().Text(), "Could not load"))
	assert.False(t, strings.Contains(failed.Root().AllText(), "hunter2"))
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

	p := pageOf(t, formApp(sliderForm()), "/admin/device/dev1")

	// A badge is shown, never typed into: its word, and no input.
	assert.Equal(t, page.One(t, "the status", p.ByRole("group", "Status")).Text(), "healthy")
	_, isInput := p.Field("f0")
	assert.False(t, isInput)

	// A slider with a Store is a number input over its range, submitting its value, with a range beside it to drag.
	load, _ := p.Field("f1")
	assert.Equal(t, load.Type, "number")
	assert.Equal(t, load.Value, "70.0")
	number := page.One(t, "the number", p.ByAttr("name", "f1"))
	assert.DeepEqual(t, []string{number.Attr("min"), number.Attr("max"), number.Attr("step")}, []string{"0", "100", "0.1"})
	drag := page.One(t, "the range", p.ByAttr("type", "range"))
	assert.DeepEqual(t, []string{drag.Attr("value"), drag.Attr("min"), drag.Attr("max"), drag.Attr("step")}, []string{"70.0", "0", "100", "0.1"})
	assert.False(t, drag.Has("name")) // for script to drag, and never submitted

	// One without is a bar, which reads as its value, and submits nothing.
	assert.Equal(t, page.One(t, "the cap", p.ByRole("group", "Cap")).Text(), "7")
	_, isInput = p.Field("f2")
	assert.False(t, isInput)
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
		p := pageOf(t, formApp(f), "/admin/device/dev1")
		lowest := strconv.FormatFloat(s.Min, 'f', -1, 64)
		highest := strconv.FormatFloat(s.Max, 'f', -1, 64)
		// The number is the control, with the rules and the step; the range beside it is for script, and is not submitted.
		number := page.One(t, "the number", p.ByAttr("name", "f0"))
		assert.DeepEqual(t, []string{number.Attr("type"), number.Attr("value"), number.Attr("min"), number.Attr("max"), number.Attr("step")}, []string{"number", tc.value, lowest, highest, tc.step})
		drag := page.One(t, "the range", p.ByAttr("type", "range"))
		assert.DeepEqual(t, []string{drag.Attr("value"), drag.Attr("min"), drag.Attr("max"), drag.Attr("step")}, []string{tc.value, lowest, highest, tc.step})
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

	p := pageOf(t, h, "/admin/device/dev1")
	assert.Equal(t, page.One(t, "the slider", p.ByAttr("name", "f0")).Attr("data-msg-step-mismatch"), "Must be 2 plus a multiple of 5.")
	assert.Equal(t, len(p.ByAllText("In steps of 5")), 1)

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
	refused := page.Parse(t, rec.Body.String())
	assert.True(t, strings.Contains(refused.Root().Text(), "Must be at most 100."))
	typed, _ := refused.Field("f1")
	assert.Equal(t, typed.Value, "250") // what was typed stays

	assert.Equal(t, post(h, "/admin/device/dev1", formValues("f1", "40")).Code, http.StatusSeeOther)
}
