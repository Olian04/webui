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
)

// toolArgs are the arguments of the page that the toolbars are on, which a toolbar's buttons read with ArgsOf and its forms load their models from.
type toolArgs struct {
	Minutes int
	Site    string
	Tags    []string
}

type newArgs struct{ From string }

var newDevice = webui.Page[newArgs]{Path: "/device/new", Body: webui.Stack{}}

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
	page := webui.Page[toolArgs]{Path: "/tools", Body: webui.Stack{
		webui.Toolbar{Left: items[:2], Right: items[2:]},
		webui.Table[Device]{
			Title: "Devices",
			Rows: func(context.Context) ([]Device, webui.Loaded) {
				var all []Device
				for i := range 30 {
					all = append(all, Device{Id: string(rune('a' + i%26)), Count: i})
				}
				return all, webui.Ready()
			},
			Columns: []webui.Accessor[Device]{webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.Id }}},
		},
	}}
	s.h = webui.App{Pages: webui.Pages{page, newDevice}}.MustCompile("/admin")
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

// toolbarOf is the toolbar of a page, from its root to the first panel after it.
func toolbarOf(body string) string {
	return body[strings.Index(body, `<div class="toolbar"`):strings.Index(body, `<div class="panel grows"`)]
}

func TestAToolbarIsARowOfButtonsAndTheValuesOfItsForms(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	bar := toolbarOf(serve(s.h, http.MethodGet, "/admin/tools?minutes=15&site=north&tags=edge&tags=gpu").Body.String())

	// A link is a real anchor, to the page its Args build, from the request: the page's arguments are read with ArgsOf.
	assert.Contains(t, bar, `<a class="btn btn-primary btn-sm" href="/admin/device/new?from=north">Create device</a>`)
	// An action is a button that posts, by its position.
	assert.Contains(t, bar, `<button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1">Sync now</button>`)
	// A form is a group that its Title names, of its fields' labels and values as they are read: a number, and a select as its badges.
	assert.Contains(t, bar, `<span class="toolbar-form" role="group" aria-label="Settings">`)
	assert.Contains(t, bar, `<span class="toolbar-value"><span class="toolbar-lbl">Minutes</span> <span class="toolbar-val">15</span></span>`)
	assert.Contains(t, bar, `<span class="toolbar-lbl">Site</span> <span class="toolbar-val"><span class="badges"><span class="badge badge-mute"><span class="dot" aria-hidden="true"></span>North</span></span></span>`)
	assert.Contains(t, bar, `>edge</span><span class="badge badge-mute"><span class="dot" aria-hidden="true"></span>gpu</span>`)
	for _, input := range []string{"<input type=\"number\"", "<select", "<textarea", "toolbar-apply", "data-toolbar-form", "_toolbar", `method="get"`, `<label`} {
		assert.False(t, strings.Contains(bar, input))
	}
	// The one form is the buttons': it posts the leaf and the action's position, and nothing else.
	assert.Equal(t, strings.Count(bar, "<form"), 1)
	assert.Contains(t, bar, `method="post"`)
	// The left controls and the right ones are one row with a gap between them, which takes what is left over.
	assert.Equal(t, strings.Count(bar, `<span class="toolbar-gap" aria-hidden="true"></span>`), 1)
	assert.True(t, strings.Index(bar, ">Sync now</button>") < strings.Index(bar, `class="toolbar-gap"`))
	assert.True(t, strings.Index(bar, `class="toolbar-gap"`) < strings.Index(bar, `class="toolbar-lbl">Minutes`))
}

// A toolbar with no button has nothing in it to submit, so it has no form.
func TestAToolbarOfLinksAndFormsHasNoForm(t *testing.T) {
	t.Parallel()

	s := newToolShop(
		webui.Link[webui.NoArgs, newArgs]{Label: "Create device", Page: newDevice, Args: func(context.Context, webui.NoArgs) newArgs { return newArgs{} }},
		webui.Link[webui.NoArgs, newArgs]{Label: "Create another", Page: newDevice, Args: func(context.Context, webui.NoArgs) newArgs { return newArgs{} }},
		settings(minutesInput, siteInput),
	)
	bar := toolbarOf(serve(s.h, http.MethodGet, "/admin/tools?minutes=15").Body.String())
	assert.False(t, strings.Contains(bar, "<form"))
	assert.False(t, strings.Contains(bar, "_leaf"))
	assert.Contains(t, bar, `<span class="toolbar-lbl">Minutes</span> <span class="toolbar-val">15</span>`)
	assert.Contains(t, bar, `<a class="btn btn-secondary btn-sm" href="/admin/device/new">Create device</a>`)
}

// The accessor is the one a form uses: its Store is ignored, as a table ignores it.
func TestAToolbarShowsAnAccessorReadOnlyWhateverItsStore(t *testing.T) {
	t.Parallel()

	withStore := webui.Int[toolState]{Label: "Minutes", Load: func(a toolState) int { return a.Minutes }, Store: func(a *toolState, v int) { a.Minutes = v }}
	without := webui.Int[toolState]{Label: "Minutes", Load: func(a toolState) int { return a.Minutes }}
	page := func(field webui.Accessor[toolState]) http.Handler {
		return webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: []webui.ToolbarItem{settings(field)}}}}}.MustCompile("/admin")
	}
	a := serve(page(withStore), http.MethodGet, "/admin/tools?minutes=15").Body.String()
	b := serve(page(without), http.MethodGet, "/admin/tools?minutes=15").Body.String()
	pick := func(s string) string {
		i := strings.Index(s, `<div class="toolbar"`)
		return s[i : i+strings.Index(s[i:], `</div>`)]
	}
	assert.Equal(t, pick(a), pick(b))
}

func TestAToolbarValueIsAsAReadOnlyFieldShowsItAndMayHaveHelp(t *testing.T) {
	t.Parallel()

	type state struct { // a model is plain values: an absent one is derived from them
		Size int
		Name string
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Toolbar{Left: []webui.ToolbarItem{webui.Form[state]{
		Title: "State", Load: func(context.Context) (state, webui.Loaded) { return state{}, webui.Ready() },
		Fields: []webui.Accessor[state]{
			webui.Bool[state]{Label: "Paused", Load: func(a state) *bool {
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

	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	empty := body[strings.Index(body, `<div class="toolbar"`):strings.Index(body, `</main>`)]
	// Absent is a dash, and a value is as a read-only form field shows it: a Format, a link.
	assert.Contains(t, empty, `<span class="toolbar-lbl">Paused</span> <span class="toolbar-val"><span class="dim" aria-hidden="true">—</span>`)
	assert.Contains(t, empty, `<span class="toolbar-lbl">Name</span> <span class="toolbar-val"><span class="dim" aria-hidden="true">—</span>`)
	assert.Contains(t, empty, `<span class="toolbar-val">0 KiB</span>`)
	assert.Contains(t, empty, `href="https://example.com/docs"`)
	// What the help says is in the popover beside the label.
	assert.Contains(t, empty, `<span class="tip" tabindex="0" role="note" aria-label="Help: How much is kept.">`)
	assert.False(t, strings.Contains(empty, "<form"))
}

// A toolbar may have several forms, each its own model, side by side in the row.
func TestAToolbarMayHaveFormsOfDifferentModels(t *testing.T) {
	t.Parallel()

	type queue struct{ Depth int }
	depth := webui.Form[queue]{
		Title: "Queue", Load: func(context.Context) (queue, webui.Loaded) { return queue{Depth: 128}, webui.Ready() },
		Fields: []webui.Accessor[queue]{webui.Int[queue]{Label: "Depth", Load: func(q queue) int { return q.Depth }}},
	}
	page := webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: []webui.ToolbarItem{settings(minutesInput), depth}}}
	body := serve(webui.App{Pages: webui.Pages{page}}.MustCompile("/admin"), http.MethodGet, "/admin/tools?minutes=15").Body.String()
	assert.Contains(t, body, `<span class="toolbar-form" role="group" aria-label="Settings">`)
	assert.Contains(t, body, `<span class="toolbar-form" role="group" aria-label="Queue">`)
	assert.Contains(t, body, `<span class="toolbar-lbl">Minutes</span> <span class="toolbar-val">15</span>`)
	assert.Contains(t, body, `<span class="toolbar-lbl">Depth</span> <span class="toolbar-val">128</span>`)
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
	bar := toolbarOf(serve(s.h, http.MethodGet, "/admin/tools?minutes=15").Body.String())
	assert.Contains(t, bar, `<span class="toolbar-lbl">Broken</span> <span class="toolbar-val bad">Could not load</span>`)
	assert.False(t, strings.Contains(bar, "the backend is down")) // the cause is logged, never shown
	assert.Contains(t, bar, `<span class="toolbar-lbl">Refused</span> <span class="toolbar-val bad">Not for you.</span>`)
	assert.Contains(t, bar, ">Go</button>")
	assert.Contains(t, bar, `<span class="toolbar-lbl">Minutes</span> <span class="toolbar-val">15</span>`)
}

// A toolbar is a leaf of its own, and its forms are not: a form that says it is not final has the whole toolbar asked for again, by the shortest of them.
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
	page := webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: []webui.ToolbarItem{going(4 * time.Second), other}}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/tools?minutes=15").Body.String()
	root := body[strings.Index(body, `<div class="toolbar"`):]
	root = root[:strings.Index(root, ">")]
	assert.Contains(t, root, `data-leaf="p"`)
	assert.Contains(t, root, `data-refresh-after="4"`)
	// A toolbar has little room, and a note in the row would move what is in it: it says nothing that is seen.
	assert.False(t, strings.Contains(body, "refresh-note"))
	assert.Equal(t, strings.Count(body, "data-refresh-after="), 1)

	// Asked for alone, as script does, the toolbar answers with its own root first, which carries it.
	rec := serveLeaf(h, "/admin/tools?minutes=15", "p")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.True(t, strings.HasPrefix(rec.Body.String(), `<div class="toolbar" data-toolbar data-leaf="p" data-refresh-after="4">`))

	// A toolbar of forms that are final says nothing of it.
	final := serve(webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: []webui.ToolbarItem{settings(minutesInput)}}}}}.MustCompile("/admin"), http.MethodGet, "/admin/tools").Body.String()
	assert.False(t, strings.Contains(final, "data-refresh-after"))
	assert.Contains(t, final, `<div class="toolbar" data-toolbar data-leaf="p">`)
}

func TestAnActionOfAToolbarReadsThePagesArgumentsFromTheRequest(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	rec := post(s.h, "/admin/tools?minutes=15&site=north", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/tools?minutes=15&site=north")
	assert.Equal(t, flashOf(rec), "oSynced")
	assert.DeepEqual(t, s.ran, []toolArgs{{Minutes: 15, Site: "north"}})

	// What is not a button is not understood, and nothing is done.
	for _, act := range []string{"0", "2", "9", "-1", "x", ""} {
		rec = post(s.h, "/admin/tools", url.Values{"_leaf": {"p.0"}, "_act": {act}})
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.Equal(t, flashOf(rec), "eThat request was not understood.")
	}
	assert.Equal(t, len(s.ran), 1)
}

func TestAToolbarWithControlsAtOneEdgeHasTheGapOnTheOtherSide(t *testing.T) {
	t.Parallel()

	item := webui.Action[webui.NoArgs]{Label: "Go", Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }}
	gap := `<span class="toolbar-gap" aria-hidden="true"></span>`
	pageOf := func(tb webui.Toolbar) string {
		h := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: tb}}}.MustCompile("/admin")
		return serve(h, http.MethodGet, "/admin/tools").Body.String()
	}
	both := pageOf(webui.Toolbar{Left: []webui.ToolbarItem{item}, Right: []webui.ToolbarItem{item}})
	assert.Equal(t, strings.Count(both, gap), 1)
	onlyRight := pageOf(webui.Toolbar{Right: []webui.ToolbarItem{item}})
	assert.True(t, strings.Index(onlyRight, gap) < strings.Index(onlyRight, ">Go</button>")) // the right ones start with it
	onlyLeft := pageOf(webui.Toolbar{Left: []webui.ToolbarItem{item}})
	assert.Equal(t, strings.Count(onlyLeft, gap), 0)
}

// The Guard that leaves a button out is the one that refuses the request, and so is the one that disables it.
func TestAToolbarButtonThatIsGuardedIsLeftOutOrDisabledAndRefused(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	s.deny = errors.New("requires the editor role")
	body := serve(s.h, http.MethodGet, "/admin/tools").Body.String()
	assert.False(t, strings.Contains(body, "Sync now"))
	assert.False(t, strings.Contains(body, "requires the editor role")) // nothing says that there was a button
	assert.Contains(t, body, `href="/admin/device/new">Create device</a>`)
	assert.False(t, strings.Contains(body, `<form action="/admin/tools" method="post"`)) // and with no button there is nothing to post

	rec := post(s.h, "/admin/tools", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "erequires the editor role")
	assert.Equal(t, len(s.ran), 0)

	s.deny = webui.Disabled("requires the editor role")
	body = serve(s.h, http.MethodGet, "/admin/tools").Body.String()
	assert.Contains(t, body, `<span class="gate" data-guard="requires the editor role" tabindex="0"><button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1" disabled>Sync now</button><span class="sr-only">Not available: requires the editor role</span></span>`)

	rec = post(s.h, "/admin/tools", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
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
	page := webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: []webui.ToolbarItem{settings(team)}}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	assert.Contains(t, serve(h, http.MethodGet, "/admin/tools?site=blue").Body.String(), `</span>Blue team</span>`)

	s.err = errors.New("the database is down")
	body := serve(h, http.MethodGet, "/admin/tools").Body.String()
	assert.Contains(t, body, "Could not load")
}

func TestAToolbarIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(body webui.PageBody) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: body}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	run := func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }
	assert.Contains(t, problems(webui.Toolbar{}), "a Toolbar has no controls")
	assert.Contains(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Run: run}}}), "Toolbar.Left[0] has no Label")
	assert.Contains(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Go"}}}), "Toolbar.Left[0] has no Run")
	assert.Contains(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{
		webui.Link[webui.NoArgs, newArgs]{Page: newDevice, Args: func(context.Context, webui.NoArgs) newArgs { return newArgs{} }},
	}}), "Toolbar.Left[0] has no Label")
	// A value to read needs no Store.
	_, err := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: []webui.ToolbarItem{
		settings(webui.String[toolState]{Label: "Site", Load: func(a toolState) string { return a.Site }}),
	}}}}}.Compile("/admin")
	assert.NoError(t, err)
	// Where a control is said to be follows the group it is in.
	assert.Contains(t, problems(webui.Toolbar{Left: []webui.ToolbarItem{settings(siteInput)}, Right: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Go"}}}), "Toolbar.Right[0] has no Run")
}

// A Form in a toolbar is only a group of values to read: what would make it an input, a panel, or something that cannot be one line is refused, with what to do instead.
func TestAFormInAToolbarIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(items ...webui.ToolbarItem) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: items}}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	ok := settings(minutesInput)
	_, err := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{Left: []webui.ToolbarItem{ok}}}}}.Compile("/admin")
	assert.NoError(t, err)

	noTitle := ok
	noTitle.Title = ""
	assert.Contains(t, problems(noTitle), "Toolbar.Left[0] is a Form with no Title")

	noLoad := ok
	noLoad.Load = nil
	assert.Contains(t, problems(noLoad), "Toolbar.Left[0] is a Form that has no Load")

	noFields := ok
	noFields.Fields = nil
	assert.Contains(t, problems(noFields), "Toolbar.Left[0] is a Form with no Fields")

	withSubmit := ok
	withSubmit.Submit = webui.Action[toolState]{Run: func(context.Context, toolState) (webui.Outcome, error) { return webui.Outcome{}, nil }}
	assert.Contains(t, problems(withSubmit), "Toolbar.Left[0] is a Form with a Submit")

	// The labels of its fields are one list, across the forms of the toolbar.
	assert.Contains(t, problems(settings(siteInput), settings(siteInput)), `label "Site" is used twice`)

	text := func(f webui.String[toolState]) webui.Form[toolState] { return settings(f) }
	load := func(a toolState) string { return strconv.Itoa(a.Minutes) }
	store := func(*toolState, string) {}
	assert.Contains(t, problems(text(webui.String[toolState]{Label: "Key", Secret: true, Load: load, Store: store})), "is a Secret in a toolbar")
	assert.Contains(t, problems(text(webui.String[toolState]{Label: "Body", Multiline: true, Load: load, Store: store})), "is Multiline in a toolbar")
	assert.Contains(t, problems(settings(webui.File[toolState]{Label: "Logo", MaxSize: 1 << 20, Load: func(toolState) *webui.FileInfo { return nil }, Store: func(*toolState, *webui.Upload) {}})), "is a File in a toolbar")
}
