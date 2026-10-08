package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/Olian04/webui/pkg/webui"
)

// The site is a path argument, so each site has its own address, and the
// breadcrumb reads Collector › Sites › Stockholm: the parent is the page mounted
// at /site.
type SiteArgs struct {
	Name string `webui:"name"`
}

var (
	SiteName = webui.String[SiteSummary]{
		Label: "Site",
		Load:  func(s SiteSummary) string { return s.Name },
	}
	SiteDevicesCount = webui.Int[SiteSummary]{
		Label: "Devices",
		Load:  func(s SiteSummary) int { return s.Devices },
	}
	SiteDegraded = webui.Int[SiteSummary]{
		Label: "Degraded",
		Load:  func(s SiteSummary) int { return s.Degraded },
	}
	SiteHealth = webui.Badge[SiteSummary]{
		Label: "Health",
		Load:  func(s SiteSummary) string { return s.Health() },
		Kinds: map[string]webui.Tone{"ok": webui.ToneOK, "degraded": webui.ToneWarning},
	}
	SiteRate = webui.Slider[SiteSummary]{
		Label: "Mean rate / s", Max: 15, Precision: 2,
		Load: func(s SiteSummary) float64 { return s.Rate },
	}
)

var Sites = webui.Page[webui.NoArgs]{
	Path: "/site",
	Nav:  webui.Nav{Label: "Sites", Icon: "location-dot"},
	// A second page offering Search: typing "stock" lists devices and sites,
	// each under its page's name.
	Search: func(ctx context.Context, query string) ([]webui.SearchResult, error) {
		var results []webui.SearchResult
		for _, site := range service.Sites() {
			if strings.Contains(strings.ToLower(site.Name), strings.ToLower(query)) {
				results = append(results, webui.SearchResult{
					Title:  site.Name,
					Desc:   fmt.Sprintf("%d devices", site.Devices),
					Target: webui.Open(ctx, SiteDetail, SiteArgs{Name: site.Name}),
				})
			}
		}
		return results, nil
	},
	Body: SitesTable,
}

// SitesTable is also on the landing page: a leaf is a value, so any page can
// hold the one a page already has.
var SitesTable = webui.Table[SiteSummary]{
	Title: "Sites",
	Desc:  "An unpaged table. It only says what its rows are, and the library sorts and filters them.",
	Rows:  func(context.Context) ([]SiteSummary, error) { return service.Sites(), nil },
	RowClick: webui.Link[SiteSummary, SiteArgs]{
		Page: SiteDetail,
		Args: func(_ context.Context, s SiteSummary) SiteArgs { return SiteArgs{Name: s.Name} },
	},
	Columns: []webui.Accessor[SiteSummary]{SiteName, SiteDevicesCount, SiteDegraded, SiteHealth, SiteRate},
}

var SiteDetail = webui.Page[SiteArgs]{
	Path: "/site/{name}",
	// No Nav: it lights Sites, the page at "/site".
	Guard: func(_ context.Context, a SiteArgs) error {
		if _, ok := service.Site(a.Name); !ok {
			return fmt.Errorf("there is no site %q", a.Name)
		}
		return nil
	},
	// Two tables side by side. Each keeps its own sort, filters and page in the
	// address under its ID, so they need distinct ones: ?devices.sort=… and
	// ?alerts.sort=… do not touch each other.
	Body: webui.Split{SiteDevices, SiteAlerts},
}

var SiteDevices = webui.Table[Device]{
	ID:       "devices",
	Title:    "Devices",
	PageSize: 5,
	Rows: func(ctx context.Context) ([]Device, error) {
		// The page says which site; the user's filters narrow within it.
		devices, _ := service.Devices(map[string][]string{Site.Label: {webui.ArgsOf[SiteArgs](ctx).Name}}, nil, Order{})
		return devices, nil
	},
	RowClick: webui.Link[Device, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
	},
	Columns: []webui.Accessor[Device]{DeviceID, IP, Status, Occurrences, Rate},
}

var SiteAlerts = webui.Table[Alert]{
	ID:    "alerts",
	Title: "Open alerts",
	Rows: func(ctx context.Context) ([]Alert, error) {
		return service.AlertsAt(webui.ArgsOf[SiteArgs](ctx).Name), nil
	},
	RowClick: webui.Link[Alert, AlertArgs]{
		Page: AlertDetails,
		Args: func(_ context.Context, a Alert) AlertArgs { return AlertArgs{ID: a.ID} },
	},
	Columns: []webui.Accessor[Alert]{AlertID, Severity, AlertDevice},
}
