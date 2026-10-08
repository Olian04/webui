package main

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

var DevicesNav = webui.Nav{Label: "Devices", Section: "Platform"}

// The list page's arguments are its filters. Paging and sorting are not here:
// the table keeps those in the address under its ID.
type DevicesArgs struct {
	Site, Status, Q string
}

// The device id is a path argument, so each device has its own address.
type DeviceArgs struct {
	ID string `webui:"id"`
}

// Accessors are written once and used as a column in the list and as a field in
// the form. Without a Store an accessor is read-only.
var (
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
		Tones: map[string]webui.Tone{"healthy": webui.ToneOK, "degraded": webui.ToneWarning},
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

var Devices = webui.Page[DevicesArgs]{
	Path: "/device",
	Nav:  DevicesNav,
	Body: webui.Table[Device]{
		Title: "Devices",
		Desc:  "One Table leaf. Sort, page and row links are all URLs.",
		// The table's state is ?devices.offset, ?devices.sort and ?devices.desc.
		ID:       "devices",
		PageSize: 10,
		Load: func(ctx context.Context, q webui.Query) (webui.Rows[Device], error) {
			args, err := webui.ArgsOf[DevicesArgs](ctx)
			if err != nil {
				return webui.Rows[Device]{}, err
			}
			devices, total := service.Devices(
				DeviceFilter(args), // the page's arguments are the filter
				Order{Offset: q.Offset, Limit: q.Limit, Sort: q.Sort, Desc: q.Desc},
			)
			return webui.Rows[Device]{Items: devices, Total: total}, nil
		},
		RowClick: webui.Link[Device, DeviceArgs]{
			Page: Details,
			Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
		},
		Columns: []webui.Accessor[Device]{
			webui.Sortable[Device]{Accessor: DeviceID, Key: "id"},
			webui.Sortable[Device]{Accessor: IP, Key: "ip"},
			webui.Sortable[Device]{Accessor: Status, Key: "status"},
			Site,
			webui.Sortable[Device]{Accessor: Occurrences, Key: "count"},
			webui.Sortable[Device]{Accessor: Rate, Key: "rate"},
		},
	},
}

var Details = webui.Page[DeviceArgs]{
	Path: "/device/{id}",
	// No Label, so no sidebar entry of its own: it lights Devices instead.
	Nav: webui.Nav{Shadow: &DevicesNav},
	// Guard runs before anything is loaded, so an unknown device is never read.
	Guard: func(_ context.Context, a DeviceArgs) error {
		if _, ok := service.Device(a.ID); !ok {
			return fmt.Errorf("there is no device %q", a.ID)
		}
		return nil
	},
	Body: webui.Tabs{Panels: []webui.Tab{
		{Label: "Overview", Body: webui.Split{DeviceForm, Events}},
		{Label: "Raw", Body: Events},
	}},
}

var DeviceForm = webui.Form[Device]{
	Title: "Configuration",
	Desc:  "A Form leaf. Fields whose accessor declares no Store render read-only.",
	Load: func(ctx context.Context) (Device, error) {
		args, err := webui.ArgsOf[DeviceArgs](ctx)
		if err != nil {
			return Device{}, err
		}
		d, _ := service.Device(args.ID)
		return d, nil
	},
	Fields: []webui.Accessor[Device]{
		webui.Group[Device]{DeviceID, IP},
		Site,
		Rate,
	},
	Submit: SaveDevice,
}

var SaveDevice = webui.Action[Device]{
	Guard: canEdit[Device],
	Run: func(_ context.Context, d Device) (webui.Effect, error) {
		// Uniqueness needs the service, so no rule can catch it: it comes back
		// as a rejection the user can fix, not as an error.
		if service.IPTaken(d.IP, d.ID) {
			return webui.Effect{Fields: webui.Fields[Device]{
				{Field: IP, Message: "already in use by another device"},
			}}, nil
		}
		service.SetIP(d.ID, d.IP)
		// Stays on the page. Returning to the list with Open(ctx, Devices, ...)
		// would make Devices -> Details -> DeviceForm -> SaveDevice -> Devices an
		// initialization cycle; see the README's Open section.
		return webui.Effect{Toast: "Device saved"}, nil
	},
}

// Events is a table of a different model, beside the form. Split constrains
// nothing about what its children are about.
var Events = webui.Table[Event]{
	Title: "Recent events",
	Load: func(context.Context, webui.Query) (webui.Rows[Event], error) {
		events := service.Events()
		return webui.Rows[Event]{Items: events, Total: len(events)}, nil
	},
	Columns: []webui.Accessor[Event]{
		webui.String[Event]{Label: "Time", Load: func(e Event) string { return e.At }},
		webui.String[Event]{Label: "Kind", Load: func(e Event) string { return e.Kind }},
		webui.String[Event]{Label: "Detail", Load: func(e Event) string { return e.Detail }},
	},
}
