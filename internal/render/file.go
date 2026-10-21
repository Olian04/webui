package render

import (
	"encoding/json"
	"strings"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
	"github.com/a-h/templ"
)

// editsFile is whether a page body has a file that the visitor can choose: a file field of a form that can be submitted.
func editsFile(body ir.Node) bool {
	found := false
	ir.Walk(body, func(n ir.Node) {
		if form, ok := n.(*ir.Form); ok && form.Submit != nil {
			for f := range ir.Leaves(form.Fields) {
				found = found || (f.File != nil && f.Writable())
			}
		}
	})
	return found
}

// NeedsFile is whether page links the script that handles a file chooser, because it has one.
func (r *Renderer) NeedsFile(page *ir.Page) bool { return r.uploads[page.PathTemplate] }

// fileClient is what script is told of a file chooser, in the words the server uses: how large a file may be and what it takes, so that
// it can say so before the request is sent, and whether a picture can be shown of the file that is chosen and of the one that is stored.
// The server checks all of it again, by what the file holds, which script cannot see.
type fileClient struct {
	Max         int64    `json:"max"`
	MaxMessage  string   `json:"maxMessage"`
	Accept      []string `json:"accept,omitempty"`
	KindMessage string   `json:"kindMessage,omitempty"`
	Image       bool     `json:"image,omitempty"`   // a file that is chosen is shown, if it is a picture
	Preview     string   `json:"preview,omitempty"` // where the picture of the stored file is, a path of this site
}

// fileAttrs are the attributes of a file chooser that script reads. stored is the file that the field holds.
func fileAttrs(f ir.Field, stored *ir.StoredFile) templ.Attributes {
	c := fileClient{
		Max: f.File.Max, MaxMessage: rules.FileTooLarge(f.File.Max), Accept: rules.FileAccepts(f.File),
		KindMessage: rules.FileWrongKind(f.File), Image: rules.AcceptsImages(f.File),
	}
	// The stored file is shown as a picture only when it is one, which is known only of a field that takes nothing else, and
	// when it is on this site, since a page may only show a picture from there.
	if stored != nil && rules.OnlyImages(f.File) && strings.HasPrefix(stored.URL, "/") && !strings.HasPrefix(stored.URL, "//") && !strings.HasPrefix(stored.URL, `/\`) {
		c.Preview = stored.URL
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	return templ.Attributes{"data-file": string(b)}
}
