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
	"github.com/Olian04/webui/test/util/page"
)

// Regression tests for what the accessibility audit found: each is a defect that a person using a keyboard or a screen reader met.

// A form cannot be in a form: the browser drops the inner one, and its button then posts the outer. A table that has actions is a form, so a column's
// filter, which is a form of its own, stands beside it, and the controls in the header name it with form=.
func TestAColumnsFilterIsNotAFormInTheTablesForm(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", nil)}, nil, nil)
	p := pageOf(t, h, "/admin/t")

	// The table is a form that posts, and the filter is a form of its own that gets, which stands beside it: a form that is in a form is dropped by the browser, and
	// its controls then belong to the outer one.
	filter, _ := p.Field("table.filter.name")
	forms := p.ByAttr("id", filter.Form)
	assert.Equal(t, len(forms), 1)
	assert.Equal(t, forms[0].Tag(), "form")
	assert.Equal(t, forms[0].Attr("method"), "get")
	assert.Equal(t, len(p.ByAttr("method", "post")), 1)
	// The Apply button is the filter's, wherever it is in the page.
	assert.Equal(t, page.One(t, "the Apply button", p.ByRole("button", "Apply")).Attr("form"), filter.Form)
	// And the controls of the table, which the filter would be among if it were inside it, are not the filter's.
	for _, f := range p.Fields() {
		if f.Name == "_act" || f.Name == "_view" {
			assert.Equal(t, f.Form, "")
		}
	}
}

// A bulk button that its Guard disabled is drawn with the reason, and not left to look as the others do.
func TestADisabledBulkButtonIsDrawnWithItsReason(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, nil, []webui.Action[[]parcel]{{
		Label: "Bag",
		Guard: func(context.Context, []parcel) error { return webui.Disabled("Needs a second approver") },
		Run:   func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
	}}, nil)
	p := pageOf(t, h, "/admin/t")
	bag := page.One(t, "the Bag button", p.ByRole("button", "Bag"))
	assert.True(t, bag.Has("disabled"))
	// The reason is text beside it, which a keyboard reaches though the button, which is disabled, is not a stop.
	reason := page.One(t, "the reason", p.ByAttr("tabindex", "0"))
	assert.Equal(t, reason.Text(), "Bag Not available: Needs a second approver")
}

// A read-only value is not a control, so its label does not point at one: it names the group that the value is in.
func TestAReadOnlyFieldsLabelNamesItsValueAndDoesNotPointAtNothing(t *testing.T) {
	t.Parallel()

	p := pageOf(t, formApp(okForm()), "/admin/device/dev1")
	// The ID is read: its label names the group that the value is in, and points at no control.
	id := page.One(t, "the ID", p.ByRole("group", "ID"))
	assert.Equal(t, id.Text(), "dev1")
	for _, label := range p.ByAttr("for", "") {
		assert.True(t, len(p.ByAttr("id", label.Attr("for"))) == 1) // a label points at what is in the page
	}
	// An editable field's label is still its control's.
	assert.Equal(t, page.One(t, "the IP", p.ByLabel("IP")).Attr("name"), "f0_1")
}

// Without script the search box is a text input and a button, which does not announce a popup that cannot open.
func TestTheSearchBoxIsAPlainInputWithoutScript(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	p := pageOf(t, h, "/admin/device")
	input := page.One(t, "the search box", p.ByAttr("data-palette-input", ""))
	for _, attr := range []string{"role", "aria-expanded", "aria-controls", "aria-autocomplete"} {
		assert.False(t, input.Has(attr))
	}
	assert.Equal(t, input.Role(), "textbox") // a text input, and not a combobox that announces a popup
}

// A listbox holds options, or groups of them: the results the box lists are groups that are named, with the options in them, and what it says
// of no results is not read as an option.
func TestTheBoxesListboxHoldsOnlyOptionsAndGroups(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	list := page.Parse(t, serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=eth").Body.String())
	devices := page.One(t, "the group of devices", list.ByRole("group", "Devices"))
	assert.True(t, len(devices.ByRole("option", "")) > 0)
	for _, child := range list.Root().Children() {
		assert.True(t, child.Role() == "group" || child.Role() == "") // what is among the groups is not an option
		assert.Equal(t, child.Attr("role") == "option", false)
	}
	none := page.Parse(t, serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=zzzzzz").Body.String())
	empty := page.One(t, "what is said of no results", none.ByText("Nothing matches."))
	assert.Equal(t, empty.Attr("role"), "presentation")
	assert.Equal(t, len(none.ByRole("option", "")), 0)

	// The page of results, which is not a listbox, is as it was.
	whole := pageOf(t, h, "/admin/_webui/search?q=eth")
	assert.Equal(t, len(whole.ByRole("option", "")), 0)
	assert.True(t, len(whole.ByText("Devices")) > 0)
}

// --- display preferences -------------------------------------------------------------------------------------------------------------------------

// --- structure and semantics ---------------------------------------------------------------------------------------------------------------------

// A page has one h1, its name, which the eye finds in the breadcrumb and a reader who goes by headings finds here; its panels are h2. The way past the chrome
// is the first thing in the page, and the content is where it leads.
func TestAPageHasOneH1TheSkipLinkAndTheLandmarksForReaders(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device")
	h1 := 0
	for _, e := range p.ByRole("heading", "") {
		if e.Tag() == "h1" {
			h1++
			assert.Equal(t, e.Name(), "Devices")
		}
	}
	assert.Equal(t, h1, 1)
	// The panels are one level down, and the way past what is round the content is the first thing in the page, and leads to where the content is.
	panel := page.One(t, "the panel", p.ByRole("heading", "Devices")[1:])
	assert.Equal(t, panel.Tag(), "h2")
	skip := p.Links()[0]
	assert.DeepEqual(t, skip, page.Link{Name: "Skip to content", Href: "#main"})
	assert.Equal(t, len(p.ByAttr("id", "main")), 1)
	assert.Equal(t, p.ByAttr("id", "main")[0].Role(), "main")
	// The status bar is the page's footer, named, so that its text is in a landmark.
	assert.Equal(t, len(p.ByRole("contentinfo", "Page status")), 1)

	// A state that stops the visitor is the h2 of a page that has its h1, and the page that has no shell has its own.
	missing := pageOf(t, h, "/admin/nothing-here")
	headings := map[string]string{}
	for _, e := range missing.ByRole("heading", "") {
		headings[e.Tag()] += e.Name() + ";"
	}
	assert.Equal(t, strings.Count(headings["h1"], ";"), 1)
	assert.Equal(t, headings["h2"], "Page not found;") // what stops the visitor is a heading, and not a paragraph
}

// A header cell says which way it heads, a column of selection boxes has a name in text that is there when the box is not, and a pager says whose pages it turns.
func TestATableHasScopedHeadersAndANamedPager(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{rowAction("Drop", nil)}, []webui.Action[[]parcel]{{
		Label: "Bag", Run: func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
	}}, nil)
	p := pageOf(t, h, "/admin/t")
	table := page.One(t, "the table", p.ByRole("table", ""))
	for _, th := range table.ByRole("columnheader", "") {
		assert.Equal(t, th.Attr("scope"), "col") // which way it heads
	}
	assert.DeepEqual(t, table.Headers()[:2], []string{"Select", "Name"}) // the column of boxes has a name in text, which is there when the box is not
	assert.Equal(t, len(p.ByRole("navigation", "Table pages")), 1)
	// A button in a row is of that row, and so is its box, by the text of its first cell.
	assert.Equal(t, len(p.ByRole("button", "Drop a")), 1)
	assert.Equal(t, len(p.ByAttr("aria-label", "Select a")), 1)
}

// A filter that narrows its column says so in its name, which its icon says to the eye.
func TestAFilterThatIsOnSaysSoInItsName(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	off := pageOf(t, h, "/admin/device")
	assert.Equal(t, len(off.ByRole("button", "Filter ID")), 1)
	assert.Equal(t, len(off.ByRole("button", "Filter ID (on)")), 0)
	on := pageOf(t, h, "/admin/device?devices.filter.id=dev0")
	assert.Equal(t, len(on.ByRole("button", "Filter ID (on)")), 1)
	assert.Equal(t, len(on.ByRole("button", "Filter Status")), 1) // the others are not
}

// What describes a control is the popover of its help and rules, which is read where it is referred to though it is not drawn, and the error beneath it.
func TestAControlIsDescribedByItsHelpAndItsError(t *testing.T) {
	t.Parallel()

	h := helpedApp(helpedName("What the collector calls itself.", webui.StringRules{MaxLen: 8}))
	p := pageOf(t, h, "/admin/h")
	name := page.One(t, "the name", p.ByAttr("name", "f0"))
	assert.DeepEqual(t, describedBy(t, p, name), []string{"Help: What the collector calls itself. Rules: Up to 8 characters"})
	assert.Equal(t, len(p.ByAttr("aria-invalid", "true")), 0)

	// Refused, it is described by both: the help, and then what is wrong.
	rec := post(h, "/admin/h", formValues("f0", "far too long a name"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	refused := page.Parse(t, rec.Body.String())
	name = page.One(t, "the name", refused.ByAttr("name", "f0"))
	assert.Equal(t, name.Attr("aria-invalid"), "true")
	assert.True(t, name.Has("autofocus"))
	assert.DeepEqual(t, describedBy(t, refused, name), []string{"Help: What the collector calls itself. Rules: Up to 8 characters", "Must be at most 8 characters."})

	// A field with nothing to say has nothing to be described by.
	plain := pageOf(t, helpedApp(helpedName("", webui.StringRules{})), "/admin/h")
	assert.False(t, page.One(t, "the name", plain.ByAttr("name", "f0")).Has("aria-describedby"))
}

// A result is told by its word as well as its border: a result that went wrong says so to a reader whose app worded it any way at all.
func TestAUploadResultSaysItsToneInAWord(t *testing.T) {
	t.Parallel()

	rec := postFragment(newUploadShop(nil).h, "firmware.txt", "words")
	told := items(page.Parse(t, rec.Body.String()))
	assert.Equal(t, len(told), 1)
	assert.True(t, strings.HasPrefix(told[0], "Done: ") || strings.HasPrefix(told[0], "Failed: ") || strings.HasPrefix(told[0], "Warning: "))
}

// A question that must be answered is an alert dialog, a link that opens a tab of its own says so to a reader, and the theme switch is a switch.
func TestControlsHaveTheRolesAReaderExpects(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, []webui.Action[parcel]{{Label: "Drop", Confirm: "Drop it?", Run: noop("Drop")}}, nil, nil)
	assert.True(t, len(pageOf(t, h, "/admin/t").ByRole("alertdialog", "Drop it?")) > 0)

	p := pageOf(t, linksApp(item("Docs", "https://example.com/help"), item("Home", "/logout")), "/admin/device")
	var docs, home page.Link
	for _, l := range p.Links() {
		switch l.Href {
		case "https://example.com/help":
			docs = l
		case "/logout":
			home = l
		}
	}
	assert.True(t, strings.HasSuffix(docs.Name, "(opens in a new tab)")) // a link that opens a tab of its own says so to a reader
	assert.True(t, docs.NewTab)
	assert.False(t, strings.Contains(home.Name, "opens in a new tab"))

	// The theme switch is a switch, and there is one: the sidebar has none of its own.
	switches := p.ByRole("switch", "Light theme")
	assert.Equal(t, len(switches), 1)
	assert.Equal(t, switches[0].Attr("aria-checked"), "false")
}

// --- announcements and focus -----------------------------------------------------------------------------------------------------------------

// Each toast is its own live region with the role of what it says, the region that holds them is not one (a live region that is nested in another is read
// twice), and what dismisses one is a button where there is script, and the label of a radio where there is not.
func TestAToastIsItsOwnLiveRegionAndHasAButton(t *testing.T) {
	t.Parallel()

	h := outcomeApp()
	flashed := func(flash string) string {
		r, _ := http.NewRequest(http.MethodGet, "/admin/device/a", nil)
		r.Header.Set("Cookie", "webui_flash="+base64.RawURLEncoding.EncodeToString([]byte(flash)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Body.String()
	}
	body := flashed("oFine")
	p := page.Parse(t, body)
	// Each toast is a live region with the role of what it says (an alert for a failure), and what holds them is not one, since a live region nested in another is
	// read twice.
	assert.DeepEqual(t, toasts(t, body), []toast{{"status", "Fine"}})
	assert.DeepEqual(t, toasts(t, flashed("wCareful")), []toast{{"status", "Careful"}})
	assert.DeepEqual(t, toasts(t, flashed("eBad")), []toast{{"alert", "Bad"}})
	holder := page.One(t, "what holds the toasts", p.ByAttr("data-toasts", ""))
	assert.False(t, holder.Has("role"))
	assert.False(t, holder.Has("aria-live"))
	// What dismisses one is a button where there is script, and the label of a radio where there is not.
	assert.Equal(t, len(p.ByRole("button", "Dismiss")), 1)
	assert.Equal(t, len(p.ByRole("radio", "Dismiss")), 1)

	// The page has a place to say what script changes, empty until it does.
	var announce []page.El
	for _, l := range p.Live() {
		if l.Attr("aria-live") == "polite" && l.Text() == "" {
			announce = append(announce, l)
		}
	}
	assert.True(t, len(announce) > 0)
}

// A refused form is told in its title, and the page opens with the focus on the first control that was refused, which is read with what is wrong as its
// description, and on only that one.
func TestARefusedFormSaysSoInItsTitleAndFocusesTheFirstControlThatWasRefused(t *testing.T) {
	t.Parallel()

	h := formApp(okForm())
	rec := post(h, "/admin/device/dev1", formValues("f0_1", "not an address", "f1", "700"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	p := page.Parse(t, rec.Body.String())
	assert.True(t, strings.HasPrefix(p.Title(), "Not saved, 2 fields need attention — "))
	var focused []string
	for _, f := range p.Fields() {
		if f.Type != "hidden" && page.One(t, f.Name, p.ByAttr("name", f.Name)).Has("autofocus") {
			focused = append(focused, f.Name)
		}
	}
	assert.DeepEqual(t, focused, []string{"f0_1"}) // the IP, which comes first, and not the count
	ip := page.One(t, "the IP", p.ByAttr("name", "f0_1"))
	assert.Equal(t, len(describedBy(t, p, ip)), 2) // the help, and what is wrong

	// A page that is not an answer to a refusal has no such title, and nothing is focused for it.
	ok := pageOf(t, h, "/admin/device/dev1")
	assert.False(t, strings.Contains(ok.Title(), "Not saved"))
	assert.Equal(t, len(ok.ByAttr("autofocus", "")), 0)
}

// The sidebar's button says what it controls and whether it is open, and the search has a live region for how many results it has: what script keeps up
// to date needs both in the page it starts from.
func TestThePageHasTheSidebarButtonAndTheLiveRegionThatScriptKeepsUpToDate(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	p := pageOf(t, h, "/admin/device")
	collapse := page.One(t, "the sidebar button", p.ByAttr("title", "Collapse sidebar"))
	assert.Equal(t, collapse.Attr("aria-expanded"), "true")
	assert.Equal(t, len(p.ByAttr("id", collapse.Attr("aria-controls"))), 1) // it names what it controls
	status := page.One(t, "the search's status", pageOf(t, searchApp(rowsSearch(), nil), "/admin/device").ByAttr("data-palette-status", ""))
	assert.Equal(t, status.Attr("aria-live"), "polite")
}

// The panel that polls has a pause, which only script can honour, and the selection's count is a status.
func TestAPanelThatIsAskedForAgainCanBePausedAndASelectionCountIsAStatus(t *testing.T) {
	t.Parallel()

	p := pageOf(t, refreshApp(webui.Ready().RefreshAfter(5*time.Second)), "/admin/s")
	pause := page.One(t, "the pause button", p.ByRole("button", "Pause"))
	assert.Equal(t, pause.Attr("aria-pressed"), "false")

	bulk := pageOf(t, guardedParcels(nil, nil, []webui.Action[[]parcel]{{Label: "Bag", Run: func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil }}}, nil), "/admin/t")
	count := page.One(t, "the count", bulk.ByText("0 selected"))
	assert.Equal(t, count.Role(), "status")
}
