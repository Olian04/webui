package main

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

type AlertsArgs struct {
	Severity string
}

var (
	AlertID = webui.String[Alert]{
		Label: "ID",
		Load:  func(a Alert) string { return a.ID },
	}
	Severity = webui.Badge[Alert]{
		Label: "Severity",
		Load:  func(a Alert) string { return a.Severity },
		Tones: map[string]webui.Tone{"critical": webui.ToneCritical, "warning": webui.ToneWarning},
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

var Alerts = webui.Page[AlertsArgs]{
	Path: "/alert",
	Nav:  webui.Nav{Label: "Alerts"},
	Body: webui.Table[Alert]{
		Title: "Alerts",
		Desc:  "Bulk actions exist because the table declares them; the checkbox column is their consequence.",
		Load: func(ctx context.Context, _ webui.Query) (webui.Rows[Alert], error) {
			args, err := webui.ArgsOf[AlertsArgs](ctx)
			if err != nil {
				return webui.Rows[Alert]{}, err
			}
			alerts := service.OpenAlerts(args.Severity)
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
