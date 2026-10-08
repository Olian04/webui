// Package ir is the handoff between the declaration in pkg/webui and the
// runtime that serves it.
//
// The level is what the app means, not how it is served. Paths are templates,
// not routes: internal/router derives matching from them. That keeps every
// serving decision downstream, so the declaration layer never grows an opinion
// about HTTP.
//
// Two rules keep the boundary honest, both mechanically checkable:
//
//   - Nothing here may mention http, html, url, or any transport concept.
//     If it does, the runtime has leaked upward.
//   - Nothing here may be generic or hold a reflect.Type at run time.
//     If it does, pkg/webui has leaked downward.
//
// Everything here is plain data or a closure with an erased signature.
// pkg/webui built those closures while lowering, which is where every type
// assertion lives. Downstream packages never assert.
//
// The IR is allowed to be looser than the declaration: NumberRules on a string
// field is representable here. pkg/webui proves well-formedness before
// lowering, and re-proving it downstream is duplicated work.
package ir

// Node is a leaf or a layout. Leaves load and refresh independently; layouts
// only arrange, and never load, guard, or refresh.
type Node interface {
	Kind() NodeKind
	// Addr is the node's position in the body tree, assigned while lowering.
	// A section-refresh request names this, and it is stable for the life of
	// one compiled app.
	Addr() Addr
}

// NodeKind tags a Node for diagnostics and exhaustive switching.
type NodeKind uint8

// The node kinds. Layouts first, then leaves.
const (
	NodeStack NodeKind = iota
	NodeSplit
	NodeTabs
	NodeForm
	NodeTable
)

// Addr is a path of child indices from the page body, such as [0 1].
type Addr []int

// OutcomeKind says how an action ended, as the action meant it.
type OutcomeKind uint8

// The kinds. Success is the zero value: an action that says nothing has
// succeeded quietly.
const (
	OutcomeSuccess OutcomeKind = iota // done and accepted
	OutcomeWarning                    // done and accepted, but the user should know something
	OutcomeFailure                    // not done; the form is shown again with what was typed
	OutcomeReject                     // not done; the form is shown again with what was typed, by field
)

// Outcome is what an action reports when it runs and no error stopped it. Fields
// is set only for a rejection. Redirect is an already-resolved URL: pkg/webui
// built it with Open, which had ctx and therefore the mount prefix. The
// no-transport rule governs what gets lowered, not values flowing back through
// the runtime.
type Outcome struct {
	Kind     OutcomeKind
	Message  string
	Redirect string // empty means stay, or return to where the form was opened from
	Fields   []FieldError
}

// FieldError attaches a message to a Field by label, unique within a form.
type FieldError struct {
	Label   string
	Message string
}

// ASSERT: nodes implement Node
var (
	_ Node = (*Stack)(nil)
	_ Node = (*Split)(nil)
	_ Node = (*Tabs)(nil)
	_ Node = (*Form)(nil)
	_ Node = (*Table)(nil)
)
