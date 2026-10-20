package webui_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type contact struct {
	Email, Site, Phone, Color string
	Ends                      int
	Day                       string
}

func contactField(label string, input webui.InputType, rules webui.StringRules, get func(contact) string, set func(*contact, string)) webui.String[contact] {
	return webui.String[contact]{Label: label, Input: input, Rules: rules, Load: get, Store: set}
}

var (
	contactEmail = contactField("Email", webui.InputEmail, webui.StringRules{}, func(c contact) string { return c.Email }, func(c *contact, v string) { c.Email = v })
	contactSite  = contactField("Site", webui.InputURL, webui.StringRules{}, func(c contact) string { return c.Site }, func(c *contact, v string) { c.Site = v })
	contactPhone = contactField("Phone", webui.InputTel, webui.StringRules{}, func(c contact) string { return c.Phone }, func(c *contact, v string) { c.Phone = v })
	contactColor = contactField("Color", webui.InputColor, webui.StringRules{}, func(c contact) string { return c.Color }, func(c *contact, v string) { c.Color = v })
)

func contactApp(start contact, fields ...webui.Accessor[contact]) (http.Handler, func() contact) {
	saved := start
	if len(fields) == 0 {
		fields = []webui.Accessor[contact]{contactEmail, contactSite, contactPhone, contactColor}
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/c", Body: webui.Form[contact]{
		Load:   func(context.Context) (contact, error) { return saved, nil },
		Fields: fields,
		Submit: webui.Action[contact]{Run: func(_ context.Context, c contact) (webui.Outcome, error) {
			saved = c
			return webui.Success("Saved"), nil
		}},
	}}}}.MustCompile("/admin")
	return h, func() contact { return saved }
}

func TestAStringInputIsTheInputOfItsKindAndAnEmptyColorIsBlack(t *testing.T) {
	t.Parallel()

	h, _ := contactApp(contact{Email: "a@example.com", Site: "https://example.com", Phone: "+46 8 123"})
	body := serve(h, http.MethodGet, "/admin/c").Body.String()
	assert.Contains(t, body, `name="f0" type="email" value="a@example.com"`)
	assert.Contains(t, body, `name="f1" type="url" value="https://example.com"`)
	assert.Contains(t, body, `name="f2" type="tel" value="+46 8 123"`)
	assert.Contains(t, body, `name="f3" type="color" value="#000000"`) // a picker cannot be empty
	// The browser says what the server would say, for the two it checks.
	assert.Contains(t, body, `&#34;typeMismatch&#34;:&#34;Enter an email address.&#34;`)
	assert.Contains(t, body, `&#34;typeMismatch&#34;:&#34;Enter a web address that starts with http:// or https://.&#34;`)
	assert.Contains(t, body, `<span class="tip-item">A web address, http or https</span>`)

	set, _ := contactApp(contact{Color: "#2f9e8f"}, contactColor)
	assert.Contains(t, serve(set, http.MethodGet, "/admin/c").Body.String(), `name="f0" type="color" value="#2f9e8f"`)
}

func TestTheServerHoldsAValueToTheKindOfItsInput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		field     int
		good, bad []string
		message   string
	}{
		{0, []string{"a@example.com", "first.last+tag@sub.example.co", "a@b", " a@example.com "}, []string{"a", "a@", "@example.com", "a b@example.com", "a@@example.com", "a@-b.com"}, "Enter an email address."},
		{1, []string{"https://example.com", "http://example.com/a?b=c#d", "https://example.com:8080"}, []string{"example.com", "javascript:alert(1)", "ftp://example.com", "https://", "https://exa mple.com", "//example.com"}, "Enter a web address that starts with http:// or https://."},
		{2, []string{"+46 8 123 456", "(08) 123-45.67", "0701234567 x12"}, []string{"call me", "---", "12a45"}, "Enter a phone number."},
		{3, []string{"#2f9e8f", "#ABCDEF"}, []string{"2f9e8f", "#fff", "red", "#12345g"}, "Enter a color as #rrggbb."},
	} {
		h, saved := contactApp(contact{})
		name := "f" + strconv.Itoa(tc.field)
		for _, v := range tc.good {
			rec := post(h, "/admin/c", formValues(name, v))
			assert.Equal(t, rec.Code, http.StatusSeeOther)
		}
		before := saved()
		for _, v := range tc.bad {
			rec := post(h, "/admin/c", formValues(name, v))
			assert.Equal(t, rec.Code, http.StatusUnprocessableEntity)
			assert.Contains(t, rec.Body.String(), tc.message)
			assert.DeepEqual(t, saved(), before) // and nothing was stored
		}
		// Empty is not a value, and passes: only Required says it is wanted.
		assert.Equal(t, post(h, "/admin/c", formValues(name, "")).Code, http.StatusSeeOther)
	}
}

func TestAnInputKindIsHeldToTheOtherRulesToo(t *testing.T) {
	t.Parallel()

	email := contactField("Email", webui.InputEmail, webui.StringRules{Required: true, MaxLen: 20, EndsWith: ".com"},
		func(c contact) string { return c.Email }, func(c *contact, v string) { c.Email = v })
	h, saved := contactApp(contact{}, email)
	body := serve(h, http.MethodGet, "/admin/c").Body.String()
	assert.Contains(t, body, `type="email"`)
	assert.Contains(t, body, `maxlength="20"`)

	assert.Contains(t, post(h, "/admin/c", formValues("f0", "")).Body.String(), "This field is required.")
	assert.Contains(t, post(h, "/admin/c", formValues("f0", "someone@example.org")).Body.String(), "Must end with")
	assert.Contains(t, post(h, "/admin/c", formValues("f0", "somebody.long@example.com")).Body.String(), "Must be at most 20 characters.")
	assert.Equal(t, post(h, "/admin/c", formValues("f0", "me@example.com")).Code, http.StatusSeeOther)
	assert.Equal(t, saved().Email, "me@example.com")
}

func TestAnInputKindIsCheckedWhenTheAppCompiles(t *testing.T) {
	t.Parallel()

	problems := func(field webui.String[contact]) string {
		var all string
		for _, e := range compileErrors(t, webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/c", Body: webui.Form[contact]{
			Load:   func(context.Context) (contact, error) { return contact{}, nil },
			Fields: []webui.Accessor[contact]{field},
			Submit: webui.Action[contact]{Run: func(context.Context, contact) (webui.Outcome, error) { return webui.Outcome{}, nil }},
		}}}}) {
			all += e.Error() + "\n"
		}
		return all
	}
	load, store := func(c contact) string { return c.Email }, func(*contact, string) {}
	assert.Contains(t, problems(webui.String[contact]{Label: "E", Input: "emial", Load: load, Store: store}), `has the unknown Input "emial"`)
	assert.Contains(t, problems(webui.String[contact]{Label: "E", Input: webui.InputEmail, Secret: true, Load: load, Store: store}), "has an Input and is a Secret")
	assert.Contains(t, problems(webui.String[contact]{Label: "E", Input: webui.InputEmail, Multiline: true, Load: load, Store: store}), "has an Input and is Multiline")
	for _, rules := range []webui.StringRules{{MinLen: 3}, {MaxLen: 7}, {Contains: "a"}, {Pattern: &webui.PatternRule{Expr: "#.*", Message: "x"}}} {
		assert.Contains(t, problems(webui.String[contact]{Label: "C", Input: webui.InputColor, Rules: rules, Load: load, Store: store}), "is a color and has a length, a pattern or a text rule")
	}
}

func TestAnInputKindIsATableCellAsText(t *testing.T) {
	t.Parallel()

	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: webui.Table[contact]{
		Rows:    func(context.Context) ([]contact, error) { return []contact{{Email: "a@example.com"}}, nil },
		Columns: []webui.Accessor[contact]{contactEmail},
	}}}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/s").Body.String()
	assert.Contains(t, body, ">a@example.com<")
	assert.False(t, strings.Contains(body, `type="email"`))
}

// DateOnly: a day, picked with a date input, stored as midnight UTC.
func dayApp(start contact) (http.Handler, func() contact) {
	saved := start
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/d", Body: webui.Form[contact]{
		Load: func(context.Context) (contact, error) { return saved, nil },
		Fields: []webui.Accessor[contact]{
			webui.Timestamp[contact]{Label: "Ends", DateOnly: true, Load: func(c contact) int { return c.Ends }, Store: func(c *contact, v int) { c.Ends = v }},
			webui.Datetime[contact]{Label: "Day", DateOnly: true, Load: func(c contact) string { return c.Day }, Store: func(c *contact, v string) { c.Day = v }},
		},
		Submit: webui.Action[contact]{Run: func(_ context.Context, c contact) (webui.Outcome, error) {
			saved = c
			return webui.Success("Saved"), nil
		}},
	}}}}.MustCompile("/admin")
	return h, func() contact { return saved }
}

func TestADateOnlyFieldIsADateInputWithADayInIt(t *testing.T) {
	t.Parallel()

	ends := int(time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC).Unix())
	h, _ := dayApp(contact{Ends: ends, Day: "2026-10-09T11:27:00Z"})
	body := serve(h, http.MethodGet, "/admin/d").Body.String()
	assert.Contains(t, body, `name="f0" type="date" value="2027-01-15"`)
	assert.Contains(t, body, `name="f1" type="date" value="2026-10-09"`)
	assert.False(t, strings.Contains(body, `step="1"`))
	assert.Contains(t, body, `<span class="tip-item">Date in UTC</span>`)
	assert.False(t, strings.Contains(body, "Date and time in UTC"))
}

func TestADateOnlyFieldStoresMidnightUTCOrTheDateAlone(t *testing.T) {
	t.Parallel()

	h, saved := dayApp(contact{})
	assert.Equal(t, post(h, "/admin/d", formValues("f0", "2027-01-15", "f1", "2026-10-09")).Code, http.StatusSeeOther)
	assert.Equal(t, saved().Ends, int(time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC).Unix()))
	assert.Equal(t, saved().Day, "2026-10-09")

	// A time of day sent by hand is dropped to the day, which is what the field holds.
	assert.Equal(t, post(h, "/admin/d", formValues("f0", "2027-01-15T13:45", "f1", "2026-10-09T13:45")).Code, http.StatusSeeOther)
	assert.Equal(t, saved().Ends, int(time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC).Unix()))
	assert.Equal(t, saved().Day, "2026-10-09")

	// Cleared is stored as nothing, as a moment is.
	assert.Equal(t, post(h, "/admin/d", formValues("f0", "", "f1", "")).Code, http.StatusSeeOther)
	assert.DeepEqual(t, saved(), contact{})
	assert.Equal(t, post(h, "/admin/d", formValues("f0", "not a date")).Code, http.StatusUnprocessableEntity) // refused: unreadable
}

func TestADateOnlyColumnShowsTheDayAndItsFilterIsDates(t *testing.T) {
	t.Parallel()

	ends := int(time.Date(2027, 1, 15, 13, 45, 0, 0, time.UTC).Unix())
	table := func(dateOnly bool) string {
		h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/s", Body: webui.Table[contact]{
			Rows: func(context.Context) ([]contact, error) { return []contact{{Ends: ends}}, nil },
			Columns: []webui.Accessor[contact]{
				webui.Timestamp[contact]{Label: "Ends", DateOnly: dateOnly, Load: func(c contact) int { return c.Ends }},
			},
		}}}}.MustCompile("/admin")
		return serve(h, http.MethodGet, "/admin/s?table.min.ends=2027-01-01&table.max.ends=2027-02-01").Body.String()
	}
	day := table(true)
	assert.Contains(t, day, `datetime="2027-01-15"`)
	assert.Contains(t, day, ">2027-01-15<")
	assert.Contains(t, day, `type="date" name="table.min.ends" value="2027-01-01"`)
	assert.Contains(t, day, `type="date" name="table.max.ends" value="2027-02-01"`)

	moment := table(false)
	assert.Contains(t, moment, ">2027-01-15 13:45<")
	assert.Contains(t, moment, `type="datetime-local" name="table.min.ends" value="2027-01-01T00:00"`)
}
