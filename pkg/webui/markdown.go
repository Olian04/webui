package webui

import (
	"context"
	"fmt"
	"slices"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/markdown"
)

// Markdown is a leaf that shows one text of a model, set as markdown: headings,
// emphasis, lists, links, quotes and code, and the tables, task lists, strikethrough
// and bare links of GitHub's flavour. It is for text the application has, such as a
// runbook, release notes or a README, and it only reads: the accessor has no Store.
//
// The text is not trusted any more than any other text of the model. Raw HTML in it is
// never rendered, a link goes only to an http or https address, a mailto, or a path in the
// application and is otherwise left as its text. A path, such as /device/dev_1, is an
// address in the application whatever it is mounted at, so it is given the prefix of
// [App.Compile] and opens in the same tab; any other link opens in a tab of its own.
//
// An image is not drawn, since the page would refuse a picture from elsewhere and a
// picture is a way for a text to make the visitor's browser fetch from it: it is a link
// to the picture, named by its alt text. TrustedOrigins are the exception: a picture
// from one of them is drawn in the text.
//
// It needs no script. A fenced code block that names a language is coloured by the
// editor, which the page then loads, as it does for an [Editor]; a block that does
// not, and every block without script, is plain text in a monospace block.
type Markdown[M any] struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the
	// title.
	Desc string

	// Load returns the model that holds the text.
	Load func(ctx context.Context) (M, Loaded)

	// Content is the markdown. Its Label names the text to a screen reader.
	Content String[M]

	// TrustedOrigins are the origins the author trusts to serve pictures for this text,
	// each written as https://host or https://host:port, with nothing after it: no path, no
	// user, no wildcard, and a host in ASCII. A picture in the text from one of them is drawn
	// where it is written, and the page, and only this page, lets the browser fetch from
	// them: the content security policy of a page with a Markdown that names origins has them
	// in its img-src, and no other page does. A picture from any other origin is a link.
	//
	// Trusting an origin is a decision that its owner may see who reads the text, and may
	// choose what it shows: name only the ones that serve what you control.
	TrustedOrigins []string
}

// ASSERT: Markdown implements PageBody
var _ PageBody = Markdown[struct{}]{}

func (m Markdown[M]) validateBody(v *bodyValidator) {
	needLoad(v, "a Markdown", m.Load != nil)
	validateAccessors(v, []Accessor[M]{m.Content}, accessorSite{where: "Markdown.Content", uniqueLbl: true})
	if m.Content.Store != nil {
		v.add("a Markdown has a Store", "A Markdown only reads: remove Store, or use an Editor to change the text.")
	}
	for i, o := range m.TrustedOrigins {
		if _, err := markdown.Origin(o); err != nil {
			v.add(fmt.Sprintf("Markdown.TrustedOrigins[%d], %q, is not an origin: %v", i, o, err),
				`Write it as "https://example.com", or "https://cdn.example.com:8443" for a port that is not 443.`)
		}
	}
}

// origins are the trusted origins as the renderer and the content security policy take
// them: each in its one written form, once.
func (m Markdown[M]) origins() []string {
	var out []string
	for _, o := range m.TrustedOrigins {
		if n, err := markdown.Origin(o); err == nil && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func (m Markdown[M]) lowerBody(at ir.Addr) ir.Node {
	return &ir.Markdown{
		At: at, Title: m.Title, Desc: m.Desc,
		Load:    eraseLoad(m.Load),
		Content: lowerAccessor[M](m.Content, "m"),

		TrustedOrigins: m.origins(),
	}
}
