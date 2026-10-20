package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

func TestFormsDemonstrateRulesFloatSliderAndPlaceholder(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `placeholder="eu-north-1"`)  // Placeholder
	assert.Contains(t, body, `name="f1_0" type="number"`) // Float
	assert.Contains(t, body, `step="any"`)                //
	assert.Contains(t, body, `name="f1_1" type="number"`) // a Slider with a Store: a number, with a range to drag
	assert.Contains(t, body, `class="range-input"`)       //
	assert.Contains(t, body, `minlength="3" maxlength="32"`)

	// Rules run on the server too, and every failure is reported at once.
	rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"ab"}, "f0_1": {"70000"}, "f1_0": {"2"}, "f1_1": {"150"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	page := rec
	for _, want := range []string{"Must be at least 3 characters.", "Must be at most 65535.", "Must be at most 1.", "Must be at most 100."} {
		assert.Contains(t, page.Body.String(), want)
	}
	rec = post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
}

// The collector name has text rules and a pattern, which the browser holds as one pattern and the page tells apart.
func TestTheCollectorNameHasTextRulesAndAPattern(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `data-parts="`)
	for _, line := range []string{`Does not start with &#34;-&#34;`, `Does not end with &#34;-&#34;`, `Does not contain &#34;--&#34;`, "must be lowercase letters, digits and dashes"} {
		assert.Contains(t, body, `<span class="tip-item">`+line+`</span>`)
	}
	for value, want := range map[string]string{
		"-eu-north":  `Must not start with &#34;-&#34;.`,
		"eu-north-":  `Must not end with &#34;-&#34;.`,
		"eu--north":  `Must not contain &#34;--&#34;.`,
		"EU-north-1": "must be lowercase letters, digits and dashes",
	} {
		rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {value}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.Contains(t, rec.Body.String(), want)
	}
}

// The collector's API key is a secret: it is replaced and never read back.
func TestTheApiKeyIsASecretThatIsOnlyReplaced(t *testing.T) {
	h := handler(t)
	service.mu.Lock()
	service.settings.APIKey = "sk-live-0123456789"
	service.mu.Unlock()
	t.Cleanup(func() {
		service.mu.Lock()
		service.settings.APIKey = ""
		service.mu.Unlock()
	})

	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `type="password"`)
	assert.Contains(t, body, `placeholder="••••••••"`) // there is one, and that is all that is said
	assert.False(t, strings.Contains(body, "sk-live-0123456789"))

	rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f3": {""}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().APIKey, "sk-live-0123456789") // empty leaves it
	rec = post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f3": {"sk-new-abcdefghij"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().APIKey, "sk-new-abcdefghij")
}

func TestAReadOnlyFormHasNoSubmitAndShowsABadgeAndABar(t *testing.T) {
	body := get(handler(t), "/admin/system").Body.String()
	assert.False(t, strings.Contains(body, `type="submit"`))
	assert.Contains(t, body, `class="badge badge-ok"`)
	assert.Contains(t, body, `class="gauge-fill"`) // a Slider with no Store is a bar
}

func ingest(sample, load string) url.Values {
	return url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {sample}, "f1_1": {load}}
}

func TestIngestShowsEveryKindOfOutcome(t *testing.T) {
	h := handler(t)

	// Failure: not accepted, so the form is shown again with what was typed, and the reason.
	rec := post(h, "/admin/settings", ingest("0.95", "40"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	page := rec
	assert.Contains(t, page.Body.String(), `value="0.95"`)
	assert.Contains(t, page.Body.String(), "would drop most of them")

	// Warning: accepted, with something to be aware of.
	rec = post(h, "/admin/settings", ingest("0.5", "10"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Contains(t, flashOf(rec), "wSaved, but ingest will start shedding")

	// Success: accepted, and confirmed.
	rec = post(h, "/admin/settings", ingest("0.5", "60"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, flashOf(rec), "oSaved")
}

func TestSavingRetentionGoesOnToTheSystemPage(t *testing.T) {
	rec := post(handler(t), "/admin/retention", url.Values{"_leaf": {"p"}, "f0": {"45"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, rec.Header().Get("Location"), "/admin/system") // Then, in place of staying on the form
	assert.Equal(t, flashOf(rec), "oSaved")
}
