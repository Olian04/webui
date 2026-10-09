package markdown

import (
	"strings"
	"testing"
)

func has(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output does not contain %q:\n%s", want, got)
	}
}

func lacks(t *testing.T, got, bad string) {
	t.Helper()
	if strings.Contains(got, bad) {
		t.Errorf("output contains %q:\n%s", bad, got)
	}
}

func TestCommonMarkAndTheGitHubExtensions(t *testing.T) {
	t.Parallel()

	out := Render("# Title\n\nSome *emphasis*, **strong**, ~~gone~~ and `code`.\n\n- [x] done\n- [ ] todo\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n> quoted\n").HTML
	for _, want := range []string{"<h1>Title</h1>", "<em>emphasis</em>", "<strong>strong</strong>", "<del>gone</del>", "<code>code</code>",
		`type="checkbox"`, "<table>", "<td>1</td>", "<blockquote>"} {
		has(t, out, want)
	}
}

func TestRawHTMLIsNeverRendered(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"<script>alert(1)</script>",
		"text <img src=x onerror=alert(1)> text",
		"<div onclick=\"x()\">hi</div>",
		"<iframe src=\"https://evil\"></iframe>",
	} {
		out := Render(src).HTML
		lacks(t, out, "<script")
		lacks(t, out, "<img")
		lacks(t, out, "<iframe")
		lacks(t, out, "<div")
	}
}

func TestALinkGoesOnlyToHTTPHTTPSOrMailto(t *testing.T) {
	t.Parallel()

	out := Render("[a](https://example.com/x) [b](http://example.com) [c](mailto:a@example.com)").HTML
	has(t, out, `<a href="https://example.com/x" target="_blank" rel="noopener noreferrer">a</a>`)
	has(t, out, `href="http://example.com"`)
	has(t, out, `<a href="mailto:a@example.com" rel="noopener noreferrer">c</a>`)

	for _, dest := range []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,x", "vbscript:x", "file:///etc/passwd", "/admin/x", "../x", "#frag", "//evil.example/x", "https:///nohost"} {
		out := Render("[label](" + dest + ")").HTML
		lacks(t, out, "<a ")
		has(t, out, "label") // the text stays
	}
	// An entity cannot smuggle a scheme past the check.
	lacks(t, Render("[x](&#106;avascript:alert(1))").HTML, "<a ")
}

func TestAnAutolinkIsCheckedLikeALink(t *testing.T) {
	t.Parallel()

	has(t, Render("see https://example.com/a and www.example.com").HTML, `<a href="https://example.com/a" target="_blank" rel="noopener noreferrer">`)
	has(t, Render("<https://example.com>").HTML, `href="https://example.com"`)
	out := Render("<javascript:alert(1)> and <ftp://example.com/x>").HTML
	lacks(t, out, "<a ")
	has(t, out, "javascript:alert(1)")
	has(t, Render("<me@example.com>").HTML, `href="mailto:me@example.com"`)
}

func TestAnImageIsALinkNamedByItsAltTextAndNeverAPicture(t *testing.T) {
	t.Parallel()

	out := Render("![a chart](https://example.com/c.png)").HTML
	lacks(t, out, "<img")
	has(t, out, `<a href="https://example.com/c.png" target="_blank" rel="noopener noreferrer">a chart</a>`)

	lacks(t, Render("![x](/local.png)").HTML, "<img")
	lacks(t, Render("![x](/local.png)").HTML, "<a ")
	has(t, Render("![](https://example.com/c.png)").HTML, ">https://example.com/c.png</a>") // no alt: the address names it

	// A picture inside a link is its alt text, since a link inside a link is not markup.
	out = Render("[![logo](https://example.com/l.png)](https://example.com)").HTML
	if strings.Count(out, "<a ") != 1 {
		t.Errorf("want one link, got:\n%s", out)
	}
	has(t, out, "logo</a>")
}

func TestHighlightIsSetOnlyForAFenceThatNamesALanguage(t *testing.T) {
	t.Parallel()

	r := Render("```go\nfunc main() {}\n```\n")
	if !r.Highlight {
		t.Error("a go fence is not marked for highlighting")
	}
	has(t, r.HTML, `<code class="language-go">`)
	has(t, r.HTML, `<pre tabindex="0">`)
	for _, src := range []string{"```\nplain\n```\n", "    indented\n", "no code `at` all", "<pre>x</pre>"} {
		if Render(src).Highlight {
			t.Errorf("%q is marked for highlighting", src)
		}
	}
}

func TestCodeIsEscaped(t *testing.T) {
	t.Parallel()

	out := Render("```html\n<script>alert(1)</script>\n```\n").HTML
	lacks(t, out, "<script")
	has(t, out, "&lt;script&gt;")
}

func TestHostileInputDoesNotBreakTheRenderer(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"", "\x00\x01\xff", strings.Repeat("> ", 5000) + "x", strings.Repeat("- ", 2000) + "x",
		strings.Repeat("[", 3000) + strings.Repeat("]", 3000), strings.Repeat("*", 5001) + "x",
		"[a]: javascript:alert(1)\n\n[a]",
	} {
		out := Render(src).HTML
		lacks(t, out, "<script")
		lacks(t, out, `href="javascript`)
	}
}

func FuzzRender(f *testing.F) {
	for _, s := range []string{"# a", "[a](b)", "![a](b)", "<a href=x>", "```go\nx\n```", "| a |\n|-|\n| b |", "<http://x>", "[![a](b)](c)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Render(s).HTML
		for _, bad := range []string{"<script", "<iframe", "<img", "href=\"javascript:", "onerror=", "<object"} {
			if strings.Contains(strings.ToLower(out), bad) {
				t.Fatalf("%q renders %q", s, bad)
			}
		}
	})
}
