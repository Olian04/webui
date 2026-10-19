package rules

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/ir"
)

func TestTheTextRulesAreCheckedOnTheServer(t *testing.T) {
	r := ir.Rules{StartsWith: "https://", EndsWith: ".org", Contains: "example", NotStartsWith: "https://www.", NotEndsWith: "x.org", NotContains: " "}
	for value, want := range map[string]string{
		"https://example.org":          "",
		"https://my.example.org":       "",
		"http://example.org":           `Must start with "https://".`,
		"https://example.com":          `Must end with ".org".`,
		"https://sample.org":           `Must contain "example".`,
		"https://www.example.org":      `Must not start with "https://www.".`,
		"https://example.org?x.org":    `Must not end with "x.org".`,
		"https://example.org/a b.org":  `Must not contain " ".`,
		"HTTPS://example.org":          `Must start with "https://".`,
		"":                             "",
		"   ":                          "",
		"https://example.org ":         `Must end with ".org".`,
		"https://example.orgx":         `Must end with ".org".`,
		"https://example.org.example.": `Must end with ".org".`,
	} {
		if got := Check(r, ir.KindString, value); got != want {
			t.Errorf("Check(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestEachTextRuleSaysWhatItRequires(t *testing.T) {
	for name, tc := range map[string]struct {
		r    ir.Rules
		msg  string
		line string
	}{
		"StartsWith":    {ir.Rules{StartsWith: "ab"}, `Must start with "ab".`, `Starts with "ab"`},
		"EndsWith":      {ir.Rules{EndsWith: "ab"}, `Must end with "ab".`, `Ends with "ab"`},
		"Contains":      {ir.Rules{Contains: "ab"}, `Must contain "ab".`, `Contains "ab"`},
		"NotStartsWith": {ir.Rules{NotStartsWith: "ab"}, `Must not start with "ab".`, `Does not start with "ab"`},
		"NotEndsWith":   {ir.Rules{NotEndsWith: "ab"}, `Must not end with "ab".`, `Does not end with "ab"`},
		"NotContains":   {ir.Rules{NotContains: "ab"}, `Must not contain "ab".`, `Does not contain "ab"`},
	} {
		parts := Parts(tc.r)
		if len(parts) != 1 || parts[0].Message != tc.msg {
			t.Errorf("%s: Parts = %v, want one with the message %q", name, parts, tc.msg)
		}
		if lines := TextLines(tc.r); len(lines) != 1 || lines[0] != tc.line {
			t.Errorf("%s: TextLines = %q, want %q", name, lines, tc.line)
		}
		if got := Validity(tc.r, ir.KindString)["patternMismatch"]; got != tc.msg {
			t.Errorf("%s: the browser is told %q, want %q", name, got, tc.msg)
		}
	}
}

func TestTheTextRulesComeBeforeThePatternAndAfterTheLength(t *testing.T) {
	p, err := Compile(`[a-z.\/:]+`, "must be a lowercase address")
	if err != nil {
		t.Fatal(err)
	}
	r := ir.Rules{MinLen: 4, StartsWith: "http", NotContains: "..", Pattern: p}
	for value, want := range map[string]string{
		"htt":         "Must be at least 4 characters.",
		"ftp://a.b":   `Must start with "http".`,
		"http://a..b": `Must not contain "..".`,
		"http://A.b":  "must be a lowercase address",
		"http://a.b":  "",
	} {
		if got := Check(r, ir.KindString, value); got != want {
			t.Errorf("Check(%q) = %q, want %q", value, got, want)
		}
	}
	parts := Parts(r)
	if len(parts) != 3 || parts[0].Message != `Must start with "http".` || parts[1].Message != `Must not contain "..".` || parts[2].Message != "must be a lowercase address" || parts[2].Expr != `[a-z.\/:]+` {
		t.Errorf("Parts = %+v", parts)
	}
	want := `Must start with "http". Must not contain "..". must be a lowercase address`
	if got := Validity(r, ir.KindString)["patternMismatch"]; got != want {
		t.Errorf("the browser is told %q, want %q", got, want)
	}
}

func TestTheBrowsersPatternIsTheOnlyConditionOrAllOfThem(t *testing.T) {
	p, _ := Compile(`\d+`, "")
	for name, tc := range map[string]struct {
		r    ir.Rules
		want string
	}{
		"none":                   {ir.Rules{}, ""},
		"the pattern":            {ir.Rules{Pattern: p}, `\d+`},
		"one rule":               {ir.Rules{StartsWith: "a.b"}, `a\.b[\s\S]*`},
		"not contains":           {ir.Rules{NotContains: "@"}, `(?![\s\S]*@)[\s\S]*`},
		"not ends":               {ir.Rules{NotEndsWith: "x"}, `(?![\s\S]*x$)[\s\S]*`},
		"two":                    {ir.Rules{StartsWith: "a", EndsWith: "z"}, `(?=(?:a[\s\S]*)$)(?=(?:[\s\S]*z)$)[\s\S]*`},
		"a rule and the pattern": {ir.Rules{Contains: "-", Pattern: p}, `(?=(?:[\s\S]*-[\s\S]*)$)(?=(?:\d+)$)[\s\S]*`},
	} {
		if got := BrowserPattern(tc.r); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

func TestATextIsWrittenSoThatABrowserReadsItAsThatText(t *testing.T) {
	for in, want := range map[string]string{
		"plain":       "plain",
		"a.b*c":       `a\.b\*c`,
		"(x)[y]{z}":   `\(x\)\[y\]\{z\}`,
		`a\b/c|d^$+?`: `a\\b\/c\|d\^\$\+\?`,
		"a-b":         "a-b",
		"é😀":          "é😀",
		"tab\tend":    `tab\u0009end`,
		"\u2028":      "\\u2028",
	} {
		if got := escapeLiteral(in); got != want {
			t.Errorf("escapeLiteral(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTextThatNoValueCanHoldIsFound(t *testing.T) {
	for name, tc := range map[string]struct {
		r    ir.Rules
		want string // what the first problem says
	}{
		"fine":                      {ir.Rules{StartsWith: "a", NotStartsWith: "b", Contains: "c", NotContains: "d", MaxLen: 10}, ""},
		"a line break":              {ir.Rules{Contains: "a\nb"}, "Contains has a line break in it"},
		"starts and contains not":   {ir.Rules{StartsWith: "http://", NotContains: "//"}, `StartsWith "http://" has NotContains "//" in it`},
		"ends and contains not":     {ir.Rules{EndsWith: ".org", NotContains: "."}, `EndsWith ".org" has NotContains "." in it`},
		"contains and contains not": {ir.Rules{Contains: "ab", NotContains: "ab"}, `Contains "ab" has NotContains "ab" in it`},
		"starts and starts not":     {ir.Rules{StartsWith: "https://", NotStartsWith: "http"}, `StartsWith "https://" starts with NotStartsWith "http"`},
		"ends and ends not":         {ir.Rules{EndsWith: "a.org", NotEndsWith: ".org"}, `EndsWith "a.org" ends with NotEndsWith ".org"`},
		"longer than the most":      {ir.Rules{StartsWith: "abcdef", MaxLen: 5}, `StartsWith "abcdef" is longer than MaxLen, 5`},
		"as long as the most":       {ir.Rules{StartsWith: "abcde", MaxLen: 5}, ""},
		"starts that is not a not":  {ir.Rules{StartsWith: "http", NotStartsWith: "https"}, ""},
	} {
		got := ""
		if ps := Unsatisfiable(tc.r); len(ps) > 0 {
			got = ps[0][0]
		}
		if got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

// TestTheBrowsersPatternAgreesWithTheServer is run in a browser's engine when there is one to run, node: for each
// combination of the rules, over a set of values, a value is accepted by the server exactly when the browser's
// pattern matches it, and the part that the browser's engine finds that it does not match is the first one that the
// server says. It is skipped without node, since the committed corpus holds the answers of the patterns.
func TestTheBrowsersPatternAgreesWithTheServer(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	p, err := Compile(`[a-z0-9.\/:\-]+`, "must be an address")
	if err != nil {
		t.Fatal(err)
	}
	values := []string{
		"", "a", "http://a.org", "https://a.org", "https://www.a.org", "http://a..org", "https://a.org ", "ftp://a.example.org", "a.org",
		"https://a.org/x.org", "x.org", "HTTPS://A.ORG", "https://é.org", "https://a.org\n", "ab", "ba", "a.b*c", "(x)", "x|y", "tab\there", "😀.org",
	}
	combos := []ir.Rules{
		{StartsWith: "https://"},
		{EndsWith: ".org"},
		{Contains: "a."},
		{NotStartsWith: "https://www."},
		{NotEndsWith: "x.org"},
		{NotContains: " "},
		{NotContains: ".."},
		{StartsWith: "https://", EndsWith: ".org"},
		{StartsWith: "http", NotStartsWith: "https", NotContains: ".."},
		{Contains: "a.b*c"}, {Contains: "(x)"}, {NotContains: "x|y"}, {Contains: "é"}, {EndsWith: "😀.org"},
		{Pattern: p}, {StartsWith: "https://", Pattern: p}, {StartsWith: "https://", EndsWith: ".org", NotContains: " ", Pattern: p},
		{NotEndsWith: "x.org", Pattern: p}, {Contains: "\t"}, {Contains: "ab", NotStartsWith: "b", NotEndsWith: "a"},
	}
	type tc struct {
		Pattern string
		Parts   [][2]string
		Want    []string // what the server says for each value
	}
	var cases []tc
	for _, r := range combos {
		c := tc{Pattern: BrowserPattern(r)}
		for _, part := range Parts(r) {
			c.Parts = append(c.Parts, [2]string{part.Expr, part.Message})
		}
		for _, v := range values {
			c.Want = append(c.Want, Check(r, ir.KindString, v))
		}
		cases = append(cases, c)
	}
	in, _ := json.Marshal(map[string]any{"values": values, "cases": cases})
	const script = `
const d = JSON.parse(require('fs').readFileSync(0, 'utf8'));
const out = [];
for (const c of d.cases) {
  for (const flags of ['v', 'u']) {
    const re = new RegExp('^(?:' + c.Pattern + ')$', flags);
    const parts = c.Parts.map((p) => new RegExp('^(?:' + p[0] + ')$', flags));
    d.values.forEach((v, i) => {
      const blank = v.trim() === '';
      const matches = blank || re.test(v); // a browser does not check an empty value; the server does not check a blank one
      const first = blank ? '' : (parts.findIndex((r) => !r.test(v)) >= 0 ? c.Parts[parts.findIndex((r) => !r.test(v))][1] : '');
      out.push({ pattern: c.Pattern, flags, value: v, matches, first, want: c.Want[i] });
    });
  }
}
console.log(JSON.stringify(out));`
	cmd := exec.Command(node, "-e", script) //nolint:gosec // the node on the PATH is what the test is for
	cmd.Stdin = strings.NewReader(string(in))
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	var got []struct {
		Pattern, Flags, Value, First, Want string
		Matches                            bool
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, g := range got {
		if g.Matches != (g.Want == "") {
			t.Errorf("[%s] %q: pattern %s: the browser says matches=%v, the server says %q", g.Flags, g.Value, g.Pattern, g.Matches, g.Want)
		}
		if g.First != g.Want {
			t.Errorf("[%s] %q: pattern %s: the part found is %q, the server says %q", g.Flags, g.Value, g.Pattern, g.First, g.Want)
		}
	}
	if len(got) != 2*len(combos)*len(values) {
		t.Errorf("%d results", len(got))
	}
}
