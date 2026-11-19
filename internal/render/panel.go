package render

import (
	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
)

// frame is the frame of a panel, as every kind of panel draws it: its title and description, how it failed when it did, and where to try again. The kind adds what is its own,
// a footer or room to grow, to the props it gets.
func (r *Renderer) frame(at ir.Pos, title, desc string, failed bool, refusal *ir.Refusal, retry string) c.PanelProps {
	return c.PanelProps{Title: title, Desc: desc, Failed: failed, Refused: refusedOf(refusal), Retry: retry, ViewID: ViewID(at)}
}

// postForm is the form that posts to the panel at, from the page as it is drawn: its address, and the panel it says it is for. attrs are what the kind adds to the form.
func (r *Renderer) postForm(page *ir.Page, path, query map[string]string, at ir.Pos, attrs templ.Attributes) c.FormProps {
	return c.FormProps{Action: r.PageHref(page, path, query), View: ViewID(at), Attrs: attrs}
}

// Controls are the buttons and links of a toolbar or of a panel's head, by the position they were declared at, with what each is for this visitor: whether its Guard lets them
// (a button whose Guard refuses them, or a link to a page that does, is not drawn, or is drawn disabled), and where each link goes. A form of a toolbar is neither.
type Controls struct {
	Items  []ir.ToolbarItem
	Access []Access
	Hrefs  []string
}

func (v Controls) access(i int) Access {
	if i < len(v.Access) {
		return v.Access[i]
	}
	return Access{}
}

// posts is whether there is a button that posts and is drawn, and so a form to post it with. A row of links and values has none: there is nothing in it to submit.
func (v Controls) posts() bool {
	for i, it := range v.Items {
		if it.Action != nil && v.access(i).Drawn() {
			return true
		}
	}
	return false
}

// drawn is whether any control is drawn for this visitor.
func (v Controls) drawn() bool {
	for i := range v.Items {
		if v.access(i).Drawn() {
			return true
		}
	}
	return false
}
