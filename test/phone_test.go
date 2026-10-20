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

func TestAnEditablePhoneIsMarkedForScriptAndItsPageLinksTheScriptThatSplitsIt(t *testing.T) {
	t.Parallel()

	h, _ := phoneForm("SE")
	body := serve(h, http.MethodGet, "/admin/c").Body.String()
	assert.Contains(t, body, `name="f0" type="tel" value=""`)
	assert.Contains(t, body, ` data-country="SE" data-phone>`)
	script := regexp.MustCompile(`src="(/admin/_webui/phone\.[0-9a-f]+\.js)"`).FindStringSubmatch(body)
	assert.Equal(t, len(script), 2)

	// Without a default country the input says only that it is a phone.
	none, _ := phoneForm("")
	assert.False(t, strings.Contains(serve(none, http.MethodGet, "/admin/c").Body.String(), "data-country"))

	// The script is the countries of the server, then the script.
	js := serve(h, http.MethodGet, script[1])
	assert.Equal(t, js.Code, http.StatusOK)
	assert.True(t, strings.HasPrefix(js.Body.String(), `self.webuiPhoneCountries=[{"i":"AF","n":"Afghanistan","c":"93"}`))
	assert.Contains(t, js.Body.String(), `{"i":"SE","n":"Sweden","c":"46","g":[2,3,2,2],"t":1}`)
	assert.Contains(t, js.Body.String(), "A phone number, edited with script")

	// A page with no phone to edit does not link it, and neither does one whose phone is only read.
	plain := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/p", Body: webui.Form[contact]{
		Load:   func(context.Context) (contact, error) { return contact{Phone: "+46701234567"}, nil },
		Fields: []webui.Accessor[contact]{webui.Phone[contact]{Label: "Phone", Load: func(c contact) string { return c.Phone }}},
	}}}}.MustCompile("/admin")
	body = serve(plain, http.MethodGet, "/admin/p").Body.String()
	assert.False(t, strings.Contains(body, "phone."))
	assert.False(t, strings.Contains(body, "data-phone"))
	assert.Contains(t, body, "+46701234567")
	assert.False(t, strings.Contains(serve(contactAppEmailOnly(), http.MethodGet, "/admin/c").Body.String(), "phone."))
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
			Load:   func(context.Context) (contact, error) { return contact{}, nil },
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
