package webui

import (
	"errors"

	"github.com/Olian04/webui/internal/rules"
)

// PatternError is a pattern that a [PatternRule] cannot have, said in terms of what is in it and what to write instead.
// It is what [ValidatePattern] returns, and what [App.Compile] says of a pattern it refuses.
type PatternError struct {
	// Expr is the pattern, as it was written.
	Expr string

	// Problem is what is wrong with it, to follow "pattern X ": "uses a lookahead, `(?=`, which RE2 does not support".
	Problem string

	// Fix is what to do about it.
	Fix string

	message string
	cause   error
}

func (e *PatternError) Error() string { return e.message }

// Unwrap is the error of Go's regexp package, when the pattern is one that it refuses, for whoever wants its detail.
func (e *PatternError) Unwrap() error { return e.cause }

// ValidatePattern says whether expr is a pattern that a [PatternRule] can have, which is the same question that
// [App.Compile] asks of each pattern it is given. It returns nil, or a [*PatternError]. It is for a program that gets
// patterns from somewhere else, such as a schema, and wants to leave out the ones that are refused, and not have them fail
// the whole app. See "Text rules and patterns" in the package documentation for what a pattern can be.
func ValidatePattern(expr string) error {
	_, err := rules.Compile(expr, "")
	if err == nil {
		return nil
	}
	if pe, ok := errors.AsType[*rules.PatternError](err); ok {
		return &PatternError{Expr: pe.Expr, Problem: pe.Problem, Fix: pe.Fix, message: pe.Error(), cause: pe.Unwrap()}
	}
	return &PatternError{Expr: expr, Problem: err.Error(), Fix: "Rewrite the pattern.", message: err.Error(), cause: err}
}
