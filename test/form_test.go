package webui_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

var (
	formID = webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}
	formIP = webui.String[Device]{
		Label: "IP",
		Load:  func(d Device) string { return d.Ip },
		Store: func(d *Device, v string) { d.Ip = v },
		Rules: webui.StringRules{
			Required: true, MinLen: 7, MaxLen: 15,
			Pattern: &webui.PatternRule{Expr: `\d{1,3}(\.\d{1,3}){3}`, Message: "must be a valid IPv4 address"},
		},
	}
	formCount = webui.Int[Device]{
		Label: "Occurrences", Load: func(d Device) int { return d.Count }, Store: func(d *Device, v int) { d.Count = v },
		Rules: webui.NumberRules[int]{Min: ptr(0), Max: ptr(100)},
	}
)

func ptr[T any](v T) *T { return &v }

func formApp(form webui.Form[Device]) http.Handler {
	list := webui.Page[webui.NoArgs]{Path: "/device", Nav: webui.Nav{Label: "Devices"}, Body: webui.Stack{}}
	details := webui.Page[detailsArgs]{Path: "/device/{id}", Body: form}
	return webui.App{Pages: webui.Pages{list, details}}.MustCompile("/admin")
}

func okForm() webui.Form[Device] {
	return webui.Form[Device]{
		Title: "Configuration",
		Desc:  "One device.",
		Load: func(ctx context.Context) (Device, error) {
			a := webui.ArgsOf[detailsArgs](ctx)
			return Device{Id: a.Id, Ip: "10.0.0.1", Count: 7}, nil
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

	// ID has no Store: visibly read-only, and never submitted (no name).
	assert.Contains(t, body, `value="dev1" readonly>`)
	assert.False(t, strings.Contains(body, `name="f0_0"`))

	// IP carries its rules as HTML constraint attributes, and states them.
	assert.Contains(t, body, `name="f0_1" type="text" value="10.0.0.1"`)
	assert.Contains(t, body, `required minlength="7" maxlength="15" pattern="\d{1,3}(\.\d{1,3}){3}"`)
	assert.Contains(t, body, `<span class="req" aria-hidden="true">*</span>`)
	assert.Contains(t, body, "must be a valid IPv4 address")

	// A number is a number input with its bounds; zero is a real bound.
	assert.Contains(t, body, `name="f1" type="number" value="7" min="0" max="100" step="1"`)
	assert.Contains(t, body, ">0–100</div>")

	// A group is layout: its fields side by side, with no frame of its own.
	assert.Contains(t, body, `class="field-row"`)
	assert.False(t, strings.Contains(body, `class="group"`))
	assert.False(t, strings.Contains(body, "group-legend"))

	// Submit is the primary button; Cancel goes to the parent in the breadcrumb.
	assert.Contains(t, body, `class="btn btn-primary " type="submit">Save</button>`)
	assert.Contains(t, body, `class="btn btn-ghost " href="/admin/device">Cancel</a>`)
}

func TestFormSubmitIsGatedByTheSameGuardThatAuthorisesIt(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Submit.Guard = func(context.Context, Device) error { return errors.New("requires the editor role") }
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()

	assert.Contains(t, body, `<span class="gate" data-guard="requires the editor role">`)
	assert.Contains(t, body, `class="btn btn-primary " type="submit" disabled>Save</button>`)
}

func TestFormWithoutSubmitIsReadOnly(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Submit = webui.Action[Device]{}
	body := serve(formApp(f), http.MethodGet, "/admin/device/dev1").Body.String()
	assert.False(t, strings.Contains(body, `type="submit"`))
	assert.False(t, strings.Contains(body, "panel-foot"))
}

func TestFormLoadFailureFailsThePanel(t *testing.T) {
	t.Parallel()

	f := okForm()
	f.Load = func(context.Context) (Device, error) { return Device{}, errors.New("db down: hunter2") }
	rec := serve(formApp(f), http.MethodGet, "/admin/device/dev1")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "hunter2"))
}

var (
	formStatus = webui.Badge[Device]{
		Label: "Status",
		Load:  func(d Device) string { return map[bool]string{true: "healthy", false: "degraded"}[d.Count > 5] },
		Kinds: map[string]webui.Tone{"healthy": webui.ToneOK, "degraded": webui.ToneWarning},
	}
	formLoad = webui.Slider[Device]{
		Label: "Load", Min: 0, Max: 100, Precision: 1,
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

	// A slider with a Store is a range input over its range, submitting its value.
	assert.Contains(t, body, `type="range" name="f1" value="70.0" min="0" max="100" step="any"`)
	assert.Contains(t, body, `<output class="range-value" for="p-f1" data-range-value>70.0</output>`)

	// One without is a bar, and submits nothing.
	assert.Contains(t, body, `class="gauge-fill" style="width:88%;"`)
	assert.False(t, strings.Contains(body, `name="f2"`))
}

func TestWritableSliderIsHeldToItsRangeOnTheServerAndEchoesInput(t *testing.T) {
	t.Parallel()

	f := sliderForm()
	f.Submit = webui.Action[Device]{Run: func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }}
	h := formApp(f)

	rec := post(h, "/admin/device/dev1", formValues("f1", "250"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Must be at most 100.")
	assert.Contains(t, rec.Body.String(), `value="250"`)
	assert.Contains(t, rec.Body.String(), `class="range-input invalid"`)

	assert.Equal(t, post(h, "/admin/device/dev1", formValues("f1", "40")).Code, http.StatusSeeOther)
}

// A form's fields are inset from the panel's edge: its markup says so, and the
// stylesheet has the rule, so the two cannot drift apart unnoticed.
func TestAFormsFieldsAreInsetFromThePanelEdge(t *testing.T) {
	t.Parallel()

	h := formApp(okForm())
	assert.Contains(t, serve(h, http.MethodGet, "/admin/device/dev1").Body.String(), `<div class="panel-body pad">`)
	css := serve(h, http.MethodGet, "/admin/_webui/app.css").Body.String()
	assert.Contains(t, css, ".panel-body.pad {\n  padding: 4px 12px 14px;")
}
