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
	"github.com/Olian04/webui/test/util/page"
)

// What a Guard returns decides how its control is drawn, and the same Guard refuses the request: an error leaves the control out,
// and Disabled draws it disabled, with its message.

func noop(string) func(context.Context, parcel) (webui.Outcome, error) {
	return func(context.Context, parcel) (webui.Outcome, error) { return webui.Success("done"), nil }
}

// guardedParcels is a table of the two parcels with the actions given, on a page of its own.
func guardedParcels(logger *slog.Logger, actions []webui.Action[parcel], bulk []webui.Action[[]parcel], click webui.Control[parcel]) http.Handler {
	table := webui.Table[parcel]{
		Loader: webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }}, Key: func(p parcel) string { return p.Name },
		Columns: []webui.Accessor[parcel]{parcelName}, BulkActions: bulk, RowClick: click,
	}
	for _, a := range actions {
		table.Actions = append(table.Actions, a)
	}
	return webui.App{Logger: logger, Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: table}}}.MustCompile("/admin")
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

	p := pageOf(t, h, "/admin/t")
	assert.DeepEqual(t, controls(mainOf(t, p)), []control{{"Drop a", "row:0:a", false}, {"Drop b", "row:0:b", true}})
	assert.DeepEqual(t, page.One(t, "the table", p.ByRole("table", "")).Rows(), [][]string{{"a", "Drop a"}, {"b", "Drop b Not available: Already dropped"}})

	rec := post(h, "/admin/t", formValues("_act", "row:0:b"))
	assert.Equal(t, flashOf(rec), "eAlready dropped") // the message is what is told to the one who asks by hand
}

// A Disabled that is wrapped is still one, and one with nothing to say still says something.
func TestADisabledThatIsWrappedOrEmptyIsStillDisabled(t *testing.T) {
	t.Parallel()

	guard := func(err error) func(context.Context, parcel) error {
		return func(context.Context, parcel) error { return err }
	}
	wrapped := pageOf(t, guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", guard(fmt.Errorf("not now: %w", webui.Disabled("Busy"))))}, nil, nil), "/admin/t")
	assert.Equal(t, len(wrapped.ByText("Drop a Not available: Busy")), 1)
	assert.True(t, controls(mainOf(t, wrapped))[0].Disabled)

	empty := pageOf(t, guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", guard(webui.Disabled("")))}, nil, nil), "/admin/t")
	assert.Equal(t, len(empty.ByText("Drop a Not available: Not allowed.")), 1)
}

// An error with no text is still a refusal: the control is not there, as for any other error, and the request is refused.
func TestAnErrorWithNoTextHidesTheControlAndRefusesTheRequest(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", func(context.Context, parcel) error { return errors.New("") })}, nil, nil)
	p := pageOf(t, h, "/admin/t")
	assert.False(t, strings.Contains(p.Root().AllText(), "Drop"))
	assert.DeepEqual(t, page.One(t, "the table", p.ByRole("table", "")).Headers(), []string{"Name"}) // and no column for actions

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
			p := pageOf(t, s.h, "/admin/device/dev1")
			assert.Equal(t, len(p.ByRole("button", "Save")), 0)
			_, offered := p.Field("f1")
			assert.False(t, offered) // the IP is shown and not offered
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
	p := pageOf(t, h, "/admin/t")
	assert.False(t, strings.Contains(p.Root().AllText(), "First"))
	assert.DeepEqual(t, dialogs(t, p), map[string]string{"Second a": "row:1:a", "Second b": "row:1:b"}) // the position it was declared at

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
	p := pageOf(t, h, "/admin/t")
	assert.DeepEqual(t, controls(mainOf(t, p)), []control{{"Bag", "bulk:0", false}})
	assert.Equal(t, len(fieldsNamed(p, "_sel")), 2) // a box to pick each row with
	for _, n := range asked {
		assert.Equal(t, n, 1)
	}

	// None, and an error that says nothing of how to draw it: there is no button, and so nothing to select the rows for.
	h = guardedParcels(nil, nil, bulk(func(parcel) error { return errors.New("no") }), nil)
	p = pageOf(t, h, "/admin/t")
	assert.False(t, strings.Contains(p.Root().AllText(), "Bag"))
	assert.Equal(t, len(fieldsNamed(p, "_sel")), 0)
	assert.Equal(t, len(controls(mainOf(t, p))), 0)
	assert.DeepEqual(t, page.One(t, "the table", p.ByRole("table", "Table")).Headers(), []string{"Name"}) // and no column to pick with

	// None, and the first of them that says Disabled: the button is disabled, with that.
	h = guardedParcels(nil, nil, bulk(func(p parcel) error {
		if p.Name == "a" {
			return errors.New("no")
		}
		return webui.Disabled("Nothing here can be bagged")
	}), nil)
	p = pageOf(t, h, "/admin/t")
	assert.DeepEqual(t, controls(mainOf(t, p)), []control{{"Bag", "bulk:0", true}})
	assert.Equal(t, len(p.ByText("Bag Not available: Nothing here can be bagged")), 1)
	assert.Equal(t, len(fieldsNamed(p, "_sel")), 2)

	// And the Guard that decided the button also refuses the request, which names its own selection.
	rec := post(h, "/admin/t", formValues("_act", "bulk:0", "_sel", "a"))
	assert.Equal(t, flashOf(rec), "eno")
}

// A table with no rows has nothing to select, and a bulk action is not drawn for it.
func TestATableWithNoRowsDrawsNoBulkAction(t *testing.T) {
	t.Parallel()

	table := webui.Table[parcel]{
		Loader: webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return nil, webui.Ready() }}, Key: func(p parcel) string { return p.Name },
		Columns: []webui.Accessor[parcel]{parcelName},
		BulkActions: []webui.Action[[]parcel]{{
			Label: "Bag",
			Run:   func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
		}},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: table}}}.MustCompile("/admin")
	p := pageOf(t, h, "/admin/t")
	assert.False(t, strings.Contains(p.Root().AllText(), "Bag"))
	assert.Equal(t, len(p.ByText("Nothing to show")), 1)
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

	p := pageOf(t, h, "/admin/t")
	assert.DeepEqual(t, page.One(t, "the table", p.ByRole("table", "")).Headers(), []string{"Name"})
	assert.Equal(t, len(controls(mainOf(t, p))), 0) // nothing in the table posts

	open.Store(true)
	table := page.One(t, "the table", pageOf(t, h, "/admin/t").ByRole("table", ""))
	assert.DeepEqual(t, table.Headers(), []string{"Name", "Actions"})
	assert.DeepEqual(t, table.Rows(), [][]string{{"a", "Drop a"}, {"b", ""}}) // b has none
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
	text := page.Parse(t, rec.Body.String()).Root().AllText()
	assert.False(t, strings.Contains(text, "Drop"))
	assert.False(t, strings.Contains(text, "Bag"))
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
	h := webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{
		Left:  []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "One", Guard: refused, Run: run}},
		Right: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "Two", Run: run}},
	}}}}.MustCompile("/admin")

	p := pageOf(t, h, "/admin/tools")
	assert.DeepEqual(t, controls(mainOf(t, p)), []control{{"Two", "1", false}}) // and Two is still the second

	// With every button refused, no toolbar is drawn: not an empty row, and not a form.
	h = webui.App{Pages: webui.Pages{webui.Page[toolArgs]{Path: "/tools", View: webui.Toolbar{
		Left: []webui.ToolbarItem{webui.Action[webui.NoArgs]{Label: "One", Guard: refused, Run: run}},
	}}}}.MustCompile("/admin")
	p = pageOf(t, h, "/admin/tools")
	assert.Equal(t, len(controls(mainOf(t, p))), 0)
	assert.Equal(t, len(mainOf(t, p).ByRole("button", "")), 0)
	_, posts := p.Field("_view")
	assert.False(t, posts)
}

// A link is drawn when the page it leads to would open for the visitor, and is not when that page's Guard refuses them.
func TestALinkToAPageThatRefusesTheVisitorIsNotDrawn(t *testing.T) {
	t.Parallel()

	var open atomic.Bool
	dest := webui.Page[webui.NoArgs]{Path: "/dest", View: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error {
		if open.Load() {
			return nil
		}
		return errors.New("editors only")
	}}
	args := func(context.Context, parcel) webui.NoArgs { return webui.NoArgs{} }
	table := webui.Table[parcel]{
		Loader: webui.RowsLoader[parcel]{Load: func(context.Context) ([]parcel, webui.Loaded) { return parcelsData, webui.Ready() }}, Key: func(p parcel) string { return p.Name },
		Columns:  []webui.Accessor[parcel]{parcelName},
		RowClick: webui.Link[parcel, webui.NoArgs]{Page: dest, Args: args},
	}
	bar := webui.Toolbar{Left: []webui.ToolbarItem{webui.Link[webui.NoArgs, webui.NoArgs]{
		Label: "Open it", Page: dest, Args: func(context.Context, webui.NoArgs) webui.NoArgs { return webui.NoArgs{} },
	}}}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: webui.Stack{bar, table}}, dest}}.MustCompile("/admin")

	visitor := pageOf(t, h, "/admin/t")
	assert.Equal(t, len(linksTo(visitor, "/admin/dest")), 0)
	assert.False(t, strings.Contains(visitor.Root().AllText(), "Open it"))
	assert.DeepEqual(t, page.One(t, "the table", visitor.ByRole("table", "")).Rows(), [][]string{{"a"}, {"b"}}) // the rows are plain rows

	open.Store(true)
	editor := pageOf(t, h, "/admin/t")
	assert.Equal(t, len(linksTo(editor, "/admin/dest")), 3) // the button, and a link in each row
	assert.True(t, hasLink(editor, "Open it", "/admin/dest"))
}

// A Guard of a page that panics, asked on the way to a link, answers no for it, and is logged.
func TestAPageGuardThatPanicsOnTheWayToALinkHidesTheLink(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	dest := webui.Page[webui.NoArgs]{Path: "/dest", View: webui.Stack{}, Guard: func(context.Context, webui.NoArgs) error { panic("the page guard fell over") }}
	bar := webui.Toolbar{Left: []webui.ToolbarItem{webui.Link[webui.NoArgs, webui.NoArgs]{
		Label: "Open it", Page: dest, Args: func(context.Context, webui.NoArgs) webui.NoArgs { return webui.NoArgs{} },
	}}}
	h := webui.App{Logger: slog.New(slog.NewTextHandler(&logs, nil)), Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: bar}, dest}}.MustCompile("/admin")

	rec := serve(h, http.MethodGet, "/admin/t")
	assert.Equal(t, rec.Code, http.StatusOK)
	assert.False(t, strings.Contains(page.Parse(t, rec.Body.String()).Root().AllText(), "Open it"))
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
			rec := post(h, "/admin/t", url.Values{"_view": {"p"}, "_act": {"row:0:a"}})
			assert.Equal(t, flashOf(rec), "eno")
			assert.Equal(t, ran.Load(), int32(0))
		})
	}
}
