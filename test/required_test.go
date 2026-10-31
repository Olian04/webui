package webui_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// wants is a model with a field of each kind that an input can hold: the ones that cannot be nil, and the ones that can.
type wants struct {
	Text    string
	Num     int
	Ratio   float64
	Rate    *int
	Level   *float64
	Paused  *bool
	Checked bool
	Pick    []string
}

// wantsApp is a form of one field of wants, and what it saved.
func wantsApp(start wants, field webui.Accessor[wants]) (http.Handler, func() wants) {
	var (
		mu    sync.Mutex
		saved = start
	)
	form := webui.Form[wants]{
		Load: func(context.Context) (wants, webui.Loaded) {
			mu.Lock()
			defer mu.Unlock()
			return saved, webui.Ready()
		},
		Fields: []webui.Accessor[wants]{field},
		Submit: webui.Action[wants]{Run: func(_ context.Context, w wants) (webui.Outcome, error) {
			mu.Lock()
			saved = w
			mu.Unlock()
			return webui.Success("Saved"), nil
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/w", Body: form}}}.MustCompile("/admin")
	return h, func() wants {
		mu.Lock()
		defer mu.Unlock()
		return saved
	}
}

// Required is written on the accessor, once, whatever the accessor holds: an empty submission is refused, and what was stored is left alone.
func TestRequiredOnAnAccessorRefusesAnEmptySubmission(t *testing.T) {
	t.Parallel()

	start := wants{Text: "x", Num: 5, Ratio: 0.5, Rate: new(3), Level: new(2.0), Paused: new(true), Pick: []string{"a"}}
	for _, field := range map[string]webui.Accessor[wants]{
		"String":      webui.String[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		"Email":       webui.Email[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		"Phone":       webui.Phone[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		"Website":     webui.Website[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		"Int":         webui.Int[wants]{Label: "F", Required: true, Load: func(w wants) int { return w.Num }, Store: func(w *wants, v int) { w.Num = v }},
		"Float":       webui.Float[wants]{Label: "F", Required: true, Load: func(w wants) float64 { return w.Ratio }, Store: func(w *wants, v float64) { w.Ratio = v }},
		"OptionalInt": webui.OptionalInt[wants]{Label: "F", Required: true, Load: func(w wants) *int { return w.Rate }, Store: func(w *wants, v *int) { w.Rate = v }},
		"OptionalFloat": webui.OptionalFloat[wants]{Label: "F", Required: true, Load: func(w wants) *float64 { return w.Level },
			Store: func(w *wants, v *float64) { w.Level = v }},
		"OptionalSlider": webui.OptionalSlider[wants]{Label: "F", Min: 0, Max: 10, StepSize: 1, Required: true, Load: func(w wants) *float64 { return w.Level },
			Store: func(w *wants, v *float64) { w.Level = v }},
		"OptionalBool": webui.OptionalBool[wants]{Label: "F", Required: true, Load: func(w wants) *bool { return w.Paused }, Store: func(w *wants, v *bool) { w.Paused = v }},
		"Select": webui.Select[wants]{Label: "F", Required: true, Options: []webui.Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}},
			Load: func(w wants) []string { return w.Pick }, Store: func(w *wants, v []string) { w.Pick = v }},
	} {
		h, saved := wantsApp(start, field)
		rec := post(h, "/admin/w", formValues("f0", ""))
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.Contains(t, rec.Body.String(), "F")
		assert.DeepEqual(t, saved(), start)
	}
}

// An accessor fills its field in a form with its default: the value that Load gives, for a type that cannot be nil, so that the visitor sees what is stored;
// and nothing for one that can, which is "not set".
func TestAnAccessorFillsItsFieldWithItsDefault(t *testing.T) {
	t.Parallel()

	zero := wants{}
	body := func(field webui.Accessor[wants]) string {
		h, _ := wantsApp(zero, field)
		return serve(h, http.MethodGet, "/admin/w").Body.String()
	}
	// Not nil: the zero value is what is shown, and is what a visitor can see they are about to store.
	assert.Contains(t, body(webui.Int[wants]{Label: "F", Load: func(w wants) int { return w.Num }, Store: func(w *wants, v int) { w.Num = v }}), `type="number" value="0"`)
	assert.Contains(t, body(webui.Float[wants]{Label: "F", Load: func(w wants) float64 { return w.Ratio }, Store: func(w *wants, v float64) { w.Ratio = v }}), `type="number" value="0"`)
	// Nil: nothing is shown, which is not zero.
	assert.Contains(t, body(webui.OptionalInt[wants]{Label: "F", Load: func(w wants) *int { return w.Rate }, Store: func(w *wants, v *int) { w.Rate = v }}), `type="number" value=""`)
	assert.Contains(t, body(webui.OptionalFloat[wants]{Label: "F", Load: func(w wants) *float64 { return w.Level }, Store: func(w *wants, v *float64) { w.Level = v }}), `type="number" value=""`)
}

// Without Required, an input of a type that cannot be nil that is emptied stores the zero value, which is what the form showed it as; Required refuses it.
// A number that may be absent is the Optional one.
func TestAnEmptiedNumberThatIsNotRequiredStoresZero(t *testing.T) {
	t.Parallel()

	num := webui.Int[wants]{Label: "F", Load: func(w wants) int { return w.Num }, Store: func(w *wants, v int) { w.Num = v }}
	h, saved := wantsApp(wants{Num: 5}, num)
	rec := post(h, "/admin/w", formValues("f0", ""))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, saved().Num, 0)

	num.Required = true
	h, saved = wantsApp(wants{Num: 5}, num)
	rec = post(h, "/admin/w", formValues("f0", ""))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Equal(t, saved().Num, 5)
}

// The rules of a text are about its shape, and an empty one is not checked against them: whether empty is refused is Required.
func TestTheRulesOfATextDoNotRefuseAnEmptyValueAndRequiredDoes(t *testing.T) {
	t.Parallel()

	text := webui.String[wants]{Label: "F", Rules: webui.StringRules{MinLen: 3}, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }}
	h, _ := wantsApp(wants{Text: "abc"}, text)
	assert.Equal(t, post(h, "/admin/w", formValues("f0", "")).Code, http.StatusSeeOther)
	assert.Equal(t, post(h, "/admin/w", formValues("f0", "ab")).Code, http.StatusUnprocessableEntity)

	text.Required = true
	h, _ = wantsApp(wants{Text: "abc"}, text)
	rec := post(h, "/admin/w", formValues("f0", ""))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.True(t, strings.Contains(rec.Body.String(), "This field is required."))
}

// A read-only accessor has no submission to refuse, so Required on one is a mistake that Compile names.
func TestRequiredOnAnOptionalSliderThatIsReadOnlyIsRefused(t *testing.T) {
	t.Parallel()

	slider := webui.OptionalSlider[wants]{Label: "F", Min: 0, Max: 10, Required: true, Load: func(w wants) *float64 { return w.Level }}
	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/w", Body: webui.Form[wants]{
		Load:   func(context.Context) (wants, webui.Loaded) { return wants{}, webui.Ready() },
		Fields: []webui.Accessor[wants]{slider},
		Submit: webui.Action[wants]{Run: func(context.Context, wants) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	}}}}
	_, err := app.Compile("/admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is Required but has no Store")
}
