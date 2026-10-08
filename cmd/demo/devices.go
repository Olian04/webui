package main

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// The device id is a path argument, so each device has its own address. The query
// arguments have no control on the page; they arrive in the address:
//
//   - Minutes narrows what Events shows (the alerts link here with ?minutes=15)
//
// This page is reached from the device list, from a site and from the alerts. It
// does not say where Cancel goes: the library remembers which page's link the user
// followed, sort and filters included, and Cancel and a saved device return there.
type DeviceArgs struct {
	ID      string `webui:"id"`
	Minutes int
}

// Accessors are written once and used as a column in the list and as a field in
// the form. Without a Store an accessor is read-only.
var (
	// The Label is what a Load receives in Query.Sort and Query.Filters, and what
	// names the column in the address: ?devices.sort=ip, ?devices.filter.status=...
	DeviceID = webui.String[Device]{
		Label: "ID",
		Load:  func(d Device) string { return d.ID },
	}
	IP = webui.String[Device]{
		Label: "IP",
		Load:  func(d Device) string { return d.IP },
		Store: func(d *Device, v string) { d.IP = v },
		Rules: webui.StringRules{
			Required: true, MinLen: 7, MaxLen: 15,
			Pattern: &webui.PatternRule{Expr: `\d{1,3}(\.\d{1,3}){3}`, Message: "must be a valid IPv4 address"},
		},
	}
	Status = webui.Badge[Device]{
		Label: "Status",
		Load:  func(d Device) string { return d.Status },
		Kinds: map[string]webui.Tone{
			"healthy": webui.ToneOK, "degraded": webui.ToneWarning, "quiet": webui.ToneNeutral,
		},
	}
	Site = webui.String[Device]{
		Label: "Site",
		Load:  func(d Device) string { return d.Site },
	}
	Occurrences = webui.Int[Device]{
		Label: "Occurrences",
		Load:  func(d Device) int { return d.Count },
	}
	Rate = webui.Slider[Device]{
		Label: "Rate / s", Max: 15, Precision: 2,
		Load: func(d Device) float64 { return d.Rate() },
	}
)

// The list has no arguments of its own: its paging, sorting and column filters
// are kept in the address by the table, under its ID.
// A page that another page links back to has its path declared as a constant. The
// list links to a device by its variable (Details), and a saved device returns to
// the list by DevicesPath; naming the variable Devices from the save action would
// be an initialization cycle, because Devices already reaches that action through
// Details. A constant is not a variable, so it is not.
const DevicesPath webui.PageID[webui.NoArgs] = "/device"

var Devices = webui.Page[webui.NoArgs]{
	Path: DevicesPath,
	Nav:  webui.Nav{Label: "Devices", Icon: "display", Section: "Platform"},
	Body: webui.Table[Device]{
		Title: "Devices",
		Desc:  "One Table leaf. Sort, page and row links are all URLs.",
		// The table's state is ?devices.offset, ?devices.sort, ?devices.desc, a
		// ?devices.filter.<column> per filtered column, and ?devices.min.<column>
		// and ?devices.max.<column> for the numeric ones.
		ID:       "devices",
		PageSize: 10,
		// Search makes every device findable from the search box in the top bar. This
		// table has Load, so it is handed the typed text in Query.Search; a table with
		// Rows needs nothing else, and the library looks in every column.
		Search: true,
		// Load, not Rows: the source pages itself, as a database would, so it is
		// handed the window, the sort and the filters and does that work. Every
		// other table in the demo says only what its rows are, with Rows, and
		// leaves the rest to the library.
		Load: func(_ context.Context, q webui.Query) (webui.Rows[Device], error) {
			if q.Search != "" {
				found := service.Find(q.Search, q.Limit)
				return webui.Rows[Device]{Items: found, Total: len(found)}, nil
			}
			// A column is named by its Label. The numeric columns, Occurrences and
			// Rate / s, arrive as bounds.
			devices, total := service.Devices(q.Filters, boundsOf(q.Ranges), Order{Offset: q.Offset, Limit: q.Limit, Sort: q.Sort, Desc: q.Desc})
			return webui.Rows[Device]{Items: devices, Total: total}, nil
		},
		RowClick: webui.Link[Device, DeviceArgs]{
			Page: Details,
			Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
		},
		Columns: []webui.Accessor[Device]{DeviceID, IP, Status, Site, Occurrences, Rate},
	},
}

var Details = webui.Page[DeviceArgs]{
	Path: "/device/{id}",
	// No Nav, so no sidebar entry of its own: it lights Devices, the page at "/device".
	// Guard runs before anything is loaded, so an unknown device is never read.
	Guard: func(_ context.Context, a DeviceArgs) error {
		if _, ok := service.Device(a.ID); !ok {
			return fmt.Errorf("there is no device %q", a.ID)
		}
		return nil
	},
	// The selected tab is ?tabs.tab=raw-events, named by its label; the first tab,
	// Overview, needs no parameter.
	Body: webui.Tabs{Panels: []webui.Tab{
		{Label: "Overview", Body: webui.Split{DeviceForm, Events}},
		{Label: "Raw events", Body: Events},
	}},
}

var DeviceForm = webui.Form[Device]{
	Title: "Configuration",
	Desc:  "A Form leaf. Fields whose accessor declares no Store render read-only.",
	Load: func(ctx context.Context) (Device, error) {
		args := webui.ArgsOf[DeviceArgs](ctx)
		d, _ := service.Device(args.ID)
		return d, nil
	},
	Fields: []webui.Accessor[Device]{
		// A Group puts its fields side by side; it is layout, with no frame.
		webui.Group[Device]{DeviceID, webui.Placeholder[Device]{Accessor: IP, Text: "10.0.0.1"}},
		webui.Group[Device]{Status, Site}, // a badge and a plain field: both read-only here
		Rate,                              // a slider with no Store is a bar
	},
	Submit: SaveDevice,
}

var SaveDevice = webui.Action[Device]{
	Guard: canEdit[Device],
	Run: func(_ context.Context, d Device) (webui.Outcome, error) {
		// Uniqueness needs the service, so no rule can catch it: it comes back as a
		// Reject, which shows the form again with what was typed and this message
		// beside the field. Success says the device was saved.
		if service.IPTaken(d.IP, d.ID) {
			return webui.Reject(webui.Field[Device](IP, "already in use by another device")), nil
		}
		service.SetIP(d.ID, d.IP)
		// No redirect: a saved form returns to the page it was opened from, and
		// stays where it is when it was opened directly.
		return webui.Success("Device saved"), nil
	},
}

// Events is a table of a different model, beside the form. Split constrains
// nothing about what its children are about. The same table is also the Raw
// tab: it is one var used twice, and its sort and filters are shared because the
// panels of one Tabs may share an ID.
var Events = webui.Table[Event]{
	Title: "Recent events",
	Desc:  "Opened with ?minutes=15 (as the alerts do), only the last 15 minutes are shown.",
	Rows: func(ctx context.Context) ([]Event, error) {
		return service.Events(webui.ArgsOf[DeviceArgs](ctx).Minutes), nil
	},
	Columns: []webui.Accessor[Event]{EventTime, EventKind, EventDetail},
}

var (
	EventTime   = webui.String[Event]{Label: "Time", Load: func(e Event) string { return e.At }}
	EventKind   = webui.String[Event]{Label: "Kind", Load: func(e Event) string { return e.Kind }}
	EventDetail = webui.String[Event]{Label: "Detail", Load: func(e Event) string { return e.Detail }}
)

// boundsOf is a table's numeric filters as the service takes them.
func boundsOf(ranges map[string]webui.Range) map[string]Bounds {
	bounds := make(map[string]Bounds, len(ranges))
	for key, r := range ranges {
		bounds[key] = Bounds(r)
	}
	return bounds
}
