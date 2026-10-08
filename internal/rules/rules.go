// Package rules compiles and checks the declarative constraints of a field.
//
// The same ir.Rules render as HTML constraint attributes in internal/render;
// this package is the server-side re-run. The pipeline order is parse → rules
// → Store, so Check parses numbers itself and reports a malformed number
// before any bound.
package rules

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/ir"
)

// Sentinels identify which constraint failed; Violation wraps one.
var (
	ErrRequired      = errors.New("value is required")
	ErrNotNumber     = errors.New("value is not a number")
	ErrMinimumLength = errors.New("value is too short")
	ErrMaximumLength = errors.New("value is too long")
	ErrMinimum       = errors.New("value is too small")
	ErrMaximum       = errors.New("value is too large")
	ErrPattern       = errors.New("value does not match pattern")
)

// Violation is one rejected value. Message is user-facing and goes beneath the
// field; Kind is a sentinel for errors.Is.
type Violation struct {
	Kind    error
	Message string
}

func (v *Violation) Error() string { return v.Message }

// Unwrap exposes the sentinel.
func (v *Violation) Unwrap() error { return v.Kind }

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
// the first violation, or nil. An empty value passes every constraint except
// Required, matching what the browser enforces.
func Check(r ir.Rules, kind ir.ValueKind, raw string) *Violation {
	if strings.TrimSpace(raw) == "" {
		if r.Required {
			return &Violation{ErrRequired, "This field is required."}
		}
		return nil
	}
	switch kind {
	case ir.KindString, ir.KindBool:
		return checkString(r, raw)
	case ir.KindInt, ir.KindInt64:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return &Violation{ErrNotNumber, "Must be a whole number."}
		}
		return checkNumber(r, float64(n))
	case ir.KindFloat:
		x, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return &Violation{ErrNotNumber, "Must be a number."}
		}
		return checkNumber(r, x)
	}
	return nil
}

func checkString(r ir.Rules, s string) *Violation {
	n := utf8.RuneCountInString(s)
	if r.MinLen > 0 && n < r.MinLen {
		return &Violation{ErrMinimumLength, fmt.Sprintf("Must be at least %d characters.", r.MinLen)}
	}
	if r.MaxLen > 0 && n > r.MaxLen {
		return &Violation{ErrMaximumLength, fmt.Sprintf("Must be at most %d characters.", r.MaxLen)}
	}
	if r.Pattern != nil && !r.Pattern.Expr.MatchString(s) {
		msg := r.Pattern.Message
		if msg == "" {
			msg = "Does not match the required format."
		}
		return &Violation{ErrPattern, msg}
	}
	return nil
}

func checkNumber(r ir.Rules, x float64) *Violation {
	if r.Min != nil && x < *r.Min {
		return &Violation{ErrMinimum, "Must be at least " + format(*r.Min) + "."}
	}
	if r.Max != nil && x > *r.Max {
		return &Violation{ErrMaximum, "Must be at most " + format(*r.Max) + "."}
	}
	return nil
}

func format(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
