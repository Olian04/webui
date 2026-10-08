package runtime

import (
	"errors"
	"fmt"

	"github.com/Olian04/webui/internal/args"
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
	page := p.App.ByPath[pathTemplate]
	if page == nil {
		return "", ErrNotMounted
	}
	path, query, err := page.Encode(argv)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return p.Prefix + args.Href(page.PathTemplate, path, query), nil
}
