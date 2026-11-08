package webui_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func addressForm(rules webui.StringRules) webui.Form[Device] {
	return webui.Form[Device]{
		Load: func(context.Context) (Device, webui.Loaded) {
			return Device{Ip: "https://a.example.org"}, webui.Ready()
		},
		Fields: []webui.Accessor[Device]{webui.String[Device]{
			Label: "Address", Load: func(d Device) string { return d.Ip }, Store: func(d *Device, v string) { d.Ip = v }, Rules: rules,
		}},
		Submit: webui.Action[Device]{Run: func(context.Context, Device) (webui.Outcome, error) { return webui.Outcome{}, nil }},
	}
}

func addressApp(rules webui.StringRules) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", View: addressForm(rules)}}}.MustCompile("/admin")
}

func submitAddress(h http.Handler, v string) (code int, body string) {
	rec := post(h, "/admin/a", url.Values{"_view": {"p"}, "f0": {v}})
	return rec.Code, rec.Body.String()
}

// The text rules are the field's pattern in the browser, and when there are several, a pattern with each as a lookahead,
// and each with its words, for script to say which one a value fails.
func TestTextRulesAreThePatternOfTheInputAndAreListedWithItsRules(t *testing.T) {
	t.Parallel()

	box := func(rules webui.StringRules) (page.El, *page.Page) {
		p := pageOf(t, addressApp(rules), "/admin/a")
		return page.One(t, "the address input", p.ByAttr("name", "f0")), p
	}
	parts := func(e page.El) []string { // the attributes of the conditions, which script reads
		var got []string
		for _, a := range []string{"data-part-0", "data-part-0-msg", "data-part-1", "data-part-2", "data-part-3", "data-part-3-msg", "data-parts"} {
			if e.Has(a) {
				got = append(got, a)
			}
		}
		return got
	}

	one, p := box(webui.StringRules{StartsWith: "https://"})
	assert.Equal(t, one.Attr("pattern"), `https:\/\/[\s\S]*`)
	assert.Equal(t, one.Attr("title"), `Must start with "https://".`)
	assert.Equal(t, len(p.ByAllText(`Starts with "https://"`)), 1)
	assert.Equal(t, len(parts(one)), 0) // one condition: the browser's words are the only ones to say

	rules := webui.StringRules{
		StartsWith: "https://", EndsWith: ".org", NotContains: " ",
		Pattern: &webui.PatternRule{Expr: `[a-z0-9.\/:\-]+`, Message: "must be a lowercase address"},
	}
	many, p := box(rules)
	assert.Equal(t, many.Attr("pattern"), `(?=(?:https:\/\/[\s\S]*)$)(?=(?:[\s\S]*\.org)$)(?=(?:(?![\s\S]* )[\s\S]*)$)(?=(?:[a-z0-9.\/:\-]+)$)[\s\S]*`)
	// Each condition is an attribute, with the words for it in another, in the order the server tests them.
	assert.Equal(t, many.Attr("data-part-0"), `https:\/\/[\s\S]*`)
	assert.Equal(t, many.Attr("data-part-0-msg"), `Must start with "https://".`)
	assert.Equal(t, many.Attr("data-part-3"), `[a-z0-9.\/:\-]+`)
	assert.Equal(t, many.Attr("data-part-3-msg"), "must be a lowercase address")
	assert.False(t, many.Has("data-parts"))
	for _, line := range []string{`Starts with "https://"`, `Ends with ".org"`, `Does not contain " "`, "must be a lowercase address"} {
		assert.Equal(t, len(p.ByAllText(line)), 1)
	}

	// Without a rule of the kind there is nothing of it in the page: the pattern is what it was written as.
	plain, _ := box(webui.StringRules{Pattern: &webui.PatternRule{Expr: `[a-z]+`, Message: "letters"}})
	assert.Equal(t, plain.Attr("pattern"), "[a-z]+")
	assert.Equal(t, len(parts(plain)), 0)
}

func TestARefusedTextIsSaidInTheWordsOfTheRule(t *testing.T) {
	t.Parallel()

	h := addressApp(webui.StringRules{
		StartsWith: "https://", EndsWith: ".org", NotContains: " ", MinLen: 12,
		Pattern: &webui.PatternRule{Expr: `[a-z0-9.\/:\- ]+`, Message: "must be a lowercase address"},
	})
	for value, want := range map[string]string{
		"https://a":                "Must be at least 12 characters.",
		"http://example.org":       `Must start with "https://".`,
		"https://example.com":      `Must end with ".org".`,
		"https://exam ple.org":     `Must not contain " ".`,
		"https://Example.org":      "must be a lowercase address",
		"HTTPS://example.org":      `Must start with "https://".`,
		"https://example.org\r\nx": "",
	} {
		code, body := submitAddress(h, value)
		if want == "" {
			continue
		}
		assert.Equal(t, code, http.StatusUnprocessableEntity)
		refused := page.Parse(t, body)
		assert.True(t, strings.Contains(refused.Root().Text(), want))
		typed, _ := refused.Field("f0")
		assert.Equal(t, typed.Value, value) // what was typed stays
	}
	code, _ := submitAddress(h, "https://example.org")
	assert.Equal(t, code, http.StatusSeeOther)
}

func TestTextThatNoValueCanHoldIsAProblemOfTheApp(t *testing.T) {
	t.Parallel()

	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", View: addressForm(webui.StringRules{
		StartsWith: "https://", NotContains: "//", Contains: "a\nb", MaxLen: 4,
	})}}}
	errs := compileErrors(t, app)
	var got []string
	for _, e := range errs {
		got = append(got, e.Detail)
	}
	assert.Equal(t, len(errs), 3)
	assert.Contains(t, strings.Join(got, "\n"), "Contains has a line break in it")
	assert.Contains(t, strings.Join(got, "\n"), `StartsWith "https://" has NotContains "//" in it`)
	assert.Contains(t, strings.Join(got, "\n"), `StartsWith "https://" is longer than MaxLen, 4`)
}
