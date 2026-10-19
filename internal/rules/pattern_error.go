package rules

import (
	"errors"
	"fmt"
	"regexp/syntax"
	"strings"
)

// PatternError is a pattern that cannot be compiled, said in terms of what is in it and what to write instead. Go's own
// message names the dialect it was reading ("invalid or unsupported Perl syntax") and a fragment of what it was given, which says
// little to someone who wrote a pattern the way they would in any other language.
type PatternError struct {
	Expr    string // the pattern, as it was written
	Problem string // what is wrong with it, to follow "pattern X ": "uses a lookahead, (?=, which RE2 does not support"
	Fix     string // what to do about it
	err     error  // Go's own error
}

func (e *PatternError) Error() string { return "pattern " + quote(e.Expr) + " " + e.Problem }

// Unwrap is Go's own error, for whoever wants the detail.
func (e *PatternError) Unwrap() error { return e.err }

// quote puts a fragment of a pattern between backticks, as written, and not as a Go string with its backslashes doubled.
func quote(s string) string {
	if strings.Contains(s, "`") {
		return fmt.Sprintf("%q", s)
	}
	return "`" + s + "`"
}

const (
	whyRE2 = "RE2 matches in time that grows with the length of the value, which is why a pattern may not have it. "
	other  = "If the condition is about the whole value (a digit somewhere, a length), check it with another rule, such as MinLen and MaxLen, or in Run."
)

// explain says what Go's error means for the pattern, for the mistakes that are known, and falls back to Go's own words.
func explain(expr string, err error) *PatternError {
	pe := &PatternError{Expr: expr, err: err}
	var se *syntax.Error
	if !errors.As(err, &se) {
		pe.Problem = "is not valid RE2: " + err.Error()
		pe.Fix = "Rewrite the pattern in RE2 syntax."
		return pe
	}
	at := quote(se.Expr)
	switch se.Code {
	case syntax.ErrInvalidPerlOp:
		switch {
		case strings.HasPrefix(se.Expr, "(?=") || strings.HasPrefix(se.Expr, "(?!"):
			pe.Problem = "uses a lookahead, " + at + ", which RE2 does not support"
			pe.Fix = whyRE2 + "Say what a match looks like instead of what must follow it. " + other
		case strings.HasPrefix(se.Expr, "(?<=") || strings.HasPrefix(se.Expr, "(?<!"):
			pe.Problem = "uses a lookbehind, " + at + ", which RE2 does not support"
			pe.Fix = whyRE2 + "Say what a match looks like instead of what must come before it. " + other
		case strings.HasPrefix(se.Expr, "(?>"):
			pe.Problem = "uses an atomic group, " + at + ", which RE2 does not support"
			pe.Fix = whyRE2 + "A plain group, (?:...), matches the same values here."
		case strings.HasPrefix(se.Expr, "(?#"):
			pe.Problem = "has a comment group, " + at + ", which RE2 does not support"
			pe.Fix = "Remove it; the pattern is written in Go, where a comment can go."
		default:
			pe.Problem = "uses " + at + ", which RE2 does not support"
			pe.Fix = "Rewrite the pattern without it; RE2 has the common Perl syntax (\\d, \\w, (?:...), lazy *?) and not all of it."
		}
	case syntax.ErrInvalidEscape:
		if len(se.Expr) == 2 && se.Expr[0] == '\\' && se.Expr[1] >= '1' && se.Expr[1] <= '9' {
			pe.Problem = "uses a backreference, " + at + ", which RE2 does not support"
			pe.Fix = whyRE2 + "A value that repeats an earlier part of itself cannot be matched by a pattern here: " + other
		} else {
			pe.Problem = "has an escape that RE2 does not know, " + at
			pe.Fix = "Put a backslash before punctuation to match it literally, and use \\d, \\w, \\s, \\b and the like for classes; a backslash before another letter is an error."
		}
	case syntax.ErrMissingParen:
		pe.Problem = "has a ( that is never closed"
		pe.Fix = "Close it with ), or write \\( for a literal one."
	case syntax.ErrUnexpectedParen:
		pe.Problem = "has a ) that closes nothing"
		pe.Fix = "Remove it, or write \\) for a literal one."
	case syntax.ErrMissingBracket:
		pe.Problem = "has a [ that is never closed"
		if se.Expr != expr {
			pe.Problem += ", at " + at // where, when it is not all of the pattern
		}
		pe.Fix = "Close it with ], or write \\[ for a literal one."
	case syntax.ErrInvalidCharRange:
		if strings.HasPrefix(se.Expr, "[:") && strings.HasSuffix(se.Expr, ":]") {
			// Go reports an unknown [:name:] as a bad range.
			pe.Problem = "has a character class that RE2 does not know, " + at
			pe.Fix = "Use a class such as \\d, \\w, \\s, or a named one such as [[:alpha:]] or \\pL."
			break
		}
		pe.Problem = "has a character range that is not valid, " + at
		pe.Fix = "Write the lower character first, as in a-z."
	case syntax.ErrInvalidCharClass:
		pe.Problem = "has a character class that RE2 does not know, " + at
		pe.Fix = "Use a class such as \\d, \\w, \\s, or a named one such as [[:alpha:]] or \\pL."
	case syntax.ErrMissingRepeatArgument:
		pe.Problem = "has " + at + " with nothing before it to repeat"
		pe.Fix = "Put what is to be repeated before it, or write a backslash before it for a literal one."
	case syntax.ErrInvalidRepeatOp:
		pe.Problem = "repeats a repetition, " + at
		pe.Fix = "Repeat each thing once, with one of *, +, ? or {n,m}. RE2 has no possessive quantifiers; (?:a+)* says it with a group."
	case syntax.ErrInvalidRepeatSize:
		pe.Problem = "asks for more repetitions than RE2 allows (1000), " + at
		pe.Fix = "Use a smaller count, or check the length with MinLen and MaxLen."
	case syntax.ErrTrailingBackslash:
		pe.Problem = "ends with a lone backslash"
		pe.Fix = "Write \\\\ for a literal backslash."
	case syntax.ErrInvalidUTF8:
		pe.Problem = "is not valid UTF-8"
		pe.Fix = "Write the pattern in UTF-8."
	case syntax.ErrInvalidNamedCapture:
		if strings.HasPrefix(se.Expr, "(?<=") || strings.HasPrefix(se.Expr, "(?<!") {
			// Go reads "(?<" as the start of a named group, so a lookbehind arrives here.
			pe.Problem = "uses a lookbehind, " + quote(se.Expr[:4]) + ", which RE2 does not support"
			pe.Fix = whyRE2 + "Say what a match looks like instead of what must come before it. " + other
			break
		}
		pe.Problem = "has a named group that is not valid, " + at
		pe.Fix = "Write it as (?P<name>...) or (?<name>...), with a name of letters, digits and underscores."
	case syntax.ErrNestingDepth:
		pe.Problem = "nests groups too deeply"
		pe.Fix = "Flatten the pattern."
	case syntax.ErrLarge:
		pe.Problem = "is too large for RE2"
		pe.Fix = "Shorten the pattern, or check part of it with another rule."
	case syntax.ErrInternalError:
		fallthrough
	default:
		pe.Problem = "is not valid RE2: " + err.Error()
		pe.Fix = "Rewrite the pattern in RE2 syntax."
	}
	return pe
}
