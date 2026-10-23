package webui_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
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

// A button that its Guard disabled is not made enabled by a selection, which would also let it post without the question that it dropped.
func TestASelectionDoesNotEnableAGuardedBulkButton(t *testing.T) {
	t.Parallel()

	h := guardedParcels(nil, nil, []webui.Action[[]parcel]{{
		Label: "Bag",
		Guard: func(context.Context, []parcel) error { return webui.Disabled("Needs a second approver") },
		Run:   func(context.Context, []parcel) (webui.Outcome, error) { return webui.Success("bagged"), nil },
	}}, nil)
	body := serve(h, http.MethodGet, "/admin/t").Body.String()
	assert.Contains(t, body, `<span class="gate" data-guard="Needs a second approver" tabindex="0">`)

	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	sel := js[strings.Index(js, "function syncSelection"):]
	sel = sel[:strings.Index(sel, "b.disabled = n === 0")]
	assert.Contains(t, sel, "if (b.closest('.gate')) return;")
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

// Choosing a country with the arrow keys fires a change for each key, so the focus stays on the selector: moving it leaves a keyboard user one country.
func TestThePhoneSelectorKeepsTheFocus(t *testing.T) {
	t.Parallel()

	h, _ := phoneForm("SE")
	js := serve(h, http.MethodGet, assetURL(t, h, "phone.js")).Body.String()
	assert.Contains(t, js, "select.addEventListener('change', show);")
	assert.False(t, strings.Contains(js, "input.focus()"))
}

// A search box is a combobox over a listbox that script fills, and without script it is a text input and a button, which does not announce a
// popup that cannot open; script sets what makes it one.
func TestTheSearchBoxIsAComboboxOnlyWhenScriptMakesItOne(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	input := body[strings.Index(body, "<input data-palette-input"):]
	input = input[:strings.Index(input, ">")]
	for _, attr := range []string{"role=", "aria-expanded", "aria-controls", "aria-autocomplete"} {
		assert.False(t, strings.Contains(input, attr))
	}
	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	for _, want := range []string{"input.setAttribute('role', 'combobox');", "input.setAttribute('aria-controls', box.id);", "input.setAttribute('aria-autocomplete', 'list');"} {
		assert.Contains(t, js, want)
	}
}

// A listbox holds options, or groups of them: the results the box lists are groups that are named, with the options in them, and what it says
// of no results is not read as an option.
func TestTheBoxesListboxHoldsOnlyOptionsAndGroups(t *testing.T) {
	t.Parallel()

	h := searchApp(rowsSearch(), nil)
	list := serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=eth").Body.String()
	assert.Contains(t, list, `<div role="group" aria-label="Devices">`)
	assert.Contains(t, list, `<div class="pal-label" aria-hidden="true">Devices</div>`)
	none := serve(searchBox(h), http.MethodGet, "/admin/_webui/search?q=zzzzzz").Body.String()
	assert.Contains(t, none, `<div class="pal-empty" role="presentation">Nothing matches.</div>`)

	// The page of results, which is not a listbox, is as it was.
	page := serve(h, http.MethodGet, "/admin/_webui/search?q=eth").Body.String()
	assert.False(t, strings.Contains(page, `role="option"`))
	assert.Contains(t, page, `<div class="pal-label">Devices</div>`)
}

// --- display preferences -------------------------------------------------------------------------------------------------------------------------

// The text that a reader needs is not in the colour that is for decoration: --text-3 is a step under what a person can be asked to read.
func TestNoTextThatIsNeededIsInTheDecorativeColour(t *testing.T) {
	t.Parallel()

	h, _ := phoneForm("SE")
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	for _, selector := range []string{
		".search input::placeholder", ".input::placeholder", ".kbd", ".refresh-note", ".pager-export-note", ".file-hint", ".msel-hint", ".udiff-n",
		".status-loading", ".sb-opt", ".filter-btn", ".filter-note",
	} {
		rule := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{[^}]*\}`).FindString(css)
		assert.True(t, rule != "")
		assert.False(t, strings.Contains(rule, "--text-3"))
	}

	// What is typed in the search box is text too, and the box around it is quiet, so the input says its own colour.
	assert.Contains(t, regexp.MustCompile(`\.search input\s*\{[^}]*\}`).FindString(css), "color: var(--text);")
}

// Windows high contrast and the like take every colour that a page chose, so what is drawn with a background or a shadow, a checked box, a focus ring,
// the active tab, goes unless it is drawn again with the system's colours, or with an outline or a border, which stay.
func TestTheStylesheetHasWhatForcedColorsAndReducedMotionNeed(t *testing.T) {
	t.Parallel()

	h, _ := phoneForm("SE")
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()

	forced := css[strings.Index(css, "@media (forced-colors: active) {"):]
	forced = forced[:strings.Index(forced, "\n}\n")]
	for _, want := range []string{
		".checkbox:checked", "background: Highlight;", "background: HighlightText;", ".msel-opt.on .msel-check", // a checked box is seen
		".input:focus", ".msel-field:focus-visible", ".side-resize:focus-visible", "outline-color: Highlight;", // focus is seen
		".tab.active", ".nav-item.active", ".pal-item.active", // where the reader is
	} {
		assert.Contains(t, forced, want)
	}
	more := css[strings.Index(css, "@media (prefers-contrast: more) {"):]
	more = more[:strings.Index(more, "\n}\n")]
	assert.Contains(t, more, ":root:not([data-theme])") // the system's light theme, which a stored choice does not set, is one of the rules it must beat
	assert.Contains(t, more, "--control-bd: var(--control-bd-strong);")
	assert.Contains(t, more, "--border-2: var(--control-bd-strong);")

	// Nothing that can take the focus has it taken away with no replacement: an outline is made transparent, which is drawn in forced colors, and not none.
	for _, rule := range regexp.MustCompile(`([^{}]+)\{[^{}]*outline:\s*none;[^{}]*\}`).FindAllStringSubmatch(css, -1) {
		selector := strings.TrimSpace(rule[1])
		if selector != ".search input" && selector != ".rowlink:focus-visible" && selector != "main:focus" {
			t.Errorf("%q takes the outline away", selector)
		}
	}

	motion := css[strings.Index(css, "@media (prefers-reduced-motion: reduce) {"):]
	motion = motion[:strings.Index(motion, "\n}\n")]
	for _, want := range []string{"animation-duration: 0.01ms !important;", "animation-iteration-count: 1 !important;", "transition-duration: 0.01ms !important;", ".spin {"} {
		assert.Contains(t, motion, want)
	}
	// What was drawn for a reader that moves is a still cue when it does not: a panel that is busy is dimmed.
	assert.Contains(t, motion, "[data-leaf][aria-busy='true'] tbody")
}

// A narrow window, which is what 320 pixels and 400 percent make of any, keeps its search.
func TestANarrowWindowKeepsItsSearch(t *testing.T) {
	t.Parallel()

	h, _ := phoneForm("SE")
	css := serve(h, http.MethodGet, assetURL(t, h, "app.css")).Body.String()
	narrow := css[strings.Index(css, "@media (max-width: 820px) {"):]
	narrow = narrow[:strings.Index(narrow, "\n}\n")]
	assert.False(t, regexp.MustCompile(`\.search\s*\{\s*display:\s*none`).MatchString(narrow))
	assert.Contains(t, narrow, ".search .kbd")
}

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
	assert.True(t, strings.Index(body, `<body><a class="skip" href="#main">Skip to content</a>`) > 0)
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
	assert.Contains(t, serve(h, http.MethodGet, "/admin/t").Body.String(), `class="confirm" role="alertdialog"`)

	links := serve(linksApp(item("Docs", "https://example.com/help"), item("Home", "/logout")), http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, links, `<span>Docs</span> <span class="sr-only">(opens in a new tab)</span>`)
	assert.False(t, strings.Contains(strings.SplitN(links, "Home</span>", 2)[1][:30], "opens in a new tab"))

	assert.Contains(t, links, `type="checkbox" role="switch" aria-label="Light theme"`)
}

// A change of the fragment alone, which following the skip link is, fires popstate as going back does, and must not reload the page: it would send the
// focus back to the top, which is where the skip link is. Only another address is another state.
func TestAFragmentOnlyChangeDoesNotReloadThePage(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	assert.Contains(t, js, "if (location.pathname + location.search !== shownAt) location.reload();")
	assert.False(t, strings.Contains(js, "history.pushState(null, '', href);\n"+"      shownAt")) // the address is noted where it is pushed, once
	assert.Equal(t, strings.Count(js, "moveTo(href);"), 2)
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
	assert.Contains(t, body, `<div class="toast" role="status">`)
	assert.Contains(t, toast("wCareful"), `<div class="toast warn" role="status">`)
	assert.Contains(t, body, `<button class="toast-close toast-close-js" type="button" title="Dismiss" aria-label="Dismiss" data-toast-dismiss>`)
	assert.Contains(t, body, `<label class="toast-close toast-close-nojs" title="Dismiss">`)

	assert.Contains(t, toast("eBad"), `<div class="toast err" role="alert">`)

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

// What script does to the page without a navigation is said, and the focus is kept where it would be lost: these are the tokens of the shipped script, since
// there is no harness for it.
func TestTheScriptSaysWhatItChangesAndKeepsTheFocus(t *testing.T) {
	t.Parallel()

	h, _ := tableApp(nil)
	js := serve(h, http.MethodGet, assetURL(t, h, "enhance.js")).Body.String()
	for _, want := range []string{
		// A panel that is replaced says what it holds now, and takes the focus that was in it to its title.
		"function announce(text)", "describeLeaf(next, 'updated')", "if (hadFocus) refocus(next);", "title.tabIndex = -1;",
		// Panels that are replaced in the background are not from under someone who is typing in them.
		"if (current && !current.contains(document.activeElement)) current.replaceWith(next); // not from under",
		// A panel that is asked for again can be stopped, and says when it is done.
		"[data-refresh-pause]", "if (el.hasAttribute('data-paused')) return;", "describeLeaf(next, 'is up to date')",
		// Toasts: put in again to be announced, an error stays, a message that went well stays long enough to read.
		"var TOAST_MS = 8000;", "if (el.classList.contains('err') || el.classList.contains('warn')) return;", "[data-toast-dismiss]", "var again = toast.cloneNode(true);",
		// A choice that is required says so, and says what is missing; the page opens with the focus on the field that was refused.
		"field.setAttribute('aria-required', 'true');", "err.setAttribute('role', 'alert');", "if (sel.hasAttribute('autofocus')) field.focus();",
		// The search says how many results it has, and one Escape closes one thing.
		"function paletteStatus(text)", "e.stopPropagation(); // one Escape closes one thing",
	} {
		assert.Contains(t, js, want)
	}
	assert.Equal(t, strings.Count(js, "!current.contains(document.activeElement)"), 2)
	// The sidebar's button keeps its name, and says what it controls and whether it is open.
	assert.False(t, strings.Contains(js, "'Expand sidebar'"))
	body := serve(h, http.MethodGet, "/admin/device").Body.String()
	assert.Contains(t, body, `aria-expanded="true" aria-controls="sidebar" title="Collapse sidebar"`)
	assert.Contains(t, body, `<aside class="sidebar" id="sidebar">`)
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

// The upload's queue is a live region from the start, out of sight and not hidden, the focus does not fall to the page when the button it was on is disabled,
// and the preview of a chosen image is a button.
func TestTheUploadKeepsItsLiveRegionAndTheFocus(t *testing.T) {
	t.Parallel()

	s := newUploadShop(nil)
	up := serve(s.h, http.MethodGet, assetURL(t, s.h, "upload.js")).Body.String()
	for _, want := range []string{"quiet(true);", "if (on && focused) cancel.focus();", "(submit || input).focus();"} {
		assert.Contains(t, up, want)
	}
	assert.False(t, strings.Contains(up, "list.hidden"))
	file := serve(s.h, http.MethodGet, assetURL(t, s.h, "file.js")).Body.String()
	assert.Contains(t, file, "element('button', 'file-thumb-chosen')")
	assert.Contains(t, file, "input.setAttribute('aria-invalid', 'true');")
}
