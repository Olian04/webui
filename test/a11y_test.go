package webui_test

import (
	"context"
	"net/http"
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
