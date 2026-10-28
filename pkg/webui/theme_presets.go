package webui

// The presets are whole looks: each pins a Mode, gives a Canvas that the surfaces, borders and text are derived from, and an
// Accent that is deep enough to carry a white label. They are values, so an app uses one as it is (App.Theme: webui.ThemeForest)
// or copies it and changes a field:
//
//	t := webui.ThemeForest
//	t.Accent = "#1f7a6d"
//
// Each is checked by Compile like any Theme, and a test holds every one of them to it, in the mode it is drawn in.
var (
	// ThemeMidnight is dark, on a navy background, with a blue accent.
	ThemeMidnight = Theme{Mode: ModeDark, Canvas: "#0e1224", Accent: "#4361e8"}

	// ThemeEmber is dark, on near black, with a burnt orange accent.
	ThemeEmber = Theme{Mode: ModeDark, Canvas: "#0a0a0a", Accent: "#c2410c"}

	// ThemeForest is dark, on a deep green background, with a green accent.
	ThemeForest = Theme{Mode: ModeDark, Canvas: "#0f1f1a", Accent: "#2a7f5f"}

	// ThemeViolet is dark, on a plum background, with a violet accent.
	ThemeViolet = Theme{Mode: ModeDark, Canvas: "#17121f", Accent: "#7c3aed"}

	// ThemePaper is light, on a warm white, with a blue accent.
	ThemePaper = Theme{Mode: ModeLight, Canvas: "#fafaf9", Accent: "#2563eb"}

	// ThemeSand is light, on a sand background, with a brown accent.
	ThemeSand = Theme{Mode: ModeLight, Canvas: "#efe4d6", Accent: "#9a3412"}
)
