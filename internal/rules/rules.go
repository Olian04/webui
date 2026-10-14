// Package rules compiles and checks the declarative constraints of a field.
//
// The same ir.Rules render as HTML constraint attributes in internal/render;
// this package is the server-side re-run. The pipeline order is parse → rules
// → Store, so Check parses numbers itself and reports a malformed number
// before any bound.
package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/ir"
)

// Compile builds an anchored Pattern. Go's regexp is RE2: lookahead and backreferences
// are rejected, which the error says rather than "invalid regex".
func Compile(expr, message string) (*ir.Pattern, error) {
	// Anchored, because an HTML pattern attribute must match the whole value
	// and the server has to reach the same verdict as the browser.
	re, err := regexp.Compile(`^(?:` + expr + `)$`)
	if err != nil {
		return nil, fmt.Errorf("pattern %q is not valid RE2 (no lookahead or backreferences): %w", expr, err)
	}
	return &ir.Pattern{Expr: re, Source: expr, Message: message}, nil
}

// Check applies rules to one raw submitted value of the given kind and returns
// the first violation, or "". A violation is user-facing, and goes beneath the field. An empty value passes every constraint except
// Required, matching what the browser enforces.
func Check(r ir.Rules, kind ir.ValueKind, raw string) string {
	if strings.TrimSpace(raw) == "" {
		if r.Required {
			return "This field is required."
		}
		return ""
	}
	switch kind {
	case ir.KindString:
		return checkString(r, raw)
	case ir.KindInt:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return "Must be a whole number."
		}
		return checkNumber(r, float64(n))
	case ir.KindFloat:
		x, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return "Must be a number."
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
	n := utf8.RuneCountInString(s)
	if r.MinLen > 0 && n < r.MinLen {
		return fmt.Sprintf("Must be at least %d characters.", r.MinLen)
	}
	if r.MaxLen > 0 && n > r.MaxLen {
		return fmt.Sprintf("Must be at most %d characters.", r.MaxLen)
	}
	if r.Pattern != nil && !r.Pattern.Expr.MatchString(s) {
		msg := r.Pattern.Message
		if msg == "" {
			msg = "Does not match the required format."
		}
		return msg
	}
	return ""
}

func checkNumber(r ir.Rules, x float64) string {
	if r.Min != nil && x < *r.Min {
		return "Must be at least " + format(*r.Min) + "."
	}
	if r.Max != nil && x > *r.Max {
		return "Must be at most " + format(*r.Max) + "."
	}
	return ""
}

func format(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
