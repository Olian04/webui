package render

import (
	"github.com/Olian04/webui/internal/ir"
)

// editsPhone is whether a page body has a phone number that the visitor can edit: a field of a form that can be submitted.
func editsPhone(body ir.Node) bool {
	found := false
	ir.Walk(body, func(n ir.Node) {
		switch n := n.(type) {
		case *ir.Form:
			if n.Submit != nil {
				for f := range ir.Leaves(n.Fields) {
					found = found || (f.Writable() && f.Rules.Format == ir.FormatTel)
				}
			}
		}
	})
	return found
}

// NeedsPhone is whether page links the script that splits a phone number, because it has one to edit.
func (r *Renderer) NeedsPhone(page *ir.Page) bool { return r.phones[page.PathTemplate] }

// scripts are the page's scripts: the one every page has, the phone's for a page that edits a number, and the file's for a page that
// has a file to choose.
func (r *Renderer) scripts(d Doc) []string {
	s := []string{r.AssetHref("enhance.js")}
	if d.Phone {
		s = append(s, r.AssetHref("phone.js"))
	}
	if d.File {
		s = append(s, r.AssetHref("file.js"))
	}
	return s
}
