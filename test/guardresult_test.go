package webui_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// What a Guard returns decides how its control is drawn, and the same Guard refuses the request: an error leaves the control out,
// and Disabled draws it disabled, with its message.

func noop(string) func(context.Context, parcel) (webui.Outcome, error) {
	return func(context.Context, parcel) (webui.Outcome, error) { return webui.Success("done"), nil }
}

// guardedParcels is a table of the two parcels with the actions given, on a page of its own.
func guardedParcels(logger *slog.Logger, actions []webui.Action[parcel], bulk []webui.Action[[]parcel], click webui.RowClick[parcel]) http.Handler {
	table := webui.Table[parcel]{
		Rows: func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }, Key: func(p parcel) string { return p.Name },
		Columns: []webui.Accessor[parcel]{parcelName}, Actions: actions, BulkActions: bulk, RowClick: click,
	}
	return webui.App{Logger: logger, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: table}}}.MustCompile("/admin")
}

func rowAction(label string, guard func(context.Context, parcel) error) webui.Action[parcel] {
	return webui.Action[parcel]{Label: label, Guard: guard, Run: noop(label)}
}

func TestADisabledRowActionIsDrawnDisabledWithItsMessageAndStillRefused(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", func(_ context.Context, p parcel) error {
		if p.Name == "b" {
			return webui.Disabled("Already dropped")
		}
		return nil
	})}, nil, nil)

	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `name="_act" value="row:0:a">Drop</button>`)
	assert.Contains(t, body, `<span class="gate" data-guard="Already dropped" tabindex="0"><button class="btn btn-secondary btn-sm" type="submit" name="_act" value="row:0:b" disabled>Drop</button><span class="sr-only">Not available: Already dropped</span></span>`)

	rec := post(h, "/admin/t", formValues("_act", "row:0:b"))
	assert.Equal(t, flashOf(rec), "eAlready dropped") // the message is what is told to the one who asks by hand
}

// A Disabled that is wrapped is still one, and one with nothing to say still says something.
func TestADisabledThatIsWrappedOrEmptyIsStillDisabled(t *testing.T) {
	t.Parallel()

	guard := func(err error) func(context.Context, parcel) error {
		return func(context.Context, parcel) error { return err }
	}
	wrapped := serve(guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", guard(fmt.Errorf("not now: %w", webui.Disabled("Busy"))))}, nil, nil), http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, wrapped, `data-guard="Busy"`)

	empty := serve(guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", guard(webui.Disabled("")))}, nil, nil), http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, empty, `data-guard="Not allowed."`)
}

// An error with no text is still a refusal: the control is not there, as for any other error, and the request is refused.
func TestAnErrorWithNoTextHidesTheControlAndRefusesTheRequest(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", func(context.Context, parcel) error { return errors.New("") })}, nil, nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.False(t, strings.Contains(body, "Drop"))
	assert.False(t, strings.Contains(body, `class="actions"`))

	rec := post(h, "/admin/t", formValues("_act", "row:0:a"))
	assert.Equal(t, flashOf(rec), "eNot allowed.")
}

// A form that may not be saved is read, whatever the error says, and it is so for an error with no text too.
func TestAFormWhoseSubmitIsRefusedIsReadOnlyWhateverTheErrorSays(t *testing.T) {
	t.Parallel()

	for name, err := range map[string]error{"error": errors.New("no"), "empty": errors.New(""), "disabled": webui.Disabled("Locked")} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := newShop()
			s.subGate = err
			body := serve(s.h, http.MethodGet, "/admin/device/dev1").Body.String()
			assert.False(t, strings.Contains(body, ">Save</button>"))
			assert.False(t, strings.Contains(body, `name="f1"`)) // the IP is shown and not offered
		})
	}
}

// Skipping an action that is not drawn does not renumber the others: a button posts the position its action was declared at.
func TestAnActionThatIsNotDrawnLeavesTheOthersTheirPositions(t *testing.T) {
	t.Parallel()

	ran := map[string]int{}
	var mu sync.Mutex
	do := func(name string) func(context.Context, parcel) (webui.Outcome, error) {
		return func(context.Context, parcel) (webui.Outcome, error) {
			mu.Lock()
			ran[name]++
			mu.Unlock()
			return webui.Success(name), nil
		}
	}
	actions := []webui.Action[parcel]{
		{Label: "First", Confirm: "Sure?", Guard: func(context.Context, parcel) error { return errors.New("no") }, Run: do("first")},
		{Label: "Second", Confirm: "Sure?", Run: do("second")},
	}
	h := guardedParcels(nil, actions, nil, nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.False(t, strings.Contains(body, ">First<"))
	assert.Contains(t, body, `value="row:1:a">Second</button>`)
	assert.Contains(t, body, `id="confirm-p-r0a1"`) // the dialog's id has the position too

	post(h, "/admin/t", formValues("_act", "row:1:a"))
	post(h, "/admin/t", formValues("_act", "row:0:a")) // refused, though hidden
	assert.DeepEqual(t, ran, map[string]int{"second": 1})
}

// A bulk action's Guard is asked of each row as a selection of that one, and never of nothing; the button is drawn if it accepts any.
func TestABulkActionIsDrawnIfItsGuardAcceptsAnyOneRow(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var asked []int
	bulk := func(accept func(parcel) error) []webui.Action[[]parcel] {
		return []webui.Action[[]parcel]{{
			Label: "Bag",
			Guard: func(_ context.Context, ps []parcel) error {
				mu.Lock()
				asked = append(asked, len(ps))
				mu.Unlock()
				return accept(ps[0]) // a panic, were it ever empty
			},
			Run: func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
		}}
	}

	// One row that it accepts is enough.
	h := guardedParcels(nil, nil, bulk(func(p parcel) error {
		if p.Name == "b" {
			return nil
		}
		return errors.New("no")
	}), nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `value="bulk:0">Bag</button>`)
	assert.Contains(t, body, `class="pick"`)
	assert.False(t, strings.Contains(body, `class="gate"`))
	for _, n := range asked {
		assert.Equal(t, n, 1)
	}

	// None, and an error that says nothing of how to draw it: there is no button, and so nothing to select the rows for.
	h = guardedParcels(nil, nil, bulk(func(parcel) error { return errors.New("no") }), nil)
	body = serve(h, http.MethodGet, "/admin/t").Body.String()
	for _, absent := range []string{"Bag", `class="pick"`, `class="actionbar"`, `name="_sel"`, `data-pickall`, `<form action="/admin/t"`} {
		assert.False(t, strings.Contains(body, absent))
	}
	assert.Contains(t, body, `<table aria-label="Table"><colgroup><col style="width:100.00%;"> </colgroup>`)

	// None, and the first of them that says Disabled: the button is disabled, with that.
	h = guardedParcels(nil, nil, bulk(func(p parcel) error {
		if p.Name == "a" {
			return errors.New("no")
		}
		return webui.Disabled("Nothing here can be bagged")
	}), nil)
	body = serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `<span class="gate" data-guard="Nothing here can be bagged" tabindex="0">`)
	assert.Contains(t, body, `class="pick"`)

	// And the Guard that decided the button also refuses the request, which names its own selection.
	rec := post(h, "/admin/t", formValues("_act", "bulk:0", "_sel", "a"))
	assert.Equal(t, flashOf(rec), "eno")
}

// A table with no rows has nothing to select, and a bulk action is not drawn for it.
func TestATableWithNoRowsDrawsNoBulkAction(t *testing.T) {
	t.Parallel()

	table := webui.Table[parcel]{
		Rows: func(context.Context) ([]parcel, webui.Loaded) { return nil, webui.Ready() }, Key: func(p parcel) string { return p.Name },
		Columns: []webui.Accessor[parcel]{parcelName},
		BulkActions: []webui.Action[[]parcel]{{
			Label: "Bag",
			Run:   func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: table}}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.False(t, strings.Contains(body, "Bag"))
	assert.Contains(t, body, "Nothing to show")
}

// The column of actions is there when a row of the window has an action drawn, and an empty cell is what a row without one has.
func TestTheColumnOfActionsComesAndGoesWithTheRowsThatHaveOne(t *testing.T) {
	t.Parallel()

	var open atomic.Bool
	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", func(_ context.Context, p parcel) error {
		if open.Load() && p.Name == "a" {
			return nil
		}
		return errors.New("no")
	})}, nil, nil)

	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.False(t, strings.Contains(body, `class="actions"`))
	assert.False(t, strings.Contains(body, `<form action="/admin/t"`)) // nothing in the table posts

	open.Store(true)
	body = serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Equal(t, strings.Count(body, `<td class="actions">`), 2)
	assert.Contains(t, body, `<td class="actions"></td>`) // b has none
}

// A Guard that panics is the application's bug: it leaves the control out, and the panic is logged, where it would otherwise end
// the page in a server error.
func TestAGuardThatPanicsLeavesItsControlOutAndIsLogged(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	boom := func(context.Context, parcel) error { panic("the guard fell over") }
	h := guardedParcels(slog.New(slog.NewTextHandler(&logs, nil)),
		[]webui.Action[parcel]{rowAction("Drop", boom)},
		[]webui.Action[[]parcel]{{
			Label: "Bag", Guard: func(context.Context, []parcel) error { panic("the bulk guard fell over") },
			Run: func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
		}},
		webui.Action[parcel]{Guard: boom, Run: noop("click")})
	rec := serve(h, http.MethodGet, "/admin/t")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()
	assert.False(t, strings.Contains(body, "Drop"))
	assert.False(t, strings.Contains(body, "Bag"))
	assert.Contains(t, logs.String(), "a Guard panicked")
	assert.Contains(t, logs.String(), "the guard fell over")
	assert.Contains(t, logs.String(), "the bulk guard fell over")
}

// A toolbar that is left with nothing to draw is not there, and one that is left with some keeps the gap that puts the right-hand
// controls at the right edge, and the position that each button posts.
func TestAToolbarOfRefusedButtonsIsNotThereAndTheOthersKeepTheirPlaces(t *testing.T) {
	t.Parallel()

	var open atomic.Bool
	refused := func(context.Context, webui.NoArgs) error {
		if open.Load() {
			return nil
		}
		return errors.New("no")
	}
	run := func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Success("done"), nil }
	h := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{
		Left:  []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "One", Guard: refused, Run: run}},
		Right: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Two", Run: run}},
	}}}}.MustCompile("/admin")

	body := serve(h, http.MethodGet, "/admin/tools").Body.String()
	assert.False(t, strings.Contains(body, ">One<"))
	assert.Contains(t, body, `<span class="toolbar-gap" aria-hidden="true"></span>`) // Two is still at the right
	assert.Contains(t, body, `name="_act" value="1">Two</button>`)                   // and is still the second

	// With every button refused, no toolbar is drawn: not an empty row, and not a form.
	h = webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", Body: webui.Toolbar{
		Left: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "One", Guard: refused, Run: run}},
	}}}}.MustCompile("/admin")
	body = serve(h, http.MethodGet, "/admin/tools").Body.String()
	assert.False(t, strings.Contains(body, "data-toolbar"))
	assert.False(t, strings.Contains(body, `name="_leaf"`))
}

// A link is drawn when the page it leads to would open for the visitor, and is not when that page's Guard refuses them.
func TestALinkToAPageThatRefusesTheVisitorIsNotDrawn(t *testing.T) {
	t.Parallel()

	var open atomic.Bool
	dest := webui.Page[webui.NoArgs]{Path: "/dest", Body: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error {
		if open.Load() {
			return nil
		}
		return errors.New("editors only")
	}}
	args := func(context.Context, parcel) webui.NoArgs { return webui.NoArgs{} }
	table := webui.Table[parcel]{
		Rows: func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }, Key: func(p parcel) string { return p.Name },
		Columns:  []webui.Accessor[parcel]{parcelName},
		RowClick: webui.Link[parcel, webui.NoArgs]{Page: dest, Args: args},
	}
	bar := webui.Toolbar{Left: []webui.ToolbarItem{webui.Link[webui.NoArgs, webui.NoArgs]{
		Label: "Open it", Page: dest, Args: func(context.Context, webui.NoArgs) webui.NoArgs { return webui.NoArgs{} },
	}}}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Stack{bar, table}}, dest}}.MustCompile("/admin")

	visitor := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.False(t, strings.Contains(visitor, `href="/admin/dest"`))
	assert.False(t, strings.Contains(visitor, "Open it"))
	assert.False(t, strings.Contains(visitor, "data-toolbar"))
	assert.Contains(t, visitor, `<tr class="">`) // the rows are plain rows
	assert.False(t, strings.Contains(visitor, "rowlink"))

	open.Store(true)
	editor := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Equal(t, strings.Count(editor, `href="/admin/dest"`), 3) // the button, and a link in each row
	assert.Contains(t, editor, "Open it")
}

// A Guard of a page that panics, asked on the way to a link, answers no for it, and is logged.
func TestAPageGuardThatPanicsOnTheWayToALinkHidesTheLink(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	dest := webui.Page[webui.NoArgs]{Path: "/dest", Body: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error { panic("the page guard fell over") }}
	bar := webui.Toolbar{Left: []webui.ToolbarItem{webui.Link[webui.NoArgs, webui.NoArgs]{
		Label: "Open it", Page: dest, Args: func(context.Context, webui.NoArgs) webui.NoArgs { return webui.NoArgs{} },
	}}}
	h := webui.App{Logger: slog.New(slog.NewTextHandler(&logs, nil)), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: bar}, dest}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/t")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.False(t, strings.Contains(rec.Body.String(), "Open it"))
	assert.Contains(t, logs.String(), "the page guard fell over")
}

// A hidden action posted by hand is refused, whichever way it was hidden: the page is not the authority.
func TestAHiddenActionPostedByHandIsRefused(t *testing.T) {
	t.Parallel()

	for name, err := range map[string]error{"error": errors.New("no"), "disabled": webui.Disabled("no")} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var ran atomic.Int32
			h := guardedParcels(nil, []webui.Action[parcel]{{
				Label: "Drop", Guard: func(context.Context, parcel) error { return err },
				Run: func(context.Context, parcel) (webui.Outcome, error) { ran.Add(1); return webui.Success("done"), nil },
			}}, nil, nil)
			rec := post(h, "/admin/t", url.Values{"_leaf": {"p"}, "_act": {"row:0:a"}})
			assert.Equal(t, flashOf(rec), "eno")
			assert.Equal(t, ran.Load(), int32(0))
		})
	}
}

// The reason of a disabled control is kept in view: it opens below the control when there is no room above it (the first row of a
// table, with its header over it, or a toolbar), and over the buttons of the row below and not under them. There is no script test
// harness, so this is what the shipped files carry; the behaviour is looked at in a browser.
func TestTheReasonOfADisabledControlIsKeptInView(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, nil, nil, nil)
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	assert.Contains(t, css, ".gate[data-below]:hover::after")
	assert.Contains(t, css, "td.actions:focus-within")
	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	assert.Contains(t, js, "setAttribute('data-below', '')")
}
