package render

import (
	"strconv"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
	"github.com/a-h/templ"
)

// editsFile is whether a page body has a file that the visitor can choose: a file field of a form that can be submitted, or a file upload panel.
func editsFile(body ir.Node) bool {
	found := false
	ir.Walk(body, func(n ir.Node) {
		switch n := n.(type) {
		case *ir.Form:
			if n.Submit != nil {
				for f := range ir.Leaves(n.Fields) {
					found = found || (f.File != nil && f.Writable())
				}
			}
		case *ir.FileUpload:
			found = true
		}
	})
	return found
}

// hasUpload is whether a page body has a file upload panel.
func hasUpload(body ir.Node) bool {
	found := false
	ir.Walk(body, func(n ir.Node) {
		if _, ok := n.(*ir.FileUpload); ok {
			found = true
		}
	})
	return found
}

// NeedsUpload is whether page links the script that sends the files of a file upload panel, because it has one.
func (r *Renderer) NeedsUpload(page *ir.Page) bool { return r.queues[page.PathTemplate] }

// NeedsFile is whether page links the script that handles a file chooser, because it has one.
func (r *Renderer) NeedsFile(page *ir.Page) bool { return r.uploads[page.PathTemplate] }

// fileAttrs are the attributes of a file chooser that script reads.
func fileAttrs(f ir.Field) templ.Attributes { return fileAttrsOf(f.File) }

// fileAttrsOf is fileAttrs for what a file input takes. Script is told in words that the server uses, one attribute for each, so that it can
// say so before the request is sent: data-max-size (bytes) and data-max-message, data-kind-message for a field that takes only some kinds
// (the kinds themselves are the input's own accept attribute), and data-image when a file that is chosen is shown if it is a picture. data-file
// is what marks the input. The server checks all of it again, by what the file holds, which script cannot see.
func fileAttrsOf(spec *ir.FileSpec) templ.Attributes {
	attrs := templ.Attributes{
		"data-file":        "",
		"data-max-size":    strconv.FormatInt(spec.Max, 10),
		"data-max-message": rules.FileTooLarge(spec.Max),
	}
	if m := rules.FileWrongKind(spec); m != "" {
		attrs["data-kind-message"] = m
	}
	if rules.AcceptsImages(spec) {
		attrs["data-image"] = true
	}
	return attrs
}
