package webui

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// validate checks what the type system cannot, and never stops at the first
// problem: Compile reports every error at once so one fix cycle clears them
// all. It emits nothing and decides nothing about shape.
//
// Name derivation and RE2 compilation are not reimplemented here. validate
// calls internal/args and internal/rules, so it cannot disagree with lower
// about what an argument is called or whether a pattern is valid.
//
// validate and lower walk the same tree and can drift; the node counters on
// bodyValidator and bodyLowerer exist so a test can assert they visit the same
// number of nodes, which is cheaper than abstracting the traversal.
func (a App) validate() CompileErrors {
	f := a.collectFacts()
	var errs CompileErrors
	errs = append(errs, validateTheme(a.Theme)...)

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
	paths         map[string]int          // Path → how many pages declare it
	argTypes      map[string]reflect.Type // Path → argument type, for Link checks
	shadowTargets map[Nav]int             // Nav value → pages with it that can be shadowed
}

func (a App) collectFacts() *facts {
	f := &facts{
		paths:         map[string]int{},
		argTypes:      map[string]reflect.Type{},
		shadowTargets: map[Nav]int{},
	}
	for _, p := range a.Pages {
		if p == nil {
			continue
		}
		f.paths[p.pagePath()]++
		f.argTypes[p.pagePath()] = p.pageArgType()
		if n := p.pageNav(); n.Label != "" && n.Shadow == nil {
			f.shadowTargets[n]++
		}
	}
	return f
}

// bodyValidator carries one page's coordinates so every error from anywhere in
// its body names the page.
type bodyValidator struct {
	page  string // Path
	args  string // argument type name
	facts *facts

	ids   map[string]bool // view-state IDs taken on this page
	errs  []CompileError
	nodes int
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

// id checks the name a leaf's view state lives under in the address, and
// claims it. It must be a plain lower-case word, so "<id>.offset" is
// unambiguous, and unique within the page; two pages may reuse a name, since
// only one is in an address at a time. A leaf without an ID is claimed under
// its component name, so a second one on the page is a clash that says so.
func (v *bodyValidator) id(component, id string) {
	eff := effectiveID(id, strings.ToLower(component))
	if !idPattern.MatchString(eff) {
		v.add(fmt.Sprintf("%s.ID %q is not a lower-case word", component, eff),
			"Use letters, digits, '_' and '-', starting with a letter; the ID appears in the address.")
		return
	}
	if v.ids == nil {
		v.ids = map[string]bool{}
	}
	if v.ids[eff] {
		fix := "Give each table and tabs its own ID within a page."
		if id == "" {
			fix = fmt.Sprintf("Set ID on this %s; unnamed, it is called %q, as another leaf on this page already is.", component, eff)
		}
		v.add(fmt.Sprintf("the ID %q is used twice on this page", eff), fix)
		return
	}
	v.ids[eff] = true
}

// tokenValue is what a theme token may contain: colour-like characters only,
// so a token cannot close its declaration or open another.
func validTokenValue(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune(" #%.,()/_+-", r):
		default:
			return false
		}
	}
	return true
}

func validateTheme(t Theme) []CompileError {
	var errs []CompileError
	for name, value := range t.Tokens {
		if name == "" || strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
			errs = append(errs, CompileError{
				Detail: fmt.Sprintf("Theme token name %q is not lower-case letters, digits and dashes", name),
				Fix:    "Name the token as in the stylesheet without the leading dashes, such as \"primary\".",
			})
		}
		if !validTokenValue(value) {
			errs = append(errs, CompileError{
				Detail: fmt.Sprintf("Theme token %q has the value %q, which contains characters a colour does not", name, value),
				Fix:    "Use a colour or length such as #3d71d9 or rgb(61 113 217 / 14%).",
			})
		}
	}
	return errs
}
