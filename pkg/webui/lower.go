package webui

import (
	"bytes"
	"fmt"
	"image/png"

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
//	Form[M] / Table[M] in a View with an M unrelated to A -> method on View
//	Link[M, A] in Control[M]   A unknown      -> method on Control[M]
//	Accessor[M] inside Form[M] M known        -> type switch
//	Reject[M] inside Outcome   M known        -> type switch
//
// So the panels of the lowering are methods that live beside their types
// (page.go, form.go, table.go, layout.go, link.go, action.go), and the
// interiors are ordinary switches (accessor.go).
func (a App) lower() (*ir.App, error) {
	brand, err := lowerBrand(a.Brand)
	if err != nil {
		return nil, err
	}
	out := &ir.App{
		Brand:            brand,
		Description:      a.Description,
		NoConsoleMessage: a.NoConsoleMessage,
		Theme:            lowerTheme(a.Theme),
		Logger:           a.Logger,
		Secret:           a.Secret,
		Observer:         a.Observer,
		ByPath:           map[string]*ir.Page{},
	}
	for _, p := range a.Pages {
		page := p.lowerPage()
		out.Pages = append(out.Pages, page)
		out.ByPath[page.PathTemplate] = page
	}
	a.lowerNavigation(out)
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

func lowerTheme(t Theme) ir.Theme {
	return ir.Theme{
		Accent: string(t.Accent), OK: string(t.OK), Warning: string(t.Warning), Critical: string(t.Critical),
		Light: lowerModeColors(t.Light), Dark: lowerModeColors(t.Dark),
	}
}

func lowerModeColors(m ModeColors) ir.ModeColors {
	return ir.ModeColors{Canvas: string(m.Canvas), Accent: string(m.Accent), OK: string(m.OK), Warning: string(m.Warning), Critical: string(m.Critical)}
}
