package main

import (
	"context"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// extra is a field of a form with its own label, so that a form can be as tall as it is told to be.
func extra(label string) webui.String[Note] {
	return webui.String[Note]{Label: label, Load: func(n Note) string { return n.Text }, Store: func(n *Note, v string) { n.Text = v }}
}

// editable is a form that has a foot (its Save button), and as many fields as it is given.
func editable(title string, fields ...string) webui.Form[Note] {
	accessors := make([]webui.Accessor[Note], len(fields))
	for i, f := range fields {
		accessors[i] = extra(f)
	}
	return webui.Form[Note]{
		Title:  title,
		Load:   func(context.Context) (Note, error) { return Note{Text: "text"}, nil },
		Fields: accessors,
		Submit: webui.Action[Note]{Run: func(context.Context, Note) (webui.Outcome, error) { return webui.Success("Saved"), nil }},
	}
}

// The panels of a Split side by side are as tall as the tallest of them, and the rows of a Split are independent: five
// panels in two columns make three rows, each as tall as its tallest panel and not as the rows around it. A panel's
// foot (a pager, a Save button) stays at the bottom of its box.
var LayoutSplit = webui.Page[webui.NoArgs]{
	Path: "/layout/split",
	Nav:  webui.Nav{Label: "Split rows", Icon: "table-columns", Section: "Layout"},
	Body: webui.Stack{
		intro("Five panels in one Split make three rows of two columns. In each row the panels are as tall as the tallest one, with their borders lined up, and a pager or a Save button at the bottom of its box. The last row has one panel, which is as tall as it is and is not stretched to the rows above. The Split after it is its own: its rows do not follow these. Narrow the window below 1100 px and the columns become one, each panel its natural height."),
		webui.Split{
			editable("Row 1, left: a tall form", "Name", "Address", "Owner", "Region", "Notes"),
			backend("Row 1, right: two rows", 0, 2),
			backend("Row 2, left: a table with a pager", 0, 12),
			editable("Row 2, right: a short form", "Name"),
			backend("Row 3, alone: not stretched", 0, 3),
		},
		webui.Split{
			backend("Second Split, left (1 row)", 0, 1),
			backend("Second Split, right (6 rows)", 0, 6),
		},
		backend("A panel outside any Split, as tall as it is (20 ms)", 20*time.Millisecond, 2),
	},
}

// compactDevices is a short table of devices that needs no page to say which: a panel for a column or a rail, where a
// page's own table, with all of its columns, would not fit.
func compactDevices(title string, size int) webui.Table[Device] {
	return webui.Table[Device]{
		Title:    title,
		PageSize: size,
		Rows: func(context.Context) ([]Device, error) {
			devices, _ := service.Devices(nil, nil, Order{})
			return devices, nil
		},
		RowClick: webui.Link[Device, DeviceArgs]{
			Page: Details,
			Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
		},
		Columns: []webui.Accessor[Device]{DeviceID, Status, Site, Rate},
	}
}

// compactAlerts is the open alerts as three columns, five to a page and with no actions, for a panel beside others.
func compactAlerts(title string) webui.Table[Alert] {
	return webui.Table[Alert]{
		Title:    title,
		PageSize: 5,
		Rows:     func(context.Context) ([]Alert, error) { return service.OpenAlerts(), nil },
		RowClick: webui.Link[Alert, AlertArgs]{
			Page: AlertDetails,
			Args: func(_ context.Context, a Alert) AlertArgs { return AlertArgs{ID: a.ID} },
		},
		Columns: []webui.Accessor[Alert]{AlertID, Severity, AlertDevice},
	}
}

// A main panel with a rail: a long table beside a column of panels. The rail is a Stack in a Split, so it is one cell
// of the grid, as tall as the tallest of its row, and its panels keep their own height: only the panels that are direct
// children of a Split are stretched. This is the layout of a dashboard, a list with what is going on beside it.
var LayoutRail = webui.Page[webui.NoArgs]{
	Path: "/layout/rail",
	Nav:  webui.Nav{Label: "Main and rail", Icon: "table-cells-large", Section: "Layout"},
	Body: webui.Stack{
		intro("A table of devices, and beside it a rail of three panels: the collector's status, the open alerts and the sites. The rail is a Stack inside a Split, so it is one column of the grid. The table is as tall as the rail needs or its own rows do, whichever is more. The rail's table takes the height the table beside it has, so the two end together with their pagers at the bottom, and the status panel keeps its own height. Below 1100 px the rail goes under the table."),
		webui.Split{
			compactDevices("Devices", 14),
			webui.Stack{
				SystemStatus,
				compactAlerts("Open alerts"),
			},
		},
	},
}

// Columns inside columns, by putting a Split in a Split. The outer Split has two columns, and the first of them is a Split
// of its own, so the page has three: two narrow, and one as wide as both of them. The second block is the same with
// panels that say what they are, to see how heights are shared: the inner Split's panels are equal to each other, and
// the cell they are in is as tall as the other column.
var LayoutColumns = webui.Page[webui.NoArgs]{
	Path: "/layout/columns",
	Nav:  webui.Nav{Label: "Columns in columns", Icon: "grip", Section: "Layout"},
	Body: webui.Stack{
		intro("A Split whose first child is a Split: three columns, two narrow and one wide. In the first block they are a list, a status and a wide table. In the second block the panels say how tall they are: those in the inner Split are as tall as each other, and the cell they are in is as tall as the panel beside it. Narrow the window below 1100 px and every Split becomes one column."),
		webui.Split{
			webui.Split{
				compactAlerts("Alerts"),
				SystemStatus,
			},
			compactDevices("Devices", 8),
		},
		webui.Split{
			webui.Split{
				backend("Inner left: 2 rows", 0, 2),
				editable("Inner right: a form", "Name", "Owner", "Notes"),
			},
			backend("Outer right: 10 rows", 0, 10),
		},
		webui.Split{
			webui.Split{
				backend("Left pair, left", 0, 3),
				backend("Left pair, right", 0, 1),
			},
			webui.Split{
				backend("Right pair, left", 0, 1),
				backend("Right pair, right", 0, 5),
			},
		},
	},
}

// Tabs next to each other, and Tabs inside a Split. Each Tabs keeps its selection in the address under a name of its own
// (tabs.tab and tabs-2.tab), so the two are chosen separately, survive a reload and can be linked to. Only the selected
// tab of each is loaded.
var LayoutTabs = webui.Page[webui.NoArgs]{
	Path: "/layout/tabs",
	Nav:  webui.Nav{Label: "Tabs side by side", Icon: "window-restore", Section: "Layout"},
	Body: webui.Stack{
		intro("Two sets of Tabs in a Split, and a third under them. Each remembers its own tab in the address (tabs.tab, tabs-2.tab, tabs-3.tab), so you can read alerts on the left while the right shows the sites, reload, and find both as they were. Only the selected tab of each loads. The third has a Split in its first tab and a plain table in its second."),
		webui.Split{
			webui.Tabs{Panels: []webui.Tab{
				{Label: "Alerts", Body: compactAlerts("Open alerts")},
				{Label: "Devices", Body: compactDevices("Devices", 6)},
			}},
			webui.Tabs{Panels: []webui.Tab{
				{Label: "Sites", Body: SitesTable},
				{Label: "Status", Body: SystemStatus},
			}},
		},
		webui.Tabs{Panels: []webui.Tab{
			{Label: "Side by side", Body: webui.Split{compactAlerts("Alerts, again"), editable("A short form", "Name", "Owner")}},
			{Label: "Devices", Body: compactDevices("All devices", 8)},
		}},
	},
}

// Deep nesting: Tabs, a Stack, a Split, Tabs and a Split, in that order, with a panel at the bottom of each. Nothing
// limits how deep layouts go, and a page that does this is still one address, with the selection of every Tabs in it.
var LayoutDeep = webui.Page[webui.NoArgs]{
	Path: "/layout/deep",
	Nav:  webui.Nav{Label: "Nested five deep", Icon: "layer-group", Section: "Layout"},
	Body: webui.Tabs{Panels: []webui.Tab{
		{Label: "Fleet", Body: webui.Stack{
			intro("Tabs, then a Stack, then a Split, then Tabs, then a Split: the panels at the bottom are five layouts down. The outer Tabs choose between this and 'Health'; the inner Tabs, in the right column, choose between two pairs of panels. Both choices are in the address."),
			webui.Split{
				compactDevices("Devices", 8),
				webui.Tabs{Panels: []webui.Tab{
					{Label: "Alerts and sites", Body: webui.Split{compactAlerts("Alerts"), SitesTable}},
					{Label: "Status and a form", Body: webui.Split{SystemStatus, editable("A note", "Subject", "Body")}},
				}},
			},
		}},
		{Label: "Health", Body: webui.Stack{
			SystemStatus,
			webui.Split{compactAlerts("Open alerts"), SitesTable},
		}},
	}},
}

// A page whose panels are of every kind: a form, an editor, a diff and a markdown text, with the text in the tab that is
// not selected. An editor, a diff and Markdown need script and a stylesheet in the head of the page, so a page that has
// one anywhere, on a tab that is selected or not, is built whole, as 'Built whole (editor)' is.
var LayoutReview = webui.Page[webui.NoArgs]{
	Path: "/layout/review",
	Nav:  webui.Nav{Label: "Review a change", Icon: "code-compare", Section: "Layout"},
	Body: webui.Stack{
		intro("The collector's status above, and under it Tabs of the things you read before a deploy. The first tab puts the editor beside what has changed since the last deploy (a Diff), the second is the runbook, a Markdown text. The page has an editor, a diff and Markdown, so its head needs their script: it is built whole, and arrives as one piece."),
		SystemStatus,
		webui.Tabs{Panels: []webui.Tab{
			{Label: "Editor and changes", Body: webui.Split{ConfigEditor, ConfigDiff}},
			{Label: "Runbook", Body: handbookPanel("Runbook", "runbook.md", "A Markdown, in a tab.")},
		}},
	},
}

// A procedure beside the state of what it is about: a text on the left, and on the right a stack of live panels. A
// reader follows the steps and watches the alerts without leaving the page.
var LayoutRunbook = webui.Page[webui.NoArgs]{
	Path: "/layout/runbook",
	Nav:  webui.Nav{Label: "Runbook and state", Icon: "book-open", Section: "Layout"},
	Body: webui.Split{
		handbookPanel("Runbook", "incidents.md", "Markdown on the left, live panels on the right."),
		webui.Stack{
			SystemStatus,
			compactAlerts("Open alerts"),
			compactDevices("Devices", 5),
		},
	},
}

// tabsOf is a Tabs with a tab for each label, each showing a small table that says which tab it is.
func tabsOf(prefix string, labels ...string) webui.Tabs {
	tabs := make([]webui.Tab, len(labels))
	for i, label := range labels {
		tabs[i] = webui.Tab{Label: label, Body: backend(prefix+" / "+label, 0, i%3+1)}
	}
	return webui.Tabs{Panels: tabs}
}

// A Tabs has as many tabs as it is given. Three, which fit; six in each half of a Split, which do not, and the strips scroll
// sideways with their tabs at their own size; and sixteen with long labels, in a strip as wide as the page. A strip that was
// scrolled keeps where it was when a tab is followed, and one that is opened on a tab out of sight is scrolled to it.
var LayoutManyTabs = webui.Page[webui.NoArgs]{
	Path: "/layout/many-tabs",
	Nav:  webui.Nav{Label: "Many tabs", Icon: "folder-tree", Section: "Layout"},
	Body: webui.Stack{
		intro("Tabs with three tabs, which fit; two Tabs of six in a Split, each half the width of the page, which do not fit until the window is wide; and a Tabs of sixteen with long labels. When the tabs need more room than the strip has it scrolls sideways, with a thin bar, and the tabs keep their size. Choose a tab far to the right and follow it: the strip stays where it was scrolled and the tab you chose is brought into view. Open the address of a tab out of sight and the strip is scrolled to it."),
		webui.Tabs{Panels: []webui.Tab{
			{Label: "Devices", Body: compactDevices("Three tabs: devices", 5)},
			{Label: "Alerts", Body: compactAlerts("Three tabs: alerts")},
			{Label: "Sites", Body: SitesTable},
		}},
		webui.Split{
			tabsOf("Left", "Overview", "Devices", "Alerts", "Sites", "Configuration", "Audit log"),
			tabsOf("Right", "Metrics", "Retention", "Ingest", "Paging rules", "Handbook", "Runbook"),
		},
		tabsOf("Sixteen",
			"Overview", "Devices and their addresses", "Open alerts", "Sites", "Configuration", "Retention policy", "Ingest settings",
			"Paging rule", "Audit log", "Files", "Handbook", "Incidents", "Sinks", "Glossary", "Metrics", "System status"),
	},
}
