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
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/w", View: form}}}.MustCompile("/admin")
	return h, func() wants {
		mu.Lock()
		defer mu.Unlock()
		return saved
	}
}

// Required is written on the accessor, where its type has an empty value of its own, and refuses an empty submission: what was stored is left alone.
func TestRequiredOnAnAccessorRefusesAnEmptySubmission(t *testing.T) {
	t.Parallel()

	start := wants{Text: "x", Pick: []string{"a"}}
	for _, field := range []webui.Accessor[wants]{
		webui.String[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		webui.Email[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		webui.Phone[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		webui.Website[wants]{Label: "F", Required: true, Load: func(w wants) string { return w.Text }, Store: func(w *wants, v string) { w.Text = v }},
		webui.Select[wants]{Label: "F", Required: true, Options: []webui.Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}},
			Load: func(w wants) []string { return w.Pick }, Store: func(w *wants, v []string) { w.Pick = v }},
	} {
		h, saved := wantsApp(start, field)
		rec := post(h, "/admin/w", formValues("f0", ""))
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.Contains(t, rec.Body.String(), "F")
		assert.DeepEqual(t, saved(), start)
	}
}

// A number always has a value, so an empty input is refused without a flag to say so: an Int and a Float are required where they are edited. An
// optional one may be absent, and is stored as nil when it is left empty, whatever its rules. A number that is read-only has no input to refuse.
func TestANumberIsRequiredByItsTypeAndAnOptionalOneIsNot(t *testing.T) {
	t.Parallel()

	start := wants{Num: 5, Ratio: 0.5, Rate: new(3), Level: new(2.0)}
	for _, field := range []webui.Accessor[wants]{
		webui.Int[wants]{Label: "F", Load: func(w wants) int { return w.Num }, Store: func(w *wants, v int) { w.Num = v }},
		webui.Float[wants]{Label: "F", Load: func(w wants) float64 { return w.Ratio }, Store: func(w *wants, v float64) { w.Ratio = v }},
	} {
		h, saved := wantsApp(start, field)
		body := serve(h, http.MethodGet, "/admin/w").Body.String()
		assert.Contains(t, body, `required`)
		assert.Contains(t, body, `class="req"`)
		rec := post(h, "/admin/w", formValues("f0", ""))
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.Contains(t, rec.Body.String(), "This field is required.")
		assert.DeepEqual(t, saved(), start)
	}
	for _, c := range []struct {
		field  webui.Accessor[wants]
		absent func(wants) bool
	}{
		{webui.OptionalInt[wants]{Label: "F", Load: func(w wants) *int { return w.Rate }, Store: func(w *wants, v *int) { w.Rate = v }}, func(w wants) bool { return w.Rate == nil }},
		{webui.OptionalFloat[wants]{Label: "F", Load: func(w wants) *float64 { return w.Level }, Store: func(w *wants, v *float64) { w.Level = v }}, func(w wants) bool { return w.Level == nil }},
		{webui.OptionalSlider[wants]{Label: "F", Min: 0, Max: 10, StepSize: 1, Load: func(w wants) *float64 { return w.Level }, Store: func(w *wants, v *float64) { w.Level = v }}, func(w wants) bool { return w.Level == nil }},
	} {
		h, saved := wantsApp(start, c.field)
		assert.False(t, strings.Contains(serve(h, http.MethodGet, "/admin/w").Body.String(), `class="req"`))
		assert.Equal(t, post(h, "/admin/w", formValues("f0", "")).Code, http.StatusSeeOther)
		assert.True(t, c.absent(saved()))
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
