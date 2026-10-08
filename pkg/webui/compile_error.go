package webui

import (
	"fmt"
	"strings"
)

// CompileError is one declaration problem. Page is the path, empty for a
// problem with the app itself; Args is the
// argument type name. Both always appear in Error: they are the only
// coordinates available at run time.
type CompileError struct {
	Page   string
	Args   string
	Detail string
	Fix    string
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
