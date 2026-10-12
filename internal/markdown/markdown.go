// Package markdown turns the markdown of a model into HTML that is safe to put in a
// page: CommonMark with the GitHub extensions, where the text is trusted no further
// than any other text of the model.
//
// Raw HTML in the source is never rendered. A link goes only to an http or https
// address, a mailto, or a path in the application, and anything else is left as its text.
// A path, such as /device/dev_1, is an address in the application whatever it is mounted
// at, so it is given the mount's prefix and opens in the same tab. An image is not drawn,
// since the page's policy would refuse one from elsewhere and a picture is a way for
// a text to make the visitor's browser fetch from it: it is a link to the picture,
// named by its alt text. The exception is a picture from an origin the author trusts,
// which is drawn, and which the page's policy is told to allow.
package markdown

import (
	"bytes"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Result is a rendered text.
type Result struct {
	// HTML is the markup, to be put in the page as it is.
	HTML string

	// Highlight is whether the text has a fenced code block that names a language,
	// which the page's editor can colour when it has loaded.
	Highlight bool
}

var (
	highlightKey = parser.NewContextKey()
	prefixKey    = parser.NewContextKey()
	originsKey   = parser.NewContextKey()
)

// Options is what a text is rendered against.
type Options struct {
	// Prefix is the address the application is mounted at, which a link to a path in the
	// application is given. It is empty, or "/admin" and the like.
	Prefix string

	// TrustedOrigins are the origins a picture may be drawn from, each as Origin made it.
	// A picture from anywhere else is a link.
	TrustedOrigins []string
}

var converter = sync.OnceValue(func() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(transformer{}, 100))),
	)
})

// Render renders markdown. A link to a path in the application is given the mount's
// prefix. A text that cannot be rendered is shown as it is, so the reader still has it.
func Render(source string, opts Options) Result {
	var buf bytes.Buffer
	pc := parser.NewContext()
	pc.Set(prefixKey, opts.Prefix)
	pc.Set(originsKey, opts.TrustedOrigins)
	if err := converter().Convert([]byte(source), &buf, parser.WithContext(pc)); err != nil {
		return Result{HTML: "<pre>" + string(util.EscapeHTML([]byte(source))) + "</pre>"}
	}
	highlight, _ := pc.Get(highlightKey).(bool)
	// A block that scrolls can be reached by keyboard. The only "<pre>" in the output is a
	// code block's, since text is escaped.
	html := strings.ReplaceAll(buf.String(), "<pre>", `<pre tabindex="0">`)
	return Result{HTML: html, Highlight: highlight}
}

// transformer applies the rules above to the tree, before it is rendered.
type transformer struct{}

func (transformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	prefix, _ := pc.Get(prefixKey).(string)
	origins, _ := pc.Get(originsKey).([]string)
	var images, links, autos []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Image:
			images = append(images, n)
		case *ast.Link:
			links = append(links, n)
		case *ast.AutoLink:
			autos = append(autos, n)
		case *ast.FencedCodeBlock:
			if n.Language(source) != nil {
				pc.Set(highlightKey, true)
			}
		}
		return ast.WalkContinue, nil
	})

	for _, n := range images {
		if img := n.(*ast.Image); drawn(img, origins) {
			continue // from an origin the author trusts: it stays a picture, even inside a link
		}
		if inLink(n) {
			unwrap(n) // a link inside a link is not markup, so the picture is only its alt text
			continue
		}
		link := ast.NewLink()
		link.Destination, link.Title = n.(*ast.Image).Destination, n.(*ast.Image).Title
		adopt(link, n)
		if !link.HasChildren() {
			link.AppendChild(link, ast.NewString(link.Destination))
		}
		replace(n, link)
		links = append(links, link)
	}
	for _, n := range links {
		link := n.(*ast.Link)
		dest := string(link.Destination)
		switch {
		case local(dest):
			link.Destination = []byte(prefix + dest)
		case allowed(dest):
			external(link, dest)
		default:
			unwrap(link)
		}
	}
	for _, n := range autos {
		auto := n.(*ast.AutoLink)
		url := string(auto.URL(source))
		if auto.AutoLinkType != ast.AutoLinkEmail && !allowed(url) {
			replace(auto, ast.NewString(auto.Label(source)))
			continue
		}
		external(auto, url)
	}
}

// allowed reports whether a link may lead to the destination: an absolute http or
// https address, or a mailto. A relative address would be read against the page the
// text is on, which the text knows nothing about.
func allowed(dest string) bool {
	u, err := url.Parse(strings.TrimSpace(dest))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return true
	}
	return false
}

// local reports whether dest is a path in the application: it starts with a slash, and
// not two, since "//host" is another site, nor a backslash, which a browser reads as a
// slash. It has no scheme or host, and nothing a browser would strip or split on.
func local(dest string) bool {
	if len(dest) == 0 || dest[0] != '/' || strings.ContainsAny(dest, "\\ \t\r\n\x00") {
		return false
	}
	if len(dest) > 1 && dest[1] == '/' {
		return false
	}
	u, err := url.Parse(dest)
	return err == nil && u.Scheme == "" && u.Host == "" && u.Opaque == ""
}

// external makes a link to another site open in a tab of its own, which cannot reach
// back into this one.
func external(n ast.Node, dest string) {
	if u, err := url.Parse(dest); err == nil && strings.HasPrefix(strings.ToLower(u.Scheme), "http") {
		n.SetAttributeString("target", []byte("_blank"))
	}
	n.SetAttributeString("rel", []byte("noopener noreferrer"))
}

// drawn reports whether a picture is from a trusted origin, and if so makes it ready to be
// drawn: its address is the one that was checked, written out again, and the browser is asked
// to fetch it late, without a referrer. The picture is not drawn unless every one of these holds:
// an https address with a host and no user, nothing a browser would read differently from
// the way it was parsed, and an origin that is in the list.
func drawn(img *ast.Image, origins []string) bool {
	clean, ok := trusted(string(img.Destination), origins)
	if !ok {
		return false
	}
	img.Destination = []byte(clean)
	img.SetAttributeString("class", []byte("md-img"))
	img.SetAttributeString("loading", []byte("lazy"))
	img.SetAttributeString("decoding", []byte("async"))
	img.SetAttributeString("referrerpolicy", []byte("no-referrer"))
	return true
}

// trusted is the address a picture may be drawn from, written out again, if it is from
// one of the origins.
func trusted(dest string, origins []string) (string, bool) {
	if len(origins) == 0 || hasUnsafe(dest) {
		return "", false
	}
	u, err := url.Parse(strings.TrimSpace(dest))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || u.Hostname() == "" {
		return "", false
	}
	origin, err := Origin("https://" + u.Host)
	if err != nil || !slices.Contains(origins, origin) {
		return "", false
	}
	host := strings.TrimPrefix(origin, "https://")
	clean := url.URL{Scheme: "https", Host: host, Path: u.Path, RawPath: u.RawPath, RawQuery: u.RawQuery}
	return clean.String(), true
}

// hasUnsafe is whether an address holds what a browser reads differently from a parser: a
// backslash, which it takes for a slash, whitespace and control characters, which it
// strips or splits on.
func hasUnsafe(s string) bool {
	for _, r := range s {
		if r == '\\' || r <= ' ' || r == 0x7f {
			return true
		}
	}
	return false
}

// Origin is an origin as a policy and a comparison both want it: "https://host", with the
// port after it unless it is the default, and the host in lower case, without a trailing
// dot. It is written as https://example.com or https://cdn.example.com:8443 and nothing
// more: no path, query or user, no wildcard, and an ASCII host, so a name that is not
// ASCII is written as the punycode the browser would use. An error says what is wrong.
func Origin(s string) (string, error) {
	if hasUnsafe(s) || strings.ContainsAny(s, ";,\"'%*") {
		return "", errors.New("it contains a space, a quote, a backslash, a percent sign, a wildcard or another character that does not belong in an origin")
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", errors.New("it is not an address")
	}
	switch {
	case u.Scheme != "https":
		return "", errors.New("it must start with https://")
	case u.User != nil:
		return "", errors.New("it must not have a user")
	case u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.ForceQuery:
		return "", errors.New("it must be only a scheme, a host and perhaps a port, with no path, query or fragment")
	case u.Hostname() == "":
		return "", errors.New("it has no host")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if !validHost.MatchString(host) {
		return "", errors.New("its host must be a name of letters, digits and hyphens in ASCII; write a name that is not as its punycode")
	}
	port := u.Port()
	if port == "443" {
		port = ""
	}
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", errors.New("its port is not a number from 1 to 65535")
		}
		host += ":" + strconv.Itoa(n)
	}
	return "https://" + host, nil
}

var validHost = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// inLink reports whether n is inside a link.
func inLink(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if _, ok := p.(*ast.Link); ok {
			return true
		}
	}
	return false
}

// adopt moves the children of from to the end of to.
func adopt(to, from ast.Node) {
	for c := from.FirstChild(); c != nil; c = from.FirstChild() {
		from.RemoveChild(from, c)
		to.AppendChild(to, c)
	}
}

// replace puts with where n is.
func replace(n, with ast.Node) {
	n.Parent().ReplaceChild(n.Parent(), n, with)
}

// unwrap puts a link's content where the link is, and leaves the link out.
func unwrap(link ast.Node) {
	parent := link.Parent()
	for c := link.FirstChild(); c != nil; c = link.FirstChild() {
		link.RemoveChild(link, c)
		parent.InsertBefore(parent, link, c)
	}
	parent.RemoveChild(parent, link)
}
