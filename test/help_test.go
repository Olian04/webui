package webui_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

type helped struct {
	Name  string
	Count int
}

func helpedApp(fields ...webui.Accessor[helped]) http.Handler {
	return webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/h", Body: webui.Form[helped]{
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

	body := serve(helpedApp(helpedName("What the collector calls itself.", webui.StringRules{MaxLen: 8})), http.MethodGet, "/admin/h").Body.String()
	// One tip, one icon: the help under its own heading, then the rules under theirs.
	tip := `<span class="tip" tabindex="0" role="note" aria-label="Help: What the collector calls itself. Rules: Up to 8 characters"><i class="fa-solid fa-circle-info info" aria-hidden="true"></i>` +
		`<span class="tip-body" aria-hidden="true" id="p-f0-help"><span class="tip-title">Help:</span> <span class="tip-text">What the collector calls itself.</span> ` +
		`<span class="tip-title">Rules:</span> <span class="tip-item">Up to 8 characters</span></span></span>`
	assert.Contains(t, body, tip)
	assert.Equal(t, strings.Count(body, `class="tip"`), 1)
	assert.False(t, strings.Contains(body, "circle-question"))
}

// A field with a help and no rules has the popover for the help alone, and no heading for rules that are not there.
func TestAFieldWithAHelpAndNoRulesHasThePopoverForTheHelpAlone(t *testing.T) {
	t.Parallel()

	body := serve(helpedApp(helpedName("Just this.", webui.StringRules{})), http.MethodGet, "/admin/h").Body.String()
	assert.Contains(t, body, `aria-label="Help: Just this."`)
	assert.Contains(t, body, `<span class="tip-title">Help:</span> <span class="tip-text">Just this.</span> </span></span>`)
	assert.False(t, strings.Contains(body, "Rules:"))
}

func TestAFieldWithNoHelpHasNoTip(t *testing.T) {
	t.Parallel()

	body := serve(helpedApp(helpedName("", webui.StringRules{})), http.MethodGet, "/admin/h").Body.String()
	assert.False(t, strings.Contains(body, "circle-question"))
	assert.False(t, strings.Contains(body, `class="tip"`))
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
		webui.Datetime[helped]{Label: "K", Help: help, Load: str},
		webui.Timestamp[helped]{Label: "L", Help: help, Load: func(_ helped) int { return int(time.Unix(1_700_000_000, 0).Unix()) }},
		webui.OptionalInt[helped]{Label: "M", Help: help, Load: func(_ helped) *int { return nil }},
		webui.OptionalFloat[helped]{Label: "N", Help: help, Load: func(_ helped) *float64 { return nil }},
		webui.OptionalSlider[helped]{Label: "O", Help: help, Min: 0, Max: 5, Load: func(_ helped) *float64 { return nil }},
		webui.Bool[helped]{Label: "P", Help: help, Load: func(_ helped) *bool { return nil }},
	}
	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/h", Body: webui.Form[helped]{
		Load:   func(context.Context) (helped, webui.Loaded) { return helped{Name: "n", Count: 2}, webui.Ready() },
		Fields: fields,
	}}}}.MustCompile("/admin")
	body := serve(h, http.MethodGet, "/admin/h").Body.String()
	assert.Equal(t, strings.Count(body, `aria-label="Help: `+help+`"`), len(fields))
	assert.False(t, strings.Contains(body, `aria-label="Rules`))
}

func TestHelpIsInAGroupAndNotInATable(t *testing.T) {
	t.Parallel()

	body := serve(helpedApp(webui.Group[helped]{helpedName("In a group.", webui.StringRules{})}), http.MethodGet, "/admin/h").Body.String()
	assert.Contains(t, body, `aria-label="Help: In a group."`)

	table := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Table[helped]{
		Rows:    func(context.Context) ([]helped, webui.Loaded) { return []helped{{Name: "n"}}, webui.Ready() },
		Columns: []webui.Accessor[helped]{helpedName("Not in a table.", webui.StringRules{})},
	}}}}.MustCompile("/admin")
	assert.False(t, strings.Contains(serve(table, http.MethodGet, "/admin/t").Body.String(), "Not in a table."))
}

func TestHelpIsOnAToolbarValue(t *testing.T) {
	t.Parallel()

	h := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/t", Body: webui.Toolbar{Left: []webui.ToolbarItem{webui.Form[note]{
		Title: "Search", Load: func(context.Context) (note, webui.Loaded) { return note{}, webui.Ready() },
		Fields: []webui.Accessor[note]{webui.String[note]{Label: "Q", Help: "Search by name.", Load: func(note) string { return "" }}},
	}}}}}}.MustCompile("/admin")
	assert.Contains(t, serve(h, http.MethodGet, "/admin/t").Body.String(), `aria-label="Help: Search by name."`)
}
