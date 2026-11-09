package webui_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
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
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: form}}}.MustCompile("/admin")
}

func submitTask(h http.Handler, values url.Values) (code int, body string) {
	values.Set("_view", "p")
	rec := post(h, "/admin/t", values) // a refused one is the form again, with what was typed
	return rec.Code, rec.Body.String()
}

func TestASelectThatIsEditedIsAListToChooseFrom(t *testing.T) {
	t.Parallel()

	var saved task
	p := pageOf(t, taskApp(task{Name: "a", Status: []string{"done"}, Labels: []string{"bug", "docs"}}, &saved), "/admin/t")

	// One value: a drop-down list, with the choice that is held selected. It is required, so the
	// empty option asks for a choice.
	status, _ := p.Field("f1")
	assert.Equal(t, status.Type, "select")
	assert.True(t, status.Required)
	assert.Equal(t, page.One(t, "the status control", p.ByAttr("name", "f1")).Attr("data-msg-value-missing"), "This field is required.")
	assert.DeepEqual(t, option(status, ""), page.Option{Value: "", Text: "Choose…"})
	assert.True(t, option(status, "done").Selected)
	assert.Equal(t, option(status, "open").Text, "Open") // an option says its label
	assert.False(t, option(status, "open").Selected)

	// Any number: a list, with each that is held selected, and a hint on how to choose several.
	labels := page.One(t, "the labels control", p.ByAttr("name", "f2"))
	assert.True(t, labels.Has("multiple"))
	assert.Equal(t, labels.Attr("size"), "3")
	choice, _ := p.Field("f2")
	assert.True(t, option(choice, "bug").Selected)
	assert.False(t, option(choice, "ui").Selected)
	assert.Equal(t, option(choice, "ui").Text, "UI")
	assert.True(t, option(choice, "docs").Selected)
	assert.Contains(t, p.Root().AllText(), "Hold Ctrl or ⌘ to choose more than one")
	assert.False(t, hasOption(choice, "")) // a list of several has no empty option
}

func TestASelectThatIsNotRequiredCanBeLeftEmpty(t *testing.T) {
	t.Parallel()

	optional := taskStatus
	optional.Required = false
	declared := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Form[task]{
		Load:   func(context.Context) (task, webui.Loaded) { return task{Name: "a"}, webui.Ready() },
		Fields: []webui.Accessor[task]{optional},
		Submit: webui.Action[task]{Run: func(context.Context, task) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	}}
	p := pageOf(t, webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin"), "/admin/t")
	status, _ := p.Field("f0")
	assert.Equal(t, option(status, "").Text, "—")
	assert.False(t, status.Required)
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
	p := page.Parse(t, body)
	labels, _ := p.Field("f2")
	assert.True(t, option(labels, "bug").Selected)
	assert.True(t, option(labels, "ui").Selected)
	assert.False(t, option(labels, "docs").Selected)
	assert.True(t, len(p.ByAttr("aria-invalid", "true")) > 0)
}

func TestASelectIsMarkedForTheScriptThatTurnsItIntoADropDown(t *testing.T) {
	t.Parallel()

	var saved task
	p := pageOf(t, taskApp(task{Name: "a", Status: []string{"done"}, Labels: []string{"bug"}}, &saved), "/admin/t")

	// The script replaces the native control with a drop-down that shows the choices as badges,
	// so it needs the select marked, and each option's tone.
	// Without the script the same markup is the native control, named by its label.
	assert.Equal(t, len(p.ByAttr("data-select", "")), 2)
	tones := map[string]string{}
	for _, o := range p.ByRole("option", "") {
		if tone := o.Attr("data-badge"); tone != "" {
			tones[o.Attr("value")] = tone
		}
	}
	assert.Equal(t, tones["open"], "tone-warn")
	assert.Equal(t, tones["done"], "tone-ok")
	assert.Equal(t, tones["later"], "tone-mute") // no tone: muted
	assert.Equal(t, page.One(t, "the status control", p.ByLabel("Status")).Tag(), "select")
}

func TestASelectThatIsReadIsABadgeForEachValueInTheToneOfItsOption(t *testing.T) {
	t.Parallel()

	read := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[task]{
		Title: "Tasks",
		Loader: webui.RowsLoader[task]{Load: func(context.Context) ([]task, webui.Loaded) {
			return []task{
				{Name: "a", Status: []string{"open"}, Labels: []string{"bug", "ui"}},
				{Name: "b", Status: []string{"unlisted"}},
				{Name: "c"},
			}, webui.Ready()
		}},
		Columns: []webui.Accessor[task]{taskName, taskStatus, taskLabels},
	}}
	p := pageOf(t, webui.App{Pages: webui.Pages{read}}.MustCompile("/admin"), "/admin/t")

	// A badge for each value, by its label; a value the select does not know is drawn as it is. The tone is the colour, and the dot that carries it is not read.
	cells := page.One(t, "the table", p.ByRole("table", "Tasks")).ByRole("cell", "")
	badges := func(c page.El) []string { // each value is a badge of its own
		var out []string
		for _, group := range c.Children() {
			for _, b := range group.Children() {
				out = append(out, b.Text())
			}
		}
		return out
	}
	assert.DeepEqual(t, badges(cells[1]), []string{"Open"})
	assert.DeepEqual(t, badges(cells[2]), []string{"bug", "UI"})
	assert.DeepEqual(t, badges(cells[4]), []string{"unlisted"}) // a value the select does not know is drawn as it is
	assert.Equal(t, cells[5].Text(), "")                        // and none is nothing, with a dash to the eye
	assert.Equal(t, cells[7].Text(), "")
}

func TestASelectInAFormThatCannotBeEditedIsReadAndAnEmptyOneIsADash(t *testing.T) {
	t.Parallel()

	read := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Form[task]{
		Load: func(context.Context) (task, webui.Loaded) {
			return task{Name: "a", Status: []string{"done"}}, webui.Ready()
		},
		Fields: []webui.Accessor[task]{taskName, taskStatus, taskLabels}, // no Submit: read-only
	}}
	p := pageOf(t, webui.App{Pages: webui.Pages{read}}.MustCompile("/admin"), "/admin/t")
	assert.Equal(t, len(p.ByRole("combobox", ""))+len(p.ByRole("listbox", "")), 0)
	assert.Contains(t, p.Root().Text(), "Done")
	assert.Contains(t, p.Root().AllText(), "—") // no labels: a dash
}

func TestASingleSelectHandedSeveralValuesShowsTheFirstWhenItIsEdited(t *testing.T) {
	t.Parallel()

	var saved task
	status, _ := pageOf(t, taskApp(task{Name: "a", Status: []string{"later", "done"}}, &saved), "/admin/t").Field("f1")
	assert.True(t, option(status, "later").Selected)
	assert.False(t, option(status, "done").Selected)
}

func TestATableFiltersAMultiValuedColumnByAnyOfTheOptionsChosen(t *testing.T) {
	t.Parallel()

	tasks := []task{
		{Name: "one", Labels: []string{"bug"}}, {Name: "two", Labels: []string{"bug", "ui"}}, {Name: "three", Labels: []string{"docs"}},
		{Name: "four"}, {Name: "five", Labels: []string{"ui"}},
	}
	declared := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[task]{
		Title:   "Tasks",
		Loader:  webui.RowsLoader[task]{Load: func(context.Context) ([]task, webui.Loaded) { return tasks, webui.Ready() }},
		Columns: []webui.Accessor[task]{taskName, webui.Select[task]{Label: "Labels", Load: func(t task) []string { return t.Labels }}}, // no options: the rows have them
	}}
	h := webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin")

	names := func(target string) []string {
		var out []string
		for _, row := range page.One(t, "the table", pageOf(t, h, target).ByRole("table", "Tasks")).Rows() {
			if len(row) > 1 { // the row that says there is nothing to show is one cell wide
				out = append(out, row[0])
			}
		}
		return out
	}
	assert.DeepEqual(t, names("/admin/t?tasks.filter.labels=ui"), []string{"two", "five"})
	assert.DeepEqual(t, names("/admin/t?tasks.filter.labels=bug&tasks.filter.labels=docs"), []string{"one", "two", "three"})

	// The filter offers each value the rows hold once, whatever rows hold several.
	var offered []string
	for _, f := range fieldsNamed(pageOf(t, h, "/admin/t"), "tasks.filter.labels") {
		if f.Type == "checkbox" {
			offered = append(offered, f.Value)
		}
	}
	assert.DeepEqual(t, offered, []string{"bug", "docs", "ui"})
}

func TestTheSearchAndTheSortOfASelectColumnUseWhatItShows(t *testing.T) {
	t.Parallel()

	declared := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[task]{
		Title: "Tasks",
		Loader: webui.RowsLoader[task]{Load: func(context.Context) ([]task, webui.Loaded) {
			return []task{{Name: "a", Labels: []string{"ui"}}, {Name: "b", Labels: []string{"docs"}}}, webui.Ready()
		}},
		Columns: []webui.Accessor[task]{taskName, taskLabels},
	}}
	h := webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin")
	// Sorted by the labels, as text.
	asc := serve(h, http.MethodGet, "/admin/t?tasks.sort=labels").Body.String()
	assert.True(t, strings.Index(asc, ">b<") < strings.Index(asc, ">a<")) // docs before ui
}

func TestASelectIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(a webui.Select[task]) string {
		declared := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[task]{
			Loader: webui.RowsLoader[task]{Load: func(context.Context) ([]task, webui.Loaded) { return nil, webui.Ready() }}, Columns: []webui.Accessor[task]{a},
		}}
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{declared}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	load := func(task) []string { return nil }
	store := func(*task, []string) {}
	refuses(t, problems(webui.Select[task]{Label: "S"}), "Load")
	refuses(t, problems(webui.Select[task]{Label: "S", Load: load, Options: []webui.Option{{Value: ""}}}), "Options[0]", "Value")
	refuses(t, problems(webui.Select[task]{Label: "S", Load: load, Options: []webui.Option{{Value: "a"}, {Value: "a"}}}), "Options[1]", "\"a\"")
	refuses(t, problems(webui.Select[task]{Label: "S", Load: load, Options: []webui.Option{{Value: "a\x1fb"}}}), "control character")
	refuses(t, problems(webui.Select[task]{Label: "S", Load: load, Store: store}), "Store", "Options")
	refuses(t, problems(webui.Select[task]{Label: "S", Load: load, Required: true}), "Required", "Store")
}

// option is the option of a select that has the value, as a reader meets it; the zero Option if there is none.
func option(f page.Field, value string) page.Option {
	for _, o := range f.Options {
		if o.Value == value {
			return o
		}
	}
	return page.Option{}
}

// hasOption is whether a select has an option with the value.
func hasOption(f page.Field, value string) bool {
	for _, o := range f.Options {
		if o.Value == value {
			return true
		}
	}
	return false
}
