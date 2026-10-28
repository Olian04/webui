package theme

import (
	"fmt"
	"strings"
)

// Mode is the theme an app pins itself to, or none: a viewer's own choice between light and dark.
type Mode string

const (
	Auto  Mode = ""
	Dark  Mode = "dark"
	Light Mode = "light"
)

// Tone is a status colour as the design uses it: the text tint, and the faint background and border of a badge.
type Tone struct{ Text, BG, BD Color }

// Palette is the tokens of one mode, as colours. Base is the design's own; an app's colours replace some of them.
type Palette struct {
	Canvas, Panel, Raise, Input   Color
	Text, Text2, Text3            Color
	Border, Border2, ControlHeavy Color
	Hover, Active                 Color
	Blue, BlueHover, BlueText     Color
	Green, Orange, Red            Tone
	RedSolid, RedSolidHover       Color
}

// Base is the design's own palette for a mode. The stylesheet (app.css) is where these are drawn from, and a test
// keeps the two equal.
func Base(m Mode) Palette {
	if m == Light {
		return Palette{
			Canvas: RGB(0xf4, 0xf5, 0xf5), Panel: RGB(255, 255, 255), Raise: RGB(255, 255, 255), Input: RGB(255, 255, 255),
			Text: RGB(0x24, 0x29, 0x2e), Text2: RGB(0x5c, 0x62, 0x69), Text3: RGB(0x85, 0x88, 0x8e),
			Border: RGBA(36, 41, 46, 0.12), Border2: RGBA(36, 41, 46, 0.24), ControlHeavy: RGBA(36, 41, 46, 0.55),
			Hover: RGBA(36, 41, 46, 0.06), Active: RGBA(36, 41, 46, 0.1),
			Blue: RGB(0x3d, 0x71, 0xd9), BlueHover: RGB(0x32, 0x65, 0xd4), BlueText: RGB(0x1f, 0x62, 0xe0),
			Green:    Tone{RGB(0x17, 0x72, 0x44), RGBA(23, 114, 68, 0.12), RGBA(23, 114, 68, 0.28)},
			Orange:   Tone{RGB(0xa7, 0x4a, 0x0b), RGBA(167, 74, 11, 0.12), RGBA(167, 74, 11, 0.28)},
			Red:      Tone{RGB(0xc2, 0x0d, 0x54), RGBA(194, 13, 84, 0.12), RGBA(194, 13, 84, 0.28)},
			RedSolid: RGB(0xc2, 0x0d, 0x54), RedSolidHover: RGB(0xa6, 0x0b, 0x49),
		}
	}
	return Palette{
		Canvas: RGB(0x11, 0x12, 0x17), Panel: RGB(0x18, 0x1b, 0x1f), Raise: RGB(0x22, 0x25, 0x2b), Input: RGB(0x0e, 0x0f, 0x12),
		Text: RGB(204, 204, 220), Text2: RGBA(204, 204, 220, 0.65), Text3: RGBA(204, 204, 220, 0.45),
		Border: RGBA(204, 204, 220, 0.11), Border2: RGBA(204, 204, 220, 0.22), ControlHeavy: RGBA(204, 204, 220, 0.5),
		Hover: RGBA(204, 204, 220, 0.07), Active: RGBA(204, 204, 220, 0.11),
		Blue: RGB(0x3d, 0x71, 0xd9), BlueHover: RGB(0x3a, 0x6b, 0xd3), BlueText: RGB(0x6e, 0x9f, 0xff),
		Green:    Tone{RGB(0x6c, 0xcf, 0x8e), RGBA(108, 207, 142, 0.14), RGBA(108, 207, 142, 0.3)},
		Orange:   Tone{RGB(0xff, 0x98, 0x30), RGBA(255, 152, 48, 0.14), RGBA(255, 152, 48, 0.3)},
		Red:      Tone{RGB(0xf5, 0x7a, 0x7d), RGBA(245, 122, 125, 0.14), RGBA(245, 122, 125, 0.3)},
		RedSolid: RGB(0xd0, 0x3a, 0x3f), RedSolidHover: RGB(0xbd, 0x32, 0x37),
	}
}

// Spec is the app's colours, as written (hex strings), and the mode it is pinned to. An empty field is the design's own.
type Spec struct {
	Accent, OK, Warning, Critical string
	// Canvas is the page's background in the pinned Mode, from which the other surfaces, the borders and the text are derived.
	// It is one colour, which cannot be right in both modes, so it needs a Mode.
	Canvas string
	Mode   Mode
}

// Zero is a Spec that says nothing: the design as it is, with no stylesheet of its own.
func (s Spec) Zero() bool { return s == Spec{} }

// Modes are the modes a Spec's colours are drawn in: the one it is pinned to, or both.
func (s Spec) Modes() []Mode {
	if s.Mode != Auto {
		return []Mode{s.Mode}
	}
	return []Mode{Dark, Light}
}

// Result is what a Spec becomes in one mode: the palette that is drawn, and the declarations that make the stylesheet draw it.
type Result struct {
	Mode    Mode
	Palette Palette
	Decls   []string
}

// Resolve is what a Spec becomes, in each mode it covers.
func Resolve(s Spec) []Result {
	var out []Result
	for _, m := range s.Modes() {
		r := Result{Mode: m, Palette: Base(m)}
		r.neutrals(s.Canvas)
		r.accent(s.Accent)
		r.tone("green", s.OK)
		r.tone("orange", s.Warning)
		r.tone("red", s.Critical)
		out = append(out, r)
	}
	return out
}

// declare adds a token.
func (r *Result) declare(name, value string) { r.Decls = append(r.Decls, "--"+name+": "+value+";") }

// neutrals derives the surfaces, the borders and the text from the page's background, in the way the design's own palettes
// relate: in the dark theme the panel and what is raised are lifted toward white, an input is sunk, and the text is the background
// pulled most of the way to white; in the light theme the panel is lifted and the text is the background pulled most of the way
// to black. The borders, the hover and the quiet text are the text colour at the same strengths as the design gives them.
func (r *Result) neutrals(canvas string) {
	c, ok := Parse(canvas)
	if !ok {
		return
	}
	p := &r.Palette
	p.Canvas = c
	var alphas [7]float64 // text-2, text-3, border, border-2, heavy, hover, active
	if r.Mode == Light {
		p.Panel = Mix(c, 30, white)
		p.Raise = p.Panel
		p.Input = Mix(c, 10, white)
		p.Text = Mix(c, 14, black)
		alphas = [7]float64{0.72, 0.55, 0.12, 0.24, 0.55, 0.06, 0.10}
	} else {
		p.Panel = Mix(c, 96, white)
		p.Raise = Mix(c, 91, white)
		p.Input = Mix(c, 85, black)
		p.Text = Mix(c, 20, white)
		alphas = [7]float64{0.65, 0.45, 0.11, 0.22, 0.5, 0.07, 0.11}
	}
	p.Text2, p.Text3 = p.Text.withAlpha(alphas[0]), p.Text.withAlpha(alphas[1])
	p.Border, p.Border2, p.ControlHeavy = p.Text.withAlpha(alphas[2]), p.Text.withAlpha(alphas[3]), p.Text.withAlpha(alphas[4])
	p.Hover, p.Active = p.Text.withAlpha(alphas[5]), p.Text.withAlpha(alphas[6])
	for _, t := range []struct {
		name string
		c    Color
	}{
		{"canvas", p.Canvas}, {"panel", p.Panel}, {"raise", p.Raise}, {"input-bg", p.Input},
		{"text", p.Text}, {"text-2", p.Text2}, {"text-3", p.Text3},
		{"border", p.Border}, {"border-2", p.Border2}, {"control-bd-strong", p.ControlHeavy},
		{"hover", p.Hover}, {"active", p.Active},
	} {
		r.declare(t.name, t.c.CSS())
	}
}

// mixed is a colour made from the app's by color-mix, which is written to the stylesheet as the app's text and
// worked out here from the same numbers.
type mixed struct {
	text string // the colour as the app wrote it
	c    Color
	pct  int
	with string
}

func (m mixed) css() string {
	return fmt.Sprintf("color-mix(in srgb, %s %d%%, %s)", m.text, m.pct, m.with)
}

func (m mixed) color() Color {
	switch m.with {
	case "white":
		return Mix(m.c, float64(m.pct), white)
	case "black":
		return Mix(m.c, float64(m.pct), black)
	}
	return Mix(m.c, float64(m.pct), transparent)
}

// accent derives the family of the app's accent. The hover is deeper in both modes: a lighter fill under a white label loses
// the contrast it needs. The text tint is lightened to read on a dark surface and deepened to read on a light one.
func (r *Result) accent(text string) {
	c, ok := Parse(text)
	if !ok {
		return
	}
	hover, tint := mixed{text, c, 88, "black"}, mixed{text, c, 62, "white"}
	if r.Mode == Light {
		hover, tint = mixed{text, c, 86, "black"}, mixed{text, c, 80, "black"}
	}
	r.Palette.Blue, r.Palette.BlueHover, r.Palette.BlueText = c, hover.color(), tint.color()
	r.declare("blue", text)
	r.declare("blue-hover", hover.css())
	r.declare("blue-text", tint.css())
}

// tone derives a status colour's text tint, and the faint background and border of its badge. The destructive button's fill
// is the red as given, which carries a white label; the tint is for text on the page.
func (r *Result) tone(name, text string) {
	c, ok := Parse(text)
	if !ok {
		return
	}
	fg := mixed{text, c, 85, "black"}
	if r.Mode == Dark {
		fg = mixed{text, c, 60, "white"}
	}
	bg, bd := mixed{text, c, 14, "transparent"}, mixed{text, c, 30, "transparent"}
	t := Tone{fg.color(), bg.color(), bd.color()}
	switch name {
	case "green":
		r.Palette.Green = t
	case "orange":
		r.Palette.Orange = t
	default:
		r.Palette.Red = t
	}
	r.declare(name, fg.css())
	r.declare(name+"-bg", bg.css())
	r.declare(name+"-bd", bd.css())
	if name == "red" {
		hover := mixed{text, c, 88, "black"}
		r.Palette.RedSolid, r.Palette.RedSolidHover = c, hover.color()
		r.declare("red-solid", text)
		r.declare("red-solid-hover", hover.css())
	}
}

// Problem is a colour of the app's that is not legible where the design draws it.
type Problem struct{ Field, Detail, Fix string }

// Check is what in the app's colours cannot be read: the text on the surfaces it is drawn on at 4.5:1, and what is only
// decoration at 3:1, in each mode the colours are drawn in. A failure names the field of the Theme that is responsible, and the
// pair that failed, so an app is not given a stylesheet whose button label or error text is out of sight.
func Check(s Spec) []Problem {
	var out []Problem
	if c, ok := Parse(s.Canvas); ok && c.A < 1 {
		out = append(out, Problem{"Canvas", fmt.Sprintf("Theme.Canvas is %q, which is not opaque", s.Canvas), "Write it as #rgb or #rrggbb: a background that lets the page behind show through has no colour to derive the others from."})
	}
	for _, r := range Resolve(s) {
		out = append(out, checkMode(s, r)...)
	}
	return out
}

func checkMode(s Spec, r Result) []Problem {
	p := r.Palette
	var out []Problem
	where := ""
	if s.Mode == Auto {
		where = " in the " + string(r.Mode) + " theme"
	}
	direction := "darker"
	if r.Mode == Light {
		direction = "lighter"
	}
	// responsible is the field of the Theme that a failing pair is blamed on: the colour the app wrote if it wrote one, and otherwise the
	// background that the design's own colour is drawn on.
	responsible := func(field, written string) (string, string) {
		if written != "" {
			return field, fmt.Sprintf("Theme.%s is %q", field, written)
		}
		return "Canvas", fmt.Sprintf("Theme.Canvas is %q, and the design's own %s colour", s.Canvas, strings.ToLower(field))
	}
	need := func(field, written, what string, ratio, atLeast float64, fix string) {
		if ratio >= atLeast {
			return
		}
		f, subject := responsible(field, written)
		if f == "Canvas" && s.Canvas == "" {
			return // the design's own pair, which its own tests hold
		}
		out = append(out, Problem{f, fmt.Sprintf("%s: %s is %.1f:1%s, and %.1f:1 is needed", subject, what, ratio, where, atLeast), fix})
	}
	hovered := p.Hover.Over(p.Panel)
	solid := func(c Color) Color { return c.Over(p.Panel) } // a fill that is not opaque is seen over the page

	canvasFix := "Choose a " + direction + " Canvas, or a different Mode: the text is derived from it."
	for _, surface := range []struct {
		label string
		bg    Color
	}{{"the panel", p.Panel}, {"the canvas", p.Canvas}} {
		need("Canvas", s.Canvas, "its text on "+surface.label, Contrast(p.Text, surface.bg), 4.5, canvasFix)
		need("Canvas", s.Canvas, "its quiet text on "+surface.label, Contrast(p.Text2, surface.bg), 4.5, canvasFix)
		need("Canvas", s.Canvas, "its decorative text on "+surface.label, Contrast(p.Text3, surface.bg), 3, canvasFix)
	}
	need("Canvas", s.Canvas, "its quiet text on a hovered row", Contrast(p.Text2, hovered), 4.5, canvasFix)
	need("Canvas", s.Canvas, "its quiet text on the raised surface", Contrast(p.Text2, p.Raise), 4.5, canvasFix)
	need("Canvas", s.Canvas, "its quiet text on an input", Contrast(p.Text2, p.Input), 4.5, canvasFix)
	need("Canvas", s.Canvas, "the edge of a control for a visitor who asks for more contrast, on the panel", Contrast(p.ControlHeavy, p.Panel), 3, canvasFix)
	need("Canvas", s.Canvas, "the same edge on an input", Contrast(p.ControlHeavy, p.Input), 3, canvasFix)

	if s.Accent != "" || s.Canvas != "" {
		accentFix := "Choose a colour that is neither very light nor very dark: white is drawn on it, and its lighter or darker tint is drawn as text on the page."
		need("Accent", s.Accent, "white on it, the label of a primary button", Contrast(white, solid(p.Blue)), 4.5, "Choose a darker colour: white is drawn on it.")
		need("Accent", s.Accent, "white on its hover", Contrast(white, solid(p.BlueHover)), 4.5, "Choose a darker colour: white is drawn on it.")
		need("Accent", s.Accent, "its text tint, for links, on the panel", Contrast(p.BlueText, p.Panel), 4.5, accentFix)
		need("Accent", s.Accent, "its text tint on a hovered row", Contrast(p.BlueText, hovered), 4.5, accentFix)
	}
	if s.Critical != "" || s.Canvas != "" {
		need("Critical", s.Critical, "white on it, the label of a destructive button", Contrast(white, solid(p.RedSolid)), 4.5, "Choose a darker colour: white is drawn on it.")
		need("Critical", s.Critical, "white on its hover", Contrast(white, solid(p.RedSolidHover)), 4.5, "Choose a darker colour: white is drawn on it.")
	}
	for _, t := range []struct {
		field, written string
		tone           Tone
	}{{"OK", s.OK, p.Green}, {"Warning", s.Warning, p.Orange}, {"Critical", s.Critical, p.Red}} {
		if t.written == "" && s.Canvas == "" {
			continue
		}
		fix := "Choose a colour that is neither very light nor very dark: its tint is drawn as text on the page and on its own badge."
		need(t.field, t.written, "its text tint on the panel", Contrast(t.tone.Text, p.Panel), 4.5, fix)
		badge := t.tone.BG.Over(p.Panel)
		need(t.field, t.written, "its text tint on its own badge", Contrast(t.tone.Text, badge), 4.5, fix)
	}
	return out
}
