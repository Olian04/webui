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

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// toolArgs are the arguments of the page that the toolbars are on.
type toolArgs struct {
	Minutes int
	Site    string
	Tags    []string
}

type newArgs struct{ From string }

var newDevice = webui.Page[newArgs]{Path: "/device/new", Body: webui.Stack{}}

var (
	minutesInput = webui.Int[toolArgs]{ // the same accessors a form would use: their Store and Rules do nothing in a toolbar
		Label: "Minutes", Load: func(a toolArgs) int { return a.Minutes }, Store: func(a *toolArgs, v int) { a.Minutes = v },
		Rules: webui.NumberRules[int]{Min: ptr(1), Max: ptr(1440)},
	}
	siteInput = webui.Select[toolArgs]{
		Label: "Site", Load: func(a toolArgs) []string { return []string{a.Site} },
		Store:   func(a *toolArgs, v []string) { a.Site = strings.Join(v, "") },
		Options: []webui.Option{{Value: "north", Label: "North"}, {Value: "south"}},
	}
	tagsInput = webui.Select[toolArgs]{
		Label: "Tags", Multi: true, Load: func(a toolArgs) []string { return a.Tags }, Store: func(a *toolArgs, v []string) { a.Tags = v },
		Options: []webui.Option{{Value: "edge"}, {Value: "gpu"}},
	}
)

type toolShop struct {
	h    http.Handler
	mu   sync.Mutex
	ran  []toolArgs
	deny error
}

func newToolShop(items ...webui.ToolbarItem[toolArgs]) *toolShop {
	s := &toolShop{}
	if items == nil {
		items = []webui.ToolbarItem[toolArgs]{
			webui.Link[toolArgs, newArgs]{
				Label: "Create device", Role: webui.RolePrimary, Page: newDevice,
				Args: func(_ context.Context, a toolArgs) newArgs { return newArgs{From: a.Site} },
			},
			webui.Action[toolArgs]{
				Label: "Sync now",
				Guard: func(context.Context, toolArgs) error { return s.deny },
				Run: func(_ context.Context, a toolArgs) (webui.Outcome, error) {
					s.mu.Lock()
					s.ran = append(s.ran, a)
					s.mu.Unlock()
					return webui.Success("Synced"), nil
				},
			},
			minutesInput, siteInput, tagsInput,
		}
	}
	page := webui.Page[toolArgs]{Path: "/tools", Body: webui.Stack{
		webui.Toolbar[toolArgs]{Left: items[:2], Right: items[2:]},
		webui.Table[Device]{
			Title: "Devices",
			Rows: func(context.Context) ([]Device, error) {
				var all []Device
				for i := range 30 {
					all = append(all, Device{Id: string(rune('a' + i%26)), Count: i})
				}
				return all, nil
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

func TestAToolbarIsARowOfButtonsAndValuesOfThePagesArguments(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	body := serve(s.h, http.MethodGet, "/admin/tools?minutes=15&site=north&tags=edge&tags=gpu").Body.String()
	bar := body[strings.Index(body, `<div class="toolbar"`):strings.Index(body, `<div class="panel grows"`)]

	// A link is a real anchor, to the page its Args build from the page's arguments.
	assert.Contains(t, bar, `<a class="btn btn-primary btn-sm" href="/admin/device/new?from=north">Create device</a>`)
	// An action is a button that posts, by its position.
	assert.Contains(t, bar, `<button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1">Sync now</button>`)
	// An accessor is its label and its value as it is read: a number, and a select as its badges. Nothing in it can be changed.
	assert.Contains(t, bar, `<span class="toolbar-value"><span class="toolbar-lbl">Minutes</span> <span class="toolbar-val">15</span></span>`)
	assert.Contains(t, bar, `<span class="toolbar-lbl">Site</span> <span class="toolbar-val"><span class="badges"><span class="badge badge-mute"><span class="dot" aria-hidden="true"></span>North</span></span></span>`)
	assert.Contains(t, bar, `>edge</span><span class="badge badge-mute"><span class="dot" aria-hidden="true"></span>gpu</span>`)
	for _, input := range []string{"<input type=\"number\"", "<select", "<textarea", "toolbar-apply", "toolbar-args", "data-toolbar-form", "_toolbar", `method="get"`, `<label`} {
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
func TestAToolbarOfLinksAndValuesHasNoForm(t *testing.T) {
	t.Parallel()

	s := newToolShop(
		webui.Link[toolArgs, newArgs]{Label: "Create device", Page: newDevice, Args: func(context.Context, toolArgs) newArgs { return newArgs{} }},
		minutesInput, siteInput,
	)
	body := serve(s.h, http.MethodGet, "/admin/tools?minutes=15").Body.String()
	bar := body[strings.Index(body, `<div class="toolbar"`):strings.Index(body, `<div class="panel grows"`)]
	assert.False(t, strings.Contains(bar, "<form"))
	assert.False(t, strings.Contains(bar, "_leaf"))
	assert.Contains(t, bar, `<span class="toolbar-lbl">Minutes</span> <span class="toolbar-val">15</span>`)
	assert.Contains(t, bar, `<a class="btn btn-secondary btn-sm" href="/admin/device/new">Create device</a>`)
}

// The accessor is the one a form uses: its Store is ignored, as a table ignores it, and an address that used to apply an input is a page.
func TestAToolbarShowsAnAccessorReadOnlyWhateverItsStoreAndHasNoApplyingAddress(t *testing.T) {
	t.Parallel()

	withStore := webui.Int[toolArgs]{Label: "Minutes", Load: func(a toolArgs) int { return a.Minutes }, Store: func(a *toolArgs, v int) { a.Minutes = v }}
	without := webui.Int[toolArgs]{Label: "Minutes", Load: func(a toolArgs) int { return a.Minutes }}
	page := func(item webui.ToolbarItem[toolArgs]) http.Handler {
		return webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{item}}}}}.MustCompile("/admin")
	}
	a := serve(page(withStore), http.MethodGet, "/admin/tools?minutes=15").Body.String()
	b := serve(page(without), http.MethodGet, "/admin/tools?minutes=15").Body.String()
	pick := func(s string) string {
		i := strings.Index(s, `<div class="toolbar"`)
		return s[i : i+strings.Index(s[i:], `</div>`)]
	}
	assert.Equal(t, pick(a), pick(b))

	// The old address of an applied input is now only a page with an argument it does not know: it is shown, not redirected.
	rec := serve(page(withStore), http.MethodGet, "/admin/tools?_toolbar=p&t0=30")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.Contains(t, rec.Body.String(), `<span class="toolbar-lbl">Minutes</span> <span class="toolbar-val">0</span>`)
}

func TestAToolbarValueIsAsAReadOnlyFieldShowsItAndMayHaveHelp(t *testing.T) {
	t.Parallel()

	type args struct { // arguments are plain values: an absent one is derived from them
		Size int
		Name string
	}
	h := webui.App{Pages: webui.Pages{webui.Page[args]{Path: "/t", Body: webui.Toolbar[args]{Left: []webui.ToolbarItem[args]{
		webui.Bool[args]{Label: "Paused", Load: func(a args) *bool {
			if a.Size == 0 {
				return nil // not set
			}
			yes := a.Size > 5
			return &yes
		}},
		webui.Int[args]{Label: "Size", Help: "How much is kept.", Load: func(a args) int { return a.Size }, Format: func(n int) string { return strconv.Itoa(n) + " KiB" }},
		webui.String[args]{Label: "Name", Load: func(a args) string { return a.Name }},
		webui.URL[args]{Label: "Docs", Text: "Read", Load: func(args) string { return "https://example.com/docs" }},
	}}}}}.MustCompile("/admin")

	bar := func(target string) string {
		body := serve(h, http.MethodGet, target).Body.String()
		return body[strings.Index(body, `<div class="toolbar"`):strings.Index(body, `</main>`)]
	}
	// Absent is a dash, and a value is as a read-only form field shows it: a Format, a link.
	empty := bar("/admin/t")
	assert.Contains(t, empty, `<span class="toolbar-lbl">Paused</span> <span class="toolbar-val"><span class="dim" aria-hidden="true">—</span>`)
	assert.Contains(t, empty, `<span class="toolbar-lbl">Name</span> <span class="toolbar-val"><span class="dim" aria-hidden="true">—</span>`)
	assert.Contains(t, empty, `<span class="toolbar-val">0 KiB</span>`)
	assert.Contains(t, empty, `href="https://example.com/docs"`)
	// What the help says is in the popover beside the label.
	assert.Contains(t, empty, `<span class="tip" tabindex="0" role="note" aria-label="Help: How much is kept.">`)
	assert.False(t, strings.Contains(empty, "<form"))
}

func TestAToolbarWithControlsAtOneEdgeHasTheGapOnTheOtherSide(t *testing.T) {
	t.Parallel()

	run := func(context.Context, toolArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }
	item := webui.Action[toolArgs]{Label: "Go", Run: run}
	gap := `<span class="toolbar-gap" aria-hidden="true"></span>`
	pageOf := func(tb webui.Toolbar[toolArgs]) string {
		h := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: tb}}}.MustCompile("/admin")
		return serve(h, http.MethodGet, "/admin/tools").Body.String()
	}
	both := pageOf(webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{item}, Right: []webui.ToolbarItem[toolArgs]{item}})
	assert.Equal(t, strings.Count(both, gap), 1)
	onlyRight := pageOf(webui.Toolbar[toolArgs]{Right: []webui.ToolbarItem[toolArgs]{item}})
	assert.True(t, strings.Index(onlyRight, gap) < strings.Index(onlyRight, ">Go</button>")) // the right ones start with it
	onlyLeft := pageOf(webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{item}})
	assert.Equal(t, strings.Count(onlyLeft, gap), 0)
}

func TestAnActionOfAToolbarIsDoneToThePagesArguments(t *testing.T) {
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
	assert.Contains(t, body, `<span class="gate" data-guard="requires the editor role"><button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1" disabled>Sync now</button></span>`)

	rec = post(s.h, "/admin/tools", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, flashOf(rec), "erequires the editor role")
	assert.Equal(t, len(s.ran), 0)
}

// A select of a toolbar can read its options when the request is answered: they say what its badge reads.
func TestASelectInAToolbarMayReadItsOptions(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red"}, webui.Option{Value: "blue", Label: "Blue team"})
	team := webui.Select[toolArgs]{
		Label: "Team", Load: func(a toolArgs) []string { return []string{a.Site} }, LoadOptions: s.load,
	}
	page := webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{team}}}
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
	run := func(context.Context, toolArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{}), "a Toolbar has no controls")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{webui.Action[toolArgs]{Run: run}}}), "Toolbar.Left[0] has no Label")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{webui.Action[toolArgs]{Label: "Go"}}}), "Toolbar.Left[0] has no Run")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{
		webui.Link[toolArgs, newArgs]{Page: newDevice, Args: func(context.Context, toolArgs) newArgs { return newArgs{} }},
	}}), "Toolbar.Left[0] has no Label")
	// A value to read needs no Store.
	_, err := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{
		webui.String[toolArgs]{Label: "Site", Load: func(a toolArgs) string { return a.Site }},
	}}}}}.Compile("/admin")
	assert.NoError(t, err)
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{siteInput, siteInput}}), `label "Site" is used twice`)
	assert.Contains(t, problems(webui.Toolbar[webui.NoArgs]{Right: []webui.ToolbarItem[webui.NoArgs]{webui.Action[webui.NoArgs]{Label: "Go", Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }}}}),
		"A Toolbar is about the arguments of the page it is on")
	// Where a control is said to be follows the group it is in.
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Left: []webui.ToolbarItem[toolArgs]{siteInput}, Right: []webui.ToolbarItem[toolArgs]{webui.Action[toolArgs]{Label: "Go"}}}), "Toolbar.Right[0] has no Run")
}
