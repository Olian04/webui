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
	entries := map[string]bool{} // the paths of the pages that have a navigation entry
	for _, p := range a.Pages {
		if p.hasNavEntry() {
			entries[p.pagePath()] = true
		}
	}

	brand, err := lowerBrand(a.Brand)
	if err != nil {
		return nil, err
	}
	out := &ir.App{
		Brand:         brand,
		Theme:         lowerTheme(a.Theme),
		ExternalLinks: lowerExternalLinks(a.ExternalLinks),
		Secret:        a.Secret,
		Metrics:       a.Metrics,
		ByPath:        map[string]*ir.Page{},
	}
	for _, p := range a.Pages {
		page := p.lowerPage(entries)
		out.Pages = append(out.Pages, page)
		out.ByPath[page.PathTemplate] = page
	}
	return out, nil
}

// lowerBrand encodes the logo, and scales it into the favicons a browser asks for. The errors it can
// return are properties of the image, not of the declaration.
func lowerBrand(b Brand) (ir.Brand, error) {
	out := ir.Brand{Name: b.Name}
	if b.Logo != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, b.Logo); err != nil {
			return out, fmt.Errorf("webui: Brand.Logo does not encode as PNG: %w", err)
		}
		out.Logo, out.Mime = buf.Bytes(), "image/png"
	}
	switch {
	case b.NoFavicon:
	case b.Logo == nil:
		out.DefaultFavicon = true
	default:
		for _, size := range []int{32, 180} { // the tab icon, and the one a phone puts on its home screen
			data, err := favicon.PNG(b.Logo, size)
			if err != nil {
				return out, fmt.Errorf("webui: Brand.Logo does not scale to a %d px favicon: %w", size, err)
			}
			out.Favicons = append(out.Favicons, ir.Favicon{Size: size, PNG: data})
		}
	}
	return out, nil
}

// lowerNav resolves a declared Nav to the IR's. A page with no entry of its own
// borrows the nearest ancestor's: the longest proper prefix of its path, by
// segments, that is a page with an entry. The root is never an ancestor, so a
// labelled landing page does not light for every hidden page in the app.
func lowerNav(entries map[string]bool, n Nav, path string, dynamic bool) ir.Nav {
	out := ir.Nav{Label: n.Label, Icon: n.Icon, Section: n.Section, Hidden: n.Label == "" && !dynamic, Dynamic: dynamic}
	if n.Label != "" || dynamic {
		return out
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(segs) - 1; i >= 1; i-- {
		if prefix := "/" + strings.Join(segs[:i], "/"); entries[prefix] {
			out.Shadow = prefix
			break
		}
	}
	return out
}

func lowerTheme(t Theme) ir.Theme {
	return ir.Theme{Accent: string(t.Accent), OK: string(t.OK), Warning: string(t.Warning), Critical: string(t.Critical)}
}

func lowerExternalLinks(items []ExternalLink) []ir.ExternalLink {
	return conv(items, func(it ExternalLink) ir.ExternalLink {
		return ir.ExternalLink{Label: it.Label, Icon: it.Icon, Section: it.Section, URL: it.URL}
	})
}

// conv is in with f applied to each element: the conversion between a declaration's list and the
// IR's.
func conv[T, U any](in []T, f func(T) U) []U {
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// anys is rows as the []any the IR holds them in, the model's type erased.
func anys[M any](rows []M) []any { return conv(rows, func(r M) any { return r }) }
