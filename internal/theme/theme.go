package theme

import (
	"fmt"
	"strings"
)

// Mode is one of the two themes the design has: the viewer chooses between them.
type Mode string

const (
	Dark  Mode = "dark"
	Light Mode = "light"
)

// Name is the mode as a field of a Theme is named: "Dark" or "Light".
func (m Mode) Name() string {
	if m == Light {
		return "Light"
	}
	return "Dark"
}

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

// Colors are an app's colours for one mode, as written (hex strings). An empty field is the design's own.
type Colors struct {
	// Canvas is the page's background in the mode, from which the other surfaces, the borders and the text are derived. It is one colour,
	// which cannot be right in both modes, so there is one for each and none shared.
	Canvas string

	Accent, OK, Warning, Critical string
}

// Spec is the app's colours: those it gives for both modes, and in Light and Dark what it gives for one, which win over them in that mode.
type Spec struct {
	Accent, OK, Warning, Critical string
	Light, Dark                   Colors
}

// Zero is a Spec that says nothing: the design as it is, with no stylesheet of its own.
func (s Spec) Zero() bool { return s == Spec{} }

// block is what the app gave for one mode alone.
func (s Spec) block(m Mode) Colors {
	if m == Light {
		return s.Light
	}
	return s.Dark
}

// For is the colours an app has in a mode: its own for the mode where it gave one, else those it gave for both.
func (s Spec) For(m Mode) Colors {
	own := s.block(m)
	pick := func(own, shared string) string {
		if own != "" {
			return own
		}
		return shared
	}
	return Colors{
		Canvas: own.Canvas,
		Accent: pick(own.Accent, s.Accent), OK: pick(own.OK, s.OK), Warning: pick(own.Warning, s.Warning), Critical: pick(own.Critical, s.Critical),
	}
}

// written is the field of the Theme that gave a mode a colour, as it is named ("Dark.Accent", or "Accent" where it is shared), and the colour as
// the app wrote it, or "" if the design's own is in use.
func (s Spec) written(m Mode, field string) (name, value string) {
	own := s.block(m)
	switch field {
	case "Canvas":
		value = own.Canvas
	case "Accent":
		value = own.Accent
	case "OK":
		value = own.OK
	case "Warning":
		value = own.Warning
	case "Critical":
		value = own.Critical
	}
	if value != "" {
		return m.Name() + "." + field, value
	}
	switch field {
	case "Accent":
		value = s.Accent
	case "OK":
		value = s.OK
	case "Warning":
		value = s.Warning
	case "Critical":
		value = s.Critical
	}
	if value != "" {
		return field, value
	}
	return "", ""
}

// Result is what a Spec becomes in one mode: the palette that is drawn, and the declarations that make the stylesheet draw it.
// There are none where the app has said nothing for the mode.
type Result struct {
	Mode    Mode
	Palette Palette
	Decls   []string
}

// Resolve is what a Spec becomes in dark and in light, in that order.
func Resolve(s Spec) []Result {
	var out []Result
	for _, m := range []Mode{Dark, Light} {
		c := s.For(m)
		r := Result{Mode: m, Palette: Base(m)}
		r.neutrals(c.Canvas)
		r.accent(c.Accent)
		r.tone("green", c.OK)
		r.tone("orange", c.Warning)
		r.tone("red", c.Critical)
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

// Problem is a colour of the app's that is not legible where the design draws it. Field is the field of the Theme that is
// responsible, as it is named in the declaration: "Accent", "Dark.Canvas".
type Problem struct{ Field, Detail, Fix string }

// Check is what in the app's colours cannot be read: the text on the surfaces it is drawn on at 4.5:1, and what is only
// decoration at 3:1, in each mode. A failure names the field of the Theme that is responsible, and the pair that failed, so
// an app is not given a stylesheet whose button label or error text is out of sight.
func Check(s Spec) []Problem {
	var out []Problem
	for _, m := range []Mode{Dark, Light} {
		if c, ok := Parse(s.block(m).Canvas); ok && c.A < 1 {
			out = append(out, Problem{m.Name() + ".Canvas", fmt.Sprintf("Theme.%s.Canvas is %q, which is not opaque", m.Name(), s.block(m).Canvas), "Write it as #rgb or #rrggbb: a background that lets the page behind show through has no colour to derive the others from."})
		}
	}
	for _, r := range Resolve(s) {
		out = append(out, checkMode(s, r)...)
	}
	return out
}

func checkMode(s Spec, r Result) []Problem {
	p := r.Palette
	c := s.For(r.Mode)
	canvas := r.Mode.Name() + ".Canvas"
	var out []Problem
	where := " in the " + string(r.Mode) + " theme"
	direction := "darker"
	if r.Mode == Light {
		direction = "lighter"
	}
	// responsible is the field of the Theme that a failing pair is blamed on: the colour the app wrote if it wrote one, and otherwise the
	// canvas that the design's own colour is drawn on.
	responsible := func(field string) (string, string) {
		if name, value := s.written(r.Mode, field); name != "" {
			return name, fmt.Sprintf("Theme.%s is %q", name, value)
		}
		return canvas, fmt.Sprintf("Theme.%s is %q, and the design's own %s colour", canvas, c.Canvas, strings.ToLower(field))
	}
	need := func(field, what string, ratio, atLeast float64, fix string) {
		if ratio >= atLeast {
			return
		}
		name, subject := responsible(field)
		if name == canvas && c.Canvas == "" {
			return // the design's own pair, which its own tests hold
		}
		out = append(out, Problem{name, fmt.Sprintf("%s: %s is %.1f:1%s, and %.1f:1 is needed", subject, what, ratio, where, atLeast), fix})
	}
	hovered := p.Hover.Over(p.Panel)
	solid := func(c Color) Color { return c.Over(p.Panel) } // a fill that is not opaque is seen over the page

	canvasFix := "Choose a " + direction + " colour for Theme." + canvas + ": the surfaces, the borders and the text of the " + string(r.Mode) + " theme are derived from it."
	for _, surface := range []struct {
		label string
		bg    Color
	}{{"the panel", p.Panel}, {"the canvas", p.Canvas}} {
		need("Canvas", "its text on "+surface.label, Contrast(p.Text, surface.bg), 4.5, canvasFix)
		need("Canvas", "its quiet text on "+surface.label, Contrast(p.Text2, surface.bg), 4.5, canvasFix)
		need("Canvas", "its decorative text on "+surface.label, Contrast(p.Text3, surface.bg), 3, canvasFix)
	}
	need("Canvas", "its quiet text on a hovered row", Contrast(p.Text2, hovered), 4.5, canvasFix)
	need("Canvas", "its quiet text on the raised surface", Contrast(p.Text2, p.Raise), 4.5, canvasFix)
	need("Canvas", "its quiet text on an input", Contrast(p.Text2, p.Input), 4.5, canvasFix)
	need("Canvas", "the edge of a control for a visitor who asks for more contrast, on the panel", Contrast(p.ControlHeavy, p.Panel), 3, canvasFix)
	need("Canvas", "the same edge on an input", Contrast(p.ControlHeavy, p.Input), 3, canvasFix)

	if c.Accent != "" || c.Canvas != "" {
		accentFix := "Choose a colour that is neither very light nor very dark: white is drawn on it, and its lighter or darker tint is drawn as text on the page."
		need("Accent", "white on it, the label of a primary button", Contrast(white, solid(p.Blue)), 4.5, "Choose a darker colour: white is drawn on it.")
		need("Accent", "white on its hover", Contrast(white, solid(p.BlueHover)), 4.5, "Choose a darker colour: white is drawn on it.")
		need("Accent", "its text tint, for links, on the panel", Contrast(p.BlueText, p.Panel), 4.5, accentFix)
		need("Accent", "its text tint on a hovered row", Contrast(p.BlueText, hovered), 4.5, accentFix)
	}
	if c.Critical != "" || c.Canvas != "" {
		need("Critical", "white on it, the label of a destructive button", Contrast(white, solid(p.RedSolid)), 4.5, "Choose a darker colour: white is drawn on it.")
		need("Critical", "white on its hover", Contrast(white, solid(p.RedSolidHover)), 4.5, "Choose a darker colour: white is drawn on it.")
	}
	for _, t := range []struct {
		field, written string
		tone           Tone
	}{{"OK", c.OK, p.Green}, {"Warning", c.Warning, p.Orange}, {"Critical", c.Critical, p.Red}} {
		if t.written == "" && c.Canvas == "" {
			continue
		}
		fix := "Choose a colour that is neither very light nor very dark: its tint is drawn as text on the page and on its own badge."
		need(t.field, "its text tint on the panel", Contrast(t.tone.Text, p.Panel), 4.5, fix)
		badge := t.tone.BG.Over(p.Panel)
		need(t.field, "its text tint on its own badge", Contrast(t.tone.Text, badge), 4.5, fix)
	}
	return out
}
