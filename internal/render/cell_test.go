package render

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

func drawn(t *testing.T, comp templ.Component) string {
	t.Helper()

	var b bytes.Buffer
	assert.NoError(t, comp.Render(context.Background(), &b))
	return b.String()
}

// A cell that holds plain text is written without the components that a cell is made of otherwise, and has to be what they wrote, to the byte, for every kind of text, either alignment,
// and every way that a cell can be more than text, in which case it is drawn by them.
func TestAPlainCellIsWhatTheComponentsWrote(t *testing.T) {
	t.Parallel()

	r, err := New(&ir.App{}, "", []byte("0123456789abcdef"))
	assert.NoError(t, err)

	texts := []string{"", "plain", "  padded  ", "<b>bold</b> & \"quoted\" 'single'", "line one\nline two", "Ünï ✓", "a\tb", "&amp;"}
	checked := 0
	for _, text := range texts {
		for _, align := range []c.Align{c.AlignStart, c.AlignEnd} {
			for _, display := range []ir.Display{ir.DisplayText, ir.DisplayLink, ir.DisplayTime} {
				for _, multiline := range []bool{false, true} {
					for _, nullable := range []bool{false, true} {
						for _, how := range []struct {
							href     string
							external bool
							button   bool
							click    string
							first    bool
						}{
							{}, {first: true}, {href: "/admin/x", first: true}, {href: "/admin/x"}, {button: true, first: true, click: "7"}, {button: true, click: "7"},
							{href: "/admin/x?a=1&b=\"2\"", first: true, external: true}, {href: "javascript:alert(1)", first: true}, {href: "https://example.com/a b", first: true, external: true},
							{button: true, first: true, click: `r"1<a>&'x`}, {button: true, first: true}, {href: "/admin/x", button: true, first: true, click: "9"},
						} {
							col := columnView{Align: align, Field: ir.Field{
								Display: display, Multiline: multiline, Nullable: nullable,
								Get: func(any) string { return text },
							}}
							fast := drawn(t, r.cell(col, nil, how.href, how.external, how.button, how.click, how.first))
							slow := drawn(t, r.cellComponents(col, nil, how.href, how.external, how.button, how.click, how.first))
							if fast != slow {
								t.Fatalf("text %q, %v, display %v, multiline %v, nullable %v, %+v:\nfast: %s\nslow: %s", text, align, display, multiline, nullable, how, fast, slow)
							}
							checked++
						}
					}
				}
			}
		}
	}
	assert.True(t, checked > 1500)
	// What a cell says is the text it was given, whatever the text holds: markup in it is text.
	for _, text := range texts {
		cell := page.Parse(t, "<table><tbody><tr>"+drawn(t, r.cell(columnView{Field: ir.Field{Get: func(any) string { return text }}}, nil, "", false, false, "", false))+"</tr></tbody></table>")
		row := page.One(t, "the table", cell.ByRole("table", "")).Rows()
		assert.Equal(t, len(row), 1)
		assert.Equal(t, len(row[0]), 1)
		assert.Equal(t, row[0][0], strings.Join(strings.Fields(text), " ")) // as a reader reads it: white space is one space
	}
}

// A cell that leads somewhere is a link to an address that a browser may follow and to no other.
func TestACellThatIsALinkLeadsOnlyWhereABrowserMayFollow(t *testing.T) {
	t.Parallel()

	r, err := New(&ir.App{}, "", []byte("0123456789abcdef"))
	assert.NoError(t, err)

	link := func(href string) []page.Link {
		field := ir.Field{Display: ir.DisplayLink, Get: func(any) string { return "go" }}
		cell := drawn(t, r.cell(columnView{Field: field}, nil, href, true, false, "", true))
		return page.Parse(t, "<table><tbody><tr>"+cell+"</tr></tbody></table>").Links()
	}
	got := link("https://example.com/a b")
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].Href, "https://example.com/a b")
	assert.True(t, got[0].NewTab)
	for _, href := range []string{"javascript:alert(1)", "data:text/html,<b>x</b>"} {
		for _, l := range link(href) {
			assert.False(t, strings.HasPrefix(l.Href, "javascript:") || strings.HasPrefix(l.Href, "data:"))
		}
	}
}
