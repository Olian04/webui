package webui_test

import (
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// A Theme gives the four colours that carry meaning for both modes, and, in Light and Dark, what it gives one mode alone: a Canvas, from which
// that mode's surfaces, borders and text are derived, and a colour that wins over the shared one. The viewer still chooses between the modes. A
// preset is a Theme that gives a Canvas for each. Compile checks what a theme derives can be read, and says which field and which pair when it cannot.
func ExampleTheme() {
	// A preset, as it is: a look in light and a look in dark.
	_, err := webui.App{Theme: webui.ThemeForest}.Compile("/admin")
	fmt.Println(err)

	// Copied, and its accent changed to one that can carry a white label: the accent is shared, so it is the accent of both modes.
	mine := webui.ThemeForest
	mine.Accent = "#1f7a6d"
	_, err = webui.App{Theme: mine}.Compile("/admin")
	fmt.Println(err)

	// Your own: a canvas for the dark theme alone, with the design's own light theme, and an accent for the light theme that differs.
	_, err = webui.App{Theme: webui.Theme{
		Accent: "#4361e8",
		Dark:   webui.ModeColors{Canvas: "#0e1224"},
		Light:  webui.ModeColors{Accent: "#2f4fd0"},
	}}.Compile("/admin")
	fmt.Println(err)

	// Too light for the white label of a primary button: an error that names the field, the pair and what to change.
	mine.Light.Accent = "#a7f3d0"
	_, err = webui.App{Theme: mine}.Compile("/admin")
	fmt.Println(err != nil)

	// A canvas of the wrong lightness for its mode: the dark theme's text is light, and cannot be read on a white page.
	_, err = webui.App{Theme: webui.Theme{Dark: webui.ModeColors{Canvas: "#ffffff"}}}.Compile("/admin")
	fmt.Println(err != nil)

	// Output:
	// <nil>
	// <nil>
	// <nil>
	// true
	// true
}
