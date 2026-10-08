package main

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

var (
	AlertID = webui.String[Alert]{
		Label: "ID",
		Load:  func(a Alert) string { return a.ID },
	}
	Severity = webui.Badge[Alert]{
		Label: "Severity",
		Load:  func(a Alert) string { return a.Severity },
		Kinds: map[string]webui.Tone{
			"critical": webui.ToneCritical, "warning": webui.ToneWarning, "info": webui.ToneNeutral,
		},
	}
	AlertDevice = webui.String[Alert]{
		Label: "Device",
		Load:  func(a Alert) string { return a.Device },
	}
	Message = webui.String[Alert]{
		Label: "Message",
		Load:  func(a Alert) string { return a.Message },
	}
)

var Alerts = webui.Page[webui.NoArgs]{
	Path: "/alert",
	Nav:  webui.Nav{Label: "Alerts", Icon: "bell"},
	Body: webui.Table[Alert]{
		Title: "Alerts",
		Desc:  "Bulk actions exist because the table declares them; the checkbox column is their consequence.",
		// Rows is all it takes: the library filters, sorts, pages and searches them
		// by the columns, so there is no query code on this page.
		Search: true,
		Rows:   func(context.Context) ([]Alert, error) { return service.OpenAlerts(), nil },
		// A row opens that alert's own page, whose id is a path argument.
		RowClick: webui.Link[Alert, AlertArgs]{
			Page: AlertDetails,
			Args: func(_ context.Context, a Alert) AlertArgs { return AlertArgs{ID: a.ID} },
		},
		// Actions and bulk actions name rows by Key, never by position.
		Key:         func(a Alert) string { return a.ID },
		Columns:     []webui.Accessor[Alert]{AlertID, Severity, AlertDevice, Message},
		Actions:     []webui.Action[Alert]{Dismiss},
		BulkActions: []webui.Action[[]Alert]{Acknowledge, Delete},
	},
}

type AlertArgs struct {
	ID string `webui:"id"`
}

// AlertDetails is one alert: a form of read-only fields whose only action is to
// acknowledge it, beside the device it is about.
var AlertDetails = webui.Page[AlertArgs]{
	Path: "/alert/{id}",
	// No Nav: it lights Alerts, the page at "/alert".
	Guard: func(_ context.Context, a AlertArgs) error {
		if _, ok := service.Alert(a.ID); !ok {
			return fmt.Errorf("there is no alert %q", a.ID)
		}
		return nil
	},
	Body: webui.Split{AlertForm, AlertDeviceTable},
}

// A form with no writable field: every accessor lacks a Store, so it only shows
// the alert, and its Submit is what a person does about it.
var AlertForm = webui.Form[Alert]{
	Title: "Alert",
	Desc:  "A Form whose fields are all read-only, with one action.",
	Load: func(ctx context.Context) (Alert, error) {
		args := webui.ArgsOf[AlertArgs](ctx)
		a, _ := service.Alert(args.ID)
		return a, nil
	},
	Fields: []webui.Accessor[Alert]{
		webui.Group[Alert]{AlertID, Severity},
		AlertDevice,
		Message,
	},
	Submit: AcknowledgeAlert,
}

var AcknowledgeAlert = webui.Action[Alert]{
	Label: "Acknowledge",
	Guard: canEdit[Alert],
	Run: func(_ context.Context, a Alert) (webui.Outcome, error) {
		service.Acknowledge(a.ID)
		return webui.Success("Acknowledged " + a.ID), nil
	},
}

// AlertDeviceTable is the device the alert is about, as a one-row table whose row
// opens it, narrowed to the last 15 minutes: the Link builds the destination's
// arguments, query ones too.
var AlertDeviceTable = webui.Table[Device]{
	ID:    "device",
	Title: "Device",
	Rows: func(ctx context.Context) ([]Device, error) {
		alert, _ := service.Alert(webui.ArgsOf[AlertArgs](ctx).ID)
		device, ok := service.Device(alert.Device)
		if !ok {
			return nil, nil
		}
		return []Device{device}, nil
	},
	RowClick: webui.Link[Device, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID, Minutes: 15} },
	},
	Columns: []webui.Accessor[Device]{DeviceID, IP, Status, Site},
}

var Dismiss = webui.Action[Alert]{
	Label: "Dismiss",
	Guard: canEdit[Alert],
	Run: func(_ context.Context, a Alert) (webui.Outcome, error) {
		service.Acknowledge(a.ID)
		return webui.Success("Dismissed " + a.ID), nil
	},
}

// A bulk action's Guard receives the selection, so it runs when the action does.
var Acknowledge = webui.Action[[]Alert]{
	Label: "Acknowledge",
	Role:  webui.RolePrimary,
	Guard: canEdit[[]Alert],
	Run: func(_ context.Context, alerts []Alert) (webui.Outcome, error) {
		for _, a := range alerts {
			service.Acknowledge(a.ID)
		}
		return webui.Success(fmt.Sprintf("Acknowledged %d", len(alerts))), nil
	},
}

// A destructive action is drawn in the critical colour. Like every action it is
// gated by its Guard: as a viewer it is disabled, with the reason.
var Delete = webui.Action[[]Alert]{
	Label: "Delete",
	Role:  webui.RoleDestructive,
	Guard: canEdit[[]Alert],
	Run: func(_ context.Context, alerts []Alert) (webui.Outcome, error) {
		ids := make([]string, len(alerts))
		for i, a := range alerts {
			ids[i] = a.ID
		}
		service.DeleteAlerts(ids...)
		return webui.Success(fmt.Sprintf("Deleted %d", len(alerts))), nil
	},
}
