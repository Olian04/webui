package webui_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

// toolArgs are the arguments of the page that the toolbars are on, which a toolbar's buttons read with ArgsOf and its forms load their models from.
type toolArgs struct {
	Minutes int
	Site    string
	Tags    []string
}

type newArgs struct{ From string }

var newDevice = webui.Page[newArgs]{Path: "/device/new", View: webui.Stack{}}

// toolState is the model that the toolbar's form loads, here from the page's arguments, and shows.
type toolState struct {
	Minutes int
	Site    string
	Tags    []string
}

func loadToolState(ctx context.Context) (toolState, webui.Loaded) {
	a := webui.ArgsOf[toolArgs](ctx)
	return toolState(a), webui.Ready()
}

var (
	minutesInput = webui.Int[toolState]{ // the same accessors a form would use: their Store and Rules do nothing in a toolbar
		Label: "Minutes", Load: func(a toolState) int { return a.Minutes }, Store: func(a *toolState, v int) { a.Minutes = v },
		Rules: webui.NumberRules[int]{Min: new(1), Max: new(1440)},
	}
	siteInput = webui.Select[toolState]{
		Label: "Site", Load: func(a toolState) []string { return []string{a.Site} },
		Store:   func(a *toolState, v []string) { a.Site = strings.Join(v, "") },
		Options: []webui.Option{{Value: "north", Label: "North"}, {Value: "south"}},
	}
	tagsInput = webui.Select[toolState]{
		Label: "Tags", Multi: true, Load: func(a toolState) []string { return a.Tags }, Store: func(a *toolState, v []string) { a.Tags = v },
		Options: []webui.Option{{Value: "edge"}, {Value: "gpu"}},
	}
)

// settings is a toolbar's form of the page's state.
func settings(fields ...webui.Accessor[toolState]) webui.Form[toolState] {
	return webui.Form[toolState]{Title: "Settings", Load: loadToolState, Fields: fields}
}

type toolShop struct {
	h    http.Handler
	mu   sync.Mutex
	ran  []toolArgs
	deny error
}

func newToolShop(items ...webui.ToolbarItem) *toolShop {
	s := &toolShop{}
	if items == nil {
		items = []webui.ToolbarItem{
			webui.Link[webui.NoArgs, newArgs]{
				Label: "Create device", Role: webui.RolePrimary, Page: newDevice,
				Args: func(ctx context.Context, _ webui.NoArgs) newArgs {
					return newArgs{From: webui.ArgsOf[toolArgs](ctx).Site}
				},
			},
			webui.Action[webui.NoArgs]{
				Label: "Sync now",
				Guard: func(context.Context, webui.NoArgs) error { return s.deny },
				Run: func(ctx context.Context, _ webui.NoArgs) (webui.Outcome, error) {
					s.mu.Lock()
					s.ran = append(s.ran, webui.ArgsOf[toolArgs](ctx))
					s.mu.Unlock()
					return webui.Success("Synced"), nil
				},
			},
			settings(minutesInput, siteInput, tagsInput),
		}
	}
	tools := webui.Page[toolArgs]{Path: "/tools", View: webui.Stack{
		webui.Toolbar{Left: items[:2], Right: items[2:]},
		webui.Table[Device]{
			Title: "Devices",
			Loader: webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) {
				var all []Device
				for i := range 30 {
					all = append(all, Device{Id: string(rune('a' + i%26)), Count: i})
				}
				return all, webui.Ready()
			}},
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
		},
	}}
	s.h = webui.App{Pages: webui.Pages{tools, newDevice}}.MustCompile("/admin")
	return s
}

func flashOf(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "webui_flash" {
			raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
			return string(raw)
		}
	}
	return ""
}

// toolbarOf is the toolbar of a page: the first panel of the pages here, which put it before everything else.
func toolbarOf(t *testing.T, p *page.Page) page.El {
	t.Helper()
	views := p.Views()
	if len(views) == 0 {
		t.Fatal("no panel in the page")
	}
	return views[0]
}

// valuesOf is what the values of a toolbar's form say, by the label of each: the first thing in a value is its label and the last is what it reads.
func valuesOf(g page.El) map[string]string {
	got := map[string]string{}
	for _, value := range g.Children() {
		kids := value.Children()
		if len(kids) >= 2 {
			got[kids[0].Text()] = kids[len(kids)-1].Text()
		}
	}
	return got
}

// inputs are the controls below the element that a visitor could type in or choose with: the fields that are not what a button posts.
func inputs(e page.El) []string {
	var got []string
	for _, f := range e.Fields() {
		if f.Type != "hidden" && f.Type != "button" {
			got = append(got, f.Name)
		}
	}
	return got
}

func TestAToolbarIsARowOfButtonsAndTheValuesOfItsForms(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	bar := toolbarOf(t, pageOf(t, s.h, "/admin/tools?minutes=15&site=north&tags=edge&tags=gpu"))

	// A link is a real anchor, to the page its Args build, from the request: the page's arguments are read with ArgsOf.
	assert.DeepEqual(t, bar.Links(), []page.Link{{Name: "Create device", Href: "/admin/device/new?from=north"}})
	// An action is a button that posts, by its position.
	assert.DeepEqual(t, controls(bar), []control{{"Sync now", "1", false}})
	// A form is a group that its Title names, of its fields' labels and values as they are read: a number, and a select as its badges.
	group := page.One(t, "the Settings", bar.ByRole("group", "Settings"))
	got := valuesOf(group)
	assert.Equal(t, got["Minutes"], "15")
	assert.Equal(t, got["Site"], "North")
	assert.True(t, strings.HasPrefix(got["Tags"], "edge"))
	assert.Equal(t, len(group.ByText("gpu")), 1)
	assert.Equal(t, len(inputs(bar)), 0) // values to read, and nothing to type in
	// The one form is the buttons': it posts the panel and the action's position, and nothing else.
	names := []string{}
	for _, f := range bar.Fields() {
		names = append(names, f.Name)
	}
	assert.DeepEqual(t, names, []string{"_view", "_act"})
	// The buttons come first, and the values after them.
	assert.True(t, bar.ByRole("button", "Sync now")[0].Before(group))
}

// A toolbar with no button has nothing in it to submit, so it has no form.
func TestAToolbarOfLinksAndFormsHasNoForm(t *testing.T) {
	t.Parallel()

	s := newToolShop(
		webui.Link[webui.NoArgs, newArgs]{Label: "Create device", Page: newDevice, Args: func(context.Context, webui.NoArgs) newArgs { return newArgs{} }},
		webui.Link[webui.NoArgs, newArgs]{Label: "Create another", Page: newDevice, Args: func(context.Context, webui.NoArgs) newArgs { return newArgs{} }},
		settings(minutesInput, siteInput),
	)
	bar := toolbarOf(t, pageOf(t, s.h, "/admin/tools?minutes=15"))
	assert.Equal(t, len(bar.Fields()), 0)
	assert.Equal(t, valuesOf(page.One(t, "the Settings", bar.ByRole("group", "Settings")))["Minutes"], "15")
	assert.DeepEqual(t, bar.Links(), []page.Link{{Name: "Create device", Href: "/admin/device/new"}, {Name: "Create another", Href: "/admin/device/new"}})
}

// The accessor is the one a form uses: its Store is ignored, as a table ignores it.
func TestAToolbarShowsAnAccessorReadOnlyWhateverItsStore(t *testing.T) {
	t.Parallel()

	withStore := webui.Int[toolState]{Label: "Minutes", Load: func(a toolState) int { return a.Minutes }, Store: func(a *toolState, v int) { a.Minutes = v }}
	without := webui.Int[toolState]{Label: "Minutes", Load: func(a toolState) int { return a.Minutes }}
	app := func(field webui.Accessor[toolState]) http.Handler {
		return webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: []webui.ToolbarItem{settings(field)}}}}}.MustCompile("/admin")
	}
	a := toolbarOf(t, pageOf(t, app(withStore), "/admin/tools?minutes=15"))
	b := toolbarOf(t, pageOf(t, app(without), "/admin/tools?minutes=15"))
	assert.Equal(t, a.Text(), b.Text())
	assert.Equal(t, len(a.Fields()), len(b.Fields()))
	assert.Equal(t, len(inputs(a)), 0)
}

func TestAToolbarValueIsAsAReadOnlyFieldShowsItAndMayHaveHelp(t *testing.T) {
	t.Parallel()

	type state struct { // a model is plain values: an absent one is derived from them
		Size int
		Name string
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: webui.Toolbar{Left: []webui.ToolbarItem{webui.Form[state]{
		Title: "State", Load: func(context.Context) (state, webui.Loaded) { return state{}, webui.Ready() },
		Fields: []webui.Accessor[state]{
			webui.OptionalBool[state]{Label: "Paused", Load: func(a state) *bool {
				if a.Size == 0 {
					return nil // not set
				}
				yes := a.Size > 5
				return &yes
			}},
			webui.Int[state]{Label: "Size", Help: "How much is kept.", Load: func(a state) int { return a.Size }, Format: func(n int) string { return strconv.Itoa(n) + " KiB" }},
			webui.String[state]{Label: "Name", Load: func(a state) string { return a.Name }},
			webui.URL[state]{Label: "Docs", Text: "Read", Load: func(state) string { return "https://example.com/docs" }},
		},
	}}}}}}.MustCompile("/admin")

	bar := toolbarOf(t, pageOf(t, h, "/admin/t"))
	// Absent is empty, and a value is as a read-only form field shows it: a Format, a link.
	assert.DeepEqual(t, valuesOf(page.One(t, "the State", bar.ByRole("group", "State"))), map[string]string{
		"Paused": "empty", "Size": "0 KiB", "Name": "empty", "Docs": "Read(opens in a new tab)",
	})
	assert.DeepEqual(t, bar.Links(), []page.Link{{Name: "Read(opens in a new tab)", Href: "https://example.com/docs", NewTab: true}})
	// What the help says is in the popover beside the label.
	assert.Equal(t, len(bar.ByRole("note", "Help: How much is kept.")), 1)
	assert.Equal(t, len(bar.Fields()), 0)
}

// A toolbar may have several forms, each its own model, side by side in the row.
func TestAToolbarMayHaveFormsOfDifferentModels(t *testing.T) {
	t.Parallel()

	type queue struct{ Depth int }
	depth := webui.Form[queue]{
		Title: "Queue", Load: func(context.Context) (queue, webui.Loaded) { return queue{Depth: 128}, webui.Ready() },
		Fields: []webui.Accessor[queue]{webui.Int[queue]{Label: "Depth", Load: func(q queue) int { return q.Depth }}},
	}
	tools := webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: []webui.ToolbarItem{settings(minutesInput), depth}}}
	bar := toolbarOf(t, pageOf(t, webui.App{Pages: webui.Pages{tools}}.MustCompile("/admin"), "/admin/tools?minutes=15"))
	assert.DeepEqual(t, valuesOf(page.One(t, "the Settings", bar.ByRole("group", "Settings"))), map[string]string{"Minutes": "15"})
	assert.DeepEqual(t, valuesOf(page.One(t, "the Queue", bar.ByRole("group", "Queue"))), map[string]string{"Depth": "128"})
}

// A form that cannot be loaded says so in its place in the row, and the rest of the toolbar, its buttons and the other forms, stands.
func TestAFormThatCannotBeLoadedDoesNotTakeTheToolbarDown(t *testing.T) {
	t.Parallel()

	broken := webui.Form[toolState]{Title: "Broken", Load: func(context.Context) (toolState, webui.Loaded) {
		return toolState{}, webui.Failed(errors.New("the backend is down"))
	}, Fields: []webui.Accessor[toolState]{tagsInput}}
	refused := webui.Form[toolState]{Title: "Refused", Load: func(context.Context) (toolState, webui.Loaded) {
		return toolState{}, webui.Failed(webui.Forbidden("Not for you."))
	}, Fields: []webui.Accessor[toolState]{siteInput}}
	run := func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }
	s := newToolShop(
		webui.Action[webui.NoArgs]{Label: "Go", Run: run},
		webui.Action[webui.NoArgs]{Label: "Again", Run: run},
		broken, refused, settings(minutesInput),
	)
	bar := toolbarOf(t, pageOf(t, s.h, "/admin/tools?minutes=15"))
	assert.Equal(t, page.One(t, "the Broken", bar.ByRole("group", "Broken")).Text(), "Broken Could not load")
	assert.False(t, strings.Contains(bar.AllText(), "the backend is down")) // the cause is logged, never shown
	assert.Equal(t, page.One(t, "the Refused", bar.ByRole("group", "Refused")).Text(), "Refused Not for you.")
	assert.DeepEqual(t, controls(bar), []control{{"Go", "0", false}, {"Again", "1", false}})
	assert.Equal(t, valuesOf(page.One(t, "the Settings", bar.ByRole("group", "Settings")))["Minutes"], "15")
}

// A toolbar is a panel of its own, and its forms are not: a form that says it is not final has the whole toolbar asked for again, by the shortest of them.
func TestAToolbarWhoseFormIsNotFinalIsAskedForAgain(t *testing.T) {
	t.Parallel()

	going := func(d time.Duration) webui.Form[toolState] {
		f := settings(minutesInput)
		f.Load = func(ctx context.Context) (toolState, webui.Loaded) {
			m, _ := loadToolState(ctx)
			return m, webui.Ready().RefreshAfter(d)
		}
		return f
	}
	other := going(9 * time.Second)
	other.Title = "Other"
	other.Fields = []webui.Accessor[toolState]{siteInput}
	tools := webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: []webui.ToolbarItem{going(4 * time.Second), other}}}
	h := webui.App{Pages: webui.Pages{tools}}.MustCompile("/admin")

	p := pageOf(t, h, "/admin/tools?minutes=15")
	bar := toolbarOf(t, p)
	assert.Equal(t, bar.Attr("data-view"), "p")
	assert.Equal(t, bar.Attr("data-refresh-after"), "4")
	// A toolbar has little room, and a note in the row would move what is in it: it says nothing that is seen.
	assert.False(t, strings.Contains(strings.ToLower(bar.AllText()), "refresh"))
	assert.Equal(t, len(p.ByAttr("data-refresh-after", "")), 1)

	// Asked for alone, as script does, the toolbar answers with its own root first, which carries it.
	rec := serveView(h, "/admin/tools?minutes=15", "p")
	assert.Equal(t, rec.Code, http.StatusOK)
	alone := page.Parse(t, rec.Body.String())
	assert.True(t, page.IsFragment(rec.Body.String()))
	assert.Equal(t, toolbarOf(t, alone).Attr("data-view"), "p")
	assert.Equal(t, toolbarOf(t, alone).Attr("data-refresh-after"), "4")

	// A toolbar of forms that are final says nothing of it.
	final := pageOf(t, webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: []webui.ToolbarItem{settings(minutesInput)}}}}}.MustCompile("/admin"), "/admin/tools")
	assert.Equal(t, len(final.ByAttr("data-refresh-after", "")), 0)
	assert.Equal(t, toolbarOf(t, final).Attr("data-view"), "p")
}

func TestAnActionOfAToolbarReadsThePagesArgumentsFromTheRequest(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	rec := post(s.h, "/admin/tools?minutes=15&site=north", url.Values{"_view": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/tools?minutes=15&site=north")
	assert.Equal(t, flashOf(rec), "oSynced")
	assert.DeepEqual(t, s.ran, []toolArgs{{Minutes: 15, Site: "north"}})

	// What is not a button is not understood, and nothing is done.
	for _, act := range []string{"0", "2", "9", "-1", "x", ""} {
		rec = post(s.h, "/admin/tools", url.Values{"_view": {"p.0"}, "_act": {act}})
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.Equal(t, flashOf(rec), "eThat request was not understood.")
	}
	assert.Equal(t, len(s.ran), 1)
}

// The Guard that leaves a button out is the one that refuses the request, and so is the one that disables it.
func TestAToolbarButtonThatIsGuardedIsLeftOutOrDisabledAndRefused(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	s.deny = errors.New("requires the editor role")
	p := pageOf(t, s.h, "/admin/tools")
	assert.False(t, strings.Contains(p.Root().AllText(), "Sync now"))
	assert.False(t, strings.Contains(p.Root().AllText(), "requires the editor role")) // nothing says that there was a button
	assert.True(t, hasLink(p, "Create device", "/admin/device/new"))
	assert.Equal(t, len(controls(toolbarOf(t, p))), 0) // and with no button there is nothing to post

	rec := post(s.h, "/admin/tools", url.Values{"_view": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "erequires the editor role")
	assert.Equal(t, len(s.ran), 0)

	s.deny = webui.Disabled("requires the editor role")
	bar := toolbarOf(t, pageOf(t, s.h, "/admin/tools"))
	assert.DeepEqual(t, controls(bar), []control{{"Sync now", "1", true}})
	assert.True(t, strings.Contains(bar.Text(), "Sync now Not available: requires the editor role"))

	rec = post(s.h, "/admin/tools", url.Values{"_view": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, flashOf(rec), "erequires the editor role")
	assert.Equal(t, len(s.ran), 0)
}

// A select of a toolbar can read its options when the request is answered: they say what its badge reads.
func TestASelectInAToolbarMayReadItsOptions(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red"}, webui.Option{Value: "blue", Label: "Blue team"})
	team := webui.Select[toolState]{
		Label: "Team", Load: func(a toolState) []string { return []string{a.Site} }, LoadOptions: s.load,
	}
	tools := webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: []webui.ToolbarItem{settings(team)}}}
	h := webui.App{Pages: webui.Pages{tools}}.MustCompile("/admin")

	team1 := toolbarOf(t, pageOf(t, h, "/admin/tools?site=blue"))
	assert.Equal(t, valuesOf(page.One(t, "the Settings", team1.ByRole("group", "Settings")))["Team"], "Blue team")

	s.err = errors.New("the database is down")
	assert.Equal(t, len(pageOf(t, h, "/admin/tools").ByText("Could not load")) > 0, true)
}

func TestAToolbarIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(body webui.View) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: body}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	run := func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }
	refuses(t, problems(webui.Toolbar{}), "Toolbar")
	refuses(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Run: run}}}), "Toolbar.Left[0]", "Label")
	refuses(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Go"}}}), "Toolbar.Left[0]", "Run")
	refuses(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{
		webui.Link[webui.NoArgs, newArgs]{Page: newDevice, Args: func(context.Context, webui.NoArgs) newArgs { return newArgs{} }},
	}}), "Toolbar.Left[0]", "Label")
	// A value to read needs no Store.
	_, err := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: []webui.ToolbarItem{
		settings(webui.String[toolState]{Label: "Site", Load: func(a toolState) string { return a.Site }}),
	}}}}}.Compile("/admin")
	assert.NoError(t, err)
	// Where a control is said to be follows the group it is in.
	refuses(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{settings(siteInput)}, Right: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Go"}}}), "Toolbar.Right[0]", "Run")
}

// A Form in a toolbar is only a group of values to read: what would make it an input, a panel, or something that cannot be one line is refused, with what to do instead.
func TestAFormInAToolbarIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(items ...webui.ToolbarItem) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: items}}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	ok := settings(minutesInput)
	_, err := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{Left: []webui.ToolbarItem{ok}}}}}.Compile("/admin")
	assert.NoError(t, err)

	noTitle := ok
	noTitle.Title = ""
	refuses(t, problems(noTitle), "Toolbar.Left[0]", "Title")

	noLoad := ok
	noLoad.Load = nil
	refuses(t, problems(noLoad), "Toolbar.Left[0]", "Load")

	noFields := ok
	noFields.Fields = nil
	refuses(t, problems(noFields), "Toolbar.Left[0]", "Fields")

	withSubmit := ok
	withSubmit.Submit = webui.Action[toolState]{Run: func(context.Context, toolState) (webui.Outcome, error) { return webui.Outcome{}, nil }}
	refuses(t, problems(withSubmit), "Toolbar.Left[0]", "Submit")

	// The labels of its fields are one list, across the forms of the toolbar.
	refuses(t, problems(settings(siteInput), settings(siteInput)), "\"Site\"", "twice")

	text := func(f webui.String[toolState]) webui.Form[toolState] { return settings(f) }
	load := func(a toolState) string { return strconv.Itoa(a.Minutes) }
	store := func(*toolState, string) {}
	refuses(t, problems(settings(webui.Secret[toolState]{Label: "Key", Load: func(a toolState) bool { return a.Minutes != 0 }, Store: store})), "Secret", "toolbar")
	refuses(t, problems(text(webui.String[toolState]{Label: "Body", Multiline: true, Load: load, Store: store})), "Multiline", "toolbar")
	refuses(t, problems(settings(webui.File[toolState]{Label: "Logo", MaxSize: 1 << 20, Load: func(toolState) *webui.FileInfo { return nil }, Store: func(*toolState, *webui.Upload) {}})), "File", "toolbar")
}
