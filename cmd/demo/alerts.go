package main

import (
	"cmp"
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
		Load: func(_ context.Context, q webui.Query) (webui.Rows[Alert], error) {
			alerts := alertRows(service.OpenAlerts(), q)
			return webui.Rows[Alert]{Items: alerts, Total: len(alerts)}, nil
		},
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
	Run: func(_ context.Context, a Alert) (webui.Effect, error) {
		service.Acknowledge(a.ID)
		return webui.Effect{Toast: "Acknowledged " + a.ID}, nil
	},
}

// AlertDeviceTable is the device the alert is about, as a one-row table whose row
// opens it, narrowed to the last 15 minutes: the Link builds the destination's
// arguments, query ones too.
var AlertDeviceTable = webui.Table[Device]{
	ID:    "device",
	Title: "Device",
	Load: func(ctx context.Context, _ webui.Query) (webui.Rows[Device], error) {
		args := webui.ArgsOf[AlertArgs](ctx)
		alert, _ := service.Alert(args.ID)
		device, ok := service.Device(alert.Device)
		if !ok {
			return webui.Rows[Device]{}, nil
		}
		return webui.Rows[Device]{Items: []Device{device}, Total: 1}, nil
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
	Run: func(_ context.Context, a Alert) (webui.Effect, error) {
		service.Acknowledge(a.ID)
		return webui.Effect{Toast: "Dismissed " + a.ID}, nil
	},
}

// A bulk action's Guard receives the selection, so it runs when the action does.
var Acknowledge = webui.Action[[]Alert]{
	Label: "Acknowledge",
	Role:  webui.RolePrimary,
	Guard: canEdit[[]Alert],
	Run: func(_ context.Context, alerts []Alert) (webui.Effect, error) {
		for _, a := range alerts {
			service.Acknowledge(a.ID)
		}
		return webui.Effect{Toast: fmt.Sprintf("Acknowledged %d", len(alerts))}, nil
	},
}

// A destructive action is drawn in the critical colour. Like every action it is
// gated by its Guard: as a viewer it is disabled, with the reason.
var Delete = webui.Action[[]Alert]{
	Label: "Delete",
	Role:  webui.RoleDestructive,
	Guard: canEdit[[]Alert],
	Run: func(_ context.Context, alerts []Alert) (webui.Effect, error) {
		ids := make([]string, len(alerts))
		for i, a := range alerts {
			ids[i] = a.ID
		}
		service.DeleteAlerts(ids...)
		return webui.Effect{Toast: fmt.Sprintf("Deleted %d", len(alerts))}, nil
	},
}

// alertRows filters and sorts alerts as a table's Query asks. The alerts page
// and each site's alerts share it, since they show the same columns.
//
// No Key on these accessors, so Query.Sort and Query.Filters use the Label.
// Severity has a fixed set of options (its Kinds), so its filter arrives as the
// options chosen.
func alertRows(alerts []Alert, q webui.Query) []Alert {
	alerts = filteredBy(alerts, q.Filters, map[string]func(a Alert, values []string) bool{
		AlertID.Label:     containing(func(a Alert) string { return a.ID }),
		Severity.Label:    oneOf(func(a Alert) string { return a.Severity }),
		AlertDevice.Label: containing(func(a Alert) string { return a.Device }),
		Message.Label:     containing(func(a Alert) string { return a.Message }),
	})
	return sortedBy(alerts, q.Sort, q.Desc, map[string]func(x, y Alert) int{
		AlertID.Label:     func(x, y Alert) int { return cmp.Compare(x.ID, y.ID) },
		Severity.Label:    func(x, y Alert) int { return cmp.Compare(x.Severity, y.Severity) },
		AlertDevice.Label: func(x, y Alert) int { return cmp.Compare(x.Device, y.Device) },
		Message.Label:     func(x, y Alert) int { return cmp.Compare(x.Message, y.Message) },
	})
}
