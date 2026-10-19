package rules

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

// corpus is what testdata/gen.mjs recorded of a browser's engine: for each pattern, whether new RegExp(...) took it with
// the "v" flag and with the "u" flag (the one that a browser without "v" uses), and for each input, whether it matched.
type corpus struct {
	Engine   string
	White    [][2]rune // the code points that \s matches
	NotDot   [][2]rune // the ones that a dot does not
	Inputs   []string
	Patterns []struct {
		Pattern string
		V, U    struct {
			OK      bool
			Matches string
		}
	}
}

func loadCorpus(t *testing.T) corpus {
	t.Helper()
	b, err := os.ReadFile("testdata/portable.json")
	if err != nil {
		t.Fatal(err)
	}
	var c corpus
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// compiled is the expression for Go that Compile makes, and whether it did.
func compiled(expr string) (*regexp.Regexp, bool) {
	p, err := Compile(expr, "")
	if err != nil {
		return nil, false
	}
	return p.Expr, true
}

// TestThePortableSubsetIsWhatBothEnginesAgreeOn holds Compile to the browser: a pattern it accepts is read by both flags and
// matches the same values as Go does, over every input in the corpus. Nothing that a browser would ignore, or read
// otherwise, is let through.
func TestThePortableSubsetIsWhatBothEnginesAgreeOn(t *testing.T) {
	c := loadCorpus(t)
	accepted := 0
	for _, p := range c.Patterns {
		re, ok := compiled(p.Pattern)
		if !ok {
			continue
		}
		accepted++
		if !p.V.OK || !p.U.OK {
			t.Errorf("%q is accepted, and a browser refuses it (v: %v, u: %v)", p.Pattern, p.V.OK, p.U.OK)
			continue
		}
		if p.V.Matches != p.U.Matches {
			t.Errorf("%q is accepted, and the flags v and u differ on it", p.Pattern)
		}
		for i, in := range c.Inputs {
			want := p.V.Matches[i] == '1'
			if got := re.MatchString(in); got != want {
				t.Errorf("%q on %q: Go says %v, the browser says %v", p.Pattern, in, got, want)
			}
		}
	}
	if accepted < 100 {
		t.Errorf("only %d patterns of the corpus are accepted: the check has become too narrow to be a subset worth having", accepted)
	}
}

// narrower are the patterns that both engines read alike on the corpus and that are refused anyway, each with the reason.
// A pattern that is refused and not here is one that an engine reads differently or not at all, which the test above
// holds Compile to; this is the rest, so that what is left out on purpose is written down.
var narrower = map[string]string{
	`(?<x>a)`:              "a named group has no use in a pattern",
	`(?i:a)`:               "flags in a group are newer than the browsers that have the v flag; [Aa] says it",
	`(?s:.)`:               "flags in a group, as above",
	`\0`:                   "a browser refuses \\0 before a digit; write \\x00",
	`\p{C}`:                "what is unassigned depends on the version of Unicode of the engine",
	`\p{Cn}`:               "as \\p{C}",
	`\p{Co}`:               "as \\p{C}",
	`\p{Cs}`:               "as \\p{C}",
	`\p{Letter}`:           "a long name, which Go has only from a later version than the oldest that the library supports",
	`\p{Lowercase_Letter}`: "as \\p{Letter}",
	`\p{Any}`:              "write a dot, or [\\s\\S]",
	`\p{ASCII}`:            "write [\\x00-\\x7f]",
}

func TestWhatIsLeftOutOnPurposeIsWrittenDown(t *testing.T) {
	c := loadCorpus(t)
	for _, p := range c.Patterns {
		if _, ok := compiled(p.Pattern); ok || !p.V.OK || !p.U.OK || p.V.Matches != p.U.Matches {
			continue
		}
		re, err := regexp.Compile(`^(?:` + p.Pattern + `)$`)
		if err != nil {
			continue
		}
		same := true
		for i, in := range c.Inputs {
			same = same && re.MatchString(in) == (p.V.Matches[i] == '1')
		}
		if _, listed := narrower[p.Pattern]; same && !listed {
			t.Errorf("%q reads alike in both engines and is refused, and is not said to be on purpose", p.Pattern)
		}
	}
	for expr, why := range narrower {
		if _, ok := compiled(expr); ok {
			t.Errorf("%q is listed as left out (%s) and is accepted", expr, why)
		}
	}
}

// TestWhiteSpaceAndTheDotAreWhatABrowserMeansByThem is every code point: Go's \s and dot are not a browser's, so Compile
// writes them out as the characters that the browser's are.
func TestWhiteSpaceAndTheDotAreWhatABrowserMeansByThem(t *testing.T) {
	c := loadCorpus(t)
	in := func(rs [][2]rune, r rune) bool {
		for _, x := range rs {
			if r >= x[0] && r <= x[1] {
				return true
			}
		}
		return false
	}
	for _, tc := range []struct {
		expr string
		want func(rune) bool
	}{
		{`\s`, func(r rune) bool { return in(c.White, r) }},
		{`\S`, func(r rune) bool { return !in(c.White, r) }},
		{`[\s]`, func(r rune) bool { return in(c.White, r) }},
		{`[^\s]`, func(r rune) bool { return !in(c.White, r) }},
		{`[\S]`, func(r rune) bool { return !in(c.White, r) }},
		{`[^\S]`, func(r rune) bool { return in(c.White, r) }},
		{`.`, func(r rune) bool { return !in(c.NotDot, r) }},
		{`[^.]`, func(r rune) bool { return r != '.' }},
	} {
		re, ok := compiled(tc.expr)
		if !ok {
			t.Fatalf("%q is refused", tc.expr)
		}
		for r := rune(0); r <= 0x10ffff; r++ {
			if r >= 0xd800 && r <= 0xdfff {
				continue
			}
			if got := re.MatchString(string(r)); got != tc.want(r) {
				t.Errorf("%q on U+%04X: Go says %v, the browser says %v", tc.expr, r, got, tc.want(r))
				break
			}
		}
	}
}

// TestAConstructThatABrowserDoesNotReadIsSaid is the words of the refusals, for what is written most often by someone who
// knows Go, or Perl, or Python.
func TestAConstructThatABrowserDoesNotReadIsSaid(t *testing.T) {
	for name, tc := range map[string]struct{ expr, problem, fix string }{
		"begin text":          {`\Aa`, "uses `\\A`, which a browser does not have", "write ^ and $"},
		"end text":            {`a\z`, "uses `\\z`, which a browser does not have", "write ^ and $"},
		"quote":               {`\Qa.b\E`, "uses `\\Q`, which a browser does not have", "a\\.b"},
		"python named group":  {`(?P<x>a)`, "uses `(?P<`", "(?:...)"},
		"flags":               {`(?i)a`, "uses `(?i)`, a group with flags", "[Aa]"},
		"property no braces":  {`\pL`, "uses `\\pL`, which a browser does not read without braces", `\p{L}`},
		"script":              {`\p{Greek}`, "uses `\\p{Greek}`, which is not a name that Go and a browser both know", "[α-ω]"},
		"escaped letter":      {`\a`, "uses `\\a`, which is not an escape that both Go and a browser read", "\\xHH"},
		"hex braces":          {`\x{41}`, "uses `\\x{`", `\xHH`},
		"escaped punctuation": {`\-`, "uses `\\-`, a backslash before a character that has no meaning", "Write `-` without the backslash"},
		"escaped underscore":  {`\_`, "uses `\\_`, a backslash before a character that has no meaning", "without the backslash"},
		"octal":               {`\012`, "uses `\\0`", "\\xHH"},
		"loose brace":         {`a{`, "uses `{`", `\{`},
		"count without min":   {`a{,3}`, "uses `{`", `{n,m}`},
		"loose bracket":       {`a]`, "uses `]`", `\]`},
		"loose close brace":   {`a}`, "uses `}`", `\}`},
		"repeat an anchor":    {`^*`, "uses `^*`, which repeats something that is not a character", "remove it"},
		"repeat a boundary":   {`\b+`, "uses `\\b+`, which repeats something that is not a character", "remove it"},
		"class dash first":    {`[-a]`, "uses `-` in a class", `\-`},
		"class dash last":     {`[a-]`, "uses `-` in a class", `\-`},
		"class paren":         {`[(]`, "uses `(` in a class", `\(`},
		"class slash":         {`[/]`, "uses `/` in a class", `\/`},
		"class bar":           {`[a|b]`, "uses `|` in a class", `\|`},
		"class twice":         {`[@@]`, "uses `@@` in a class, where a browser reads two of them as an operator", `\@`},
		"class and":           {`[a&&b]`, "uses `&&` in a class", ""},
		"class nested":        {`[[:alpha:]]`, "uses `[` in a class", `\[`},
		"class class range":   {`[\d-z]`, "uses `-` in a class", ""},
		"class two dashes":    {`[a-z-9]`, "uses `-` in a class", ""},
		"class empty":         {`[]a]`, "uses `]` in a class", `\]`},
		"class property":      {`[\pL]`, "uses `\\pL`", `\p{L}`},
		"class escape":        {`[\_]`, "uses `\\_`", "without the backslash"},
	} {
		_, err := Compile(tc.expr, "")
		var pe *PatternError
		if !errors.As(err, &pe) {
			t.Errorf("%s: %q was accepted or not a PatternError: %v", name, tc.expr, err)
			continue
		}
		if !strings.Contains(pe.Problem, tc.problem) {
			t.Errorf("%s: problem is %q, want it to hold %q", name, pe.Problem, tc.problem)
		}
		if !strings.Contains(pe.Fix, tc.fix) {
			t.Errorf("%s: fix is %q, want it to hold %q", name, pe.Fix, tc.fix)
		}
	}
}

// TestThePortableSubsetKeepsWhatItAccepts is what the check lets through, and what Go is given for it.
func TestThePortableSubsetKeepsWhatItAccepts(t *testing.T) {
	for _, expr := range []string{
		`abc`, `a|b`, `(a|b)*c`, `(?:ab)+`, `a{2,3}?`, `^a.*z$`, `\d{1,3}(\.\d{1,3}){3}`, `[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`,
		`[\p{L}\p{M}\- ]+`, `\p{Lu}\p{Ll}+`, `[\x20-\x7e]`, `[^\s]+`, `\bword\b`, `https?://[^ ]+`, `#[0-9a-f]{6}`, `日本|é`,
		`[a\]b]`, `[\^a]`, `[a^]`, `[*]`, `[.]`, `[\w.]+`,
	} {
		if _, ok := compiled(expr); !ok {
			_, err := Compile(expr, "")
			t.Errorf("%q is refused: %v", expr, err)
		}
	}
}

// TestAPatternIsAnchoredInTheWayOfTheBrowser: whatever the pattern has at its top level, it matches the whole value.
func TestAPatternIsAnchoredInTheWayOfTheBrowser(t *testing.T) {
	re, ok := compiled(`ab|cd`)
	if !ok || re.MatchString("abx") || re.MatchString("xcd") || !re.MatchString("cd") {
		t.Errorf("ab|cd is not anchored as a whole")
	}
}
