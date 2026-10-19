package main

import (
	"context"
	"fmt"

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
	SiteHealth = webui.Select[SiteSummary]{
		Label:   "Health",
		Load:    func(s SiteSummary) []string { return []string{s.Health()} },
		Options: []webui.Option{{Value: "ok", Tone: webui.ToneOK}, {Value: "degraded", Tone: webui.ToneWarning}},
	}
	SiteRate = webui.Slider[SiteSummary]{
		Label: "Mean rate / s", Max: 15, StepSize: 0.01,
		Load: func(s SiteSummary) float64 { return s.Rate },
	}
)

var Sites = webui.Page[webui.NoArgs]{
	Path: "/site",
	Nav:  webui.Nav{Label: "Sites", Icon: "location-dot", Section: "Platform"},
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
	// A page with path arguments has no entry of its own, since an entry is a link with one address
	// and this has one for each site. NavEntries gives it one for each, which it loads when a page is
	// drawn: the icon and the section are the group's, and each entry says its own words. The page of a
	// site lights its entry, and so does any page below it that borrows its place.
	Nav: webui.Nav{Icon: "location-dot", Section: "Sites"},
	NavEntries: func(context.Context) ([]webui.NavEntry[SiteArgs], error) {
		var entries []webui.NavEntry[SiteArgs]
		for _, s := range service.Sites() {
			entries = append(entries, webui.NavEntry[SiteArgs]{Args: SiteArgs{Name: s.Name}, Label: s.Name})
		}
		return entries, nil
	},

	// Search makes the sites findable from the search box: typing "stock" finds the site beside
	// the devices that match. The page says what a site is as a result, and SearchRows, for
	// rows that are held in memory, does the matching over the fields it is given.
	Search: webui.SearchRows(
		func(context.Context) ([]SiteSummary, error) { return service.Sites(), nil },
		func(s SiteSummary) webui.Hit[SiteArgs] {
			return webui.Hit[SiteArgs]{Args: SiteArgs{Name: s.Name}, Title: s.Name, Desc: s.Health()}
		},
		SiteName, SiteHealth,
	),
	Guard: func(_ context.Context, a SiteArgs) error {
		if _, ok := service.Site(a.Name); !ok {
			return fmt.Errorf("there is no site %q", a.Name)
		}
		return nil
	},
	// Two tables side by side. Each keeps its own sort, filters and page in the
	// address under its title, so ?devices.sort=… and ?open-alerts.sort=… do not
	// touch each other.
	Body: webui.Split{SiteDevices, SiteAlerts},
}

var SiteDevices = webui.Table[Device]{
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
