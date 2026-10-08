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

	// Search answers the global search with this page's own results, or is nil
	// when the page offers none. It runs without arguments, which is why a page
	// that offers it has no path arguments.
	Search func(ctx context.Context, query string) ([]SearchResult, error)

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
	KindInt64
)

// SearchResult is one hit in the global search. Href is resolved, prefix
// included, by the time it gets here.
type SearchResult struct {
	Title string
	Desc  string
	Href  string
}
