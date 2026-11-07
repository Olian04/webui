package ir_test

import (
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

// tree has every layout, so that Children and Walk are held to the order a page shows them: a Stack's and a Columns' children as they are,
// a When's child, and a Tabs' bodies, selected or not.
func tree() (root ir.Node, want []ir.Node) {
	a, b, c, d, e := &ir.Markdown{}, &ir.Markdown{}, &ir.Markdown{}, &ir.Markdown{}, &ir.Markdown{}
	when := &ir.When{Child: c}
	tabs := &ir.Tabs{Tabs: []ir.Tab{{Key: "x", Child: d}, {Key: "y", Child: e}}}
	cols := &ir.Columns{Nodes: []ir.Node{when, tabs}}
	root = &ir.Stack{Nodes: []ir.Node{a, b, cols}}
	return root, []ir.Node{root, a, b, cols, when, c, tabs, d, e}
}

func TestChildren(t *testing.T) {
	root, _ := tree()
	assert.Equal(t, len(root.Children()), 3)
	assert.Equal(t, len((&ir.Markdown{}).Children()), 0)
	assert.Equal(t, len((&ir.When{Child: &ir.Markdown{}}).Children()), 1)
	assert.Equal(t, len((&ir.Tabs{Tabs: []ir.Tab{{}, {}}}).Children()), 2)
}

func TestWalkVisitsParentsBeforeChildrenInPageOrder(t *testing.T) {
	root, want := tree()
	var got []ir.Node
	ir.Walk(root, func(n ir.Node) { got = append(got, n) })
	assert.Equal(t, len(got), len(want))
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("visit %d: got %T %p, want %T %p", i, got[i], got[i], want[i], want[i])
		}
	}
}

func TestWalkNilVisitsNothing(t *testing.T) {
	ir.Walk(nil, func(ir.Node) { t.Fatal("visited") })
}
