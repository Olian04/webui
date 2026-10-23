package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/phone"
	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func phoneForm(country string) (http.Handler, func() contact) {
	return contactApp(contact{}, webui.Phone[contact]{
		Label: "Phone", DefaultCountry: country,
		Load: func(c contact) string { return c.Phone }, Store: func(c *contact, v string) { c.Phone = v },
	})
}

func TestAPhoneIsStoredAsE164WhenItHasACountryCodeAndAsTypedWhenItHasNone(t *testing.T) {
	t.Parallel()

	h, saved := phoneForm("")
	for in, want := range map[string]string{
		"+46 70 123 45 67":  "+46701234567",
		"+1 (415) 555-2671": "+14155552671",
		"070-123 45 67":     "070-123 45 67",
		"0701234567 x12":    "0701234567 x12",
		"":                  "",
	} {
		assert.Equal(t, post(h, "/admin/c", formValues("f0", in)).Code, http.StatusSeeOther)
		assert.Equal(t, saved().Phone, want)
	}
	// An international number is held to what E.164 can hold, whatever its spacing.
	for _, bad := range []string{"+123", "+1234567890123456", "+46 70 abc"} {
		rec := post(h, "/admin/c", formValues("f0", bad))
		assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
		assert.Contains(t, rec.Body.String(), "Enter a phone number.")
	}
}

// selectedCountry is the ISO code of the option of a phone's selector that the page says is selected, "" for none.
func selectedCountry(body string) string {
	m := regexp.MustCompile(`<option value="([A-Z]{2})"[^>]*selected`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}

func TestAnEditablePhoneIsASelectorOfCountriesAndAnInputInTheHTMLAndItsPageLinksTheScriptThatSpacesIt(t *testing.T) {
	t.Parallel()

	h, _ := phoneForm("SE")
	body := serve(h, http.MethodGet, "/admin/c").Body.String()
	assert.Contains(t, body, `<div class="phone" data-phone>`)
	assert.Contains(t, body, `<select class="input phone-cc" name="f0.cc" aria-label="Country calling code">`)
	assert.Contains(t, body, `<option value="SE" data-code="46" data-groups="2,3,2,2" data-trunk selected>+46 Sweden</option>`)
	assert.Equal(t, strings.Count(body, "<option value="), len(phone.All())+1) // every country, and none
	assert.Equal(t, selectedCountry(body), "SE")
	assert.Contains(t, body, `name="f0" type="tel" value=""`)
	assert.Contains(t, body, `autocomplete="tel-national"`)
	assert.Contains(t, body, `inputmode="tel"`)
	script := regexp.MustCompile(`src="(/admin/_webui/phone\.[0-9a-f]+\.js)"`).FindStringSubmatch(body)
	assert.Equal(t, len(script), 2)

	// What is stored shows as the national number of its country, and a number with no country of its own as it was typed.
	held, _ := contactApp(contact{Phone: "+46701234567"}, contactPhone)
	body = serve(held, http.MethodGet, "/admin/c").Body.String()
	assert.Equal(t, selectedCountry(body), "SE")
	assert.Contains(t, body, `value="70 123 45 67"`)
	typed, _ := contactApp(contact{Phone: "070-123 45 67"}, contactPhone)
	body = serve(typed, http.MethodGet, "/admin/c").Body.String()
	assert.Equal(t, selectedCountry(body), "")
	assert.Contains(t, body, `value="070-123 45 67"`)

	// With no country of the field the visitor's languages choose, the likeliest first, and with none of them there is none.
	none, _ := phoneForm("")
	lang := func(header string) string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin/c", nil)
		if header != "" {
			req.Header.Set("Accept-Language", header)
		}
		none.ServeHTTP(rec, req)
		assert.Contains(t, strings.Join(rec.Header().Values("Vary"), ","), "Accept-Language")
		return selectedCountry(rec.Body.String())
	}
	assert.Equal(t, lang("sv-SE,sv;q=0.9,en;q=0.8"), "SE")
	assert.Equal(t, lang("en;q=0.5, de-DE;q=0.9"), "DE")
	assert.Equal(t, lang("en"), "")
	assert.Equal(t, lang(""), "")
	// The field's own country is not the visitor's to change.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/c", nil)
	req.Header.Set("Accept-Language", "de-DE")
	h.ServeHTTP(rec, req)
	assert.Equal(t, selectedCountry(rec.Body.String()), "SE")

	// The script no longer carries the countries, nor builds anything: they are the page's.
	js := serve(h, http.MethodGet, script[1])
	assert.Equal(t, js.Code, http.StatusOK)
	assert.False(t, strings.Contains(js.Body.String(), "webuiPhoneCountries"))
	assert.False(t, strings.Contains(js.Body.String(), "createElement"))
	assert.Contains(t, js.Body.String(), "data-groups")

	// A page with no phone to edit does not link it, and neither does one whose phone is only read.
	plain := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", Body: webui.Form[contact]{
		Load:   func(context.Context) (contact, webui.Loaded) { return contact{Phone: "+46701234567"}, webui.Ready() },
		Fields: []webui.Accessor[contact]{webui.Phone[contact]{Label: "Phone", Load: func(c contact) string { return c.Phone }}},
	}}}}.MustCompile("/admin")
	body = serve(plain, http.MethodGet, "/admin/p").Body.String()
	assert.False(t, strings.Contains(body, "phone."))
	assert.False(t, strings.Contains(body, "data-phone"))
	assert.Contains(t, body, "+46701234567")
	assert.False(t, strings.Contains(serve(contactAppEmailOnly(), http.MethodGet, "/admin/c").Body.String(), "phone."))
}

func TestACountryAndANationalNumberAreOneNumberThatThePostHoldsToTheRulesAndStores(t *testing.T) {
	t.Parallel()

	h, saved := phoneForm("")
	for _, c := range []struct{ cc, in, want string }{
		{"SE", "070 123 45 67", "+46701234567"},
		{"SE", "70 123 45 67", "+46701234567"},
		{"US", "(415) 555-2671", "+14155552671"},
		{"SE", "+49 151 2345 6789", "+4915123456789"}, // a whole number has its own code
		{"", "070 123 45 67", "070 123 45 67"},        // no country: as typed
		{"SE", "", ""},
	} {
		assert.Equal(t, post(h, "/admin/c", formValues("f0.cc", c.cc, "f0", c.in)).Code, http.StatusSeeOther)
		assert.Equal(t, saved().Phone, c.want)
	}

	// A number that is refused keeps the country that was chosen, as it was typed, in the form that answers.
	rec := post(h, "/admin/c", formValues("f0.cc", "SE", "f0", "call me"))
	assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
	assert.Contains(t, rec.Body.String(), "Enter a phone number.")
	assert.Equal(t, selectedCountry(rec.Body.String()), "SE")
	assert.Contains(t, rec.Body.String(), `value="call me"`)
}

func TestAPhoneInAGroupIsComposedToo(t *testing.T) {
	t.Parallel()

	h, saved := contactApp(contact{}, webui.Group[contact]{contactEmail, contactPhone})
	body := serve(h, http.MethodGet, "/admin/c").Body.String()
	assert.Contains(t, body, `name="f0_1.cc"`)
	assert.Equal(t, post(h, "/admin/c", formValues("f0_0", "a@example.com", "f0_1.cc", "SE", "f0_1", "070 123 45 67")).Code, http.StatusSeeOther)
	assert.Equal(t, saved().Phone, "+46701234567")
}

func contactAppEmailOnly() http.Handler {
	h, _ := contactApp(contact{}, contactEmail)
	return h
}

func TestADefaultCountryIsOneOfTheCountries(t *testing.T) {
	t.Parallel()

	field := func(code string) webui.Phone[contact] {
		return webui.Phone[contact]{Label: "Phone", DefaultCountry: code, Load: func(c contact) string { return c.Phone }, Store: func(*contact, string) {}}
	}
	problems := func(code string) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/c", Body: webui.Form[contact]{
			Load:   func(context.Context) (contact, webui.Loaded) { return contact{}, webui.Ready() },
			Fields: []webui.Accessor[contact]{field(code)},
			Submit: webui.Action[contact]{Run: func(context.Context, contact) (webui.Outcome, error) { return webui.Outcome{}, nil }},
		}}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	assert.Contains(t, problems("ZZ"), `has the unknown DefaultCountry "ZZ"`)
	assert.Contains(t, problems("se"), `has the unknown DefaultCountry "se"`)
}
