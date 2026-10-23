package webui_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
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
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: addressForm(rules)}}}.MustCompile("/admin")
}

func submitAddress(h http.Handler, v string) (code int, body string) {
	rec := post(h, "/admin/a", url.Values{"_leaf": {"p"}, "f0": {v}})
	return rec.Code, rec.Body.String()
}

// The text rules are the field's pattern in the browser, and when there are several, a pattern with each as a lookahead,
// and each with its words, for script to say which one a value fails.
func TestTextRulesAreThePatternOfTheInputAndAreListedWithItsRules(t *testing.T) {
	t.Parallel()

	one := serve(addressApp(webui.StringRules{StartsWith: "https://"}), http.MethodGet, "/admin/a").Body.String()
	assert.Contains(t, one, `pattern="https:\/\/[\s\S]*"`)
	assert.Contains(t, one, `title="Must start with &#34;https://&#34;."`)
	assert.Contains(t, one, `<span class="tip-item">Starts with &#34;https://&#34;</span>`)
	assert.False(t, strings.Contains(one, "data-part-")) // one condition: the browser's words are the only ones to say

	rules := webui.StringRules{
		StartsWith: "https://", EndsWith: ".org", NotContains: " ",
		Pattern: &webui.PatternRule{Expr: `[a-z0-9.\/:\-]+`, Message: "must be a lowercase address"},
	}
	many := serve(addressApp(rules), http.MethodGet, "/admin/a").Body.String()
	assert.Contains(t, many, `pattern="(?=(?:https:\/\/[\s\S]*)$)(?=(?:[\s\S]*\.org)$)(?=(?:(?![\s\S]* )[\s\S]*)$)(?=(?:[a-z0-9.\/:\-]+)$)[\s\S]*"`)
	// Each condition is an attribute, with the words for it in another, in the order the server tests them.
	assert.Contains(t, many, `data-part-0="https:\/\/[\s\S]*" data-part-0-msg="Must start with &#34;https://&#34;."`)
	assert.Contains(t, many, `data-part-3="[a-z0-9.\/:\-]+" data-part-3-msg="must be a lowercase address"`)
	assert.False(t, strings.Contains(many, "data-parts"))
	for _, line := range []string{`Starts with &#34;https://&#34;`, `Ends with &#34;.org&#34;`, `Does not contain &#34; &#34;`, "must be a lowercase address"} {
		assert.Contains(t, many, `<span class="tip-item">`+line+`</span>`)
	}

	// Without a rule of the kind there is nothing of it in the page: the pattern is what it was written as.
	plain := serve(addressApp(webui.StringRules{Pattern: &webui.PatternRule{Expr: `[a-z]+`, Message: "letters"}}), http.MethodGet, "/admin/a").Body.String()
	assert.Contains(t, plain, `pattern="[a-z]+"`)
	assert.False(t, strings.Contains(plain, "data-part-"))
}

func TestARefusedTextIsSaidInTheWordsOfTheRule(t *testing.T) {
	t.Parallel()

	h := addressApp(webui.StringRules{
		StartsWith: "https://", EndsWith: ".org", NotContains: " ", MinLen: 12,
		Pattern: &webui.PatternRule{Expr: `[a-z0-9.\/:\- ]+`, Message: "must be a lowercase address"},
	})
	for value, want := range map[string]string{
		"https://a":                "Must be at least 12 characters.",
		"http://example.org":       `Must start with &#34;https://&#34;.`,
		"https://example.com":      `Must end with &#34;.org&#34;.`,
		"https://exam ple.org":     `Must not contain &#34; &#34;.`,
		"https://Example.org":      "must be a lowercase address",
		"HTTPS://example.org":      `Must start with &#34;https://&#34;.`,
		"https://example.org\r\nx": "",
	} {
		code, body := submitAddress(h, value)
		if want == "" {
			continue
		}
		assert.Equal(t, code, http.StatusUnprocessableEntity)
		assert.Contains(t, body, `<div class="err" id="p-f0-error"><i class="fa-solid fa-circle-exclamation" aria-hidden="true"></i> `+want)
	}
	code, _ := submitAddress(h, "https://example.org")
	assert.Equal(t, code, http.StatusSeeOther)
}

func TestTextThatNoValueCanHoldIsAProblemOfTheApp(t *testing.T) {
	t.Parallel()

	app := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: addressForm(webui.StringRules{
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
