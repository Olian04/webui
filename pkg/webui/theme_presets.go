package webui

// The presets are whole looks: each has an Accent that is deep enough to carry a white label and a Canvas for the dark theme and one
// for the light, from which the surfaces, borders and text of that theme are derived. The viewer still chooses between the two, so a preset
// is not a lock-in to either. They are values, so an app uses one as it is (App.Theme: webui.ThemeForest) or copies it and
// changes a field:
//
//	t := webui.ThemeForest
//	t.Accent = "#1f7a6d"
//
// The Accent is shared by both modes, so changing it changes both. Each preset is checked by Compile like any Theme, and a test holds every
// one of them to it, in both modes.
var (
	// ThemeMidnight is a navy, with a blue accent.
	ThemeMidnight = Theme{Accent: "#4361e8", Dark: ModeColors{Canvas: "#0e1224"}, Light: ModeColors{Canvas: "#eef0f9"}}

	// ThemeEmber is near black, or a warm white, with a burnt orange accent.
	ThemeEmber = Theme{Accent: "#c2410c", Dark: ModeColors{Canvas: "#0a0a0a"}, Light: ModeColors{Canvas: "#faf5f1"}}

	// ThemeForest is a deep green, or a pale one, with a green accent.
	ThemeForest = Theme{Accent: "#2a7f5f", Dark: ModeColors{Canvas: "#0f1f1a"}, Light: ModeColors{Canvas: "#eef5f1"}}

	// ThemeViolet is a plum, or a pale lilac, with a violet accent.
	ThemeViolet = Theme{Accent: "#7c3aed", Dark: ModeColors{Canvas: "#17121f"}, Light: ModeColors{Canvas: "#f6f2fb"}}

	// ThemePaper is a warm white, or a warm charcoal, with a blue accent.
	ThemePaper = Theme{Accent: "#2563eb", Dark: ModeColors{Canvas: "#1b1b1a"}, Light: ModeColors{Canvas: "#fafaf9"}}

	// ThemeSand is a sand, or a dark brown, with a rust accent.
	ThemeSand = Theme{Accent: "#a8400f", Dark: ModeColors{Canvas: "#1f1a14"}, Light: ModeColors{Canvas: "#efe4d6"}}
)
