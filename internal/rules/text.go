package rules

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/ir"
)

// A text rule is a condition on what a string starts with, ends with or contains: six of them, one for each way and its
// negation. The server tests them with the strings package. The browser has no attribute for them, and one for a
// pattern, so they are written as the pattern that they are, a JavaScript expression that a value matches when it holds
// them, and the field's pattern attribute is all of them and the pattern together (see [BrowserPattern]).
var textRules = []struct {
	name  string                // the field of the rules
	of    func(ir.Rules) string // what it says
	holds func(value, of string) bool
	msg   string                  // for %q of what it says: the violation
	line  string                  // the same, as a line in the list of rules of the field
	expr  func(lit string) string // what a value that holds it matches, for lit as an escaped literal
}{
	{"StartsWith", func(r ir.Rules) string { return r.StartsWith }, strings.HasPrefix,
		"Must start with %q.", "Starts with %q", func(lit string) string { return lit + anyText }},
	{"EndsWith", func(r ir.Rules) string { return r.EndsWith }, strings.HasSuffix,
		"Must end with %q.", "Ends with %q", func(lit string) string { return anyText + lit }},
	{"Contains", func(r ir.Rules) string { return r.Contains }, strings.Contains,
		"Must contain %q.", "Contains %q", func(lit string) string { return anyText + lit + anyText }},
	{"NotStartsWith", func(r ir.Rules) string { return r.NotStartsWith }, func(v, of string) bool { return !strings.HasPrefix(v, of) },
		"Must not start with %q.", "Does not start with %q", func(lit string) string { return "(?!" + lit + ")" + anyText }},
	{"NotEndsWith", func(r ir.Rules) string { return r.NotEndsWith }, func(v, of string) bool { return !strings.HasSuffix(v, of) },
		"Must not end with %q.", "Does not end with %q", func(lit string) string { return "(?!" + anyText + lit + "$)" + anyText }},
	{"NotContains", func(r ir.Rules) string { return r.NotContains }, func(v, of string) bool { return !strings.Contains(v, of) },
		"Must not contain %q.", "Does not contain %q", func(lit string) string { return "(?!" + anyText + lit + ")" + anyText }},
}

// anyText is every text, with a line end too: a dot is not that, and [^] is not read by every browser.
const anyText = `[\s\S]*`

// checkText is the message of the first text rule that the value does not hold, or "".
func checkText(r ir.Rules, s string) string {
	for _, t := range textRules {
		if of := t.of(r); of != "" && !t.holds(s, of) {
			return fmt.Sprintf(t.msg, of)
		}
	}
	return ""
}

// TextLines are the text rules of a field in words, for the list of its rules.
func TextLines(r ir.Rules) []string {
	var out []string
	for _, t := range textRules {
		if of := t.of(r); of != "" {
			out = append(out, fmt.Sprintf(t.line, of))
		}
	}
	return out
}

// HasText is whether the field has a text rule.
func HasText(r ir.Rules) bool {
	for _, t := range textRules {
		if t.of(r) != "" {
			return true
		}
	}
	return false
}

// Part is one condition of those that a pattern attribute holds, and what is said when a value does not hold it.
type Part struct {
	Expr    string // a JavaScript regular expression, which the whole value matches when it holds the condition
	Message string
}

// Parts are the conditions that make up the field's pattern, in the order that the server tests them, the pattern
// last: each text rule, and the pattern. The browser can only say that a value does not match the pattern attribute,
// which is all of them; the page says which, with these, in the words that the server uses.
func Parts(r ir.Rules) []Part {
	var out []Part
	for _, t := range textRules {
		if of := t.of(r); of != "" {
			out = append(out, Part{Expr: t.expr(escapeLiteral(of)), Message: fmt.Sprintf(t.msg, of)})
		}
	}
	if r.Pattern != nil {
		out = append(out, Part{Expr: r.Pattern.Source, Message: msgPattern(r.Pattern)})
	}
	return out
}

// BrowserPattern is the pattern attribute of the field, or "": the pattern as it was written when it is all there is, the
// expression of a text rule when it is the only one, and when there are more each one as a lookahead that must hold of
// the whole value, since a pattern attribute is one expression. Lookahead is for the browser alone: it is not
// something that a pattern may use, which the server runs too, and which is why the text rules are not patterns.
func BrowserPattern(r ir.Rules) string {
	parts := Parts(r)
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0].Expr
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString("(?=(?:" + p.Expr + ")$)")
	}
	b.WriteString(anyText)
	return b.String()
}

// escapeLiteral writes a text so that a JavaScript regular expression, with the u or the v flag, reads it as that text.
func escapeLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case strings.ContainsRune(`^$\.*+?()[]{}|/`, r):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f || r == 0x2028 || r == 0x2029:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Unsatisfiable are the text rules that no value can hold, each as what is wrong and what to do. A field with one can
// never be saved, which is a mistake of the app, and found when it starts.
func Unsatisfiable(r ir.Rules) (problems [][2]string) {
	add := func(what, fix string) { problems = append(problems, [2]string{what, fix}) }
	for _, t := range textRules {
		if of := t.of(r); strings.ContainsAny(of, "\r\n") {
			add(fmt.Sprintf("%s has a line break in it", t.name), "A value is one line, so no value can hold it. Use a text without one.")
		}
	}
	for _, p := range []struct {
		name, of string
	}{{"StartsWith", r.StartsWith}, {"EndsWith", r.EndsWith}, {"Contains", r.Contains}} {
		if p.of == "" {
			continue
		}
		if n := r.NotContains; n != "" && strings.Contains(p.of, n) {
			add(fmt.Sprintf("%s %q has NotContains %q in it", p.name, p.of, n), "A value that holds the first holds the second too, so no value is accepted. Change one of them.")
		}
		if r.MaxLen > 0 && utf8.RuneCountInString(p.of) > r.MaxLen {
			add(fmt.Sprintf("%s %q is longer than MaxLen, %d", p.name, p.of, r.MaxLen), "A value has at most MaxLen characters, so none can hold it. Raise MaxLen, or shorten the text.")
		}
	}
	if r.StartsWith != "" && r.NotStartsWith != "" && strings.HasPrefix(r.StartsWith, r.NotStartsWith) {
		add(fmt.Sprintf("StartsWith %q starts with NotStartsWith %q", r.StartsWith, r.NotStartsWith), "A value cannot start with the first and not with the second. Change one of them.")
	}
	if r.EndsWith != "" && r.NotEndsWith != "" && strings.HasSuffix(r.EndsWith, r.NotEndsWith) {
		add(fmt.Sprintf("EndsWith %q ends with NotEndsWith %q", r.EndsWith, r.NotEndsWith), "A value cannot end with the first and not with the second. Change one of them.")
	}
	return problems
}
