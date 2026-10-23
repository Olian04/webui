package webui_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type task struct {
	Name   string
	Status []string
	Labels []string
}

var (
	taskName   = webui.String[task]{Label: "Name", Load: func(t task) string { return t.Name }}
	taskStatus = webui.Select[task]{
		Label:    "Status",
		Load:     func(t task) []string { return t.Status },
		Store:    func(t *task, v []string) { t.Status = v },
		Required: true,
		Options: []webui.Option{
			{Value: "open", Label: "Open", Tone: webui.ToneWarning}, {Value: "done", Label: "Done", Tone: webui.ToneOK}, {Value: "later"},
		},
	}
	taskLabels = webui.Select[task]{
		Label:   "Labels",
		Load:    func(t task) []string { return t.Labels },
		Store:   func(t *task, v []string) { t.Labels = v },
		Multi:   true,
		Options: []webui.Option{{Value: "bug"}, {Value: "ui", Label: "UI"}, {Value: "docs"}},
	}
)

// taskApp is a form of one task, whose saved value it records.
func taskApp(start task, saved *task) http.Handler {
	form := webui.Form[task]{
		Load:   func(context.Context) (task, webui.Loaded) { return start, webui.Ready() },
		Fields: []webui.Accessor[task]{taskName, taskStatus, taskLabels},
		Submit: webui.Action[task]{Run: func(_ context.Context, t task) (webui.Outcome, error) {
			*saved = t
			return webui.Success("Saved"), nil
		}},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: form}}}.MustCompile("/admin")
}

func submitTask(h http.Handler, values url.Values) (code int, body string) {
	values.Set("_leaf", "p")
	rec := post(h, "/admin/t", values) // a refused one is the form again, with what was typed
	return rec.Code, rec.Body.String()
}

func TestASelectThatIsEditedIsAListToChooseFrom(t *testing.T) {
	t.Parallel()

	var saved task
	body := serve(taskApp(task{Name: "a", Status: []string{"done"}, Labels: []string{"bug", "docs"}}, &saved), http.MethodGet, "/admin/t").Body.String()

	// One value: a drop-down list, with the choice that is held selected. It is required, so the
	// empty option asks for a choice.
	assert.Contains(t, body, `<select data-select id="p-f1" class="input" name="f1" required data-msg-value-missing="This field is required.">`)
	assert.Contains(t, body, `<option value="">Choose…</option>`)
	assert.Contains(t, body, `<option value="done" data-badge="badge-ok" selected>Done</option>`)
	assert.Contains(t, body, `<option value="open" data-badge="badge-warn">Open</option>`) // an option says its label
	assert.False(t, strings.Contains(body, `<option value="open" selected>`))

	// Any number: a list, with each that is held selected, and a hint on how to choose several.
	assert.Contains(t, body, `<select data-select id="p-f2" class="input" name="f2" multiple size="3">`)
	assert.Contains(t, body, `<option value="bug" data-badge="badge-mute" selected>bug</option>`)
	assert.Contains(t, body, `<option value="ui" data-badge="badge-mute">UI</option>`)
	assert.Contains(t, body, `<option value="docs" data-badge="badge-mute" selected>docs</option>`)
	assert.Contains(t, body, "Hold Ctrl or ⌘ to choose more than one")
	assert.False(t, strings.Contains(body, `<option value="">`+"—")) // a list of several has no empty option
}

func TestASelectThatIsNotRequiredCanBeLeftEmpty(t *testing.T) {
	t.Parallel()

	optional := taskStatus
	optional.Required = false
	page := webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Form[task]{
		Load:   func(context.Context) (task, webui.Loaded) { return task{Name: "a"}, webui.Ready() },
		Fields: []webui.Accessor[task]{optional},
		Submit: webui.Action[task]{Run: func(context.Context, task) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	}}
	body := serve(webui.App{Pages: webui.Pages{page}}.MustCompile("/admin"), http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `<option value="">—</option>`)
	assert.False(t, strings.Contains(body, " required"))
}

func TestWhatWasChosenIsStoredAsAListOfTheValues(t *testing.T) {
	t.Parallel()

	var saved task
	h := taskApp(task{Name: "a", Status: []string{"open"}}, &saved)

	code, _ := submitTask(h, url.Values{"f1": {"done"}, "f2": {"docs", "bug"}})
	assert.Equal(t, code, http.StatusSeeOther)
	assert.DeepEqual(t, saved.Status, []string{"done"})
	assert.DeepEqual(t, saved.Labels, []string{"docs", "bug"}) // each sent, in the order sent

	// Nothing chosen from a list of several is none, and the model is told so.
	saved = task{Labels: []string{"x"}}
	code, _ = submitTask(h, url.Values{"f1": {"open"}})
	assert.Equal(t, code, http.StatusSeeOther)
	assert.Equal(t, len(saved.Labels), 0)
}

func TestARequiredSelectWithNothingChosenIsRefused(t *testing.T) {
	t.Parallel()

	var saved task
	code, body := submitTask(taskApp(task{Name: "a"}, &saved), url.Values{"f1": {""}})
	assert.Equal(t, code, http.StatusUnprocessableEntity) // the form again, not a redirect on
	assert.Contains(t, body, "This field is required.")
	assert.Equal(t, len(saved.Status), 0)
}

func TestASelectRefusesWhatItDidNotOffer(t *testing.T) {
	t.Parallel()

	var saved task
	h := taskApp(task{Name: "a", Status: []string{"open"}}, &saved)
	for name, values := range map[string]url.Values{
		"a value that is not an option":   {"f1": {"deleted"}},
		"a value of the other select":     {"f1": {"bug"}},
		"two for a select of one":         {"f1": {"open", "done"}},
		"a value that is not an option 2": {"f1": {"open"}, "f2": {"bug", "nope"}},
		"the same option twice":           {"f1": {"open"}, "f2": {"bug", "bug"}},
		"a list separator in a value":     {"f1": {"open\x1fdone"}},
	} {
		code, _ := submitTask(h, values)
		if code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422, the form again", name, code)
		}
	}
	assert.Equal(t, len(saved.Status), 0) // none of them was saved

	_, body := submitTask(h, url.Values{"f1": {"open", "done"}})
	assert.Contains(t, body, "Choose only one.")
	_, body = submitTask(h, url.Values{"f1": {"deleted"}})
	assert.Contains(t, body, "Choose from the options offered.")
}

func TestARejectedSubmissionKeepsWhatWasChosen(t *testing.T) {
	t.Parallel()

	var saved task
	// Status is missing, so it is refused; what was chosen in the labels is shown again.
	_, body := submitTask(taskApp(task{Name: "a", Labels: []string{"docs"}}, &saved), url.Values{"f1": {""}, "f2": {"bug", "ui"}})
	assert.Contains(t, body, `<option value="bug" data-badge="badge-mute" selected>bug</option>`)
	assert.Contains(t, body, `<option value="ui" data-badge="badge-mute" selected>UI</option>`)
	assert.False(t, strings.Contains(body, `<option value="docs" selected>`))
	assert.Contains(t, body, `aria-invalid="true"`)
}

func TestASelectIsMarkedForTheScriptThatTurnsItIntoADropDown(t *testing.T) {
	t.Parallel()

	var saved task
	body := serve(taskApp(task{Name: "a", Status: []string{"done"}, Labels: []string{"bug"}}, &saved), http.MethodGet, "/admin/t").Body.String()

	// The script replaces the native control with a drop-down that shows the choices as badges,
	// so it needs the select marked, each option's tone, and the label's id to name the popup.
	// Without the script the same markup is the native control.
	assert.Equal(t, 2, strings.Count(body, "<select data-select "))
	assert.Contains(t, body, `<option value="open" data-badge="badge-warn">Open</option>`)
	assert.Contains(t, body, `<option value="done" data-badge="badge-ok" selected>Done</option>`)
	assert.Contains(t, body, `<option value="later" data-badge="badge-mute">later</option>`) // no tone: muted
	assert.Contains(t, body, `<label class="lbl" id="p-f1-label" for="p-f1">`)
}

func TestASelectThatIsReadIsABadgeForEachValueInTheToneOfItsOption(t *testing.T) {
	t.Parallel()

	read := webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Table[task]{
		Title: "Tasks",
		Rows: func(context.Context) ([]task, webui.Loaded) {
			return []task{
				{Name: "a", Status: []string{"open"}, Labels: []string{"bug", "ui"}},
				{Name: "b", Status: []string{"unlisted"}},
				{Name: "c"},
			}, webui.Ready()
		},
		Columns: []webui.Accessor[task]{taskName, taskStatus, taskLabels},
	}}
	body := serve(webui.App{Pages: webui.Pages{read}}.MustCompile("/admin"), http.MethodGet, "/admin/t").Body.String()

	assert.Contains(t, body, `<span class="badge badge-warn"><span class="dot" aria-hidden="true"></span>Open</span>`) // its label, its tone
	assert.Contains(t, body, `<span class="badges">`)
	assert.Equal(t, strings.Count(body, `class="badge badge-mute"`), 3) // bug and UI, and the value that is no option
	assert.Contains(t, body, `>UI</span>`)
	assert.Contains(t, body, `>unlisted</span>`) // a value the select does not know is drawn as it is, neutral
}

func TestASelectInAFormThatCannotBeEditedIsReadAndAnEmptyOneIsADash(t *testing.T) {
	t.Parallel()

	read := webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Form[task]{
		Load: func(context.Context) (task, webui.Loaded) {
			return task{Name: "a", Status: []string{"done"}}, webui.Ready()
		},
		Fields: []webui.Accessor[task]{taskName, taskStatus, taskLabels}, // no Submit: read-only
	}}
	body := serve(webui.App{Pages: webui.Pages{read}}.MustCompile("/admin"), http.MethodGet, "/admin/t").Body.String()
	assert.False(t, strings.Contains(body, "<select"))
	assert.Contains(t, body, `badge-ok"><span class="dot" aria-hidden="true"></span>Done</span>`)
	assert.Contains(t, body, `<span class="dim" aria-hidden="true">—</span>`) // no labels
}

func TestASingleSelectHandedSeveralValuesShowsTheFirstWhenItIsEdited(t *testing.T) {
	t.Parallel()

	var saved task
	body := serve(taskApp(task{Name: "a", Status: []string{"later", "done"}}, &saved), http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `<option value="later" data-badge="badge-mute" selected>later</option>`)
	assert.False(t, strings.Contains(body, `<option value="done" selected>`))
}

func TestATableFiltersAMultiValuedColumnByAnyOfTheOptionsChosen(t *testing.T) {
	t.Parallel()

	tasks := []task{
		{Name: "one", Labels: []string{"bug"}}, {Name: "two", Labels: []string{"bug", "ui"}}, {Name: "three", Labels: []string{"docs"}},
		{Name: "four"}, {Name: "five", Labels: []string{"ui"}},
	}
	page := webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Table[task]{
		Title:   "Tasks",
		Rows:    func(context.Context) ([]task, webui.Loaded) { return tasks, webui.Ready() },
		Columns: []webui.Accessor[task]{taskName, webui.Select[task]{Label: "Labels", Load: func(t task) []string { return t.Labels }}}, // no options: the rows have them
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	names := func(target string) []string {
		var out []string
		for _, n := range []string{"one", "two", "three", "four", "five"} {
			if strings.Contains(serve(h, http.MethodGet, target).Body.String(), `>`+n+`</`) {
				out = append(out, n)
			}
		}
		return out
	}
	assert.DeepEqual(t, names("/admin/t?tasks.filter.labels=ui"), []string{"two", "five"})
	assert.DeepEqual(t, names("/admin/t?tasks.filter.labels=bug&tasks.filter.labels=docs"), []string{"one", "two", "three"})

	// The filter offers each value the rows hold once, whatever rows hold several.
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	filter := body[strings.Index(body, `<div class="filter-title">Labels</div>`):]
	filter = filter[:strings.Index(filter, `</form>`)]
	assert.Equal(t, strings.Count(filter, `type="checkbox"`), 3)
	for _, v := range []string{"bug", "docs", "ui"} {
		assert.Contains(t, filter, `value="`+v+`"`)
	}
}

func TestTheSearchAndTheSortOfASelectColumnUseWhatItShows(t *testing.T) {
	t.Parallel()

	page := webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Table[task]{
		Title: "Tasks",
		Rows: func(context.Context) ([]task, webui.Loaded) {
			return []task{{Name: "a", Labels: []string{"ui"}}, {Name: "b", Labels: []string{"docs"}}}, webui.Ready()
		},
		Columns: []webui.Accessor[task]{taskName, taskLabels},
	}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	// Sorted by the labels, as text.
	asc := serve(h, http.MethodGet, "/admin/t?tasks.sort=labels").Body.String()
	assert.True(t, strings.Index(asc, ">b<") < strings.Index(asc, ">a<")) // docs before ui
}

func TestASelectIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(a webui.Select[task]) string {
		page := webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Table[task]{
			Rows: func(context.Context) ([]task, webui.Loaded) { return nil, webui.Ready() }, Columns: []webui.Accessor[task]{a},
		}}
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{page}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	load := func(task) []string { return nil }
	store := func(*task, []string) {}
	assert.Contains(t, problems(webui.Select[task]{Label: "S"}), "has no Load")
	assert.Contains(t, problems(webui.Select[task]{Label: "S", Load: load, Options: []webui.Option{{Value: ""}}}), "Options[0] has no Value")
	assert.Contains(t, problems(webui.Select[task]{Label: "S", Load: load, Options: []webui.Option{{Value: "a"}, {Value: "a"}}}), `Options[1], "a", is listed twice`)
	assert.Contains(t, problems(webui.Select[task]{Label: "S", Load: load, Options: []webui.Option{{Value: "a\x1fb"}}}), "control character")
	assert.Contains(t, problems(webui.Select[task]{Label: "S", Load: load, Store: store}), "has a Store but no Options")
	assert.Contains(t, problems(webui.Select[task]{Label: "S", Load: load, Required: true}), "is Required but has no Store")
}
