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
// named by its alt text.
package markdown

import (
	"bytes"
	"net/url"
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
)

var converter = sync.OnceValue(func() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(transformer{}, 100))),
	)
})

// Render renders markdown. A link to a path in the application is given prefix, the
// address the application is mounted at, which is empty or "/admin" and the like. A text
// that cannot be rendered is shown as it is, so the reader still has it.
func Render(source, prefix string) Result {
	var buf bytes.Buffer
	pc := parser.NewContext()
	pc.Set(prefixKey, prefix)
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
