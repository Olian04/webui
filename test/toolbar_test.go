package webui_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	minutesInput = webui.Int[toolArgs]{
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
		webui.Toolbar[toolArgs]{Items: items},
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

func TestAToolbarIsARowOfButtonsAndInputsForThePagesArguments(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	body := serve(s.h, http.MethodGet, "/admin/tools?minutes=15&site=north").Body.String()

	// A link is a real anchor, to the page its Args build from the page's arguments.
	assert.Contains(t, body, `<a class="btn btn-primary btn-sm" href="/admin/device/new?from=north">Create device</a>`)
	// An action is a button that posts, by its position.
	assert.Contains(t, body, `<button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1">Sync now</button>`)
	// An input shows the page's argument, and belongs to the form that applies it.
	assert.Contains(t, body, `name="t2" type="number" value="15" min="1" max="1440" step="1"`)
	assert.Contains(t, body, `form="p-0-args"`)
	assert.Contains(t, body, `<option value="north" data-badge="badge-mute" selected>North</option>`)
	// The form that applies them carries the rest of the address, and none of the inputs' own.
	form := body[strings.Index(body, `<form id="p-0-args"`):]
	form = form[:strings.Index(form, `</form>`)]
	assert.Contains(t, form, `method="get" action="/admin/tools"`)
	assert.Contains(t, form, `name="_toolbar" value="p.0"`)
	assert.Contains(t, form, `name="minutes" value="15"`)
	assert.Contains(t, form, `name="site" value="north"`)
	assert.Contains(t, body, `class="btn btn-ghost btn-sm toolbar-apply" type="submit" form="p-0-args">Apply</button>`)
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

// The Guard that disables a button is the one that refuses the request.
func TestAToolbarButtonThatIsGuardedIsDisabledWithItsReasonAndRefused(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	s.deny = errors.New("requires the editor role")
	body := serve(s.h, http.MethodGet, "/admin/tools").Body.String()
	assert.Contains(t, body, `<span class="gate" data-guard="requires the editor role"><button class="btn btn-secondary btn-sm" type="submit" name="_act" value="1" disabled>Sync now</button></span>`)

	rec := post(s.h, "/admin/tools", url.Values{"_leaf": {"p.0"}, "_act": {"1"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "erequires the editor role")
	assert.Equal(t, len(s.ran), 0)
}

// An input is applied to the page's arguments, and the visitor is sent to the page they make, with the rest of the
// address kept, a table's sort among it, and its first page.
func TestAnInputOfAToolbarGivesThePageTheArgumentsItMakes(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	where := func(target string) string {
		rec := serve(s.h, http.MethodGet, target)
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		return rec.Header().Get("Location")
	}

	assert.Equal(t, where("/admin/tools?_toolbar=p.0&t2=30"), "/admin/tools?minutes=30")
	// What the other arguments were is kept, and what the inputs did not name.
	assert.Equal(t, where("/admin/tools?site=south&_toolbar=p.0&t2=30&t3=north"), "/admin/tools?minutes=30&site=north")
	// A select of several is one value for each.
	assert.Equal(t, where("/admin/tools?minutes=5&_toolbar=p.0&t2=5&t4=edge&t4=gpu"), "/admin/tools?minutes=5&tags=edge&tags=gpu")
	// An input that is emptied sets its argument to nothing, and so does a select with nothing chosen.
	assert.Equal(t, where("/admin/tools?minutes=5&site=north&tags=gpu&_toolbar=p.0&t2="), "/admin/tools")
	// The sort and the filters of a table stay, and its page does not.
	assert.Equal(t, where("/admin/tools?devices.sort=id&devices.filter.id=a&_toolbar=p.0&t2=30"), "/admin/tools?devices.filter.id=a&devices.sort=id&minutes=30")
	assert.Equal(t, where("/admin/tools?devices.offset=25&devices.sort=id&_toolbar=p.0&t2=30"), "/admin/tools?devices.offset=25&devices.sort=id&minutes=30") // the form drops it, as a hand-made address may keep it
}

// What the rules refuse is applied in none of the inputs, and the visitor is where they were, told why.
func TestInputsOfAToolbarAreHeldToTheirRules(t *testing.T) {
	t.Parallel()

	s := newToolShop()
	for name, target := range map[string]string{
		"too many":  "/admin/tools?minutes=5&_toolbar=p.0&t2=9999",
		"too few":   "/admin/tools?minutes=5&_toolbar=p.0&t2=0",
		"no number": "/admin/tools?minutes=5&_toolbar=p.0&t2=abc",
		"no option": "/admin/tools?minutes=5&_toolbar=p.0&t2=30&t3=west",
		"two":       "/admin/tools?minutes=5&_toolbar=p.0&t2=30&t3=north&t3=south",
	} {
		rec := serve(s.h, http.MethodGet, target)
		assert.Equal(t, rec.Code, http.StatusSeeOther)
		assert.Equal(t, rec.Header().Get("Location"), "/admin/tools?minutes=5")
		if flash := flashOf(rec); !strings.HasPrefix(flash, "e") {
			t.Errorf("%s: flash %q, want an error", name, flash)
		}
	}
	rec := serve(s.h, http.MethodGet, "/admin/tools?minutes=5&_toolbar=p.0&t2=9999")
	assert.Equal(t, flashOf(rec), "eMinutes: Must be at most 1440.")
	rec = serve(s.h, http.MethodGet, "/admin/tools?minutes=5&_toolbar=p.0&t2=30&t3=west")
	assert.Equal(t, flashOf(rec), "eSite: Choose from the options offered.")
}

// A path argument is an input like any other: the address is built from the arguments, not from the form.
func TestAnInputOfAToolbarCanChangeAPathArgument(t *testing.T) {
	t.Parallel()

	type pathArgs struct{ Id string }
	page := webui.Page[pathArgs]{Path: "/thing/{id}", Body: webui.Toolbar[pathArgs]{Items: []webui.ToolbarItem[pathArgs]{
		webui.String[pathArgs]{Label: "Id", Load: func(a pathArgs) string { return a.Id }, Store: func(a *pathArgs, v string) { a.Id = v }},
	}}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")
	rec := serve(h, http.MethodGet, "/admin/thing/a?_toolbar=p&t0=b+c")
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/thing/b%20c")
}

// A select of a toolbar can read its options when the request is answered.
func TestASelectInAToolbarMayReadItsOptions(t *testing.T) {
	t.Parallel()

	s := &teams{}
	s.set(webui.Option{Value: "red"}, webui.Option{Value: "blue", Label: "Blue team"})
	team := webui.Select[toolArgs]{
		Label: "Team", Load: func(a toolArgs) []string { return []string{a.Site} },
		Store: func(a *toolArgs, v []string) { a.Site = strings.Join(v, "") }, LoadOptions: s.load,
	}
	page := webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar[toolArgs]{Items: []webui.ToolbarItem[toolArgs]{team}}}
	h := webui.App{Pages: webui.Pages{page}}.MustCompile("/admin")

	assert.Contains(t, serve(h, http.MethodGet, "/admin/tools").Body.String(), `>Blue team</option>`)
	assert.Equal(t, serve(h, http.MethodGet, "/admin/tools?_toolbar=p&t0=green").Header().Get("Location"), "/admin/tools") // refused
	s.set(webui.Option{Value: "green"})
	assert.Equal(t, serve(h, http.MethodGet, "/admin/tools?_toolbar=p&t0=green").Header().Get("Location"), "/admin/tools?site=green")

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
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{}), "a Toolbar has no Items")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Items: []webui.ToolbarItem[toolArgs]{webui.Action[toolArgs]{Run: run}}}), "Toolbar.Items[0] has no Label")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Items: []webui.ToolbarItem[toolArgs]{webui.Action[toolArgs]{Label: "Go"}}}), "Toolbar.Items[0] has no Run")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Items: []webui.ToolbarItem[toolArgs]{
		webui.Link[toolArgs, newArgs]{Page: newDevice, Args: func(context.Context, toolArgs) newArgs { return newArgs{} }},
	}}), "Toolbar.Items[0] has no Label")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Items: []webui.ToolbarItem[toolArgs]{
		webui.String[toolArgs]{Label: "Site", Load: func(a toolArgs) string { return a.Site }},
	}}), "Toolbar.Items[0] has no Store")
	assert.Contains(t, problems(webui.Toolbar[toolArgs]{Items: []webui.ToolbarItem[toolArgs]{siteInput, siteInput}}), `label "Site" is used twice`)
	assert.Contains(t, problems(webui.Toolbar[webui.NoArgs]{Items: []webui.ToolbarItem[webui.NoArgs]{webui.Action[webui.NoArgs]{Label: "Go", Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Outcome{}, nil }}}}),
		"A Toolbar is about the arguments of the page it is on")
}
