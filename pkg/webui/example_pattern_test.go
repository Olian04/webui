package webui_test

import (
	"errors"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// A program that gets patterns from a schema leaves out the ones that are refused, and says why.
func ExampleValidatePattern() {
	for _, expr := range []string{`[a-z]+@example\.com`, `(?=.*\d)[a-z0-9]+`} {
		err := webui.ValidatePattern(expr)
		if pe, ok := errors.AsType[*webui.PatternError](err); ok {
			fmt.Println("left out:", pe.Problem)
			continue
		}
		fmt.Println("kept:", expr)
	}
	// Output:
	// kept: [a-z]+@example\.com
	// left out: uses a lookahead, `(?=`, which RE2 does not support
}
