package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

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
	assert.Contains(t, body, `data-part-0="`)
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
	assert.False(t, strings.Contains(body, `class="panel-foot`)) // nothing to save: no Save, and no Cancel
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

// A value that may be absent is stored as nothing when its input is emptied, which is not a zero.
func TestTheOptionalSettingsAreStoredAsNothingWhenEmptied(t *testing.T) {
	h := handler(t)
	t.Cleanup(func() {
		service.mu.Lock()
		service.settings.Burst, service.settings.Quota, service.settings.Paused, service.settings.Compress = nil, nil, nil, true
		service.mu.Unlock()
	})

	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `placeholder="No limit"`)
	assert.Contains(t, body, `<option value="">—</option>`)

	rec := post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f4_0": {"0"}, "f4_1": {"25"}, "f5_0": {"true"}, "f5_1": {"true"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	s := service.Settings()
	assert.True(t, s.Burst != nil && *s.Burst == 0) // a limit of zero, which is one
	assert.True(t, s.Quota != nil && *s.Quota == 25)
	assert.True(t, s.Paused != nil && *s.Paused)
	assert.True(t, s.Compress) // a checked box is sent as "true"

	rec = post(h, "/admin/settings", url.Values{"_leaf": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f4_0": {""}, "f4_1": {""}, "f5_0": {""}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	s = service.Settings()
	assert.True(t, s.Burst == nil && s.Quota == nil && s.Paused == nil)
	assert.False(t, s.Compress) // an unchecked checkbox is not sent at all, and an absent one is no
}

// Input gives a String the input of its kind, and the server holds the value to it; DateOnly makes a moment a day.
func TestTheContactFieldsHaveTheInputsOfTheirKindAndTheCertificateIsADate(t *testing.T) {
	h := handler(t)
	t.Cleanup(func() {
		service.mu.Lock()
		service.settings.Contact, service.settings.OnCall, service.settings.Webhook, service.settings.Label, service.settings.CertEnds = "oncall@example.com", "", "", "#2f9e8f", int(time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC).Unix())
		service.mu.Unlock()
	})

	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `type="email" value="oncall@example.com"`)
	assert.Contains(t, body, `type="tel"`)
	assert.Contains(t, body, `type="url"`)
	assert.Contains(t, body, `type="color" value="#2f9e8f"`)
	assert.Contains(t, body, `type="date" value="2027-01-15"`)

	form := func(contact, webhook string) url.Values {
		v := ingest("0.5", "60")
		v.Set("f6_0", contact)
		v.Set("f7_0", webhook)
		v.Set("f7_1", "#336699")
		v.Set("f8", "2027-03-01")
		return v
	}
	rec := post(h, "/admin/settings", form("not an email", "javascript:alert(1)"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Enter an email address.")
	assert.Contains(t, rec.Body.String(), "Enter a web address that starts with http:// or https://.")

	rec = post(h, "/admin/settings", form("ops@example.com", "https://hooks.example.com/x"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	s := service.Settings()
	assert.Equal(t, s.Contact, "ops@example.com")
	assert.Equal(t, s.Label, "#336699")
	assert.Equal(t, s.CertEnds, int(time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
}

// Help is a section of the popover beside a label that says what the field is for.
func TestAFieldsHelpIsInThePopoverBesideItsLabel(t *testing.T) {
	h := handler(t)
	body := get(h, "/admin/settings").Body.String()
	assert.Contains(t, body, `aria-label="Help: The share of events that are kept: 0.25 keeps one in four. Lower it when ingest is slow.`)
	assert.Contains(t, body, `<span class="tip-title">Help:</span>`)
	assert.Contains(t, body, `aria-label="Help: Events a second let through at once. Empty is no limit, which is not the same as 0, which lets none through.`)
}
