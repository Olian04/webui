package markdown

import (
	"regexp"
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

	out := Render("# Title\n\nSome *emphasis*, **strong**, ~~gone~~ and `code`.\n\n- [x] done\n- [ ] todo\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n> quoted\n", Options{}).HTML
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
		out := Render(src, Options{}).HTML
		lacks(t, out, "<script")
		lacks(t, out, "<img")
		lacks(t, out, "<iframe")
		lacks(t, out, "<div")
	}
}

func TestALinkGoesOnlyToHTTPHTTPSOrMailto(t *testing.T) {
	t.Parallel()

	out := Render("[a](https://example.com/x) [b](http://example.com) [c](mailto:a@example.com)", Options{}).HTML
	has(t, out, `<a href="https://example.com/x" target="_blank" rel="noopener noreferrer">a</a>`)
	has(t, out, `href="http://example.com"`)
	has(t, out, `<a href="mailto:a@example.com" rel="noopener noreferrer">c</a>`)

	for _, dest := range []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,x", "vbscript:x", "file:///etc/passwd", "../x", "device", "#frag", "//evil.example/x", "/\\evil.example/x", "/\tx", "https:///nohost"} {
		out := Render("[label]("+dest+")", Options{}).HTML
		lacks(t, out, "<a ")
		has(t, out, "label") // the text stays
	}
	// An entity cannot smuggle a scheme past the check.
	lacks(t, Render("[x](&#106;avascript:alert(1))", Options{}).HTML, "<a ")
}

func TestAPathInTheAppIsGivenTheMountPrefixAndOpensInTheSameTab(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ src, prefix, want string }{
		{"[x](/device)", "/admin", `<a href="/admin/device">x</a>`},
		{"[x](/device/dev_1?minutes=15&tabs.tab=raw)", "/admin", `href="/admin/device/dev_1?minutes=15&amp;tabs.tab=raw"`},
		{"[x](/)", "/admin", `<a href="/admin/">x</a>`},
		{"[x](/device#top)", "/admin", `<a href="/admin/device#top">x</a>`},
		{"[x](/device)", "", `<a href="/device">x</a>`},
		{"[x](/a/b)", "/ops/admin", `<a href="/ops/admin/a/b">x</a>`},
	} {
		out := Render(c.src, Options{Prefix: c.prefix}).HTML
		has(t, out, c.want)
		lacks(t, out, "target=")
		lacks(t, out, "rel=")
	}
}

func TestAnAddressForAnotherSiteIsNotAPathInTheApp(t *testing.T) {
	t.Parallel()

	// Each of these a browser reads as another host, or the check could be argued with.
	for _, dest := range []string{"//evil.example", "//evil.example/x", "/\\evil.example", "/%2f%2fevil", "/ /evil", "/\n/evil", "\\evil"} {
		out := Render("[x]("+dest+")", Options{Prefix: "/admin"}).HTML
		if strings.Contains(out, "evil") && strings.Contains(out, "<a ") && !strings.Contains(out, `href="/admin/`) {
			t.Errorf("%q gives a link that leaves the app:\n%s", dest, out)
		}
	}
	lacks(t, Render("[x](//evil.example)", Options{Prefix: "/admin"}).HTML, "<a ")
	lacks(t, Render("[x](/\\evil.example)", Options{Prefix: "/admin"}).HTML, "<a ")
}

func TestAnAutolinkIsCheckedLikeALink(t *testing.T) {
	t.Parallel()

	has(t, Render("see https://example.com/a and www.example.com", Options{}).HTML, `<a href="https://example.com/a" target="_blank" rel="noopener noreferrer">`)
	has(t, Render("<https://example.com>", Options{}).HTML, `href="https://example.com"`)
	out := Render("<javascript:alert(1)> and <ftp://example.com/x>", Options{}).HTML
	lacks(t, out, "<a ")
	has(t, out, "javascript:alert(1)")
	has(t, Render("<me@example.com>", Options{}).HTML, `href="mailto:me@example.com"`)
}

func TestAnImageIsALinkNamedByItsAltTextAndNeverAPicture(t *testing.T) {
	t.Parallel()

	out := Render("![a chart](https://example.com/c.png)", Options{}).HTML
	lacks(t, out, "<img")
	has(t, out, `<a href="https://example.com/c.png" target="_blank" rel="noopener noreferrer">a chart</a>`)

	lacks(t, Render("![x](/local.png)", Options{}).HTML, "<img")
	has(t, Render("![x](/local.png)", Options{Prefix: "/admin"}).HTML, `<a href="/admin/local.png">x</a>`) // a path in the app, so a link in it
	has(t, Render("![](https://example.com/c.png)", Options{}).HTML, ">https://example.com/c.png</a>")     // no alt: the address names it

	// A picture inside a link is its alt text, since a link inside a link is not markup.
	out = Render("[![logo](https://example.com/l.png)](https://example.com)", Options{}).HTML
	if strings.Count(out, "<a ") != 1 {
		t.Errorf("want one link, got:\n%s", out)
	}
	has(t, out, "logo</a>")
}

func TestHighlightIsSetOnlyForAFenceThatNamesALanguage(t *testing.T) {
	t.Parallel()

	r := Render("```go\nfunc main() {}\n```\n", Options{})
	if !r.Highlight {
		t.Error("a go fence is not marked for highlighting")
	}
	has(t, r.HTML, `<code class="language-go">`)
	has(t, r.HTML, `<pre tabindex="0">`)
	for _, src := range []string{"```\nplain\n```\n", "    indented\n", "no code `at` all", "<pre>x</pre>"} {
		if Render(src, Options{}).Highlight {
			t.Errorf("%q is marked for highlighting", src)
		}
	}
}

func TestCodeIsEscaped(t *testing.T) {
	t.Parallel()

	out := Render("```html\n<script>alert(1)</script>\n```\n", Options{}).HTML
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
		out := Render(src, Options{}).HTML
		lacks(t, out, "<script")
		lacks(t, out, `href="javascript`)
	}
}

func FuzzRender(f *testing.F) {
	for _, s := range []string{"# a", "[a](b)", "![a](b)", "<a href=x>", "```go\nx\n```", "| a |\n|-|\n| b |", "<http://x>", "[![a](b)](c)", "[x](/a)", "[x](//a)", "[x](/\\a)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Render(s, Options{Prefix: "/admin"}).HTML
		for _, bad := range []string{"<script", "<iframe", "<img", "href=\"javascript:", "onerror=", "<object"} {
			if strings.Contains(strings.ToLower(out), bad) {
				t.Fatalf("%q renders %q", s, bad)
			}
		}
		// A link leaves for the web, a mail client, or somewhere under the app's prefix.
		for _, m := range regexp.MustCompile(`href="([^"]*)"`).FindAllStringSubmatch(out, -1) {
			h := strings.ToLower(m[1])
			if !strings.HasPrefix(h, "https://") && !strings.HasPrefix(h, "http://") && !strings.HasPrefix(h, "mailto:") && !strings.HasPrefix(h, "/admin/") {
				t.Fatalf("%q renders a link to %q", s, m[1])
			}
		}
	})
}

var cdn = Options{TrustedOrigins: []string{"https://cdn.example.com"}}

func TestAPictureFromATrustedOriginIsDrawn(t *testing.T) {
	t.Parallel()

	out := Render("![The logo](https://cdn.example.com/a/b.png?v=2)", cdn).HTML
	has(t, out, `<img src="https://cdn.example.com/a/b.png?v=2" alt="The logo"`)
	has(t, out, `loading="lazy"`)
	has(t, out, `decoding="async"`)
	has(t, out, `referrerpolicy="no-referrer"`)
	has(t, out, `class="md-img"`)
	lacks(t, out, "<a ") // drawn, not linked

	// A title stays, and what was checked is what is written out: the host is lower case.
	out = Render(`![x](https://CDN.Example.COM/p.png "A title")`, cdn).HTML
	has(t, out, `src="https://cdn.example.com/p.png"`)
	has(t, out, `title="A title"`)
}

func TestAPictureIsOnlyDrawnFromTheExactOriginThatWasNamed(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"https://evil.example/x.png",
		"https://evil.cdn.example.com/x.png",         // a subdomain is another origin
		"https://cdn.example.com.evil.example/x.png", // a name that starts like it
		"https://notcdn.example.com/x.png",
		"http://cdn.example.com/x.png",               // not https
		"https://cdn.example.com:8443/x.png",         // another port
		"//cdn.example.com/x.png",                    // no scheme: the page's own
		"https://cdn.example.com@evil.example/x.png", // the user, not the host
		"https://evil.example#@cdn.example.com/x.png",
		"https://user:pass@cdn.example.com/x.png", // credentials are never sent along
		`https://cdn.example.com\@evil.example/x.png`,
		"https://cdn.example.com%2f@evil.example/x.png",
		"data:image/svg+xml,<svg/>",
		"javascript:alert(1)",
		"/local.png",
		"x.png",
		"HTTPS:cdn.example.com/x.png",
	} {
		out := Render("![x]("+src+")", cdn).HTML
		lacks(t, out, "<img")
	}
}

func TestAPictureIsDrawnWhateverTheCaseOrDefaultPortOrTrailingDotOfItsHost(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"https://cdn.example.com:443/x.png", "https://CDN.EXAMPLE.COM/x.png", "HTTPS://cdn.example.com/x.png", "https://cdn.example.com./x.png"} {
		out := Render("![x]("+src+")", cdn).HTML
		has(t, out, `<img src="https://cdn.example.com/x.png"`) // written out as it was checked
	}
	port := Options{TrustedOrigins: []string{"https://cdn.example.com:8443"}}
	has(t, Render("![x](https://cdn.example.com:8443/x.png)", port).HTML, `<img src="https://cdn.example.com:8443/x.png"`)
	lacks(t, Render("![x](https://cdn.example.com/x.png)", port).HTML, "<img")
}

func TestATrustedPictureInsideALinkStaysAPictureInsideIt(t *testing.T) {
	t.Parallel()

	out := Render("[![logo](https://cdn.example.com/l.png)](https://example.com)", cdn).HTML
	if strings.Count(out, "<a ") != 1 || strings.Count(out, "<img") != 1 {
		t.Errorf("want one link around one picture, got:\n%s", out)
	}
	has(t, out, `<a href="https://example.com"`)

	// An untrusted one is still only its alt text, since a link inside a link is not markup.
	out = Render("[![logo](https://evil.example/l.png)](https://example.com)", cdn).HTML
	if strings.Count(out, "<a ") != 1 {
		t.Errorf("want one link, got:\n%s", out)
	}
	lacks(t, out, "<img")
}

func TestAnUntrustedPictureIsStillALinkWhenSomeOriginIsTrusted(t *testing.T) {
	t.Parallel()

	out := Render("![chart](https://evil.example/c.png)", cdn).HTML
	lacks(t, out, "<img")
	has(t, out, `<a href="https://evil.example/c.png" target="_blank" rel="noopener noreferrer">chart</a>`)
}

func TestRawHTMLIsStillNotRenderedWhenAnOriginIsTrusted(t *testing.T) {
	t.Parallel()

	out := Render(`<img src="https://cdn.example.com/x.png" onerror="alert(1)"> <script>alert(1)</script>`, cdn).HTML
	lacks(t, out, "<img")
	lacks(t, out, "<script")
}

func TestAnOriginIsWrittenOneWay(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"https://example.com":         "https://example.com",
		"https://Example.COM":         "https://example.com",
		"https://example.com:443":     "https://example.com",
		"https://example.com:8443":    "https://example.com:8443",
		"https://example.com.":        "https://example.com",
		"https://cdn-1.example.co.uk": "https://cdn-1.example.co.uk",
		"https://127.0.0.1:9000":      "https://127.0.0.1:9000",
		"https://localhost":           "https://localhost",
	} {
		got, err := Origin(in)
		if err != nil || got != want {
			t.Errorf("Origin(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{
		"", "example.com", "http://example.com", "//example.com", "https://", "https:///x",
		"https://example.com/", "https://example.com/path", "https://example.com?x=1", "https://example.com#x",
		"https://user@example.com", "https://*.example.com", "*", "https://example.com;script-src *",
		"https://example.com, https://evil.example", "https://example.com 'unsafe-inline'", "https://exa mple.com",
		"https://example.com\\evil.example", "https://exämple.com", "https://-example.com", "https://example..com",
		"https://example.com:0", "https://example.com:99999", "https://example.com:abc", "https://example.com%2f",
		"https://[::1]", "https://example.com\n", "https://example.com'", "ftp://example.com",
	} {
		if got, err := Origin(bad); err == nil {
			t.Errorf("Origin(%q) = %q, want an error", bad, got)
		}
	}
}

func FuzzRenderWithATrustedOrigin(f *testing.F) {
	for _, s := range []string{"![a](https://cdn.example.com/x)", "![a](https://cdn.example.com@evil.example/x)", "[![a](https://cdn.example.com/x)](y)", "![a](//cdn.example.com/x)", `<img src="https://cdn.example.com/x">`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Render(s, cdn).HTML
		// A picture is only ever drawn from the origin, as a host and then a path, a query or
		// nothing; and there is never anything that runs.
		for _, m := range regexp.MustCompile(`<img [^>]*`).FindAllString(out, -1) {
			src := regexp.MustCompile(`src="([^"]*)"`).FindStringSubmatch(m)
			if src == nil {
				t.Fatalf("%q draws %q with no source", s, m)
			}
			from := src[1] == "https://cdn.example.com" || strings.HasPrefix(src[1], "https://cdn.example.com/") || strings.HasPrefix(src[1], "https://cdn.example.com?")
			if !from {
				t.Fatalf("%q draws %q", s, m)
			}
		}
		for _, bad := range []string{"<script", "<iframe", "onerror=", "<object", `href="javascript:`} {
			if strings.Contains(strings.ToLower(out), bad) {
				t.Fatalf("%q renders %q", s, bad)
			}
		}
	})
}
