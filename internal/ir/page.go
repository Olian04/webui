package ir

import (
	"context"
)

// Page is one mounted page with its arguments resolved.
type Page struct {
	// PathTemplate is "/device/{id}". A template, not a route.
	PathTemplate string
	Args         []ArgSpec
	Nav          Nav
	Body         Node

	// Decode builds the argument struct from raw request values, path and
	// query merged by name. Built while lowering, so no reflection happens per
	// request. A zero value means the argument was absent.
	Decode func(raw map[string]string) (any, error)

	// Encode is the inverse: the struct back to path values and query values,
	// zero fields omitted. Open uses it to build hrefs.
	Encode func(args any) (path map[string]string, query map[string]string, err error)

	// Guard receives the decoded arguments. Runs before anything loads, on
	// every request to this page including section fetches.
	Guard func(ctx context.Context, args any) error

	// NavEntries lists the entries this page has in the navigation, each an address of the
	// page with arguments. Nil means the page has one entry, from Nav, or none. The runtime
	// calls it when it draws a page, so it is bounded and must not be trusted to be quick.
	NavEntries func(ctx context.Context) ([]NavEntry, error)

	// Search finds the things this page is the page of, by what the visitor typed in the
	// global search: at most limit of them. Nil means the page is not searched. A hit is checked
	// against this page's Guard, with the hit's own arguments, before it is offered.
	Search func(ctx context.Context, query string, limit int) ([]Hit, error)
}

// Hit is one result of a page's search: the arguments of this page that it leads to, and what
// to say about it.
type Hit struct {
	Args  any
	Title string
	Desc  string
}

// ArgSpec is one field of a page's argument struct, resolved.
type ArgSpec struct {
	Name   string // URL name, after any webui tag
	Field  string // Go field name, for diagnostics
	Kind   ValueKind
	InPath bool // path segment, else query parameter

	// List says a query parameter holds any number of values, the parameter repeated once for each ("?tag=a&tag=b"),
	// and Kind is that of one of them. The rest of a path, "{name...}", is the other list, and is KindStrings.
	List bool
}

// ValueKind is the set of argument and accessor value types.
type ValueKind uint8

// Supported value kinds. These are scalar, since a path segment is one value, except KindStrings, which only the
// rest of a path, "{name...}", takes: one element for each of its segments. A query parameter may repeat, which is
// [ArgSpec.List], and its Kind is that of one element.
const (
	KindString ValueKind = iota
	KindBool
	KindInt
	KindFloat
	KindInt64
	KindStrings
)
