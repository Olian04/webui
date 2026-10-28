package webui_test

import (
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// A preset is a whole look in one mode: it is used as it is, or copied and changed. Compile checks what a theme derives
// can be read, and says which field and which pair when it cannot.
func ExampleTheme() {
	// Used as it is: dark, on a deep green background, with the theme button left out since the viewer has no choice to make.
	_, err := webui.App{Theme: webui.ThemeForest}.Compile("/admin")
	fmt.Println(err)

	// Copied, and its accent changed to one that can carry a white label.
	mine := webui.ThemeForest
	mine.Accent = "#1f7a6d"
	_, err = webui.App{Theme: mine}.Compile("/admin")
	fmt.Println(err)

	// Too light for the white label of a primary button: an error that names the field, the pair and what to change.
	mine.Accent = "#a7f3d0"
	_, err = webui.App{Theme: mine}.Compile("/admin")
	fmt.Println(err != nil)

	// A canvas is one colour, which cannot be right in both light and dark, so it needs the mode it is drawn in.
	_, err = webui.App{Theme: webui.Theme{Canvas: "#0e1224"}}.Compile("/admin")
	fmt.Println(err != nil)

	// Output:
	// <nil>
	// <nil>
	// true
	// true
}
