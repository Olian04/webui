package webui

import (
	"fmt"

	"github.com/Olian04/webui/internal/ir"
)

// validateHeadActions checks the controls of a panel's head, where what names the panel's field. They are a toolbar's items, except that a [Form] is not one: a
// panel's head has no room for values to read, and a form of one is a panel.
func validateHeadActions(v *viewValidator, what string, items []ToolbarItem) {
	seen := map[string]bool{}
	for i, item := range items {
		at := fmt.Sprintf("%s.HeadActions[%d]", what, i)
		if item == nil {
			v.add(at+" is nil", "Remove it or declare a control.")
			continue
		}
		if item.lowerToolbarItem(ir.Pos{}, "").Form != nil {
			v.add(at+" is a Form", "A panel's head has buttons and links: an Action or a Link of NoArgs. Put a Form on the page as a panel, or in a Toolbar to show its values.")
			continue
		}
		item.validateToolbarItem(v, at, seen)
	}
}

// lowerHeadActions are the controls of a panel's head as the IR holds them, in order.
func lowerHeadActions(at ir.Pos, items []ToolbarItem) []ir.ToolbarItem {
	var out []ir.ToolbarItem
	for _, item := range items {
		if item != nil {
			out = append(out, item.lowerToolbarItem(at, "h"))
		}
	}
	return out
}
