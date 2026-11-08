package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func TestFormsDemonstrateRulesFloatSliderAndPlaceholder(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/settings")
	box := func(name string) page.El { return page.One(t, name, p.ByAttr("name", name)) }
	assert.Equal(t, box("f0_0").Attr("placeholder"), "eu-north-1") // Placeholder
	sample, _ := p.Field("f1_0")
	assert.Equal(t, sample.Type, "number") // Float
	assert.Equal(t, box("f1_0").Attr("step"), "any")
	load, _ := p.Field("f1_1")
	assert.Equal(t, load.Type, "number") // a Slider with a Store: a number, with a range to drag
	assert.True(t, len(p.ByAttr("type", "range")) > 0)
	assert.Equal(t, box("f0_0").Attr("minlength"), "3")
	assert.Equal(t, box("f0_0").Attr("maxlength"), "32")

	// Rules run on the server too, and every failure is reported at once.
	rec := post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {"ab"}, "f0_1": {"70000"}, "f1_0": {"2"}, "f1_1": {"150"}})
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	refused := page.Parse(t, rec.Body.String())
	for _, want := range []string{"Must be at least 3 characters.", "Must be at most 65535.", "Must be at most 1.", "Must be at most 100."} {
		assert.True(t, shown(refused, want))
	}
	rec = post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
}

// The collector name has text rules and a pattern, which the browser holds as one pattern and the page tells apart.
func TestTheCollectorNameHasTextRulesAndAPattern(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/settings")
	assert.True(t, page.One(t, "the name", p.ByAttr("name", "f0_0")).Has("data-part-0"))
	for _, line := range []string{`Does not start with "-"`, `Does not end with "-"`, `Does not contain "--"`, "must be lowercase letters, digits and dashes"} {
		assert.True(t, len(p.ByAllText(line)) > 0)
	}
	for value, want := range map[string]string{
		"-eu-north":  `Must not start with "-".`,
		"eu-north-":  `Must not end with "-".`,
		"eu--north":  `Must not contain "--".`,
		"EU-north-1": "must be lowercase letters, digits and dashes",
	} {
		rec := post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {value}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}})
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.True(t, shown(page.Parse(t, rec.Body.String()), want))
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

	p := pageAt(t, h, "/admin/settings")
	key, _ := p.Field("f3")
	assert.Equal(t, key.Type, "password")
	assert.Equal(t, key.Value, "")
	assert.Equal(t, page.One(t, "the key", p.ByAttr("name", "f3")).Attr("placeholder"), "••••••••") // there is one, and that is all that is said
	assert.False(t, strings.Contains(p.Root().AllText(), "sk-live-0123456789"))

	rec := post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f3": {""}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().APIKey, "sk-live-0123456789") // empty leaves it
	rec = post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f3": {"sk-new-abcdefghij"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	assert.Equal(t, service.Settings().APIKey, "sk-new-abcdefghij")
}

func TestAReadOnlyFormHasNoSubmitAndShowsABadgeAndABar(t *testing.T) {
	p := pageAt(t, handler(t), "/admin/system")
	assert.Equal(t, len(mainOf(t, p).ByRole("button", "")), 0) // nothing to save: no Save, and no Cancel
	for _, f := range mainOf(t, p).Fields() {
		assert.Equal(t, f.Type, "hidden")
	}
	assert.True(t, len(mainOf(t, p).ByRole("group", "")) > 0) // what it shows, a badge and a bar, are values to read and not inputs
}

func ingest(sample, load string) url.Values {
	return url.Values{"_view": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {sample}, "f1_1": {load}}
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
	rec := post(handler(t), "/admin/retention", url.Values{"_view": {"p"}, "f0": {"45"}})
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

	p := pageAt(t, h, "/admin/settings")
	assert.True(t, len(p.ByAttr("placeholder", "No limit")) > 0)
	paused, _ := p.Field("f5_0")
	assert.Equal(t, paused.Options[0], page.Option{Value: "", Text: "—", Selected: false})

	rec := post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f4_0": {"0"}, "f4_1": {"25"}, "f5_0": {"true"}, "f5_1": {"true"}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	s := service.Settings()
	assert.True(t, s.Burst != nil && *s.Burst == 0) // a limit of zero, which is one
	assert.True(t, s.Quota != nil && *s.Quota == 25)
	assert.True(t, s.Paused != nil && *s.Paused)
	assert.True(t, s.Compress) // a checked box is sent as "true"

	rec = post(h, "/admin/settings", url.Values{"_view": {"p"}, "f0_0": {"eu-north-1"}, "f0_1": {"8125"}, "f1_0": {"0.5"}, "f1_1": {"60"}, "f4_0": {""}, "f4_1": {""}, "f5_0": {""}})
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	s = service.Settings()
	assert.True(t, s.Burst == nil && s.Quota == nil && s.Paused == nil)
	assert.False(t, s.Compress) // an off switch is not sent at all, and an absent one is no
}

// Input gives a String the input of its kind, and the server holds the value to it; DateOnly makes a moment a day.
func TestTheContactFieldsHaveTheInputsOfTheirKindAndTheCertificateIsADate(t *testing.T) {
	h := handler(t)
	t.Cleanup(func() {
		service.mu.Lock()
		service.settings.Contact, service.settings.OnCall, service.settings.Webhook, service.settings.Label, service.settings.CertEnds = "oncall@example.com", "", "", "#2f9e8f", time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)
		service.mu.Unlock()
	})

	p := pageAt(t, h, "/admin/settings")
	for name, want := range map[string]page.Field{
		"f6_0": {Name: "f6_0", Type: "email", Value: "oncall@example.com"},
		"f7_1": {Name: "f7_1", Type: "color", Value: "#2f9e8f"},
		"f8":   {Name: "f8", Type: "date", Value: "2027-01-15"},
	} {
		got, _ := p.Field(name)
		assert.Equal(t, got.Type, want.Type)
		assert.Equal(t, got.Value, want.Value)
	}
	types := map[string]bool{}
	for _, f := range p.Fields() {
		types[f.Type] = true
	}
	assert.True(t, types["tel"] && types["url"])

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
	refused := page.Parse(t, rec.Body.String())
	assert.True(t, shown(refused, "Enter an email address."))
	assert.True(t, shown(refused, "Enter a web address that starts with http:// or https://."))

	rec = post(h, "/admin/settings", form("ops@example.com", "https://hooks.example.com/x"))
	assert.Equal(t, rec.Code, http.StatusSeeOther)
	s := service.Settings()
	assert.Equal(t, s.Contact, "ops@example.com")
	assert.Equal(t, s.Label, "#336699")
	assert.True(t, s.CertEnds.Equal(time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)))
}

// Help is a section of the popover beside a label that says what the field is for.
func TestAFieldsHelpIsInThePopoverBesideItsLabel(t *testing.T) {
	h := handler(t)
	p := pageAt(t, h, "/admin/settings")
	assert.Equal(t, len(p.ByRoleContaining("note", "Help: The share of events that are kept: 0.25 keeps one in four. Lower it when ingest is slow.")), 1)
	assert.Equal(t, len(p.ByRoleContaining("note", "Help: Events a second let through at once. Empty is no limit, which is not the same as 0, which lets none through.")), 1)
}
