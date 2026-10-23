package main

import (
	"context"
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
		OverviewSync,
		webui.Split{OverviewDevices, OverviewAlerts, OverviewEvents, OverviewSettings},
	},
}

// The values of the landing page's toolbar are accessors of the page's arguments, which this page has none of: their Load reads
// the system instead, which a toolbar may do, since a value is only read. A Store would do nothing here, and none is given.
var (
	overviewHealth = webui.Select[webui.NoArgs]{
		Label: "Health",
		Load:  func(webui.NoArgs) []string { return []string{service.System().Health} },
		Options: []webui.Option{
			{Value: "healthy", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}, {Value: "down", Tone: webui.ToneCritical},
		},
	}
	overviewVersion = webui.String[webui.NoArgs]{Label: "Version", Load: func(webui.NoArgs) string { return service.System().Version }}
	overviewUptime  = webui.String[webui.NoArgs]{Label: "Uptime", Load: func(webui.NoArgs) string { return service.System().Uptime }}
	overviewDevices = webui.Int[webui.NoArgs]{
		Label: "Devices",
		Load: func(webui.NoArgs) int {
			_, total := service.Devices(nil, nil, Order{})
			return total
		},
	}
	overviewQueue = webui.Int[webui.NoArgs]{
		Label: "Queue depth", Help: "Events that are waiting to be written.",
		Load: func(webui.NoArgs) int { return service.System().Queue },
	}
	overviewDisk = webui.Slider[webui.NoArgs]{
		Label: "Disk used (%)", Min: 0, Max: 100,
		Load: func(webui.NoArgs) float64 { return service.System().DiskUsed },
	}
)

// OverviewToolbar is the status of the collector at a glance, in the row that is a Toolbar: the values of the system, read-only, from the
// left edge, and at the right edge a button, which only an editor is offered (a viewer's Guard refuses it, so for them it is left out).
var OverviewToolbar = webui.Toolbar[webui.NoArgs]{
	Left: []webui.ToolbarItem[webui.NoArgs]{overviewHealth, overviewVersion, overviewUptime, overviewDevices, overviewQueue, overviewDisk},
	Right: []webui.ToolbarItem[webui.NoArgs]{
		webui.Action[webui.NoArgs]{
			Label: "Sync now", Role: webui.RolePrimary,
			Confirm: "Sync every device now? It reads each one and may take a while.",
			Guard:   canEdit[webui.NoArgs],
			Run: func(context.Context, webui.NoArgs) (webui.Outcome, error) {
				service.StartSync() // it goes on in the background, and the panel beside it follows it
				return webui.Success("Sync started"), nil
			},
		},
	},
}

// OverviewSync follows the job that Sync now starts. A job takes time, so the button only starts it and the panel that shows it says when it
// is not final: while the job goes on its Load calls RefreshAfter, and a page with script asks for the panel again every two seconds and
// replaces it, until it is done and the Load stops saying so. Without script the panel says to reload.
var OverviewSync = webui.Form[SyncJob]{
	Title: "Sync",
	Desc:  "Sync now starts a job that takes eight seconds. This panel is asked for again every two seconds while it goes on, and stops when it is done.",
	Load: func(ctx context.Context) (SyncJob, error) {
		job := service.SyncJob()
		if job.Running {
			webui.RefreshAfter(ctx, 2*time.Second)
		}
		return job, nil
	},
	Fields: []webui.Accessor[SyncJob]{
		webui.Group[SyncJob]{
			webui.String[SyncJob]{Label: "State", Load: func(j SyncJob) string { return j.State }},
			webui.Int[SyncJob]{Label: "Devices read", Load: func(j SyncJob) int { return j.Read }},
			webui.Int[SyncJob]{Label: "Devices", Load: func(j SyncJob) int { return j.Total }},
		},
	},
}

// OverviewDevices is the devices that need a look, as a table that pages, sorts and filters like the Devices page's, with fewer columns.
var OverviewDevices = webui.Table[Device]{
	Title:    "Devices",
	Desc:     "The same table as the Devices page's, with fewer columns. Every header sorts and filters, and a row opens its device.",
	PageSize: 9,
	Rows: func(context.Context) ([]Device, error) {
		devices, _ := service.Devices(nil, nil, Order{})
		return devices, nil
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
	Rows:     func(context.Context) ([]Alert, error) { return service.OpenAlerts(), nil },
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
	Rows:     func(context.Context) ([]Event, error) { return service.Events(0), nil },
	Columns:  []webui.Accessor[Event]{EventTime, EventKind, EventDetail},
}

// OverviewSettings is a handful of the collector's settings, each its own kind of input, in the form that the Ingest page has them all in.
var OverviewSettings = webui.Form[Settings]{
	Title:  "Collector settings",
	Desc:   "A few of the Ingest page's fields: a number with a slider, a choice of yes, no or neither, an email, a phone number with its country, a web address, a date and a colour.",
	Load:   func(context.Context) (Settings, error) { return service.Settings(), nil },
	Submit: webui.Action[Settings]{Label: "Save", Guard: canEdit[Settings], Run: SaveIngest.Run},
	Fields: []webui.Accessor[Settings]{
		webui.Group[Settings]{SampleRate, MaxLoad},
		webui.Group[Settings]{Paused, Contact},
		webui.Group[Settings]{OnCall, Label},
		webui.Group[Settings]{Webhook, CertEnds},
	},
}
