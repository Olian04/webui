package webui_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"

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
		if selector != ".search input" && selector != ".rowlink:focus-visible" {
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
