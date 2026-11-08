package webui_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// Regression tests for what the accessibility audit found: each is a defect that a person using a keyboard or a screen reader met.

// A form cannot be in a form: the browser drops the inner one, and its button then posts the outer. A table that has actions is a form, so a column's
// filter, which is a form of its own, stands beside it, and the controls in the header name it with form=.
func TestAColumnsFilterIsNotAFormInTheTablesForm(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", nil)}, nil, nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()

	start := strings.Index(body, `<form action="/admin/t" method="post"`)
	assert.True(t, start >= 0)
	end := start + strings.Index(body[start:], "</form>")
	assert.Equal(t, strings.Count(body[start:end], "<form"), 1) // the table's, and no other in it
	assert.True(t, strings.Contains(body, `<form id="p-filter-c0" method="get"`))
	assert.True(t, strings.Index(body, `<form id="p-filter-c0"`) > end) // beside the table, after it
	// The Apply button is the filter's, wherever it is in the page.
	assert.Contains(t, body, `type="submit" form="p-filter-c0">Apply</button>`)
	assert.Contains(t, body, `class="input" form="p-filter-c0" type="search"`)
}

// A bulk button that its Guard disabled is drawn with the reason, and not left to look as the others do.
func TestADisabledBulkButtonIsDrawnWithItsReason(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, nil, []webui.Action[[]parcel]{{
		Label: "Bag",
		Guard: func(context.Context, []parcel) error { return webui.Disabled("Needs a second approver") },
		Run:   func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
	}}, nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `<span class="gate" data-guard="Needs a second approver" tabindex="0">`)
}

// A read-only value is not a control, so its label does not point at one: it names the group that the value is in.
func TestAReadOnlyFieldsLabelNamesItsValueAndDoesNotPointAtNothing(t *testing.T) {
	t.Parallel()

	body := serve(formApp(okForm()), http.MethodGet, "/admin/device/dev1").Body.String()
	assert.Contains(t, body, `<label class="lbl" id="p-f0_0-label">`)
	assert.False(t, strings.Contains(body, `for="p-f0_0"`)) // nothing in the page has that id
	assert.Contains(t, body, `<div class="static" role="group" aria-labelledby="p-f0_0-label">dev1</div>`)
	// An editable field's label is still its control's.
	assert.Contains(t, body, `for="p-f0_1"`)
}

// Without script the search box is a text input and a button, which does not announce a popup that cannot open.
func TestTheSearchBoxIsAPlainInputWithoutScript(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	input := body[strings.Index(body, "<input data-palette-input"):]
	input = input[:strings.Index(input, ">")]
	for _, attr := range []string{"role=", "aria-expanded", "aria-controls", "aria-autocomplete"} {
		assert.False(t, strings.Contains(input, attr))
	}
}

// A listbox holds options, or groups of them: the results the box lists are groups that are named, with the options in them, and what it says
// of no results is not read as an option.
func TestTheBoxesListboxHoldsOnlyOptionsAndGroups(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	list := serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=eth").Body.String()
	assert.Contains(t, list, `<div role="group" aria-label="Devices">`)
	assert.Contains(t, list, `<div class="pal-label caption" aria-hidden="true">Devices</div>`)
	none := serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=zzzzzz").Body.String()
	assert.Contains(t, none, `<div class="pal-empty" role="presentation">Nothing matches.</div>`)

	// The page of results, which is not a listbox, is as it was.
	page := serve(h, http.MethodGet, "/admin/_webui/search?q=eth").Body.String()
	assert.False(t, strings.Contains(page, `role="option"`))
	assert.Contains(t, page, `<div class="pal-label caption">Devices</div>`)
}

// --- display preferences -------------------------------------------------------------------------------------------------------------------------

// --- structure and semantics ---------------------------------------------------------------------------------------------------------------------

// A page has one h1, its name, which the eye finds in the breadcrumb and a reader who goes by headings finds here; its panels are h2. The way past the chrome
// is the first thing in the page, and the content is where it leads.
func TestAPageHasOneH1TheSkipLinkAndTheLandmarksForReaders(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Equal(t, strings.Count(body, "<h1"), 1)
	assert.Contains(t, body, `<main id="main" tabindex="-1" class="scroll"><h1 class="sr-only">Devices</h1>`)
	assert.Contains(t, body, `<h2 class="panel-title">Devices</h2>`) // the panels are one level down
	assert.True(t, strings.Index(body, `<body><a class="skip raise" href="#main">Skip to content</a>`) > 0)
	// The status bar is the page's footer, named, so that its text is in a landmark.
	assert.Contains(t, body, `<footer class="statusbar" aria-label="Page status">`)

	// A state that stops the visitor is the h2 of a page that has its h1, and the page that has no shell has its own.
	missing := serve(h, http.MethodGet, "/admin/nothing-here").Body.String()
	assert.False(t, strings.Contains(missing, `<p class="state-title">`))
	assert.Contains(t, missing, `<h2 class="state-title">`)
	assert.Equal(t, strings.Count(missing, "<h1"), 1)
}

// A header cell says which way it heads, a column of selection boxes has a name in text that is there when the box is not, and a pager says whose pages it turns.
func TestATableHasScopedHeadersAndANamedPager(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", nil)}, []webui.Action[[]parcel]{{
		Label: "Bag", Run: func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
	}}, nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Equal(t, strings.Count(body, `<th scope="col"`), strings.Count(body, "<th "))
	assert.Contains(t, body, `<th scope="col" class="pick"><span class="sr-only">Select</span>`)
	assert.Contains(t, body, `<nav class="pager" aria-label="Table pages">`)
	// A button in a row is of that row, and so is its box, by the text of its first cell.
	assert.Contains(t, body, `>Drop <span class="sr-only">a</span></button>`)
	assert.Contains(t, body, `aria-label="Select a"`)
}

// A filter that narrows its column says so in its name, which its icon says to the eye.
func TestAFilterThatIsOnSaysSoInItsName(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	off := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, off, `aria-label="Filter ID"><i`)
	on := serve(h, http.MethodGet, "/admin/device?devices.filter.id=dev0").Body.String()
	assert.Contains(t, on, `aria-label="Filter ID (on)"><i`)
	assert.Contains(t, on, `aria-label="Filter Status"><i`) // the others are not
}

// What describes a control is the popover of its help and rules, which is read where it is referred to though it is not drawn, and the error beneath it.
func TestAControlIsDescribedByItsHelpAndItsError(t *testing.T) {
	t.Parallel()

	h := helpedApp(helpedName("What the collector calls itself.", webui.StringRules{MaxLen: 8}))
	body := serve(h, http.MethodGet, "/admin/h").Body.String()
	assert.Contains(t, body, `id="p-f0-help"`)               // the text of the popover has an id
	assert.Contains(t, body, `aria-describedby="p-f0-help"`) // and the input is described by it
	assert.False(t, strings.Contains(body, `aria-invalid`))

	// Refused, it is described by both: the help, and then what is wrong.
	rec := post(h, "/admin/h", formValues("f0", "far too long a name"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), `aria-invalid="true" autofocus aria-describedby="p-f0-help p-f0-error"`)

	// A field with nothing to say has nothing to be described by.
	plain := serve(helpedApp(helpedName("", webui.StringRules{})), http.MethodGet, "/admin/h").Body.String()
	assert.False(t, strings.Contains(plain, "aria-describedby"))
}

// A result is told by its word as well as its border: a result that went wrong says so to a reader whose app worded it any way at all.
func TestAUploadResultSaysItsToneInAWord(t *testing.T) {
	t.Parallel()

	rec := postFragment(newUploadShop(nil).h, "firmware.txt", "words")
	assert.Contains(t, rec.Body.String(), `<span class="sr-only">`)
	assert.Contains(t, rec.Body.String(), `: </span>`)
	assert.Contains(t, rec.Body.String(), `data-tone=`)
}

// A question that must be answered is an alert dialog, a link that opens a tab of its own says so to a reader, and the theme switch is a switch.
func TestControlsHaveTheRolesAReaderExpects(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{{Label: "Drop", Confirm: "Drop it?", Run: noop("Drop")}}, nil, nil)
	assert.Contains(t, serve(h, http.MethodGet, "/admin/t").Body.String(), `class="confirm raise" role="alertdialog"`)

	links := serve(linksApp(item("Docs", "https://example.com/help"), item("Home", "/logout")), http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, links, `<span>Docs</span> <span class="sr-only">(opens in a new tab)</span>`)
	assert.False(t, strings.Contains(strings.SplitN(links, "Home</span>", 2)[1][:30], "opens in a new tab"))

	assert.Contains(t, links, `role="switch" aria-checked="false" aria-label="Light theme" title="Light theme" data-pref-toggle="theme"`)
	assert.False(t, strings.Contains(links, `data-pref="theme"`)) // the sidebar has no switch of its own
}

// --- announcements and focus -----------------------------------------------------------------------------------------------------------------

// Each toast is its own live region with the role of what it says, the region that holds them is not one (a live region that is nested in another is read
// twice), and what dismisses one is a button where there is script, and the label of a radio where there is not.
func TestAToastIsItsOwnLiveRegionAndHasAButton(t *testing.T) {
	t.Parallel()

	h := outcomeApp()
	toast := func(flash string) string {
		r, _ := http.NewRequest(http.MethodGet, "/admin/device/a", nil)
		r.Header.Set("Cookie", "webui_flash="+base64.RawURLEncoding.EncodeToString([]byte(flash)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Body.String()
	}
	body := toast("oFine")
	assert.Contains(t, body, `<div class="toasts" data-toasts>`)
	assert.False(t, strings.Contains(body, `data-toasts role=`))
	assert.Contains(t, body, `<div class="toast raise" role="status">`)
	assert.Contains(t, toast("wCareful"), `<div class="toast raise tone-warn" role="status">`)
	assert.Contains(t, body, `<button class="toast-close toast-close-js" type="button" title="Dismiss" aria-label="Dismiss" data-toast-dismiss>`)
	assert.Contains(t, body, `<label class="toast-close toast-close-nojs" title="Dismiss">`)

	assert.Contains(t, toast("eBad"), `<div class="toast raise tone-bad" role="alert">`)

	// The page has a place to say what script changes, empty until it does.
	assert.Contains(t, body, `<div class="sr-only" role="status" aria-live="polite" data-announce></div>`)
}

// A refused form is told in its title, and the page opens with the focus on the first control that was refused, which is read with what is wrong as its
// description, and on only that one.
func TestARefusedFormSaysSoInItsTitleAndFocusesTheFirstControlThatWasRefused(t *testing.T) {
	t.Parallel()

	h := formApp(okForm())
	rec := post(h, "/admin/device/dev1", formValues("f0_1", "not an address", "f1", "700"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	assert.Contains(t, body, "<title>Not saved, 2 fields need attention — ")
	assert.Equal(t, strings.Count(body, " autofocus"), 1)
	first := body[strings.Index(body, `name="f0_1"`):]
	assert.True(t, strings.Index(first, " autofocus") < strings.Index(first, `name="f1"`)) // the IP, which comes first, and not the count
	assert.Contains(t, body, `autofocus aria-describedby="p-f0_1-help p-f0_1-error"`)

	// A page that is not an answer to a refusal has no such title, and nothing is focused for it.
	ok := serve(h, http.MethodGet, "/admin/device/dev1").Body.String()
	assert.False(t, strings.Contains(ok, "Not saved"))
	assert.False(t, strings.Contains(ok, " autofocus"))
}

// The sidebar's button says what it controls and whether it is open, and the search has a live region for how many results it has: what script keeps up
// to date needs both in the page it starts from.
func TestThePageHasTheSidebarButtonAndTheLiveRegionThatScriptKeepsUpToDate(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `aria-expanded="true" aria-controls="sidebar" title="Collapse sidebar"`)
	assert.Contains(t, body, `<aside class="sidebar drawer drawer-left" id="sidebar">`)
	assert.Contains(t, serve(searchApp(rowsSearch(), nil), http.MethodGet, "/admin/device").Body.String(), `<div class="sr-only" role="status" aria-live="polite" data-palette-status></div>`)
}

// The panel that polls has a pause, which only script can honour, and the selection's count is a status.
func TestAPanelThatIsAskedForAgainCanBePausedAndASelectionCountIsAStatus(t *testing.T) {
	t.Parallel()

	body := serve(refreshApp(webui.Ready().RefreshAfter(5*time.Second)), http.MethodGet, "/admin/s").Body.String()
	assert.Contains(t, body, `<button class="refresh-pause refresh-js" type="button" aria-pressed="false" data-refresh-pause>Pause</button>`)

	bulk := serve(guardedParcels(nil, nil, []webui.Action[[]parcel]{{Label: "Bag", Run: func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil }}}, nil), http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, bulk, `<strong data-selcount role="status">0 selected</strong>`)
}
