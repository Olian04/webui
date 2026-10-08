package main

import (
	"cmp"
	"context"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

var DevicesNav = webui.Nav{Label: "Devices", Section: "Platform"}

// The device id is a path argument, so each device has its own address.
type DeviceArgs struct {
	ID string `webui:"id"`
}

// Accessors are written once and used as a column in the list and as a field in
// the form. Without a Store an accessor is read-only.
var (
	// Key is what Load receives in Query.Sort when the list is sorted by this
	// column; without one it is the Label.
	DeviceID = webui.String[Device]{
		Label: "ID",
		Key:   "id",
		Load:  func(d Device) string { return d.ID },
	}
	IP = webui.String[Device]{
		Label: "IP",
		Key:   "ip",
		Load:  func(d Device) string { return d.IP },
		Store: func(d *Device, v string) { d.IP = v },
		Rules: webui.StringRules{
			Required: true, MinLen: 7, MaxLen: 15,
			Pattern: &webui.PatternRule{Expr: `\d{1,3}(\.\d{1,3}){3}`, Message: "must be a valid IPv4 address"},
		},
	}
	Status = webui.Badge[Device]{
		Label: "Status",
		Key:   "status",
		Load:  func(d Device) string { return d.Status },
		Kinds: map[string]webui.Tone{
			"healthy": webui.ToneOK, "degraded": webui.ToneWarning, "quiet": webui.ToneNeutral,
		},
	}
	Site = webui.String[Device]{
		Label: "Site",
		Key:   "site",
		Load:  func(d Device) string { return d.Site },
	}
	Occurrences = webui.Int[Device]{
		Label: "Occurrences",
		Key:   "count",
		Load:  func(d Device) int { return d.Count },
	}
	Rate = webui.Slider[Device]{
		Label: "Rate / s", Key: "rate", Max: 15, Precision: 2,
		Load: func(d Device) float64 { return d.Rate() },
	}
)

// The list has no arguments of its own: its paging, sorting and column filters
// are kept in the address by the table, under its ID.
var Devices = webui.Page[webui.NoArgs]{
	Path: "/device",
	Nav:  DevicesNav,
	// The global search asks every page that has a Search. Each result is a link
	// built with Open, so it carries the mount prefix and the page's arguments.
	Search: func(ctx context.Context, query string) ([]webui.SearchResult, error) {
		devices := service.Find(query, 8)
		results := make([]webui.SearchResult, len(devices))
		for i, d := range devices {
			results[i] = webui.SearchResult{
				Title:  d.ID,
				Desc:   d.IP + " · " + d.Site,
				Target: webui.Open(ctx, Details, DeviceArgs{ID: d.ID}),
			}
		}
		return results, nil
	},
	Body: webui.Table[Device]{
		Title: "Devices",
		Desc:  "One Table leaf. Sort, page and row links are all URLs.",
		// The table's state is ?devices.offset, ?devices.sort, ?devices.desc, a
		// ?devices.filter.<key> per filtered column, and ?devices.min.<key> and
		// ?devices.max.<key> for the numeric ones.
		ID:       "devices",
		PageSize: 10,
		Load: func(_ context.Context, q webui.Query) (webui.Rows[Device], error) {
			// The numeric columns, Occurrences and Rate, arrive as bounds.
			bounds := make(map[string]Bounds, len(q.Ranges))
			for key, r := range q.Ranges {
				bounds[key] = Bounds(r)
			}
			devices, total := service.Devices(q.Filters, bounds, Order{Offset: q.Offset, Limit: q.Limit, Sort: q.Sort, Desc: q.Desc})
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
	Load: func(_ context.Context, q webui.Query) (webui.Rows[Event], error) {
		// No Key on these accessors, so Query.Sort and Query.Filters use the Label.
		events := filteredBy(service.Events(), q.Filters, map[string]func(e Event, values []string) bool{
			EventTime.Label:   containing(func(e Event) string { return e.At }),
			EventKind.Label:   containing(func(e Event) string { return e.Kind }),
			EventDetail.Label: containing(func(e Event) string { return e.Detail }),
		})
		events = sortedBy(events, q.Sort, q.Desc, map[string]func(x, y Event) int{
			EventTime.Label:   func(x, y Event) int { return cmp.Compare(x.At, y.At) },
			EventKind.Label:   func(x, y Event) int { return cmp.Compare(x.Kind, y.Kind) },
			EventDetail.Label: func(x, y Event) int { return cmp.Compare(x.Detail, y.Detail) },
		})
		return webui.Rows[Event]{Items: events, Total: len(events)}, nil
	},
	Columns: []webui.Accessor[Event]{EventTime, EventKind, EventDetail},
}

var (
	EventTime   = webui.String[Event]{Label: "Time", Load: func(e Event) string { return e.At }}
	EventKind   = webui.String[Event]{Label: "Kind", Load: func(e Event) string { return e.Kind }}
	EventDetail = webui.String[Event]{Label: "Detail", Load: func(e Event) string { return e.Detail }}
)
