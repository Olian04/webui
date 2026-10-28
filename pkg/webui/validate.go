package webui

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/Olian04/webui/internal/render/assets"
	"github.com/Olian04/webui/internal/theme"
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
	errs = append(errs, validateExternalLinks(a.ExternalLinks)...)
	if n := len(a.Secret); n > 0 && n < 16 {
		errs = append(errs, CompileError{
			Detail: fmt.Sprintf("App.Secret is %d bytes, which is too short to keep an address private", n),
			Fix:    "Use at least 16 random bytes, or leave Secret empty and the app makes one when it starts.",
		})
	}

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
	page    string // Path
	args    string // argument type name
	argType reflect.Type
	facts   *facts

	errs []CompileError

	// contexts is the text of each file of an editor's Context on this page, by the language
	// and path the service sees it at. A page's TypeScript service is one program, so two
	// editors cannot give one path two texts.
	contexts map[string]string

	// compilers is the options of each language's tsconfig.json on this page, which the page's
	// one service for that language has only one set of.
	compilers map[string]string
}

func (v *bodyValidator) add(detail, fix string) {
	v.errs = append(v.errs, CompileError{Page: v.page, Args: v.args, Detail: detail, Fix: fix})
}

var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func validateTheme(t Theme) []CompileError {
	var errs []CompileError
	bad := false
	check := func(name string, value Color) {
		if value != "" && !hexColor.MatchString(string(value)) {
			bad = true
			errs = append(errs, CompileError{
				Detail: fmt.Sprintf("Theme.%s is %q, which is not a hex colour", name, string(value)),
				Fix:    "Write it as #rgb, #rrggbb or #rrggbbaa, such as \"#3d71d9\".",
			})
		}
	}
	check("Accent", t.Accent)
	check("OK", t.OK)
	check("Warning", t.Warning)
	check("Critical", t.Critical)
	for _, m := range []struct {
		name   string
		colors ModeColors
	}{{"Light", t.Light}, {"Dark", t.Dark}} {
		check(m.name+".Canvas", m.colors.Canvas)
		check(m.name+".Accent", m.colors.Accent)
		check(m.name+".OK", m.colors.OK)
		check(m.name+".Warning", m.colors.Warning)
		check(m.name+".Critical", m.colors.Critical)
	}
	if bad {
		return errs // what is not a colour has no contrast to check
	}
	for _, p := range theme.Check(specOf(t)) {
		errs = append(errs, CompileError{Detail: p.Detail, Fix: p.Fix})
	}
	return errs
}

// specOf is the Theme as internal/theme reads it.
func specOf(t Theme) theme.Spec {
	mode := func(m ModeColors) theme.Colors {
		return theme.Colors{Canvas: string(m.Canvas), Accent: string(m.Accent), OK: string(m.OK), Warning: string(m.Warning), Critical: string(m.Critical)}
	}
	return theme.Spec{
		Accent: string(t.Accent), OK: string(t.OK), Warning: string(t.Warning), Critical: string(t.Critical),
		Light: mode(t.Light), Dark: mode(t.Dark),
	}
}

// validateExternalLinks checks the app's external links: every one has a label and an
// address that is a path on this site or a web or mail address, never a script.
func validateExternalLinks(items []ExternalLink) []CompileError {
	var errs []CompileError
	for i, it := range items {
		at := fmt.Sprintf("App.ExternalLinks[%d]", i)
		bad := func(detail, fix string) { errs = append(errs, CompileError{Detail: at + " " + detail, Fix: fix}) }
		if it.Label == "" {
			bad("has no Label", "Set Label; it is the entry's text.")
		}
		if it.Icon != "" && !assets.HasIcon(it.Icon) {
			bad(fmt.Sprintf("has the Icon %q, which is not a Font Awesome Free solid icon", it.Icon),
				"Use the name of a Font Awesome Free solid icon, such as \"book\", or leave Icon empty.")
		}
		if !validURL(it.URL) {
			bad(fmt.Sprintf("has the URL %q, which is not a path on this site, or an http, https or mailto address", it.URL),
				"Use a path such as \"/logout\", or an address such as \"https://example.com/help\".")
		}
	}
	return errs
}

// validURL is whether an address may be a link: a path on this site, or an http,
// https or mailto address. Nothing else is one, whatever it holds: a script or data
// address in a link is how a stored value becomes code.
func validURL(s string) bool {
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

// needLoad reports a body that has no Load. what is the body as the message names it, such as "a Form".
func needLoad(v *bodyValidator, what string, loads bool) {
	if !loads {
		v.add(what+" has no Load", "Set Load to func(ctx) (M, webui.Loaded).")
	}
}

// validateLanguage reports a Language that is not one the editor highlights. where is the body, such as "Editor".
func validateLanguage(v *bodyValidator, where string, l Language) {
	if !l.known() {
		v.add(fmt.Sprintf("%s.Language %q is not a language the editor highlights", where, string(l)),
			"Use one of the Lang constants, such as LangJSON or LangGo; there is one for each language the editor knows.")
	}
}

// eraseLoad is a body's Load with its model's type erased, as the IR holds it.
func eraseLoad[M any](load func(context.Context) (M, Loaded)) func(context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		model, loaded := load(ctx)
		return model, loaded.apply(ctx)
	}
}
