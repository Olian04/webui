package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// The landing page is the page at "/", and it is the one that shows what the library looks like when it is all used at once: the
// collector's status in a toolbar, and below it four panels in a Split (two tables that page, sort and filter, a table of another model,
// and a form), each declared as it would be on a page of its own. It has no Nav of its own: the brand in the sidebar and the first breadcrumb both lead here.
// Without such a page the root goes to the first page it can find that needs no arguments, with no message.
var Overview = webui.Page[webui.NoArgs]{
	Path: "/",
	Body: webui.Stack{
		OverviewToolbar,
		webui.Split{OverviewDevices, OverviewAlerts, OverviewEvents, OverviewSettings},
	},
}

// Collector is what the landing page's toolbar shows of the system: its health and how big it is, in one model that is loaded together.
type Collector struct {
	SystemInfo
	Devices int
}

// The values of the landing page's toolbar are accessors of the models its Forms load, as a Form panel's fields are: the same accessors would do
// in either, and a Store on one would do nothing here, since a toolbar only shows.
var (
	overviewHealth = webui.Select[Collector]{
		Label: "Health",
		Load:  func(c Collector) []string { return []string{c.Health} },
		Options: []webui.Option{
			{Value: "healthy", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}, {Value: "down", Tone: webui.ToneCritical},
		},
	}
	overviewVersion = webui.String[Collector]{Label: "Version", Load: func(c Collector) string { return c.Version }}
	overviewUptime  = webui.String[Collector]{Label: "Uptime", Load: func(c Collector) string { return c.Uptime }}
	overviewDevices = webui.Int[Collector]{Label: "Devices", Load: func(c Collector) int { return c.Devices }}
	overviewQueue   = webui.Int[Collector]{
		Label: "Queue depth", Help: "Events that are waiting to be written.",
		Load: func(c Collector) int { return c.Queue },
	}
	overviewDisk = webui.Slider[Collector]{
		Label: "Disk used (%)", Min: 0, Max: 100,
		Load: func(c Collector) float64 { return c.DiskUsed },
	}
	overviewSyncState = webui.String[SyncJob]{
		Label: "Sync",
		Load: func(j SyncJob) string {
			if j.Running {
				return fmt.Sprintf("Syncing, %d of %d devices", j.Read, j.Total)
			}
			return j.State
		},
	}
)

// loadSync is where the sync job is, for the toolbar that shows it. While it goes on it says that what it shows is not final, and the toolbar is
// asked for again every two seconds, until it is done and the answer stops saying so.
func loadSync(context.Context) (SyncJob, webui.Loaded) {
	job := service.SyncJob()
	if job.Running {
		return job, webui.Ready().RefreshAfter(2 * time.Second)
	}
	return job, webui.Ready()
}

// OverviewToolbar is the status of the collector at a glance, in the row that is a Toolbar: the values of the system, read-only, from the
// left edge, in a Form that loads them together, and at the right edge the job that Sync now starts, in a Form of its own model, and the button,
// which only an editor is offered (a viewer's Guard refuses it, so for them it is left out). The sync job is not final while it goes on, so the
// toolbar is asked for again until it is done.
var OverviewToolbar = webui.Toolbar{
	Left: []webui.ToolbarItem{
		webui.Form[Collector]{
			Title: "System",
			Load: func(context.Context) (Collector, webui.Loaded) {
				_, devices := service.Devices(nil, nil, Order{})
				return Collector{SystemInfo: service.System(), Devices: devices}, webui.Ready()
			},
			Fields: []webui.Accessor[Collector]{overviewHealth, overviewVersion, overviewUptime, overviewDevices, overviewQueue, overviewDisk},
		},
	},
	Right: []webui.ToolbarItem{
		webui.Form[SyncJob]{Title: "Sync", Load: loadSync, Fields: []webui.Accessor[SyncJob]{overviewSyncState}},
		webui.Action[webui.NoArgs]{
			Label: "Sync now", Role: webui.RolePrimary,
			Confirm: "Sync every device now? It reads each one and may take a while.",
			Guard:   canEdit[webui.NoArgs],
			Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) {
				service.StartSync() // it goes on in the background, and the toolbar follows it
				return webui.Success("Sync started"), nil
			},
		},
	},
}

// OverviewDevices is the devices that need a look, as a table that pages, sorts and filters like the Devices page's, with fewer columns.
var OverviewDevices = webui.Table[Device]{
	Title:    "Devices",
	Desc:     "The same table as the Devices page's, with fewer columns. Every header sorts and filters, and a row opens its device.",
	PageSize: 9,
	Rows: func(context.Context) ([]Device, webui.Loaded) {
		devices, _ := service.Devices(nil, nil, Order{})
		return devices, webui.Ready()
	},
	RowClick: webui.Link[Device, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
	},
	Columns: []webui.Accessor[Device]{DeviceID, Status, Site, Occurrences, Rate},
}

// OverviewAlerts is the open alerts, with the button of a row that only an editor is offered, and that a critical alert shows disabled.
var OverviewAlerts = webui.Table[Alert]{
	Title:    "Open alerts",
	Desc:     "A row opens the alert. Dismiss is for editors, and says why it is disabled for a critical alert.",
	PageSize: 9,
	Rows:     func(context.Context) ([]Alert, webui.Loaded) { return service.OpenAlerts(), webui.Ready() },
	Key:      func(a Alert) string { return a.ID },
	RowClick: webui.Link[Alert, AlertArgs]{
		Page: AlertDetails,
		Args: func(_ context.Context, a Alert) AlertArgs { return AlertArgs{ID: a.ID} },
	},
	Columns: []webui.Accessor[Alert]{Severity, AlertDevice, Message, Raised},
	Actions: []webui.Action[Alert]{Dismiss},
}

// OverviewEvents is what the devices did in the last hour: a table of another model beside the others, which a Split does not mind.
var OverviewEvents = webui.Table[Event]{
	Title:    "Recent events",
	Desc:     "What the devices did in the last hour. A table of its own kind of row, in the same Split as the others.",
	PageSize: 8,
	Rows:     func(context.Context) ([]Event, webui.Loaded) { return service.Events(0), webui.Ready() },
	Columns:  []webui.Accessor[Event]{EventTime, EventKind, EventDetail},
}

// OverviewSettings is a handful of the collector's settings, each its own kind of input, in the form that the Ingest page has them all in.
var OverviewSettings = webui.Form[Settings]{
	Title:  "Collector settings",
	Desc:   "A few of the Ingest page's fields: a number with a slider, a choice of yes, no or neither, an email, a phone number with its country, a web address, a date and a colour.",
	Load:   func(context.Context) (Settings, webui.Loaded) { return service.Settings(), webui.Ready() },
	Submit: webui.Action[Settings]{Label: "Save", Guard: canEdit[Settings], Run: SaveIngest.Run},
	Fields: []webui.Accessor[Settings]{
		webui.Group[Settings]{SampleRate, MaxLoad},
		webui.Group[Settings]{Paused, Contact},
		webui.Group[Settings]{OnCall, Label},
		webui.Group[Settings]{Webhook, CertEnds},
	},
}
