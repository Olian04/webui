package webui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

func TestValidatePatternAcceptsWhatAPatternRuleCanHave(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{`\d{1,3}(\.\d{1,3}){3}`, `[a-z0-9\-]+`, `https?://[^ ]+`, `\p{Lu}\p{Ll}+`, `ab|cd`, ``} {
		assert.NoError(t, webui.ValidatePattern(expr))
	}
}

// What it refuses is what Compile refuses, said as it is there: what was found, and what to write instead.
func TestValidatePatternSaysWhatIsWrongAndWhatToWriteInstead(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ expr, problem, fix string }{
		"lookahead":      {`(?=.*\d)[a-z0-9]+`, "uses a lookahead", "text rule"},
		"backreference":  {`(a)\1`, "uses a backreference", "RE2"},
		"unclosed group": {`(ab`, "has a ( that is never closed", "Close it"},
		"property":       {`\pL`, "uses `\\pL`", `\p{L}`},
		"flags":          {`(?i)abc`, "a group with flags", "[Aa]"},
		"class":          {`[(]`, "uses `(` in a class", `\(`},
		"escape":         {`\A`, "which a browser does not have", "^"},
	} {
		err := webui.ValidatePattern(tc.expr)
		var pe *webui.PatternError
		if !errors.As(err, &pe) {
			t.Errorf("%s: %q gave %v, want a *PatternError", name, tc.expr, err)
			continue
		}
		assert.Equal(t, pe.Expr, tc.expr)
		if !strings.Contains(pe.Problem, tc.problem) {
			t.Errorf("%s: problem %q does not hold %q", name, pe.Problem, tc.problem)
		}
		if !strings.Contains(pe.Fix, tc.fix) {
			t.Errorf("%s: fix %q does not hold %q", name, pe.Fix, tc.fix)
		}
		assert.True(t, strings.HasPrefix(err.Error(), "pattern "))
	}
}

// Go's own error is under it, for a pattern that Go refuses.
func TestValidatePatternKeepsGosOwnErrorUnderItsOwn(t *testing.T) {
	t.Parallel()

	err := webui.ValidatePattern(`(ab`)
	assert.True(t, errors.Unwrap(err) != nil)
	assert.True(t, strings.Contains(errors.Unwrap(err).Error(), "missing closing )"))
}

// The same patterns, in an app, are what Compile refuses, so the two cannot disagree.
func TestValidatePatternAgreesWithCompile(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{`[a-z]+`, `(?=x)`, `\pL`, `a{`, `ab|cd`} {
		field := formIP
		field.Rules.Pattern = &webui.PatternRule{Expr: expr}
		f := okForm()
		f.Fields = []webui.Accessor[Device]{field}
		_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/a", Body: f}}}.Compile("/admin")
		if (err != nil) != (webui.ValidatePattern(expr) != nil) {
			t.Errorf("%q: Compile says %v, ValidatePattern says %v", expr, err, webui.ValidatePattern(expr))
		}
	}
}
