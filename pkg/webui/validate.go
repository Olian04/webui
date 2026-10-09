package webui

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"slices"
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

	ids   map[string][][]panel // view-state IDs taken on this page, and where
	scope []panel              // the tab panels the node being validated sits in
	tabs  int                  // Tabs seen so far, to tell them apart
	errs  []CompileError
}

func (v *bodyValidator) add(detail, fix string) {
	v.errs = append(v.errs, CompileError{Page: v.page, Args: v.args, Detail: detail, Fix: fix})
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// effectiveID is a leaf's ID, or its component's lower-case name when it has
// none: a Table is "table", a Tabs is "tabs".
func effectiveID(id, component string) string {
	if id != "" {
		return id
	}
	return component
}

// panel is one tab panel on the way down to a node: which Tabs, which panel.
type panel struct{ tabs, index int }

// exclusive reports whether two nodes sit in different panels of one Tabs, so
// they can never be on screen together. Their paths into the tree agree until
// the first Tabs they share, and part ways there.
func exclusive(a, b []panel) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] == b[i] {
			continue
		}
		return a[i].tabs == b[i].tabs
	}
	return false
}

// id checks the name a leaf's view state lives under in the address, and
// claims it. It must be a plain lower-case word, so "<id>.offset" is
// unambiguous, and unique within the page unless the claims are in different
// panels of one Tabs; two pages may reuse a name, since only one is in an
// address at a time. A leaf without an ID is claimed under its component name,
// so a second one on the page is a clash that says so.
func (v *bodyValidator) id(component, id string) {
	eff := effectiveID(id, strings.ToLower(component))
	if !idPattern.MatchString(eff) {
		v.add(fmt.Sprintf("%s.ID %q is not a lower-case word", component, eff),
			"Use letters, digits, '_' and '-', starting with a letter; the ID appears in the address.")
		return
	}
	if v.ids == nil {
		v.ids = map[string][][]panel{}
	}
	for _, prior := range v.ids[eff] {
		if exclusive(prior, v.scope) {
			continue
		}
		fix := "Give each table and tabs its own ID within a page."
		if id == "" {
			fix = fmt.Sprintf("Set ID on this %s; unnamed, it is called %q, as another leaf on this page already is.", component, eff)
		}
		v.add(fmt.Sprintf("the ID %q is used twice on this page", eff), fix)
		return
	}
	v.ids[eff] = append(v.ids[eff], slices.Clone(v.scope))
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
