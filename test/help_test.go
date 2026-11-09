package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/page"
)

type helped struct {
	Name  string
	Count int
}

func helpedApp(fields ...webui.Accessor[helped]) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/h", View: webui.Form[helped]{
		Load:   func(context.Context) (helped, webui.Loaded) { return helped{Name: "n", Count: 2}, webui.Ready() },
		Fields: fields,
		Submit: webui.Action[helped]{Run: func(context.Context, helped) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	}}}}.MustCompile("/admin")
}

var helpedName = func(help string, rules webui.StringRules) webui.String[helped] {
	return webui.String[helped]{
		Label: "Name", Help: help, Rules: rules,
		Load: func(h helped) string { return h.Name }, Store: func(h *helped, v string) { h.Name = v },
	}
}

func TestAFieldsHelpIsASectionOfThePopoverOfItsRulesAboveThem(t *testing.T) {
	t.Parallel()

	p := pageOf(t, helpedApp(helpedName("What the collector calls itself.", webui.StringRules{MaxLen: 8})), "/admin/h")
	// One tip, with the help under its own heading, then the rules under theirs: it is a note that is named by both, and its text is what the field is described by.
	tip := page.One(t, "the tip", p.ByRole("note", ""))
	assert.Equal(t, tip.Name(), "Help: What the collector calls itself. Rules: Up to 8 characters")
	assert.Equal(t, tip.ByAttr("aria-hidden", "true")[len(tip.ByAttr("aria-hidden", "true"))-1].AllText(), "Help: What the collector calls itself. Rules: Up to 8 characters")
	name := page.One(t, "the name", p.ByAttr("name", "f0"))
	assert.DeepEqual(t, describedBy(t, p, name), []string{"Help: What the collector calls itself. Rules: Up to 8 characters"})
}

// A field with a help and no rules has the popover for the help alone, and no heading for rules that are not there.
func TestAFieldWithAHelpAndNoRulesHasThePopoverForTheHelpAlone(t *testing.T) {
	t.Parallel()

	p := pageOf(t, helpedApp(helpedName("Just this.", webui.StringRules{})), "/admin/h")
	assert.Equal(t, page.One(t, "the tip", p.ByRole("note", "")).Name(), "Help: Just this.")
	assert.False(t, strings.Contains(p.Root().AllText(), "Rules:"))
}

func TestAFieldWithNoHelpHasNoTip(t *testing.T) {
	t.Parallel()

	assert.Equal(t, len(pageOf(t, helpedApp(helpedName("", webui.StringRules{})), "/admin/h").ByRole("note", "")), 0)
}

// A read-only field has no rules to say, and still says what it is for.
func TestAReadOnlyFieldShowsItsHelpAndEveryKindOfAccessorHasOne(t *testing.T) {
	t.Parallel()

	const help = "About this."
	str := func(h helped) string { return h.Name }
	fields := []webui.Accessor[helped]{
		webui.String[helped]{Label: "A", Help: help, Load: str},
		webui.Email[helped]{Label: "B", Help: help, Load: str},
		webui.Phone[helped]{Label: "C", Help: help, Load: str},
		webui.Website[helped]{Label: "D", Help: help, Load: str},
		webui.ColorPicker[helped]{Label: "E", Help: help, Load: str},
		webui.URL[helped]{Label: "F", Help: help, Load: str},
		webui.Int[helped]{Label: "G", Help: help, Load: func(h helped) int { return h.Count }},
		webui.Float[helped]{Label: "H", Help: help, Load: func(_ helped) float64 { return 1 }},
		webui.Slider[helped]{Label: "I", Help: help, Min: 0, Max: 5, Load: func(_ helped) float64 { return 1 }},
		webui.Select[helped]{Label: "J", Help: help, Load: func(_ helped) []string { return nil }},
		webui.Time[helped]{Label: "K", Help: help, Load: func(_ helped) time.Time { return time.Unix(1_700_000_000, 0) }},
		webui.OptionalTime[helped]{Label: "L", Help: help, Load: func(_ helped) *time.Time { return nil }},
		webui.OptionalInt[helped]{Label: "M", Help: help, Load: func(_ helped) *int { return nil }},
		webui.OptionalFloat[helped]{Label: "N", Help: help, Load: func(_ helped) *float64 { return nil }},
		webui.OptionalSlider[helped]{Label: "O", Help: help, Min: 0, Max: 5, Load: func(_ helped) *float64 { return nil }},
		webui.OptionalBool[helped]{Label: "P", Help: help, Load: func(_ helped) *bool { return nil }},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/h", View: webui.Form[helped]{
		Load:   func(context.Context) (helped, webui.Loaded) { return helped{Name: "n", Count: 2}, webui.Ready() },
		Fields: fields,
	}}}}.MustCompile("/admin")
	p := pageOf(t, h, "/admin/h")
	assert.Equal(t, len(p.ByRole("note", "Help: "+help)), len(fields))
	for _, tip := range p.ByRole("note", "") {
		assert.False(t, strings.Contains(tip.Name(), "Rules"))
	}
}

func TestHelpIsInAGroupAndNotInATable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, len(pageOf(t, helpedApp(webui.Group[helped]{helpedName("In a group.", webui.StringRules{})}), "/admin/h").ByRole("note", "Help: In a group.")), 1)

	table := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: webui.Table[helped]{
		Loader:  webui.RowsLoader[helped]{Load: func(context.Context) ([]helped, webui.Loaded) { return []helped{{Name: "n"}}, webui.Ready() }},
		Columns: []webui.Accessor[helped]{helpedName("Not in a table.", webui.StringRules{})},
	}}}}.MustCompile("/admin")
	assert.False(t, strings.Contains(pageOf(t, table, "/admin/t").Root().AllText(), "Not in a table."))
}

func TestHelpIsOnAToolbarValue(t *testing.T) {
	t.Parallel()

	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", View: webui.Toolbar{Left: []webui.ToolbarItem{webui.Form[note]{
		Title: "Search", Load: func(context.Context) (note, webui.Loaded) { return note{}, webui.Ready() },
		Fields: []webui.Accessor[note]{webui.String[note]{Label: "Q", Help: "Search by name.", Load: func(note) string { return "" }}},
	}}}}}}.MustCompile("/admin")
	assert.Equal(t, len(pageOf(t, h, "/admin/t").ByRole("note", "Help: Search by name.")), 1)
}
