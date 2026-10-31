package main

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// The device id is a path argument, so each device has its own address. The query
// argument arrives in the address, from a link:
//
//   - Minutes narrows what the Raw events tab shows (the alerts link here with ?minutes=15)
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
		Label: "IP", Help: "The address the device reports from. Two devices cannot share one.",
		Load:     func(d Device) string { return d.IP },
		Store:    func(d *Device, v string) { d.IP = v },
		Required: true,
		Rules: webui.StringRules{
			MinLen: 7, MaxLen: 15,
			Pattern: &webui.PatternRule{Expr: `\d{1,3}(\.\d{1,3}){3}`, Message: "must be a valid IPv4 address"},
		},
		Placeholder: "10.0.0.1",
	}
	// A Select with a Store is edited: one of the options, from a drop-down list, and a badge when it is read.
	Status = webui.Select[Device]{
		Label: "Status",
		Load:  func(d Device) []string { return []string{d.Status} },
		Store: func(d *Device, v []string) {
			if len(v) == 1 {
				d.Status = v[0]
			}
		},
		Options: []webui.Option{
			{Value: "healthy", Label: "Healthy", Tone: webui.ToneOK}, {Value: "degraded", Label: "Degraded", Tone: webui.ToneWarning}, {Value: "quiet", Label: "Quiet"},
		},
		Required: true,
	}
	// Multi lets it hold any number: a list to tick several of, and a badge for each when it is read.
	Tags = webui.Select[Device]{
		Label:   "Tags",
		Load:    func(d Device) []string { return d.Tags },
		Store:   func(d *Device, v []string) { d.Tags = v },
		Multi:   true,
		NoSort:  true, // a list of words has no order to sort by
		Options: []webui.Option{{Value: "edge"}, {Value: "gpu"}, {Value: "legacy", Tone: webui.ToneWarning}, {Value: "staging"}},
	}
	// Multiline is a text area, for text of more than one line: its rules are Required, MinLen and MaxLen, and a line break
	// is one character whatever the browser sent.
	Notes = webui.String[Device]{
		Label:       "Notes",
		Load:        func(d Device) string { return d.Notes },
		Store:       func(d *Device, v string) { d.Notes = v },
		Multiline:   true,
		Rules:       webui.StringRules{MaxLen: 500},
		Placeholder: "Who looks after it, and what to know",
	}
	Site = webui.String[Device]{
		Label: "Site",
		Load:  func(d Device) string { return d.Site },
	}
	// LoadOptions reads the options of a Select when a request is answered, from where the program keeps them: here,
	// the sites that there are devices at, with how many, so a site that has none is not offered and the counts
	// change when a device is moved.
	SiteChoice = webui.Select[Device]{
		Label: "Site",
		Load:  func(d Device) []string { return []string{d.Site} },
		Store: func(d *Device, v []string) {
			if len(v) == 1 {
				d.Site = v[0]
			}
		},
		LoadOptions: func(context.Context) ([]webui.Option, error) {
			var options []webui.Option
			for _, s := range service.Sites() {
				noun := "devices"
				if s.Devices == 1 {
					noun = "device"
				}
				options = append(options, webui.Option{Value: s.Name, Label: fmt.Sprintf("%s (%d %s)", s.Name, s.Devices, noun)})
			}
			return options, nil
		},
	}
	Occurrences = webui.Int[Device]{
		Label: "Occurrences",
		Load:  func(d Device) int { return d.Count },
	}
	// Format changes how the value reads where it is only read, and nothing else: this column
	// reads "1.5 GiB" and sorts, filters (the bounds are bytes) and exports as the number of bytes.
	// The library ships no formats: humanBytes is this program's own (see format.go).
	Stored = webui.Int[Device]{
		Label:  "Stored",
		Load:   func(d Device) int { return d.Stored },
		Format: humanBytes,
	}
	// A Timestamp is a moment held as Unix seconds; it reads as a Datetime does.
	LastSeen = webui.Timestamp[Device]{
		Label: "Last seen",
		Load:  func(d Device) int { return d.LastSeen },
	}
	Rate = webui.Slider[Device]{
		Label: "Rate / s", Max: 15, StepSize: 0.01,
		Load: func(d Device) float64 { return d.Rate() },
	}
	// A Series is a small chart where it is read: the shape of the last hour, beside the figure. It has no axes, and its name has the points in
	// words. A column of it is neither sorted nor filtered by.
	Recent = webui.Series[Device]{
		Label: "Last hour", Unit: "/s", Precision: 1,
		Load: func(d Device) []float64 { return d.Recent() },
		Help: "The rate in each of the last twelve five-minute windows.",
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
	// A Toolbar is a row of controls for the page it is on, and not a panel: a Link to the page that makes a device and a button
	// that does something.
	Body: webui.Stack{DevicesToolbar, DevicesTable},
}

// DevicesToolbar is the controls of the list. A Link to another page is at the left edge, and an Action, a button that posts and says how it
// ended, as every action does, is at the right. Neither has a model of its own: they are of the page, which an Action reads with ArgsOf if it needs
// it, and here it needs none. When the page is too narrow for both at the edges they flow together from the left.
var DevicesToolbar = webui.Toolbar{
	Left: []webui.ToolbarItem{
		webui.Link[webui.NoArgs, webui.NoArgs]{
			Label: "Create device", Role: webui.RolePrimary, Page: NewDevicePage,
			Args: func(context.Context, webui.NoArgs) webui.NoArgs { return webui.NoArgs{} },
		},
	},
	Right: []webui.ToolbarItem{
		webui.Action[webui.NoArgs]{
			Label:   "Sync now",
			Confirm: "Sync every device now? It reads each one and may take a while.",
			Guard:   canEdit[webui.NoArgs],
			Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) {
				return webui.Success(fmt.Sprintf("Synced %d devices", service.Sync())), nil
			},
		},
	},
}

// DevicesTable is the list.
var DevicesTable = webui.Table[Device]{
	Title: "Devices",
	Desc:  "One Table leaf. Sort, page and row links are all URLs.",
	// The table's state is kept in the address under its title: ?devices.offset,
	// ?devices.sort, ?devices.desc, a ?devices.filter.<column> per filtered
	// column, and ?devices.min.<column> and ?devices.max.<column> for the
	// numeric ones.
	PageSize: 10,
	// Export adds CSV and JSON links to the footer. A Load table is asked for every row
	// the sort and filters keep, 500 at a time from Offset 0, so a download is not a page.
	Export: true,
	// Load, not Rows: the source pages itself, as a database would, so it is
	// handed the window, the sort and the filters and does that work. Every
	// other table in the demo says only what its rows are, with Rows, and
	// leaves the rest to the library.
	Load: func(_ context.Context, q webui.Query) (webui.Window[Device], webui.Loaded) {
		// A column is named by its Label. The numeric columns, Occurrences and
		// Rate / s, arrive as bounds.
		devices, total := service.Devices(q.Filters, boundsOf(q.Ranges), Order{Offset: q.Offset, Limit: q.Limit, Sort: q.Sort, Desc: q.Desc})
		return webui.Window[Device]{Items: devices, Total: total}, webui.Ready()
	},
	RowClick: webui.Link[Device, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
	},
	Columns: []webui.Accessor[Device]{DeviceID, IP, Status, Tags, Site, Occurrences, Stored, Rate, Recent},
}

// NewDevicePage is the form that makes a device: a Form of a model that is new, so what it loads is a zero Device. The
// list's Link leads here, and a Cancel or a save returns to the list. The page is for editors: a viewer is refused it, and the
// list does not offer them the Link.
var NewDevicePage = webui.Page[webui.NoArgs]{
	Path:  "/device/new",
	Guard: canEdit[webui.NoArgs],
	Body: webui.Form[Device]{
		Title: "New device",
		Desc:  "The same fields as a device's own form, over a device that does not exist yet.",
		Load:  func(context.Context) (Device, webui.Loaded) { return Device{Status: "healthy"}, webui.Ready() },
		Fields: []webui.Accessor[Device]{
			IP,
			webui.Group[Device]{Status, SiteChoice},
			Tags,
			Notes,
		},
		Submit: webui.Action[Device]{
			Label: "Create",
			Guard: canEdit[Device],
			Run: func(ctx context.Context, d Device) (webui.Outcome, error) {
				if service.IPTaken(d.IP, "") {
					return webui.Reject(webui.Field[Device](IP, "already in use by another device")), nil
				}
				created := service.AddDevice(d)
				return webui.Success("Device created").Then(webui.Open(ctx, Details, DeviceArgs{ID: created.ID})), nil
			},
		},
	},
}

var Details = webui.Page[DeviceArgs]{
	Path: "/device/{id}",
	// Search makes every device findable from the search box in the top bar. It is handed the
	// text, and the most to return, and the source does the looking, as a database would: each
	// result is the arguments of this page, and what to say about it.
	Search: func(_ context.Context, query string, limit int) ([]webui.Hit[DeviceArgs], error) {
		var hits []webui.Hit[DeviceArgs]
		for _, d := range service.Find(query, limit) {
			hits = append(hits, webui.Hit[DeviceArgs]{Args: DeviceArgs{ID: d.ID}, Title: d.ID, Desc: d.IP + " · " + d.Site})
		}
		return hits, nil
	},
	// Crumb is what the breadcrumb and the tab title say of this page: the device's address, which a person
	// knows it by, and not the opaque id in the path. It reads "10.0.0.12", where the path says "dev_27c38b".
	Crumb: func(_ context.Context, a DeviceArgs) string {
		if d, ok := service.Device(a.ID); ok {
			return d.IP
		}
		return "" // the path's own words
	},
	// No Nav, so no sidebar entry of its own: it lights Devices, the page at "/device".
	// Guard runs before anything is loaded, so an unknown device is never read. A Refusal is what the visitor may be told:
	// a 404, which says so, with a link on, in place of a bare 403.
	Guard: func(ctx context.Context, a DeviceArgs) error {
		if _, ok := service.Device(a.ID); !ok {
			return webui.NotFound(fmt.Sprintf("There is no device %q.", a.ID)).
				WithLink("Back to the devices", webui.Open(ctx, DevicesPath, webui.NoArgs{}))
		}
		return nil
	},
	// The selected tab is ?tabs.tab=raw-events, named by its label; the first tab,
	// Overview, needs no parameter.
	Body: webui.Tabs{
		{Label: "Overview", Body: webui.Split{DeviceForm, RecentEvents}},
		{Label: "Raw events", Body: RawEvents},
	},
}

var DeviceForm = webui.Form[Device]{
	Title: "Configuration",
	Desc:  "A Form leaf. Fields whose accessor declares no Store render read-only.",
	Load: func(ctx context.Context) (Device, webui.Loaded) {
		args := webui.ArgsOf[DeviceArgs](ctx)
		d, _ := service.Device(args.ID)
		return d, webui.Ready()
	},
	Fields: []webui.Accessor[Device]{
		// A Group puts its fields side by side; it is layout, with no frame.
		webui.Group[Device]{DeviceID, IP},
		webui.Group[Device]{Status, SiteChoice}, // choices from a list: one that is written out, and one that is read when the page is asked for
		Tags,                                    // a list to tick several of
		Notes,                                   // a text area
		Rate,                                    // a slider with no Store is a bar
		LastSeen,
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
		service.SaveDevice(d)
		// No redirect: a saved form returns to the page it was opened from, and
		// stays where it is when it was opened directly.
		return webui.Success("Device saved"), nil
	},
}

// RecentEvents is a table of a different model, beside the form. Split constrains nothing about what its children are about.
// It is always the last 15 minutes, whatever the Raw events tab is narrowed to.
var RecentEvents = webui.Table[Event]{
	Title:   "Recent events",
	Desc:    "The last 15 minutes, whatever the Raw events tab shows.",
	Rows:    func(context.Context) ([]Event, webui.Loaded) { return service.Events(15), webui.Ready() },
	Columns: []webui.Accessor[Event]{EventTime, EventKind, EventDetail},
}

// RawEvents is the events as far back as the page's Minutes says, which a link sets (the alerts' links do), and all of them when it is empty.
var RawEvents = webui.Table[Event]{
	Title: "Raw events",
	Desc:  "Narrowed by ?minutes= (as the alerts' links do): all of them without.",
	Rows: func(ctx context.Context) ([]Event, webui.Loaded) {
		return service.Events(webui.ArgsOf[DeviceArgs](ctx).Minutes), webui.Ready()
	},
	Columns: []webui.Accessor[Event]{EventTime, EventKind, EventDetail},
}

var (
	EventTime = webui.String[Event]{Label: "Time", Load: func(e Event) string { return e.At }}
	EventKind = webui.String[Event]{Label: "Kind", Load: func(e Event) string { return e.Kind }}
	// NoSort leaves a column out of the table's sorting, and NoFilter out of its filters: free text that is read and not
	// ordered by has no use for a sort link.
	EventDetail = webui.String[Event]{Label: "Detail", NoSort: true, Load: func(e Event) string { return e.Detail }}
)

// boundsOf is a table's numeric filters as the service takes them.
func boundsOf(ranges map[string]webui.Range) map[string]Bounds {
	bounds := make(map[string]Bounds, len(ranges))
	for key, r := range ranges {
		bounds[key] = Bounds(r)
	}
	return bounds
}
