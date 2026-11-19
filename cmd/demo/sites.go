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
		// A mean that is a bar on the page has no number to type a bound for: its column is sorted and not filtered.
		NoFilter: true,
	}
)

// SitesPath is the list of sites, a constant for the reason that AlertsPath is.
const SitesPath webui.PageID[webui.NoArgs] = "/site"

var Sites = webui.Page[webui.NoArgs]{
	Path: SitesPath,
	View: webui.Stack{SitesTable, SitesCards},
}

// SitesCards is the same sites as cards in a grid that wraps, as many to a row as there is room for: CardsGrid, which is what a Cards is when it is given no Layout, and is written out here
// beside the Activity page's cards, which scroll sideways.
var SitesCards = webui.Cards[SiteSummary]{
	Title:    "Sites at a glance",
	Layout:   webui.CardsGrid,
	PageSize: 6,
	Loader:   webui.RowsLoader[SiteSummary]{Load: func(context.Context) ([]SiteSummary, webui.Loaded) { return service.Sites(), webui.Ready() }},
	Headline: SiteName,
	Open: webui.Link[SiteSummary, SiteArgs]{
		Page: SiteDetail,
		Args: func(_ context.Context, s SiteSummary) SiteArgs { return SiteArgs{Name: s.Name} },
	},
	Meta:  []webui.Accessor[SiteSummary]{SiteDevicesCount, SiteDegraded},
	Badge: SiteHealth,
}

// SitesTable is the list of sites. A panel is a value, so a page can hold the one another page has.
var SitesTable = webui.Table[SiteSummary]{
	Title:  "Sites",
	Desc:   "An unpaged table. It only says what its rows are, and the library sorts and filters them.",
	Loader: webui.RowsLoader[SiteSummary]{Load: func(context.Context) ([]SiteSummary, webui.Loaded) { return service.Sites(), webui.Ready() }},
	RowClick: webui.Link[SiteSummary, SiteArgs]{
		Page: SiteDetail,
		Args: func(_ context.Context, s SiteSummary) SiteArgs { return SiteArgs{Name: s.Name} },
	},
	Columns: []webui.Accessor[SiteSummary]{SiteName, SiteDevicesCount, SiteDegraded, SiteHealth, SiteRate},
}

// siteEntries is the sidebar's entry for each site, which is loaded when a page is drawn: each says its own words.
func siteEntries(context.Context) ([]webui.NavEntry[SiteArgs], error) {
	var entries []webui.NavEntry[SiteArgs]
	for _, s := range service.Sites() {
		entries = append(entries, webui.NavEntry[SiteArgs]{Args: SiteArgs{Name: s.Name}, Label: s.Name})
	}
	return entries, nil
}

var SiteDetail = webui.Page[SiteArgs]{
	Path: "/site/{name}",
	// Search makes the sites findable from the search box: typing "stock" finds the site beside
	// the devices that match. The page says what a site is as a result, and SearchRows, for
	// rows that are held in memory, does the matching over the fields it is given.
	Search: webui.SearchRows(
		func(context.Context) ([]SiteSummary, webui.Loaded) { return service.Sites(), webui.Ready() },
		func(s SiteSummary) webui.Hit[SiteArgs] {
			return webui.Hit[SiteArgs]{Args: SiteArgs{Name: s.Name}, Title: s.Name, Desc: s.Health()}
		},
		SiteName, SiteHealth,
	),
	Guard: func(ctx context.Context, a SiteArgs) error {
		if _, ok := service.Site(a.Name); !ok {
			return webui.NotFound(fmt.Sprintf("There is no site %q.", a.Name)).
				WithLink("Back to the sites", webui.Open(ctx, SitesPath, webui.NoArgs{}))
		}
		return nil
	},
	// Two tables side by side. Each keeps its own sort, filters and page in the
	// address under its title, so ?devices.sort=… and ?open-alerts.sort=… do not
	// touch each other.
	View: webui.Split{SiteDevices, SiteAlerts},
}

var SiteDevices = webui.Table[Device]{
	Title:    "Devices",
	PageSize: 5,
	Loader: webui.RowsLoader[Device]{Load: func(ctx context.Context) ([]Device, webui.Loaded) {
		// The page says which site; the user's filters narrow within it.
		devices, _ := service.Devices(map[string][]string{Site.Label: {webui.ArgsOf[SiteArgs](ctx).Name}}, nil, Order{})
		return devices, webui.Ready()
	}},
	RowClick: webui.Link[Device, DeviceArgs]{
		Page: Details,
		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
	},
	Columns: []webui.Accessor[Device]{DeviceID, IP, Status, Occurrences, Rate},
}

var SiteAlerts = webui.Table[Alert]{
	Title: "Open alerts",
	Loader: webui.RowsLoader[Alert]{Load: func(ctx context.Context) ([]Alert, webui.Loaded) {
		return service.AlertsAt(webui.ArgsOf[SiteArgs](ctx).Name), webui.Ready()
	}},
	RowClick: webui.Link[Alert, AlertArgs]{
		Page: AlertDetails,
		Args: func(_ context.Context, a Alert) AlertArgs { return AlertArgs{ID: a.ID} },
	},
	Columns: []webui.Accessor[Alert]{AlertID, Severity, AlertDevice},
}
