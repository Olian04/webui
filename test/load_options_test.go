package webui_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// teams is what an app keeps its options in: it changes between requests.
type teams struct {
	mu   sync.Mutex
	list []webui.Option
	err  error
	read int
}

func (s *teams) set(options ...webui.Option) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = options
}

func (s *teams) load(context.Context) ([]webui.Option, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.read++
	return append([]webui.Option(nil), s.list...), s.err
}

type issue struct {
	Name string
	Team []string
}

func issueTeam(s *teams) webui.Select[issue] {
	return webui.Select[issue]{
		Label:       "Team",
		Load:        func(t issue) []string { return t.Team },
		Store:       func(t *issue, v []string) { t.Team = v },
		LoadOptions: s.load,
	}
}

func issueForm(s *teams, saved *issue) http.Handler {
	form := webui.Form[issue]{
		Load: func(context.Context) (issue, webui.Loaded) {
			return issue{Name: "a", Team: []string{"red"}}, webui.Ready()
		},
		Fields: []webui.Accessor[issue]{issueTeam(s)},
		Submit: webui.Action[issue]{Run: func(_ context.Context, t issue) (webui.Outcome, error) {
			*saved = t
			return webui.Success("Saved"), nil
		}},
	}
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: form}}}.MustCompile("/admin")
}

func submitTeam(h http.Handler, value string) (int, string) {
	rec := post(h, "/admin/t", url.Values{"_view": {"p"}, "f0": {value}})
	return rec.Code, rec.Body.String()
}

// The options are the app's, read when a request is answered: a form offers what there is now, and holds a
// submission to it.
func TestASelectThatReadsItsOptionsOffersAndChecksWhatThereIsNow(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red", Label: "Red team", Tone: webui.ToneCritical}, webui.Option{Value: "blue"})
	var saved issue
	h := issueForm(s, &saved)

	offered := func(p *page.Page) []page.Option {
		team, _ := p.Field("f0")
		return team.Options
	}
	assert.DeepEqual(t, offered(pageOf(t, h, "/admin/t")), []page.Option{{Value: "", Text: "—"}, {Value: "red", Text: "Red team", Selected: true}, {Value: "blue", Text: "blue"}})

	code, _ := submitTeam(h, "green")
	assert.Equal(t, code, http.StatusUnprocessableEntity) // not an option yet
	assert.Equal(t, len(saved.Team), 0)

	// Someone adds a team: the next request offers it and accepts it, with no restart.
	s.set(webui.Option{Value: "red"}, webui.Option{Value: "blue"}, webui.Option{Value: "green", Label: "Green"})
	assert.True(t, slices.Contains(offered(pageOf(t, h, "/admin/t")), page.Option{Value: "green", Text: "Green"}))
	code, _ = submitTeam(h, "green")
	assert.Equal(t, code, http.StatusSeeOther)
	assert.DeepEqual(t, saved.Team, []string{"green"})

	// And one that is taken away is refused, though a form that was opened before still offers it.
	s.set(webui.Option{Value: "red"})
	code, body := submitTeam(h, "blue")
	assert.Equal(t, code, http.StatusUnprocessableEntity)
	refused := page.Parse(t, body)
	assert.True(t, strings.Contains(refused.Root().Text(), "Choose from the options offered."))
	assert.DeepEqual(t, offered(refused), []page.Option{{Value: "", Text: "—"}, {Value: "red", Text: "red"}}) // the form drawn again has the options as they are now
}

func TestASelectThatIsReadSaysWhatTheOptionsReadNow(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red", Label: "Red team", Tone: webui.ToneCritical})
	form := webui.Form[issue]{
		Load: func(context.Context) (issue, webui.Loaded) {
			return issue{Team: []string{"red", "gone"}}, webui.Ready()
		},
		Fields: []webui.Accessor[issue]{webui.Select[issue]{Label: "Team", Load: func(t issue) []string { return t.Team }, LoadOptions: s.load}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: form}}}.MustCompile("/admin")
	team := page.One(t, "the Team", pageOf(t, h, "/admin/t").ByRole("group", "Team"))
	assert.True(t, strings.Contains(team.Text(), "Red team"))
	assert.True(t, strings.Contains(team.Text(), "gone")) // a value that is not an option says what it is
}

// A table's filter is the options as they are, with Load, where the source does the filtering. An address can only ask
// for what is offered.
func TestATablesFilterOffersTheOptionsThatAreReadAndKeepsOnlyThose(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red"}, webui.Option{Value: "blue"})
	var asked []map[string][]string
	var mu sync.Mutex
	declared := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[issue]{
		Title: "Issues",
		Loader: webui.PagedLoader[issue]{Load: func(_ context.Context, q webui.PagedQuery) (webui.PagedWindow[issue], webui.Loaded) {
			mu.Lock()
			asked = append(asked, q.Filters)
			mu.Unlock()
			return webui.PagedWindow[issue]{Items: []issue{{Name: "a", Team: []string{"red"}}}, Total: 1}, webui.Ready()
		}},
		Columns: []webui.Accessor[issue]{
			webui.String[issue]{Label: "Name", Load: func(t issue) string { return t.Name }},
			webui.Select[issue]{Label: "Team", Load: func(t issue) []string { return t.Team }, LoadOptions: s.load},
		},
	}}
	h := webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin")

	filterValues := func() []string {
		var out []string
		for _, f := range fieldsNamed(pageOf(t, h, "/admin/t"), "issues.filter.team") {
			out = append(out, f.Value)
		}
		return out
	}
	assert.DeepEqual(t, filterValues(), []string{"red", "blue"})

	s.set(webui.Option{Value: "red"}, webui.Option{Value: "blue"}, webui.Option{Value: "green"})
	assert.DeepEqual(t, filterValues(), []string{"red", "blue", "green"})

	mu.Lock()
	asked = nil
	mu.Unlock()
	serve(h, http.MethodGet, "/admin/t?issues.filter.team=green&issues.filter.team=purple")
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, len(asked), 1)
	assert.DeepEqual(t, asked[0]["Team"], []string{"green"}) // purple is not an option, so it never reaches the source
}

// With Rows the filter is the values the rows hold, as it is for options that are listed; the options say how they read.
func TestATableThatHoldsItsRowsFiltersByTheValuesItsRowsHoldAndSaysThemAsTheOptionsDo(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red"}, webui.Option{Value: "blue"})
	declared := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[issue]{
		Title: "Issues",
		Loader: webui.RowsLoader[issue]{Load: func(context.Context) ([]issue, webui.Loaded) {
			return []issue{{Name: "a", Team: []string{"red"}}, {Name: "b", Team: []string{"mauve"}}}, webui.Ready()
		}},
		Columns: []webui.Accessor[issue]{
			webui.String[issue]{Label: "Name", Load: func(t issue) string { return t.Name }},
			webui.Select[issue]{Label: "Team", Load: func(t issue) []string { return t.Team }, LoadOptions: s.load},
		},
	}}
	h := webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin")
	var held []string
	for _, f := range fieldsNamed(pageOf(t, h, "/admin/t"), "issues.filter.team") {
		held = append(held, f.Value)
	}
	assert.DeepEqual(t, held, []string{"mauve", "red"}) // no row holds blue
	narrowed := page.One(t, "the issues", pageOf(t, h, "/admin/t?issues.filter.team=mauve").ByRole("table", "Issues"))
	assert.DeepEqual(t, cellsOf(t, narrowed, "Name"), []string{"b"})
}

// A source that fails is the panel's failure, as a failed Load is, and the page stays.
func TestASourceOfOptionsThatFailsFailsWhatNeededIt(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red"})
	var saved issue
	h := issueForm(s, &saved)
	s.err = errors.New("the database is down")

	rec := serve(h, http.MethodGet, "/admin/t")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), "Could not load")
	assert.False(t, strings.Contains(rec.Body.String(), "the database is down")) // the cause is logged, not shown

	code, _ := submitTeam(h, "red")
	assert.Equal(t, code, http.StatusSeeOther) // a POST that fails is answered with a redirect and a toast, as any is
	assert.Equal(t, len(saved.Team), 0)        // and nothing was saved

	s.err = nil
	assert.Equal(t, serve(h, http.MethodGet, "/admin/t").Code, http.StatusOK)
	code, _ = submitTeam(h, "red")
	assert.Equal(t, code, http.StatusSeeOther)
}

// Options that cannot be told apart, or are not words, are the source's mistake, found when it is read.
func TestOptionsThatAreReadAreHeldToWhatOptionsAre(t *testing.T) {
	t.Parallel()

	for name, options := range map[string][]webui.Option{
		"an empty value":      {{Value: ""}},
		"a value twice":       {{Value: "a"}, {Value: "a"}},
		"a control character": {{Value: "a\x1fb"}},
	} {
		s := &teams{}
		s.set(options...)
		var saved issue
		rec := serve(issueForm(s, &saved), http.MethodGet, "/admin/t")
		if !strings.Contains(rec.Body.String(), "Could not load") {
			t.Errorf("%s: the options were accepted", name)
		}
	}
}

func TestASelectThatReadsItsOptionsIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(a webui.Select[task]) string {
		page := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[task]{
			Loader: webui.RowsLoader[task]{Load: func(context.Context) ([]task, webui.Loaded) { return nil, webui.Ready() }}, Columns: []webui.Accessor[task]{a},
		}}
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{page}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	load := func(task) []string { return nil }
	source := func(context.Context) ([]webui.Option, error) { return nil, nil }
	refuses(t, problems(webui.Select[task]{Label: "S", Load: load, Options: []webui.Option{{Value: "a"}}, LoadOptions: source}), "Options", "LoadOptions")

	// A Store with a source is a select with something to choose from.
	page := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Form[task]{
		Load:   func(context.Context) (task, webui.Loaded) { return task{}, webui.Ready() },
		Fields: []webui.Accessor[task]{webui.Select[task]{Label: "S", Load: load, Store: func(*task, []string) {}, LoadOptions: source}},
	}}
	_, err := webui.App{Pages: webui.Pages{page}}.Compile("/admin")
	assert.NoError(t, err)
}

// A downloaded cell says what the option says, as the table does.
func TestAnExportSaysWhatTheOptionsReadNow(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red", Label: "Red team"})
	declared := webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[issue]{
		Title:  "Issues",
		Export: true,
		Loader: webui.RowsLoader[issue]{Load: func(context.Context) ([]issue, webui.Loaded) {
			return []issue{{Name: "a", Team: []string{"red"}}}, webui.Ready()
		}},
		Columns: []webui.Accessor[issue]{
			webui.String[issue]{Label: "Name", Load: func(t issue) string { return t.Name }},
			webui.Select[issue]{Label: "Team", Load: func(t issue) []string { return t.Team }, LoadOptions: s.load},
		},
	}}
	h := webui.App{Pages: webui.Pages{declared}}.MustCompile("/admin")
	out := records(t, serve(h, http.MethodGet, "/admin/t?issues.export=csv").Body.String())
	assert.DeepEqual(t, out, [][]string{{"Name", "Team"}, {"a", "Red team"}})
}

// A page's search finds a row by what an option says.
func TestASearchFindsARowByWhatTheOptionsReadNow(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "10.0.0.1", Label: "Edge router"})
	byTeam := webui.Select[Device]{Label: "Role", Load: func(d Device) []string { return []string{d.Ip} }, LoadOptions: s.load}
	h := searchApp(webui.SearchRows(
		func(context.Context) ([]Device, webui.Loaded) { return searchDevices(), webui.Ready() },
		deviceHit, byTeam,
	), nil)
	got := search(t, h, "edge")
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Title, "dev-eth 0")

	s.set(webui.Option{Value: "10.0.0.2", Label: "Edge router"})
	got = search(t, h, "edge")
	assert.Equal(t, len(got.Results), 1)
	assert.Equal(t, got.Results[0].Title, "dev-eth 1")
}
