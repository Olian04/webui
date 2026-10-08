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
	Nav:  webui.Nav{Label: "Alerts"},
	Body: webui.Table[Alert]{
		Title: "Alerts",
		Desc:  "Bulk actions exist because the table declares them; the checkbox column is their consequence.",
		Load: func(_ context.Context, q webui.Query) (webui.Rows[Alert], error) {
			// No Key on these accessors, so Query.Sort and Query.Filters use the
			// Label. Severity has a fixed set of options (its Kinds), so its filter
			// arrives as the options chosen.
			alerts := filteredBy(service.OpenAlerts(), q.Filters, map[string]func(a Alert, values []string) bool{
				AlertID.Label:     containing(func(a Alert) string { return a.ID }),
				Severity.Label:    oneOf(func(a Alert) string { return a.Severity }),
				AlertDevice.Label: containing(func(a Alert) string { return a.Device }),
				Message.Label:     containing(func(a Alert) string { return a.Message }),
			})
			alerts = sortedBy(alerts, q.Sort, q.Desc, map[string]func(x, y Alert) int{
				AlertID.Label:     func(x, y Alert) int { return cmp.Compare(x.ID, y.ID) },
				Severity.Label:    func(x, y Alert) int { return cmp.Compare(x.Severity, y.Severity) },
				AlertDevice.Label: func(x, y Alert) int { return cmp.Compare(x.Device, y.Device) },
				Message.Label:     func(x, y Alert) int { return cmp.Compare(x.Message, y.Message) },
			})
			return webui.Rows[Alert]{Items: alerts, Total: len(alerts)}, nil
		},
		// Actions and bulk actions name rows by Key, never by position.
		Key:         func(a Alert) string { return a.ID },
		Columns:     []webui.Accessor[Alert]{AlertID, Severity, AlertDevice, Message},
		Actions:     []webui.Action[Alert]{Dismiss},
		BulkActions: []webui.Action[[]Alert]{Acknowledge},
	},
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
