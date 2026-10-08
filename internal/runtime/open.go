package runtime

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/Olian04/webui/internal/args"
	"github.com/Olian04/webui/internal/ir"
)

// ErrNotMounted is returned by Open for a page that is not in this program, or
// whose declaration no longer matches what was compiled.
var ErrNotMounted = errors.New("that page is not mounted in this app")

// Open resolves a page through the program, not through the declaration, so
// the href includes the mount prefix and a page that was never mounted, or was
// mutated after Compile, is reported rather than producing a dead link.
// argv is the destination's argument struct. The address carries the page's own
// arguments only: the destination's tables and tabs land on their defaults.
func (r *Request) Open(pathTemplate string, argv any) (string, error) {
	return r.program.Open(pathTemplate, argv)
}

// Open is Request.Open without a request, for the runtime's own use.
func (p *Program) Open(pathTemplate string, argv any) (string, error) {
	return p.open(pathTemplate, argv, "")
}

// open is Open, remembering where the user is coming from when the destination
// has a form to cancel out of. The library does this and not the page: a form
// reached from several pages then goes back to whichever one it was reached
// from, and no page has to declare it.
func (p *Program) open(pathTemplate string, argv any, from string) (string, error) {
	page := p.App.ByPath[pathTemplate]
	if page == nil {
		return "", ErrNotMounted
	}
	path, query, err := page.Encode(argv)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	if from = bounded(from); from != "" && hasForm(page.Body) {
		query[args.FromKey] = from
	}
	return p.Prefix + args.Href(page.PathTemplate, path, query), nil
}

// maxFrom bounds an address kept in an address. Each page in a chain adds its
// own, so without a bound the address would grow with every hop.
const maxFrom = 1024

// bounded is from when it fits, else from without the address it was itself
// reached from — the oldest hop is forgotten first — else nothing.
func bounded(from string) string {
	if len(from) <= maxFrom {
		return from
	}
	u, err := url.Parse(from)
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Del(args.FromKey)
	u.RawQuery = q.Encode()
	if s := u.String(); len(s) <= maxFrom {
		return s
	}
	return ""
}

// local is the address when it is one in this app, which is the only kind a
// Cancel or a redirect may take the user to: an address in the query is
// whatever the sender made it, so it is never followed anywhere else.
func (p *Program) local(address string) string {
	if !safeRedirect(address) || len(address) > maxFrom*2 {
		return ""
	}
	inApp := address == p.Prefix || strings.HasPrefix(address, p.Prefix+"/") || strings.HasPrefix(address, p.Prefix+"?")
	if !inApp || strings.HasPrefix(address, p.Prefix+"/_webui") {
		return ""
	}
	return address
}

// hasForm reports whether a page body has a form anywhere in it.
func hasForm(n ir.Node) bool {
	switch n := n.(type) {
	case *ir.Form:
		return true
	case *ir.Stack:
		return slices.ContainsFunc(n.Children, hasForm)
	case *ir.Split:
		return slices.ContainsFunc(n.Children, hasForm)
	case *ir.Tabs:
		return slices.ContainsFunc(n.Tabs, func(t ir.Tab) bool { return hasForm(t.Body) })
	}
	return false
}
