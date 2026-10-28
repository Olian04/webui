package theme

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func near(t *testing.T, what string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s: got %.4f, want %.4f", what, got, want)
	}
}

func TestParseReadsTheFormsAWebColourHas(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"#fff": "#ffffff", "#3d7": "#33dd77", "#3d71d9": "#3d71d9", "#3D71D9": "#3d71d9", "#3d71d9cc": "#3d71d9cc", "#f00a": "#ff0000aa",
	} {
		c, ok := Parse(in)
		if !ok || c.Hex() != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, c.Hex(), ok, want)
		}
	}
	for _, bad := range []string{"", "fff", "#ff", "#fffff", "#ggg", "red", "#3d71d9c", "rgb(0,0,0)"} {
		if _, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) was accepted", bad)
		}
	}
}

func TestMixIsWhatABrowserMakesOfColorMix(t *testing.T) {
	t.Parallel()

	// color-mix(in srgb, white 50%, black) is the middle of the two.
	mid := Mix(white, 50, black)
	near(t, "mid grey", mid.R, 0.5, 1e-9)
	// A mix with transparent is the colour, at a fraction of its alpha, and not a darker colour.
	c, _ := Parse("#cc2244")
	faint := Mix(c, 14, transparent)
	near(t, "alpha", faint.A, 0.14, 1e-9)
	near(t, "red is kept", faint.R, c.R, 1e-9)
	near(t, "blue is kept", faint.B, c.B, 1e-9)
	// Drawn on white it is the mix of the two by that alpha.
	on := faint.Over(white)
	near(t, "over white", on.R, c.R*0.14+0.86, 1e-9)
	// The hover of the design's own accent, as the stylesheet has it.
	if got := Mix(RGB(0x3d, 0x71, 0xd9), 88, black).Hex(); got != "#3663bf" {
		t.Errorf("88%% of the accent over black is %s", got)
	}
}

func TestContrastIsTheWCAGRatio(t *testing.T) {
	t.Parallel()

	near(t, "black on white", Contrast(black, white), 21, 1e-9)
	near(t, "white on white", Contrast(white, white), 1, 1e-9)
	// #767676 is the lightest grey that has 4.5:1 on white.
	g, _ := Parse("#767676")
	near(t, "grey on white", Contrast(g, white), 4.54, 0.01)
	// A translucent colour is laid over the background first.
	half := black.withAlpha(0.5)
	near(t, "half black on white", Contrast(half, white), Contrast(Color{0.5, 0.5, 0.5, 1}, white), 1e-9)
}

// The design's palettes are written twice, in the stylesheet and in Base, because the check that is made at Compile needs the numbers in Go.
// This holds the two together: a token that is changed in one and not in the other fails here, naming it.
func TestBaseIsTheStylesheetsPalette(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../render/assets/css/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	for mode, selector := range map[Mode]string{Dark: ":root,\n[data-theme='dark'] {", Light: "[data-theme='light'] {"} {
		start := strings.Index(css, "\n"+selector)
		if start < 0 {
			t.Fatalf("no block for %s", mode)
		}
		block := css[start : start+strings.Index(css[start:], "\n}")]
		tok := map[string]Color{}
		for _, m := range regexp.MustCompile(`--([a-z0-9-]+):\s*([^;]+);`).FindAllStringSubmatch(block, -1) {
			if c, ok := cssColour(m[2]); ok {
				tok[m[1]] = c
			}
		}
		p := Base(mode)
		for name, want := range map[string]Color{
			"canvas": p.Canvas, "panel": p.Panel, "raise": p.Raise, "input-bg": p.Input,
			"text": p.Text, "text-2": p.Text2, "text-3": p.Text3,
			"border": p.Border, "border-2": p.Border2, "control-bd-strong": p.ControlHeavy, "hover": p.Hover, "active": p.Active,
			"blue": p.Blue, "blue-hover": p.BlueHover, "blue-text": p.BlueText,
			"green": p.Green.Text, "green-bg": p.Green.BG, "green-bd": p.Green.BD,
			"orange": p.Orange.Text, "orange-bg": p.Orange.BG, "orange-bd": p.Orange.BD,
			"red": p.Red.Text, "red-bg": p.Red.BG, "red-bd": p.Red.BD, "red-solid": p.RedSolid, "red-solid-hover": p.RedSolidHover,
		} {
			got, ok := tok[name]
			if !ok {
				t.Errorf("%s: no --%s in the stylesheet", mode, name)
				continue
			}
			if got.Hex() != want.Hex() {
				t.Errorf("%s: --%s is %s in the stylesheet and %s in Base", mode, name, got.Hex(), want.Hex())
			}
		}
	}
}

// cssColour reads the forms the stylesheet writes a colour in: hex, rgb() and rgba().
func cssColour(v string) (Color, bool) {
	v = strings.TrimSpace(v)
	if c, ok := Parse(v); ok {
		return c, true
	}
	m := regexp.MustCompile(`^rgba?\(\s*(\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\s*\)$`).FindStringSubmatch(v)
	if m == nil {
		return Color{}, false
	}
	n := func(s string) int { i, _ := strconv.Atoi(s); return i }
	c := RGB(n(m[1]), n(m[2]), n(m[3]))
	if m[4] != "" {
		c.A, _ = strconv.ParseFloat(m[4], 64)
	}
	return c, true
}

func TestTheDesignsOwnPaletteIsLegibleSoNothingIsReportedForAnEmptySpec(t *testing.T) {
	t.Parallel()

	if got := Check(Spec{}); len(got) != 0 {
		t.Errorf("the design's own palette was reported: %v", got)
	}
}

func TestAnAccentThatIsTooLightForTheLabelOfAButtonIsReported(t *testing.T) {
	t.Parallel()

	got := Check(Spec{Accent: "#2f9e8f"})
	if len(got) == 0 {
		t.Fatal("a 3.3:1 accent was let through")
	}
	for _, p := range got {
		if p.Field != "Accent" {
			t.Errorf("blamed %s: %s", p.Field, p.Detail)
		}
	}
	if !strings.Contains(got[0].Detail, "white on it, the label of a primary button") || !strings.Contains(got[0].Detail, "3.3:1") {
		t.Errorf("the first problem does not name the pair: %s", got[0].Detail)
	}
	// A deeper one of the same hue is fine.
	if got := Check(Spec{Accent: "#1f7a6d"}); len(got) != 0 {
		t.Errorf("a legible accent was reported: %v", got)
	}
}

func TestAStatusColourIsCheckedInTheModeItIsDrawnIn(t *testing.T) {
	t.Parallel()

	// A yellow cannot be read on white: say in which theme it fails, and which field is to blame.
	got := Check(Spec{Warning: "#fff200"})
	if len(got) == 0 {
		t.Fatal("a yellow that cannot be read on white was let through")
	}
	for _, p := range got {
		if p.Field != "Warning" {
			t.Errorf("blamed %s", p.Field)
		}
		if !strings.Contains(p.Detail, "in the light theme") && !strings.Contains(p.Detail, "in the dark theme") {
			t.Errorf("a problem does not say which mode failed: %s", p.Detail)
		}
	}
}

func TestACanvasDerivesSurfacesAndTextThatAreLegible(t *testing.T) {
	t.Parallel()

	for name, s := range map[string]Spec{
		"navy":  {Dark: Colors{Canvas: "#0f1220"}},
		"black": {Dark: Colors{Canvas: "#000000"}},
		"green": {Dark: Colors{Canvas: "#0f1f1a"}},
		"sand":  {Light: Colors{Canvas: "#efe4d6"}},
		"white": {Light: Colors{Canvas: "#ffffff"}},
		"grey":  {Light: Colors{Canvas: "#f4f5f5"}},
		"both":  {Dark: Colors{Canvas: "#0f1220"}, Light: Colors{Canvas: "#eef0f9"}},
	} {
		if got := Check(s); len(got) != 0 {
			t.Errorf("%s: %v", name, got)
		}
	}
	// The design's own canvases are what the derivation gives the design's own surfaces, to within what the eye can tell.
	d := Resolve(Spec{Dark: Colors{Canvas: "#111217"}})[0].Palette
	for name, pair := range map[string][2]Color{"panel": {d.Panel, Base(Dark).Panel}, "raise": {d.Raise, Base(Dark).Raise}, "input": {d.Input, Base(Dark).Input}} {
		near(t, name+" (dark)", Contrast(pair[0], pair[1]), 1, 0.06)
	}
}

func TestACanvasOfTheWrongLightnessForItsModeIsReportedAgainstTheCanvasOfThatMode(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		spec  Spec
		field string
		fix   string
	}{
		"white in the dark theme": {Spec{Dark: Colors{Canvas: "#ffffff"}}, "Dark.Canvas", "darker colour for Theme.Dark.Canvas"},
		"black in the light one":  {Spec{Light: Colors{Canvas: "#000000"}}, "Light.Canvas", "lighter colour for Theme.Light.Canvas"},
		"a mid grey":              {Spec{Dark: Colors{Canvas: "#808080"}}, "Dark.Canvas", "darker colour for Theme.Dark.Canvas"},
	} {
		got := Check(tc.spec)
		if len(got) == 0 {
			t.Errorf("%s was let through", name)
			continue
		}
		if got[0].Field != tc.field || !strings.Contains(got[0].Fix, tc.fix) {
			t.Errorf("%s: %+v", name, got[0])
		}
	}
	if got := Check(Spec{Dark: Colors{Canvas: "#11121780"}}); len(got) == 0 || !strings.Contains(got[0].Detail, "Theme.Dark.Canvas") || !strings.Contains(got[0].Detail, "not opaque") {
		t.Errorf("a translucent canvas: %v", got)
	}
}

// A colour a mode gives wins over the one given for both, in that mode only.
func TestAModesOwnColourWinsOverTheSharedOneInThatModeOnly(t *testing.T) {
	t.Parallel()

	s := Spec{Accent: "#1f7a6d", Dark: Colors{Accent: "#4361e8"}}
	if got := s.For(Dark).Accent; got != "#4361e8" {
		t.Errorf("dark accent = %s", got)
	}
	if got := s.For(Light).Accent; got != "#1f7a6d" {
		t.Errorf("light accent = %s", got)
	}
	res := Resolve(s)
	if res[0].Mode != Dark || res[1].Mode != Light {
		t.Fatalf("modes are dark then light: %v %v", res[0].Mode, res[1].Mode)
	}
	if !strings.Contains(strings.Join(res[0].Decls, "\n"), "--blue: #4361e8;") || !strings.Contains(strings.Join(res[1].Decls, "\n"), "--blue: #1f7a6d;") {
		t.Errorf("the declarations do not follow: %v / %v", res[0].Decls, res[1].Decls)
	}
}

// A problem names the field that is responsible, as the declaration spells it: the mode's own where it gave the colour.
func TestAProblemNamesTheFieldThatGaveTheColour(t *testing.T) {
	t.Parallel()

	shared := Check(Spec{Accent: "#2f9e8f"})
	if len(shared) == 0 || shared[0].Field != "Accent" {
		t.Errorf("a shared accent is blamed as Accent: %v", shared)
	}
	own := Check(Spec{Accent: "#1f7a6d", Light: Colors{Accent: "#2f9e8f"}})
	if len(own) == 0 {
		t.Fatal("a light accent that is too pale was let through")
	}
	for _, p := range own {
		if p.Field != "Light.Accent" || !strings.Contains(p.Detail, "in the light theme") {
			t.Errorf("blamed %s: %s", p.Field, p.Detail)
		}
	}
}

func TestOnlyTheModesThatSayAnythingHaveADeclaration(t *testing.T) {
	t.Parallel()

	res := Resolve(Spec{Dark: Colors{Canvas: "#0f1220"}})
	if len(res[0].Decls) == 0 || len(res[1].Decls) != 0 {
		t.Errorf("the mode that has a canvas has a stylesheet and the other has none: %d / %d", len(res[0].Decls), len(res[1].Decls))
	}
	decls := strings.Join(res[0].Decls, "\n")
	for _, want := range []string{"--canvas: #0f1220;", "--panel: ", "--text: "} {
		if !strings.Contains(decls, want) {
			t.Errorf("no %q in\n%s", want, decls)
		}
	}
	if !(Spec{}).Zero() || (Spec{Light: Colors{Accent: "#1f7a6d"}}).Zero() {
		t.Error("Zero is the spec that says nothing")
	}
}
