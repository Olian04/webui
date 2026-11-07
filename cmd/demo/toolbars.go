package main

import (
	"context"

	"github.com/Olian04/webui/pkg/webui"
)

// Reading is the model of a toolbar's values that need nothing from the service: each value is what its accessor says.
type Reading struct{}

// readout is a value of a toolbar that is a label and a text, with a help when it is given one.
func readout(label, text, help string) webui.Accessor[Reading] {
	return webui.String[Reading]{Label: label, Help: help, Load: func(Reading) string { return text }}
}

// readouts is a group of values of a toolbar, which the toolbar loads together.
func readouts(title string, values ...webui.Accessor[Reading]) webui.Form[Reading] {
	return webui.Form[Reading]{
		Title:  title,
		Load:   func(context.Context) (Reading, webui.Loaded) { return Reading{}, webui.Ready() },
		Fields: values,
	}
}

// button is an action of a toolbar that does nothing but say so.
func button(label string, role webui.Role) webui.Action[webui.NoArgs] {
	return webui.Action[webui.NoArgs]{
		Label: label, Role: role,
		Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) { return webui.Success(label), nil },
	}
}

// A trend and a bar in a toolbar are what a toolbar draws that is not a text.
var (
	toolbarBar = webui.Slider[Reading]{Label: "Disk used (%)", Min: 0, Max: 100, Load: func(Reading) float64 { return 71 }}
	toolbarLat = webui.Trend[Throughput]{
		Label: "Latency", LowerIsBetter: true, Help: "The 95th percentile over the last day, and its change since yesterday.",
		Load: func(t Throughput) ([]float64, *float64) { return t.LatencyHistory, &t.LatencyChange },
	}
)

// fixture is a toolbar under the line that says what it is a fixture of, for the width to be changed under.
func fixture(says string, tb webui.Toolbar) webui.Stack {
	return webui.Stack{intro(says), tb}
}

// LayoutToolbars is the page to change the width of the window under, to see how a toolbar fills its rows. Each toolbar is one of the cases, with what it
// should do said above it: a group that is in a row whole or in the next one, one that flows from its own edge, and what is beside what.
var LayoutToolbars = webui.Page[webui.NoArgs]{
	Path: "/layout/toolbars",
	Nav:  webui.Nav{Label: "Toolbars", Icon: "grip", Section: "Layout"},
	View: webui.Stack{
		intro("Every toolbar below is one case of how a toolbar fills its rows. Change the width of the window, down to a phone's (640 px and under, where a trend is only its change, a bar only its number, and a description is its underlined label), and read each against what is said above it. The group with fewer values is whole: in a row or in the next one, never split. The group with more flows in rows from its own edge."),
		fixture("1. Many on the left, few on the right (the landing page). The values flow from the left. Sync and its button are one group at the right edge, and when they do not fit beside the last value, both go to the next row, at its right edge. They are never one in each row.", webui.Toolbar{
			Left: []webui.ToolbarItem{
				readouts("System", readout("Health", "healthy", ""), readout("Version", "2.4.1", ""), readout("Uptime", "9d 4h", ""), readout("Devices", "37", ""),
					readout("Queue depth", "128", "Events that are waiting to be written."), toolbarBar),
				webui.Form[Throughput]{Title: "Throughput", Load: func(context.Context) (Throughput, webui.Loaded) { return service.Throughput(), webui.Ready() }, Fields: []webui.Accessor[Throughput]{toolbarLat}},
			},
			Right: []webui.ToolbarItem{readouts("Sync", readout("Sync", "Idle", "")), button("Sync now", webui.RolePrimary)},
		}),
		fixture("2. Few on the left, many on the right: the same, swapped. The two controls at the left are whole. The values at the right flow in rows from the right edge, beside them when there is room for the widest of them, and in the next row when there is not.", webui.Toolbar{
			Left: []webui.ToolbarItem{readouts("Sync", readout("Sync", "Idle", "")), button("Sync now", webui.RolePrimary)},
			Right: []webui.ToolbarItem{
				readouts("System", readout("Health", "healthy", ""), readout("Version", "2.4.1", ""), readout("Uptime", "9d 4h", ""), readout("Devices", "37", ""),
					readout("Queue depth", "128", "Events that are waiting to be written."), toolbarBar),
				webui.Form[Throughput]{Title: "Throughput", Load: func(context.Context) (Throughput, webui.Loaded) { return service.Throughput(), webui.Ready() }, Fields: []webui.Accessor[Throughput]{toolbarLat}},
			},
		}),
		fixture("3. The same number on each side: the left flows, as in the first. Three values and a button at each edge.", webui.Toolbar{
			Left:  []webui.ToolbarItem{readouts("Left", readout("Alpha", "1", ""), readout("Beta", "2", ""), readout("Gamma", "3", "")), button("Left action", webui.RoleSecondary)},
			Right: []webui.ToolbarItem{readouts("Right", readout("Delta", "4", ""), readout("Epsilon", "5", ""), readout("Zeta", "6", "")), button("Right action", webui.RolePrimary)},
		}),
		fixture("4. Values on the left only: they flow from the left, with nothing at the right.", webui.Toolbar{
			Left: []webui.ToolbarItem{readouts("Left only", readout("Health", "healthy", ""), readout("Version", "2.4.1", ""), readout("Uptime", "9d 4h", ""), readout("Devices", "37", ""), readout("Queue depth", "128", ""), readout("Region", "eu-north-1", ""), readout("Owner", "platform", ""))},
		}),
		fixture("5. Values on the right only: they are at the right edge, in rows that are at the right edge.", webui.Toolbar{
			Right: []webui.ToolbarItem{readouts("Right only", readout("Health", "healthy", ""), readout("Version", "2.4.1", ""), readout("Uptime", "9d 4h", ""), readout("Devices", "37", ""), readout("Queue depth", "128", ""), readout("Region", "eu-north-1", ""), readout("Owner", "platform", ""))},
		}),
		fixture("6. One button at each edge: the two are one row, a button at each edge, and when they do not fit the right one is in the next row, at the right edge.", webui.Toolbar{
			Left:  []webui.ToolbarItem{button("A button at the left", webui.RoleSecondary)},
			Right: []webui.ToolbarItem{button("A button at the right", webui.RolePrimary)},
		}),
		fixture("7. A group wider than a row: the long values of the right flow among themselves, each row at the right edge, and are not cut off or out of the window. The button at the left is whole.", webui.Toolbar{
			Left: []webui.ToolbarItem{button("Go", webui.RolePrimary)},
			Right: []webui.ToolbarItem{readouts("Long", readout("Description", "A value that is long enough to be a row of its own on a narrow window", ""),
				readout("Owner", "the platform team, on call this week", ""), readout("Region", "eu-north-1, the northern region", ""))},
		}),
		fixture("8. A trend and a bar among the values, and values with a description: wide, a trend has its chart and its arrow, a bar its track, and a description an icon; on a phone, only the change, only the number, and the label underlined (hover or focus it).", webui.Toolbar{
			Left: []webui.ToolbarItem{
				readouts("Helped", readout("Queue depth", "128", "Events that are waiting to be written."), readout("Lag", "4 s", "How far behind the newest event the writer is."), toolbarBar),
				webui.Form[Throughput]{Title: "Throughput", Load: func(context.Context) (Throughput, webui.Loaded) { return service.Throughput(), webui.Ready() }, Fields: []webui.Accessor[Throughput]{toolbarLat}},
			},
			Right: []webui.ToolbarItem{button("Refresh", webui.RoleSecondary)},
		}),
	},
}
