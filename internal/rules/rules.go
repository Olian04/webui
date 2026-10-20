// Package rules compiles and checks the declarative constraints of a field.
//
// The same ir.Rules render as HTML constraint attributes in internal/render;
// this package is the server-side re-run. The pipeline order is parse → rules
// → Store, so Check parses numbers itself and reports a malformed number
// before any bound.
package rules

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/ir"
)

// Compile builds an anchored Pattern. Go's regexp is RE2: lookahead and backreferences
// are rejected, and so is anything else it cannot read, which the error, a [PatternError], says in terms of the pattern
// and what to write instead, rather than "invalid regex".
func Compile(expr, message string) (*ir.Pattern, error) {
	// The pattern alone first, so that what is reported is about what was written, and so that a pattern that is
	// not balanced cannot close the group it is put in and change what is anchored ("a)|(b" is not a pattern).
	if _, err := regexp.Compile(expr); err != nil {
		return nil, explain(expr, err)
	}
	// Then in the syntax that a browser reads too, since the pattern is also the input's pattern attribute, and what it
	// does not read is ignored without a word. What Go is given is the pattern written as the browser reads it.
	gx, pe := portable(expr)
	if pe != nil {
		return nil, pe
	}
	// Anchored, because an HTML pattern attribute must match the whole value
	// and the server has to reach the same verdict as the browser.
	re, err := regexp.Compile(`^(?:` + gx + `)$`)
	if err != nil {
		return nil, explain(expr, err)
	}
	return &ir.Pattern{Expr: re, Source: expr, Message: message}, nil
}

// The words of each violation, in one place: Check says them beneath a field, and Validity hands them to the browser,
// so that a field refused on the page says what it says when it is refused by the server.
const (
	msgRequired    = "This field is required."
	msgWholeNumber = "Must be a whole number."
	msgNumber      = "Must be a number."
	msgFormat      = "Does not match the required format."
)

func msgMinLen(n int) string  { return fmt.Sprintf("Must be at least %d characters.", n) }
func msgMaxLen(n int) string  { return fmt.Sprintf("Must be at most %d characters.", n) }
func msgMin(x float64) string { return "Must be at least " + format(x) + "." }

// msgStep says what a value that is not a step from the minimum should be: a multiple of the step, or the minimum and
// that many steps.
func msgStep(r ir.Rules) string {
	if r.Step == 1 && stepBase(r) == 0 {
		return msgWholeNumber
	}
	if base := stepBase(r); base != 0 {
		return "Must be " + format(base) + " plus a multiple of " + format(r.Step) + "."
	}
	return "Must be a multiple of " + format(r.Step) + "."
}

// stepBase is where the steps are counted from, as for an HTML input: its minimum.
func stepBase(r ir.Rules) float64 {
	if r.Min != nil {
		return *r.Min
	}
	return 0
}

// offStep is whether a number is not a whole number of steps from the base. The browser counts in decimals, so a
// little room is left for the binary ones: 0.3 is three steps of 0.1.
func offStep(r ir.Rules, x float64) bool {
	if r.Step <= 0 {
		return false
	}
	n := (x - stepBase(r)) / r.Step
	return math.Abs(n-math.Round(n)) > 1e-9*math.Max(1, math.Abs(n))
}
func msgMax(x float64) string { return "Must be at most " + format(x) + "." }

func msgPattern(p *ir.Pattern) string {
	if p.Message == "" {
		return msgFormat
	}
	return p.Message
}

// Validity is what the browser says for a field that fails a constraint, by the names of the properties of the
// [ValidityState] that it reports it by (valueMissing, tooShort, patternMismatch, rangeUnderflow, ...), for the
// constraints the field has: the same words as Check, so that the page and the server agree. It is for the
// Constraint Validation API's setCustomValidity, which is how a page says what it means in place of "Please match the
// format requested." A field that has no constraint has no entry, and one that is not a number, a text or a choice
// is given the kind of a text.
func Validity(r ir.Rules, kind ir.ValueKind) map[string]string {
	out := map[string]string{}
	if r.Required {
		out["valueMissing"] = msgRequired
	}
	switch kind {
	case ir.KindString:
		// The browser flags a value of an email or a web address input that is not one; the others it does not check.
		if r.Format == ir.FormatEmail || r.Format == ir.FormatURL {
			out["typeMismatch"] = formatMessage(r.Format)
		}
		if r.MinLen > 0 {
			out["tooShort"] = msgMinLen(r.MinLen)
		}
		if r.MaxLen > 0 {
			out["tooLong"] = msgMaxLen(r.MaxLen)
		}
		// The text rules and the pattern are all the pattern attribute. When there is more than one the browser cannot say
		// which a value fails, and this says all of them; the page tells them apart with Parts.
		if parts := Parts(r); len(parts) > 0 {
			msgs := make([]string, len(parts))
			for i, p := range parts {
				msgs[i] = p.Message
			}
			out["patternMismatch"] = strings.Join(msgs, " ")
		}
	case ir.KindInt, ir.KindFloat:
		bad := msgNumber
		if kind == ir.KindInt {
			bad = msgWholeNumber
			out["stepMismatch"] = msgWholeNumber // a number with a fraction, for a whole number
		}
		out["badInput"] = bad
		if r.Step > 0 && kind == ir.KindFloat {
			out["stepMismatch"] = msgStep(r)
		}
		if r.Min != nil {
			out["rangeUnderflow"] = msgMin(*r.Min)
		}
		if r.Max != nil {
			out["rangeOverflow"] = msgMax(*r.Max)
		}
	}
	return out
}

// Check applies rules to one raw submitted value of the given kind and returns
// the first violation, or "". A violation is user-facing, and goes beneath the field. An empty value passes every constraint except
// Required, matching what the browser enforces.
func Check(r ir.Rules, kind ir.ValueKind, raw string) string {
	if strings.TrimSpace(raw) == "" {
		if r.Required {
			return msgRequired
		}
		return ""
	}
	switch kind {
	case ir.KindString:
		return checkString(r, raw)
	case ir.KindInt:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return msgWholeNumber
		}
		return checkNumber(r, float64(n))
	case ir.KindFloat:
		x, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return msgNumber
		}
		return checkNumber(r, x)
	}
	return ""
}

// CheckSelect checks what was submitted for a select, the values joined by ListSep: at most
// one unless the field takes several, each once, and each one that the field offers. A browser
// only sends what it offered, so anything else is a request made by hand, and is refused.
func CheckSelect(f ir.Field, raw string) string {
	values := ir.SplitValues(raw)
	if !f.Multi && len(values) > 1 {
		return "Choose only one."
	}
	seen := map[string]bool{}
	for _, v := range values {
		if seen[v] {
			return "Choose each option once."
		}
		seen[v] = true
		if len(f.Choices) > 0 {
			if _, ok := f.Choice(v); !ok {
				return "Choose from the options offered."
			}
		}
	}
	return ""
}

func checkString(r ir.Rules, s string) string {
	if msg := checkFormat(r.Format, s); msg != "" {
		return msg
	}
	n := utf8.RuneCountInString(s)
	if r.MinLen > 0 && n < r.MinLen {
		return msgMinLen(r.MinLen)
	}
	if r.MaxLen > 0 && n > r.MaxLen {
		return msgMaxLen(r.MaxLen)
	}
	if msg := checkText(r, s); msg != "" {
		return msg
	}
	if r.Pattern != nil && !r.Pattern.Expr.MatchString(s) {
		return msgPattern(r.Pattern)
	}
	return ""
}

func checkNumber(r ir.Rules, x float64) string {
	if offStep(r, x) { // the browser says it before the bounds
		return msgStep(r)
	}
	if r.Min != nil && x < *r.Min {
		return msgMin(*r.Min)
	}
	if r.Max != nil && x > *r.Max {
		return msgMax(*r.Max)
	}
	return ""
}

func format(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
