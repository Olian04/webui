// Package page reads a response body the way a person, or the screen reader of one, does: by role and name, by the text that is shown, by the label of a field.
// A test that asks "is there a link called Devices to /admin/device" does not break when the class that styles it is renamed, and one that asks for the
// class does.
//
// It models a client that runs no script: <noscript> is read as part of the page, and what the page says only to a reader (an element that is
// aria-hidden, or hidden) is left out of Text and kept in AllText. It imports no package of this module, as test/util/assert does not.
package page

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// TB is the testing surface this package calls. *testing.T and *testing.B implement it.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Page is a parsed response body: a whole document, or the fragment that a request for one panel is answered with.
type Page struct{ root El }

// El is an element of a page.
type El struct{ n *html.Node }

// Parse reads body. It stops the test when body cannot be read as HTML, which html.Parse says for almost nothing, so in practice it does not.
func Parse(t TB, body string) *Page {
	t.Helper()
	doc, err := html.ParseWithOptions(strings.NewReader(body), html.ParseOptionEnableScripting(false))
	if err != nil {
		t.Fatalf("the body is not HTML: %v", err)
	}
	return &Page{root: El{doc}}
}

// IsFragment is whether body is a piece of a page, as the answer to a request for one panel is, and not a document.
func IsFragment(body string) bool {
	head := strings.ToLower(strings.TrimSpace(body))
	return !strings.HasPrefix(head, "<!doctype") && !strings.HasPrefix(head, "<html")
}

// Root is the whole page as an element, to search below.
func (p *Page) Root() El { return p.root }

// Title is the text of the document's <title>, which is what a tab and the history say.
func (p *Page) Title() string {
	for _, e := range p.root.descendants() {
		if e.n.Data == "title" && e.n.Parent != nil {
			return collapse(rawText(e.n))
		}
	}
	return ""
}

// ByRole finds the elements of a role, in document order. name is what each is called, exactly, which for a link or a button is its text and for a
// table or a navigation its label; "" finds them whatever they are called. A role is the element's own (a, button, h2, table, nav) or the one it is
// given with the role attribute.
func (p *Page) ByRole(role, name string) []El { return p.root.ByRole(role, name) }

// ByRoleContaining is ByRole for an element whose name contains name.
func (p *Page) ByRoleContaining(role, name string) []El { return p.root.ByRoleContaining(role, name) }

// ByRoleMatching is ByRole for an element whose name matches re.
func (p *Page) ByRoleMatching(role string, re *regexp.Regexp) []El {
	return p.root.ByRoleMatching(role, re)
}

// ByText finds the innermost elements whose text contains text, in document order.
func (p *Page) ByText(text string) []El { return p.root.ByText(text) }

// ByAllText is ByText over AllText: it finds what only a reader that asks is told, such as the rules in the popover beside a field's label.
func (p *Page) ByAllText(text string) []El { return p.root.ByAllText(text) }

// ByLabel finds the form controls that are named label: by a <label>, an aria-label or an aria-labelledby.
func (p *Page) ByLabel(label string) []El { return p.root.ByLabel(label) }

// ByAttr finds the elements that have the attribute, and when value is not "" that give it that value. It is for what a script or a post depends on
// (name, data-view, data-refresh-after) and is not a way to find a class.
func (p *Page) ByAttr(attr, value string) []El { return p.root.ByAttr(attr, value) }

// Links are the links of the page, in order.
func (p *Page) Links() []Link { return p.root.Links() }

// Field is the form control that posts name, if there is one.
func (p *Page) Field(name string) (Field, bool) { return p.root.Field(name) }

// Fields are the form controls of the page that post something, in order.
func (p *Page) Fields() []Field { return p.root.Fields() }

// Views are the elements that a request can ask for alone, by the id that data-view says, in order.
func (p *Page) Views() []El { return p.root.ByAttr("data-view", "") }

// Live are the live regions of the page: what a screen reader says as it appears, which is what a notice is.
func (p *Page) Live() []El { return p.root.Live() }

// One returns the only element of els, and stops the test when there are none or several, so that "the Devices table" is a table and not a guess.
func One(t TB, what string, els []El) El {
	t.Helper()
	if len(els) != 1 {
		t.Fatalf("want exactly one %s, found %d", what, len(els))
	}
	return els[0]
}

// Link is a link as a reader meets it.
type Link struct {
	// Name is what the link is called, which is its text unless it has an aria-label.
	Name string
	// Href is where it leads, as written.
	Href string
	// Current is whether it says that it is the page, or the place, that the reader is at (aria-current).
	Current bool
	// NewTab is whether it opens in a tab of its own.
	NewTab bool
}

// Field is a form control that posts a value.
type Field struct {
	Name     string
	Type     string // text, number, checkbox, select, textarea, hidden, ...
	Value    string // what it posts: the value of an input, the selected option of a select (all, comma-joined, for a multiple), the text of a textarea
	Checked  bool
	Disabled bool
	Required bool
	// Options are the options of a select, each by its value and its text.
	Options []Option
}

// Option is an option of a select.
type Option struct {
	Value, Text string
	Selected    bool
}

// ByRole is Page.ByRole below the element.
func (e El) ByRole(role, name string) []El {
	return e.find(role, func(got string) bool { return name == "" || got == name })
}

// ByRoleContaining is Page.ByRoleContaining below the element.
func (e El) ByRoleContaining(role, name string) []El {
	return e.find(role, func(got string) bool { return strings.Contains(got, name) })
}

// ByRoleMatching is ByRole for an element whose name matches re.
func (e El) ByRoleMatching(role string, re *regexp.Regexp) []El {
	return e.find(role, re.MatchString)
}

func (e El) find(role string, ok func(name string) bool) []El {
	var out []El
	for _, d := range e.below() {
		if d.n.Type == html.ElementNode && d.Role() == role && ok(d.Name()) {
			out = append(out, d)
		}
	}
	return out
}

// ByText is Page.ByText below the element.
func (e El) ByText(text string) []El { return e.byText(text, El.Text) }

func (e El) byText(text string, read func(El) string) []El {
	var out []El
	for _, d := range e.below() {
		if d.n.Type != html.ElementNode || !strings.Contains(read(d), text) {
			continue
		}
		inner := false
		for _, c := range d.below() {
			if c.n.Type == html.ElementNode && strings.Contains(read(c), text) {
				inner = true
				break
			}
		}
		if !inner {
			out = append(out, d)
		}
	}
	return out
}

// ByAllText is Page.ByAllText below the element.
func (e El) ByAllText(text string) []El { return e.byText(text, El.AllText) }

// ByLabel is Page.ByLabel below the element.
func (e El) ByLabel(label string) []El {
	var out []El
	for _, d := range e.below() {
		if isControl(d.n) && d.Name() == label {
			out = append(out, d)
		}
	}
	return out
}

// ByAttr is Page.ByAttr below the element.
func (e El) ByAttr(attr, value string) []El {
	var out []El
	for _, d := range e.below() {
		if d.n.Type != html.ElementNode {
			continue
		}
		if v, ok := d.attr(attr); ok && (value == "" || v == value) {
			out = append(out, d)
		}
	}
	return out
}

// Links is Page.Links below the element.
func (e El) Links() []Link {
	var out []Link
	for _, a := range e.ByRole("link", "") {
		href, _ := a.attr("href")
		cur, _ := a.attr("aria-current")
		target, _ := a.attr("target")
		out = append(out, Link{Name: a.Name(), Href: href, Current: cur != "" && cur != "false", NewTab: target == "_blank"})
	}
	return out
}

// Live is Page.Live below the element.
func (e El) Live() []El {
	var out []El
	for _, d := range e.below() {
		if d.n.Type != html.ElementNode {
			continue
		}
		role, _ := d.attr("role")
		live, _ := d.attr("aria-live")
		if role == "status" || role == "alert" || live == "polite" || live == "assertive" {
			out = append(out, d)
		}
	}
	return out
}

// Attr is the value of an attribute, "" when it has none.
func (e El) Attr(name string) string {
	v, _ := e.attr(name)
	return v
}

// Has is whether the element has the attribute, whatever it says.
func (e El) Has(name string) bool {
	_, ok := e.attr(name)
	return ok
}

// Tag is the element's own name: a, td, section.
func (e El) Tag() string { return e.n.Data }

// Text is what the element shows to a reader: its text with the runs of white space made one space, without what is hidden (hidden, aria-hidden,
// script and style) and without what a <details> that is shut keeps from view but for its summary.
func (e El) Text() string { return collapse(visibleText(e.n, false)) }

// AllText is Text with what is aria-hidden kept: the popover that says the rules of a field is there for a reader that asks, and a screen reader's
// virtual cursor does not stop at it.
func (e El) AllText() string { return collapse(visibleText(e.n, true)) }

// Hidden is whether the element is not shown: it, or an element above it, is hidden or aria-hidden.
func (e El) Hidden() bool {
	for n := e.n; n != nil && n.Type == html.ElementNode; n = n.Parent {
		if hiddenNode(n) {
			return true
		}
	}
	return false
}

// Role is the element's role: the one it says with the role attribute, or the one its tag has ("" for an element that has none).
func (e El) Role() string {
	if r := strings.Fields(e.Attr("role")); len(r) > 0 {
		return r[0]
	}
	return implicitRole(e)
}

// Name is what the element is called to a reader: aria-labelledby, aria-label, then what is native to the element (a label, an alt, a caption, a
// value), then its text for the roles that are named by it, then its title.
func (e El) Name() string {
	if ids := strings.Fields(e.Attr("aria-labelledby")); len(ids) > 0 {
		var parts []string
		for _, id := range ids {
			if t := e.byID(id); t != nil {
				parts = append(parts, El{t}.AllText())
			}
		}
		if s := strings.Join(parts, " "); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(e.Attr("aria-label")); s != "" {
		return s
	}
	if s := e.nativeName(); s != "" {
		return s
	}
	if namedByContent(e.Role()) {
		if s := e.Text(); s != "" {
			return s
		}
	}
	return strings.TrimSpace(e.Attr("title"))
}

// Before is whether the element comes before other in the page, in the order that a reader meets them.
func (e El) Before(other El) bool {
	for _, d := range (El{e.root()}).descendants() {
		switch d.n {
		case e.n:
			return true
		case other.n:
			return false
		}
	}
	return false
}

// Children are the elements directly below the element.
func (e El) Children() []El {
	var out []El
	for c := e.n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, El{c})
		}
	}
	return out
}

// Headers are the column headers of a table: the text of the <th> of its first row.
func (e El) Headers() []string {
	for _, row := range e.ByRole("row", "") {
		var out []string
		for _, c := range row.Children() {
			if c.Role() == "columnheader" {
				out = append(out, c.Text())
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// Rows are the rows of a table that have cells, each as the text of its cells, in order: the header row is not one of them.
func (e El) Rows() [][]string {
	var out [][]string
	for _, row := range e.ByRole("row", "") {
		var cells []string
		for _, c := range row.Children() {
			if r := c.Role(); r == "cell" || r == "rowheader" || r == "gridcell" {
				cells = append(cells, c.Text())
			}
		}
		if len(cells) > 0 {
			out = append(out, cells)
		}
	}
	return out
}

// Field is Page.Field below the element.
func (e El) Field(name string) (Field, bool) {
	for _, f := range e.Fields() {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// Fields is Page.Fields below the element.
func (e El) Fields() []Field {
	var out []Field
	seen := map[string]int{}
	for _, d := range e.below() {
		if d.n.Type != html.ElementNode || !isControl(d.n) {
			continue
		}
		name := d.Attr("name")
		if name == "" {
			continue
		}
		f := Field{Name: name, Type: controlType(d), Disabled: d.Has("disabled"), Required: d.Has("required"), Checked: d.Has("checked")}
		switch d.n.Data {
		case "textarea":
			f.Value = strings.TrimPrefix(rawText(d.n), "\n")
		case "select":
			var chosen []string
			for _, o := range d.ByRole("option", "") {
				val := o.Attr("value")
				if !o.Has("value") {
					val = o.Text()
				}
				f.Options = append(f.Options, Option{Value: val, Text: o.Text(), Selected: o.Has("selected")})
				if o.Has("selected") {
					chosen = append(chosen, val)
				}
			}
			f.Value = strings.Join(chosen, ",")
		default:
			f.Value = d.Attr("value")
		}
		if i, ok := seen[name]; ok && (f.Type == "radio" || f.Type == "checkbox") {
			// A group of radios or checkboxes posts the one that is checked; the first of them stands for the group unless another is checked.
			if f.Checked {
				out[i] = f
			}
			continue
		}
		seen[name] = len(out)
		out = append(out, f)
	}
	return out
}

func (e El) attr(name string) (string, bool) {
	for _, a := range e.n.Attr {
		if a.Key == name {
			return a.Val, true
		}
	}
	return "", false
}

// below is every element under the element, in document order, and not the element itself.
func (e El) below() []El { return e.descendants()[1:] }

// descendants is the element and all below it, in document order.
func (e El) descendants() []El {
	var out []El
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		out = append(out, El{n})
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(e.n)
	return out
}

func (e El) root() *html.Node {
	n := e.n
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}

func (e El) byID(id string) *html.Node {
	for _, d := range (El{e.root()}).descendants() {
		if d.n.Type == html.ElementNode && d.Attr("id") == id {
			return d.n
		}
	}
	return nil
}

// nativeName is the name that the element's own markup gives it.
func (e El) nativeName() string {
	switch e.n.Data {
	case "input", "select", "textarea", "button", "meter", "progress", "output":
		if id := e.Attr("id"); id != "" {
			for _, l := range (El{e.root()}).descendants() {
				if l.n.Type == html.ElementNode && l.n.Data == "label" && l.Attr("for") == id {
					if s := l.Text(); s != "" {
						return s
					}
				}
			}
		}
		for p := e.n.Parent; p != nil; p = p.Parent {
			if p.Type == html.ElementNode && p.Data == "label" {
				if s := (El{p}).textWithout(e.n); s != "" {
					return s
				}
			}
		}
		if e.n.Data == "input" {
			switch controlType(e) {
			case "submit", "button", "reset":
				if v := strings.TrimSpace(e.Attr("value")); v != "" {
					return v
				}
				if controlType(e) == "submit" {
					return "Submit"
				}
			case "image":
				return strings.TrimSpace(e.Attr("alt"))
			}
			return strings.TrimSpace(e.Attr("placeholder"))
		}
		if e.n.Data == "textarea" {
			return strings.TrimSpace(e.Attr("placeholder"))
		}
	case "img":
		return strings.TrimSpace(e.Attr("alt"))
	case "table":
		for _, c := range e.Children() {
			if c.n.Data == "caption" {
				return c.Text()
			}
		}
	case "fieldset":
		for _, c := range e.Children() {
			if c.n.Data == "legend" {
				return c.Text()
			}
		}
	}
	return ""
}

func (e El) textWithout(skip *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n == skip || hiddenNode(n) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(e.n)
	return collapse(b.String())
}

func hiddenNode(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.Data {
	case "script", "style", "template", "head":
		return true
	}
	for _, a := range n.Attr {
		if a.Key == "hidden" || (a.Key == "aria-hidden" && a.Val == "true") {
			return true
		}
	}
	return false
}

func visibleText(n *html.Node, withAriaHidden bool) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "template", "head":
				return
			}
			for _, a := range n.Attr {
				if a.Key == "hidden" || (a.Key == "aria-hidden" && a.Val == "true" && !withAriaHidden) {
					return
				}
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		closed := n.Type == html.ElementNode && n.Data == "details" && !withAriaHidden && !(El{n}).Has("open")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if closed && (c.Type != html.ElementNode || c.Data != "summary") {
				continue // a details that is shut shows its summary and nothing else
			}
			walk(c)
		}
		if n.Type == html.ElementNode && blockLike(n.Data) {
			b.WriteByte(' ')
		}
	}
	walk(n)
	return b.String()
}

func blockLike(tag string) bool {
	switch tag {
	case "div", "p", "li", "tr", "td", "th", "h1", "h2", "h3", "h4", "h5", "h6", "section", "article", "ul", "ol", "table", "form", "br", "option", "label", "button":
		return true
	}
	return false
}

func rawText(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

var space = regexp.MustCompile(`\s+`)

func collapse(s string) string { return strings.TrimSpace(space.ReplaceAllString(s, " ")) }

func isControl(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.Data {
	case "input", "select", "textarea", "button":
		return true
	}
	return false
}

func controlType(e El) string {
	switch e.n.Data {
	case "select", "textarea", "button":
		return e.n.Data
	}
	if t := strings.ToLower(e.Attr("type")); t != "" {
		return t
	}
	return "text"
}

func namedByContent(role string) bool {
	switch role {
	case "button", "link", "heading", "cell", "columnheader", "rowheader", "gridcell", "listitem", "option", "tab", "menuitem", "treeitem", "checkbox", "radio", "switch", "status", "alert", "tooltip", "caption", "term", "definition", "paragraph":
		return true
	}
	return false
}

func implicitRole(e El) string {
	switch tag := e.n.Data; tag {
	case "a", "area":
		if e.Has("href") {
			return "link"
		}
	case "article", "aside", "main", "nav", "form", "dialog", "table", "img", "meter", "output", "option", "li", "tr", "td", "th", "progress", "hr", "fieldset", "details":
		switch tag {
		case "aside":
			return "complementary"
		case "nav":
			return "navigation"
		case "img":
			if e.Has("alt") && e.Attr("alt") == "" {
				return "presentation"
			}
			return "img"
		case "li":
			return "listitem"
		case "tr":
			return "row"
		case "td":
			return "cell"
		case "th":
			if e.Attr("scope") == "row" {
				return "rowheader"
			}
			return "columnheader"
		case "progress":
			return "progressbar"
		case "hr":
			return "separator"
		case "fieldset", "details":
			return "group"
		case "output":
			return "status"
		}
		return tag
	case "button", "summary":
		return "button"
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return "heading"
	case "ul", "ol":
		return "list"
	case "thead", "tbody", "tfoot":
		return "rowgroup"
	case "textarea":
		return "textbox"
	case "select":
		if size, _ := strconv.Atoi(e.Attr("size")); e.Has("multiple") || size > 1 {
			return "listbox"
		}
		return "combobox"
	case "header", "footer":
		for p := e.n.Parent; p != nil; p = p.Parent {
			if p.Type == html.ElementNode {
				switch p.Data {
				case "article", "aside", "main", "nav", "section":
					return ""
				}
			}
		}
		if tag == "header" {
			return "banner"
		}
		return "contentinfo"
	case "section":
		if e.Attr("aria-label") != "" || e.Attr("aria-labelledby") != "" {
			return "region"
		}
	case "input":
		switch controlType(e) {
		case "button", "submit", "reset", "image":
			return "button"
		case "checkbox":
			return "checkbox"
		case "radio":
			return "radio"
		case "range":
			return "slider"
		case "number":
			return "spinbutton"
		case "search":
			return "searchbox"
		case "hidden", "file", "color", "date", "datetime-local", "month", "week", "time", "password":
			return ""
		}
		return "textbox"
	}
	return ""
}
