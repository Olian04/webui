package render

import (
	"encoding/json"

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

// NeedsFile is whether page links the script that handles a file chooser, because it has one.
func (r *Renderer) NeedsFile(page *ir.Page) bool { return r.uploads[page.PathTemplate] }

// fileClient is what script is told of a file chooser, in the words the server uses: how large a file may be and what it takes, so that
// it can say so before the request is sent, and whether a picture can be shown of the file that is chosen.
// The server checks all of it again, by what the file holds, which script cannot see.
type fileClient struct {
	Max         int64    `json:"max"`
	MaxMessage  string   `json:"maxMessage"`
	Accept      []string `json:"accept,omitempty"`
	KindMessage string   `json:"kindMessage,omitempty"`
	Image       bool     `json:"image,omitempty"` // a file that is chosen is shown, if it is a picture
}

// fileAttrs are the attributes of a file chooser that script reads.
func fileAttrs(f ir.Field) templ.Attributes { return fileAttrsOf(f.File) }

// fileAttrsOf is fileAttrs for what a file input takes.
func fileAttrsOf(spec *ir.FileSpec) templ.Attributes {
	c := fileClient{
		Max: spec.Max, MaxMessage: rules.FileTooLarge(spec.Max), Accept: rules.FileAccepts(spec),
		KindMessage: rules.FileWrongKind(spec), Image: rules.AcceptsImages(spec),
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	return templ.Attributes{"data-file": string(b)}
}
