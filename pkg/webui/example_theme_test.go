package webui_test

import (
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// A Theme gives the four colours that carry meaning for both modes, and, in Light and Dark, what it gives one mode alone: a Canvas, from which
// that mode's surfaces, borders and text are derived, and a colour that wins over the shared one. The viewer still chooses between the modes, which
// are the design's own light and dark. Compile checks what a theme derives can be read, and says which field and which pair when it cannot.
func ExampleTheme() {
	// An accent for both modes.
	_, err := webui.App{Theme: webui.Theme{Accent: "#1f7a6d"}}.Compile("/admin")
	fmt.Println(err)

	// A canvas for the dark theme alone, with the design's own light theme, and an accent for the light theme that differs.
	mine := webui.Theme{
		Accent: "#4361e8",
		Dark:   webui.ModeColors{Canvas: "#0e1224"},
		Light:  webui.ModeColors{Accent: "#2f4fd0"},
	}
	_, err = webui.App{Theme: mine}.Compile("/admin")
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
	// true
	// true
}
