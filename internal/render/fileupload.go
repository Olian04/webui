package render

import (
	"fmt"
	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// FileUploadView is a file upload panel with what it needs of this request: where it posts, and what the visitor may do with it. The runtime fills it.
type FileUploadView struct {
	Node *ir.FileUpload
	Page *ir.Page

	// Path and Query are the page's current address, as its arguments and view state encode.
	Path, Query map[string]string

	// Form is the options that the visitor chooses beside the file, as a form with no submit that is drawn so (its fields are the inputs), with the defaults that Load
	// gave; it is read-only when the panel is disabled.
	Form FormView

	Failed  bool        // the options could not be loaded or read: the panel says so, the page stands
	Refusal *ir.Refusal // what Load said the visitor may be told, in place of "Could not load"

	// Gate is what the panel is for this visitor: its Guard, asked with a file that has not arrived, refused them (Hidden), or disabled it, with a reason.
	Gate Gate
}

// uploadField is the name of the file input, which the runtime reads the file from.
const uploadField = "file"

// inputID is the id of the panel's file input, which its label is for.
func (v FileUploadView) inputID() string { return "up-" + LeafID(v.Node.At) }

// attrs are the attributes of the file input: what script reads, and for a visitor who may not, that it is disabled (which script then leaves alone).
func (v FileUploadView) attrs() templ.Attributes {
	if v.Gate.Reason != "" {
		return templ.Attributes{"disabled": true}
	}
	return fileAttrsOf(v.Node.Spec)
}

// ruleLines are what the panel says of itself in the popover beside the label: what it takes, and how large.
func (v FileUploadView) ruleLines() []string { return rules.FileLines(v.Node.Spec) }

// formAttrs are the attributes of the panel's form: that it sends a file, and what script is told of how, as JSON in data-upload.
func (v FileUploadView) formAttrs() templ.Attributes {
	return templ.Attributes{"enctype": "multipart/form-data", "data-upload": fmt.Sprintf(`{"multiple":%t}`, v.Node.Multiple)}
}
