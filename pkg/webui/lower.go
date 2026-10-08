package webui

import (
	"bytes"
	"fmt"
	"image/png"
	"strings"

	"github.com/Olian04/webui/internal/favicon"
	"github.com/Olian04/webui/internal/ir"
)

// lower erases the declaration's type parameters into ir: generics gone,
// reflection spent, closures wrapped. Every type assertion in the system is
// created here, so nothing downstream asserts.
//
// Mapping only. validate has already run, so lower decides nothing: a missing
// field or a bad pattern is impossible by the time it gets here. It may repeat
// work — recompiling a Pattern rather than threading the compiled value
// through — but it never repeats a decision. The one error it can return is a
// logo that will not encode, which is a property of the image, not the
// declaration.
//
// ERASURE RULE, which decides the shape of everything here. A concrete type
// can be named in a type switch only where all its parameters are in scope:
//
//	Page[A] in a.Pages         A unknown      -> method on pageLike
//	Form[M] / Table[M] in Body M unrelated to A -> method on PageBody
//	Link[M, A] in RowClick[M]  A unknown      -> method on RowClick[M]
//	Accessor[M] inside Form[M] M known        -> type switch
//	Reject[M] inside Outcome   M known        -> type switch
//
// So the leaves of the lowering are methods that live beside their types
// (page.go, form.go, table.go, layout.go, link.go, action.go), and the
// interiors are ordinary switches (accessor.go).
func (a App) lower() (*ir.App, error) {
	l := &appLowerer{entries: map[string]bool{}}
	for _, p := range a.Pages {
		if p.pageNav().Label != "" {
			l.entries[p.pagePath()] = true
		}
	}

	out := &ir.App{
		Brand:  ir.Brand{Name: a.Brand.Name},
		Theme:  lowerTheme(a.Theme),
		ByPath: map[string]*ir.Page{},
	}
	if a.Brand.Logo != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, a.Brand.Logo); err != nil {
			return nil, fmt.Errorf("webui: Brand.Logo does not encode as PNG: %w", err)
		}
		out.Brand.Logo, out.Brand.Mime = buf.Bytes(), "image/png"
	}
	switch {
	case a.Brand.NoFavicon:
	case a.Brand.Logo == nil:
		out.Brand.DefaultFavicon = true
	default:
		for _, size := range []int{32, 180} { // the tab icon, and the one a phone puts on its home screen
			data, err := favicon.PNG(a.Brand.Logo, size)
			if err != nil {
				return nil, fmt.Errorf("webui: Brand.Logo does not scale to a %d px favicon: %w", size, err)
			}
			out.Brand.Favicons = append(out.Brand.Favicons, ir.Favicon{Size: size, PNG: data})
		}
	}
	for _, p := range a.Pages {
		page := p.lowerPage(l)
		out.Pages = append(out.Pages, page)
		out.ByPath[page.PathTemplate] = page
	}
	return out, nil
}

// appLowerer is what per-page lowering needs from the whole app.
type appLowerer struct {
	entries map[string]bool // the paths of the pages that have a navigation entry
}

// nav resolves a declared Nav to the IR's. A page with no entry of its own
// borrows the nearest ancestor's: the longest proper prefix of its path, by
// segments, that is a page with an entry. The root is never an ancestor, so a
// labelled landing page does not light for every hidden page in the app.
func (l *appLowerer) nav(n Nav, path string) ir.Nav {
	out := ir.Nav{Label: n.Label, Icon: n.Icon, Section: n.Section, Hidden: n.Label == ""}
	if n.Label != "" {
		return out
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(segs) - 1; i >= 1; i-- {
		if prefix := "/" + strings.Join(segs[:i], "/"); l.entries[prefix] {
			out.Shadow = prefix
			break
		}
	}
	return out
}

func lowerTheme(t Theme) ir.Theme {
	return ir.Theme{Accent: string(t.Accent), OK: string(t.OK), Warning: string(t.Warning), Critical: string(t.Critical)}
}
