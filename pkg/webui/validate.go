package webui

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/Olian04/webui/internal/render/assets"
)

// validate checks what the type system cannot, and never stops at the first
// problem: Compile reports every error at once so one fix cycle clears them
// all. It emits nothing and decides nothing about shape.
//
// Name derivation and RE2 compilation are not reimplemented here. validate
// calls internal/args and internal/rules, so it cannot disagree with lower
// about what an argument is called or whether a pattern is valid.

func (a App) validate() []CompileError {
	f := a.collectFacts()
	var errs []CompileError
	errs = append(errs, validateTheme(a.Theme)...)
	errs = append(errs, validateMenu(a.Menu)...)

	for i, p := range a.Pages {
		if p == nil {
			errs = append(errs, CompileError{
				Detail: fmt.Sprintf("Pages[%d] is nil", i),
				Fix:    "Remove it, or declare a Page.",
			})
			continue
		}
		errs = append(errs, p.validatePage(f)...)
	}
	return errs
}

// facts is the set of things the cross-page checks need before any page is
// validated, because they need the whole set.
type facts struct {
	paths    map[string]int          // Path → how many pages declare it
	argTypes map[string]reflect.Type // Path → argument type, for Link checks
}

func (a App) collectFacts() *facts {
	f := &facts{
		paths:    map[string]int{},
		argTypes: map[string]reflect.Type{},
	}
	for _, p := range a.Pages {
		if p == nil {
			continue
		}
		f.paths[p.pagePath()]++
		f.argTypes[p.pagePath()] = p.pageArgType()
	}
	return f
}

// bodyValidator carries one page's coordinates so every error from anywhere in
// its body names the page.
type bodyValidator struct {
	page  string // Path
	args  string // argument type name
	facts *facts

	errs []CompileError
}

func (v *bodyValidator) add(detail, fix string) {
	v.errs = append(v.errs, CompileError{Page: v.page, Args: v.args, Detail: detail, Fix: fix})
}

var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func validateTheme(t Theme) []CompileError {
	var errs []CompileError
	for _, f := range []struct {
		name  string
		value Color
	}{{"Accent", t.Accent}, {"OK", t.OK}, {"Warning", t.Warning}, {"Critical", t.Critical}} {
		if f.value != "" && !hexColor.MatchString(string(f.value)) {
			errs = append(errs, CompileError{
				Detail: fmt.Sprintf("Theme.%s is %q, which is not a hex colour", f.name, string(f.value)),
				Fix:    "Write it as #rgb, #rrggbb or #rrggbbaa, such as \"#3d71d9\".",
			})
		}
	}
	return errs
}

// validateMenu checks the app's menu: every entry has a label and an address that is
// a path on this site or a web or mail address, never a script.
func validateMenu(items []MenuItem) []CompileError {
	var errs []CompileError
	for i, it := range items {
		at := fmt.Sprintf("App.Menu[%d]", i)
		bad := func(detail, fix string) { errs = append(errs, CompileError{Detail: at + " " + detail, Fix: fix}) }
		if it.Label == "" {
			bad("has no Label", "Set Label; it is the entry's text.")
		}
		if it.Icon != "" && !assets.HasIcon(it.Icon) {
			bad(fmt.Sprintf("has the Icon %q, which is not a Font Awesome Free solid icon", it.Icon),
				"Use the name of a Font Awesome Free solid icon, such as \"book\", or leave Icon empty.")
		}
		if !validMenuURL(it.ExternalURL) {
			bad(fmt.Sprintf("has the ExternalURL %q, which is not a path on this site, or an http, https or mailto address", it.ExternalURL),
				"Use a path such as \"/logout\", or an address such as \"https://example.com/help\".")
		}
	}
	return errs
}

func validMenuURL(s string) bool {
	if strings.HasPrefix(s, "/") {
		return !strings.HasPrefix(s, "//") && !strings.HasPrefix(s, `/\`) && !strings.ContainsAny(s, "\n\r")
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	}
	return false
}
