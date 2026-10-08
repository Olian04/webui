package webui

import (
	"fmt"
	"strings"
)

// CompileError is one problem in a declaration. Go cannot recover the file and line
// of a struct literal at run time, so a problem is located by the page's path and
// its argument type, and both always appear in Error.
type CompileError struct {
	// Page is the path of the page the problem is on, or empty for a problem with
	// the app itself.
	Page string

	// Args is the name of the page's argument type.
	Args string

	// Detail says what is wrong.
	Detail string

	// Fix says what to do about it.
	Fix string
}

// where is the coordinates of the problem: the page's path and argument type.
func (e CompileError) where() string {
	var b strings.Builder
	if e.Page == "" {
		b.WriteString("app")
	} else {
		fmt.Fprintf(&b, "page %q", e.Page)
	}
	if e.Args != "" {
		fmt.Fprintf(&b, " (%s)", e.Args)
	}
	return b.String()
}

// Error is the problem and where it is, then its fix on a line of its own.
func (e CompileError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", e.where(), e.Detail)
	if e.Fix != "" {
		fmt.Fprintf(&b, "\n  Fix: %s", e.Fix)
	}
	return b.String()
}

// CompileErrors is every problem found in one Compile. Error joins them.
type CompileErrors []CompileError

// Error is every problem, one after another.
func (e CompileErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "\n")
}
