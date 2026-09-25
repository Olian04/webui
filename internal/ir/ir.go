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

import "context"

// App is one compiled program. It carries no mount prefix: where the app is
// mounted is a transport concern, passed to the runtime separately.
type App struct {
	Brand  Brand
	Theme  Theme
	Pages  []*Page
	ByPath map[string]*Page // destination resolution, not a route table
}

// Brand is the product identity in the chrome. Logo is encoded bytes; how to
// serve them is the runtime's choice.
type Brand struct {
	Name string
	Logo []byte
	Mime string
}

// Theme is resolved visual settings. The runtime turns tokens into CSS.
type Theme struct {
	Tokens map[string]string
}

// Page is one mounted page with its arguments resolved.
type Page struct {
	// PathTemplate is "/device/{id}". A template, not a route.
	PathTemplate string
	Args         []ArgSpec
	Nav          Nav
	Body         Node

	// Decode builds the argument struct from raw request values. Built while
	// lowering, so no reflection happens per request.
	Decode func(path map[string]string, query map[string][]string) (any, error)

	// Guard receives the decoded arguments. Runs before anything loads, on
	// every request to this page including section fetches.
	Guard func(ctx context.Context, args any) error
}

// ArgSpec is one field of a page's argument struct, resolved.
type ArgSpec struct {
	Name   string // URL name, after any webui tag
	Field  string // Go field name, for diagnostics
	Kind   ValueKind
	InPath bool // path segment, else query parameter
}

// ValueKind is the set of argument and accessor value types.
type ValueKind uint8

// Supported value kinds. A URL carries one value per name, so these are scalar.
const (
	KindString ValueKind = iota
	KindBool
	KindInt
	KindFloat
)

// Nav is a navbar entry. Shadow is the PathTemplate of the page whose entry
// stays active while this page is open; the declaration's pointer identity is
// resolved to that template here.
type Nav struct {
	Label  string
	Shadow string
	Hidden bool
}

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

// Stack arranges children vertically.
type Stack struct {
	At       Addr
	Children []Node
}

// Kind reports the node kind.
func (s *Stack) Kind() NodeKind { return NodeStack }

// Addr reports the node's position in the body tree.
func (s *Stack) Addr() Addr { return s.At }

// Split arranges children side by side.
type Split struct {
	At       Addr
	Children []Node
}

// Kind reports the node kind.
func (s *Split) Kind() NodeKind { return NodeSplit }

// Addr reports the node's position in the body tree.
func (s *Split) Addr() Addr { return s.At }

// Tabs arranges children as labeled panels.
type Tabs struct {
	At   Addr
	Tabs []Tab
}

// Kind reports the node kind.
func (t *Tabs) Kind() NodeKind { return NodeTabs }

// Addr reports the node's position in the body tree.
func (t *Tabs) Addr() Addr { return t.At }

// Tab is one labeled panel.
type Tab struct {
	Label string
	Body  Node
}

// Form is a leaf holding one model.
type Form struct {
	At Addr

	// Load returns the model. Its concrete type is known only to the closures
	// pkg/webui built; downstream passes it back opaquely.
	Load func(ctx context.Context) (any, error)

	Fields []Field
	Submit *Action

	// Bind applies submitted values to a fresh model. It applies everything it
	// can and returns every failure, rather than stopping at the first, so one
	// round trip shows the user all of them.
	Bind func(ctx context.Context, values map[string]string) (model any, errs []FieldError)
}

// Kind reports the node kind.
func (f *Form) Kind() NodeKind { return NodeForm }

// Addr reports the node's position in the body tree.
func (f *Form) Addr() Addr { return f.At }

// Table is a leaf listing rows.
type Table struct {
	At Addr

	// Load returns the rows and the total matching count. Total is -1 when the
	// loader does not know it, which disables "of N" and last-page detection.
	Load func(ctx context.Context) (rows []any, total int, err error)

	Columns  []Field
	RowClick RowTarget // nil means rows are not clickable
	Actions  []*Action
	Bulk     []*Action
}

// Kind reports the node kind.
func (t *Table) Kind() NodeKind { return NodeTable }

// Addr reports the node's position in the body tree.
func (t *Table) Addr() Addr { return t.At }

// Field is one accessor, lowered. The same Field can render as a table cell
// or a form input. Decorators from the declaration are flattened into it.
type Field struct {
	Label string
	Kind  ValueKind
	Group []Field // non-empty means a group; Get and Set are then nil

	// Get formats the value for display. Set parses and assigns, and is nil
	// for a read-only accessor, which is what a missing Store lowers to.
	Get func(model any) string
	Set func(model any, raw string) error

	Rules Rules

	SortKey     string // from Sortable; empty means not sortable
	Placeholder string // from Placeholder
}

// Rules is one struct for every value kind. pkg/webui guarantees only the
// applicable parts are set, so downstream renders unconditionally.
type Rules struct {
	Required bool

	MinLen int // strings; 0 means no minimum
	MaxLen int // strings; 0 means no maximum

	Min *Bound // numbers; nil means unbounded
	Max *Bound

	Pattern *Pattern
}

// Bound is a numeric limit with the message shown when it fails.
type Bound struct {
	N       float64
	Message string
}

// Pattern is an RE2 expression with the message shown when it fails. RE2 is
// the intersection of Go's regexp and JavaScript's, so the same expression
// runs on both sides. pkg/webui compiled it while lowering; it is valid here.
type Pattern struct {
	Expr    string
	Message string
}

// Action is a mutation. Guard receives the subject: one model, or the
// selection for a bulk action. It gates the control at render time and
// authorises the request before Run.
type Action struct {
	Label string
	Guard func(ctx context.Context, subject any) error
	Run   func(ctx context.Context, subject any) (Effect, error)
	Bulk  bool
}

// RowTarget is what activating a row does.
type RowTarget interface {
	isRowTarget()
}

// Link navigates. The destination is named, not built: only the runtime knows
// the mount prefix.
type Link struct {
	Dest  string // PathTemplate of the target page
	Args  func(ctx context.Context, row any) map[string]string
	Guard func(ctx context.Context, row any) error
}

func (*Link) isRowTarget() {}

// ActionTarget runs an action instead of navigating.
type ActionTarget struct {
	Action *Action
}

func (*ActionTarget) isRowTarget() {}

// Effect is the outcome of an understood action. Fields with a nil error is a
// rejection the user can fix; an error is something they cannot fix by editing
// the form.
//
// Effect is a request-time value rather than part of the declaration, so
// Redirect is an already-resolved URL: pkg/webui built it with Open, which had
// ctx and therefore the mount prefix. The no-transport rule governs what gets
// lowered, not values flowing back through the runtime.
type Effect struct {
	Toast    string
	Redirect string // empty means no redirect
	Fields   []FieldError

	// Stale names models whose leaves should reload. Empty reloads the page.
	Stale []string
}

// FieldError attaches a message to a Field by label, unique within a form.
type FieldError struct {
	Label   string
	Message string
}

// ASSERT: nodes implement Node; row targets implement RowTarget
var (
	_ Node      = (*Stack)(nil)
	_ Node      = (*Split)(nil)
	_ Node      = (*Tabs)(nil)
	_ Node      = (*Form)(nil)
	_ Node      = (*Table)(nil)
	_ RowTarget = (*Link)(nil)
	_ RowTarget = (*ActionTarget)(nil)
)
