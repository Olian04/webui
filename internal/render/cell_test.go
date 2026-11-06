package render

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	c "github.com/Olian04/webui/internal/render/templates/components"
	"github.com/Olian04/webui/test/util/assert"
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
	assert.Equal(t, drawn(t, r.cell(columnView{Field: ir.Field{Get: func(any) string { return "x<y" }}}, nil, "", false, false, "", false)), `<td class="">x&lt;y</td>`)
	assert.Equal(t, drawn(t, r.cell(columnView{Align: c.AlignEnd, Field: ir.Field{Get: func(any) string { return "12" }}}, nil, "", false, false, "", false)), `<td class="num">12</td>`)
}
