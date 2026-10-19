package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// The mistakes that are known are said in terms of the pattern, with what to write instead, and not in Go's words, which name a
// dialect ("invalid or unsupported Perl syntax") and a fragment of a pattern that is wrapped in more than the author wrote.
func TestAKnownMistakeInAPatternIsExplained(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		expr    string
		problem string // what the message says, after "pattern `...` "
		fix     string // a piece of what to do
	}{
		"lookahead":            {`(?=.*\d)[a-z0-9]+`, "uses a lookahead, `(?=`, which RE2 does not support", "MinLen and MaxLen"},
		"negative lookahead":   {`a(?!b)`, "uses a lookahead, `(?!`, which RE2 does not support", "what a match looks like"},
		"lookbehind":           {`(?<=a)b`, "uses a lookbehind, `(?<=`, which RE2 does not support", "what a match looks like"},
		"negative lookbehind":  {`(?<!a)b`, "uses a lookbehind, `(?<!`, which RE2 does not support", "what a match looks like"},
		"atomic group":         {`(?>a+)b`, "uses an atomic group, `(?>`, which RE2 does not support", "(?:...)"},
		"backreference":        {`(a)\1`, "uses a backreference, `\\1`, which RE2 does not support", "another rule"},
		"possessive":           {`a++`, "repeats a repetition, `++`", "(?:a+)*"},
		"nothing to repeat":    {`*a`, "has `*` with nothing before it to repeat", "before it"},
		"unclosed paren":       {`(abc`, "has a ( that is never closed", `\(`},
		"unmatched paren":      {`abc)`, "has a ) that closes nothing", `\)`},
		"unclosed bracket":     {`[abc`, "has a [ that is never closed", `\[`},
		"backwards range":      {`[z-a]`, "has a character range that is not valid, `z-a`", "lower character first"},
		"unknown escape":       {`\y`, "has an escape that RE2 does not know, `\\y`", "\\d, \\w"},
		"trailing backslash":   {`abc\`, "ends with a lone backslash", "\\\\"},
		"too many repetitions": {`a{1001}`, "asks for more repetitions than RE2 allows (1000), `{1001}`", "MinLen and MaxLen"},
		"unknown class":        {`[[:nope:]]`, "has a character class that RE2 does not know, `[:nope:]`", "[[:alpha:]]"},
		"bad named group":      {`(?P<a-b>x)`, "has a named group that is not valid, `(?P<a-b>`", "(?P<name>"},
		"a repeat of a repeat": {`a**`, "repeats a repetition, `**`", "one of *, +, ? or {n,m}"},
	} {
		_, err := Compile(c.expr, "")
		if err == nil {
			t.Errorf("%s: %q was accepted", name, c.expr)
			continue
		}
		var pe *PatternError
		if !errors.As(err, &pe) {
			t.Errorf("%s: %v is not a PatternError", name, err)
			continue
		}
		assert.Equal(t, pe.Error(), "pattern `"+c.expr+"` "+c.problem)
		assert.True(t, strings.Contains(pe.Fix, c.fix))
		assert.False(t, strings.Contains(pe.Error(), "Perl")) // Go's wording names the dialect, ours says what is in the pattern
		assert.NotNil(t, errors.Unwrap(err))                  // and Go's own error is still there for whoever wants it
	}
}

// What is reported is about the pattern as it was written, and not about the group it is put in to be anchored: a pattern that is
// not balanced cannot close that group and change what is anchored, as "a)|(b" would (^(?:a)|(b)$ is two alternatives, one of
// them not anchored at the end).
func TestAPatternThatIsNotBalancedCannotEscapeItsAnchors(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{`a)|(b`, `x)|(?:y`, `)(`} {
		_, err := Compile(expr, "")
		if err == nil {
			t.Errorf("%q was accepted", expr)
		}
	}
	p, err := Compile(`ab|cd`, "")
	assert.NoError(t, err)
	assert.False(t, p.Expr.MatchString("abx")) // both alternatives are anchored
	assert.True(t, p.Expr.MatchString("cd"))
}

// A message that is not one of the known ones is still a PatternError, in Go's words.
func TestAnUnknownMistakeFallsBackToGosWords(t *testing.T) {
	t.Parallel()

	pe := explain(`x`, errors.New("something new"))
	assert.Equal(t, pe.Error(), "pattern `x` is not valid RE2: something new")
	assert.Contains(t, pe.Fix, "RE2")
}
