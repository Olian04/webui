package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// ActivityPath is the Activity page, named by a constant so that the page of a day can lead back to it.
const ActivityPath webui.PageID[webui.NoArgs] = "/activity"

// The Activity page is what is new, rather than what there is: the pages above it are tables of rows and forms, and this one is lists of items that say what
// happened, with a headline, a line of small facts and, where it has one, a picture. It is three of one idea. A List is a column of items; Cards are the same items as
// a grid, or a row that scrolls; and each shows a few and a Show more link that shows more, in the address, so a copied link shows the same.
//
// The source of each is a plain list: it is asked for one more than is shown, which is how the leaf knows there are more without counting them.
var Activity = webui.Page[webui.NoArgs]{
	Path: ActivityPath,
	Nav:  webui.Nav{Label: "Activity", Icon: "bolt", Section: "Platform"},
	Body: webui.Stack{
		webui.Split{
			// The two views of what is new share one panel's place: only the tab that is selected is read.
			webui.Tabs{
				{Label: "Alerts", Body: ActivityAlerts},
				{Label: "Releases", Body: ActivityReleases},
			},
			ActivityOnCall,
		},
		webui.Split{ActivityCalendar, ActivityDashboards},
		ActivityEscalation,
	},
}

// Escalation is what the notice below is decided from, and says: how many critical alerts are open.
type Escalation struct{ Critical int }

// CriticalOpen is a Bool of it: the accessor that says whether there is something to escalate. A When reads it as a form or a table would, and ignores
// what it has no use for, so it needs no Label or Store here.
var CriticalOpen = webui.Bool[Escalation]{Load: func(e Escalation) bool { return e.Critical > 0 }}

// ActivityEscalation is a When: a notice that is shown only while critical alerts are open, and leaves no gap when there are none. Its Load reads what the
// condition is decided from, its Show says whether it holds, and its Body loads its own data (here the count again: the model is not handed on). Dismiss or
// acknowledge the critical alerts on the Alerts page and it is gone, with whatever was in it out of reach. A When is for what a page shows: what lets a
// visitor act on an alert is the Guard of its action.
var ActivityEscalation = webui.When[Escalation]{
	Load: func(context.Context) (Escalation, webui.Loaded) {
		return Escalation{Critical: service.CriticalOpen()}, webui.Ready()
	},
	Show: CriticalOpen,
	Body: webui.Markdown[Escalation]{
		Title: "Escalation",
		Load: func(context.Context) (Escalation, webui.Loaded) {
			return Escalation{Critical: service.CriticalOpen()}, webui.Ready()
		},
		Content: webui.String[Escalation]{Label: "Escalation", Load: func(e Escalation) string {
			return fmt.Sprintf("**%d critical alerts are open.** Page the primary on call, then acknowledge them on the Alerts page. This notice is a `When`: it is here only while there are any.", e.Critical)
		}},
	},
}

// ReleaseDayArgs is the day that a day of the calendar leads to: its date, as the calendar writes it in the address.
type ReleaseDayArgs struct{ Date string }

// ReleaseDay is what was published on one day, which is where a day of ActivityCalendar leads. Its path is under the Activity page's, so that the page lights Activity in the
// menu, and it has no Nav of its own.
var ReleaseDay = webui.Page[ReleaseDayArgs]{
	Path: "/activity/releases/{date}",
	Guard: func(ctx context.Context, a ReleaseDayArgs) error {
		if _, err := time.Parse(time.DateOnly, a.Date); err != nil {
			return webui.NotFound(fmt.Sprintf("There is no day %q.", a.Date)).WithLink("Back to the activity", webui.Open(ctx, ActivityPath, webui.NoArgs{}))
		}
		return nil
	},
	Body: webui.List[Release]{
		Title: "Published",
		Load: func(ctx context.Context, _ int) ([]Release, webui.Loaded) {
			day, _ := time.Parse(time.DateOnly, webui.ArgsOf[ReleaseDayArgs](ctx).Date)
			return service.ReleasesBetween(day, day.AddDate(0, 0, 1)), webui.Ready()
		},
		Headline: releaseTitle,
		Link:     func(r Release) string { return r.Link },
		Meta:     []webui.Accessor[Release]{releaseAuthor, releaseDownloads},
		Badge:    releaseChannel,
	},
}

// ActivityCalendar is the releases of a month, a day of it marked when something was published and a link when there is a page that says what. The source is asked for the days
// that the grid shows, and the month is in the address, as "calendar.month", with real links to the months either side. A day is a day in UTC, and what the calendar takes for today is
// the demo's own, since its data is of one day.
var ActivityCalendar = webui.Calendar[Release]{
	Title: "Release calendar",
	Desc:  "The same releases, by the day they were published. A day with a dot has one or more, and leads to a page that lists them. The arrows change the month, in the address.",
	Load: func(_ context.Context, span webui.Span) ([]Release, webui.Loaded) {
		return service.ReleasesBetween(span.From, span.To), webui.Ready()
	},
	Date: func(r Release) time.Time { return r.When },
	Open: webui.Link[webui.Day, ReleaseDayArgs]{
		Page: ReleaseDay,
		Args: func(_ context.Context, d webui.Day) ReleaseDayArgs {
			return ReleaseDayArgs{Date: d.Date.Format(time.DateOnly)}
		},
	},
	WeekNumbers: true,
	Today:       func() time.Time { return epoch },
}

var (
	alertMessage = webui.String[Alert]{Label: "Message", Load: func(a Alert) string { return a.Message }}

	releaseTitle     = webui.String[Release]{Label: "Title", Load: func(r Release) string { return r.Title }}
	releaseAuthor    = webui.String[Release]{Label: "Author", Load: func(r Release) string { return r.Author }}
	releaseAge       = webui.String[Release]{Label: "Age", Load: func(r Release) string { return r.Age + " ago" }}
	releaseDownloads = webui.Int[Release]{
		Label: "Downloads", Load: func(r Release) int { return r.Downloads },
		Format: func(n int) string { return fmt.Sprintf("%d downloads", n) },
	}
	releaseChannel = webui.Select[Release]{
		Label: "Channel", Load: func(r Release) []string { return []string{r.Channel} },
		Options: []webui.Option{{Value: "stable", Tone: webui.ToneOK}, {Value: "beta", Tone: webui.ToneWarning}},
	}

	personName  = webui.String[Person]{Label: "Name", Load: func(p Person) string { return p.Name }}
	personRole  = webui.String[Person]{Label: "Role", Load: func(p Person) string { return p.Role }}
	personSince = webui.String[Person]{Label: "Since", Load: func(p Person) string { return p.Since }}
	personState = webui.Select[Person]{
		Label: "State",
		Load: func(p Person) []string {
			if p.OnCall {
				return []string{"on call"}
			}
			return []string{"off"}
		},
		Options: []webui.Option{{Value: "on call", Tone: webui.ToneOK}, {Value: "off"}},
	}

	dashboardName    = webui.String[Dashboard]{Label: "Name", Load: func(d Dashboard) string { return d.Name }}
	dashboardOwner   = webui.String[Dashboard]{Label: "Owner", Load: func(d Dashboard) string { return "by " + d.Owner }}
	dashboardPanels  = webui.Int[Dashboard]{Label: "Panels", Load: func(d Dashboard) int { return d.Panels }, Format: func(n int) string { return fmt.Sprintf("%d panels", n) }}
	dashboardUpdated = webui.String[Dashboard]{Label: "Updated", Load: func(d Dashboard) string { return "changed " + d.Updated + " ago" }}
)

// ActivityAlerts is the open alerts, newest first, each leading to the alert's own page: an Open is a Link, a real anchor, which is left out for an item whose page
// refuses the visitor. The severity is a badge beside the headline, which says it in words as well as in colour.
var ActivityAlerts = webui.List[Alert]{
	Title:    "Open alerts",
	Desc:     "The same alerts as the Alerts page's table, as a list: a headline, the device and how long ago, and a badge for how bad. Show more shows more.",
	PageSize: 4,
	Load: func(_ context.Context, limit int) ([]Alert, webui.Loaded) {
		alerts := service.OpenAlerts()
		return alerts[:min(limit, len(alerts))], webui.Ready()
	},
	Headline: alertMessage,
	Open: webui.Link[Alert, AlertArgs]{
		Page: AlertDetails,
		Args: func(_ context.Context, a Alert) AlertArgs { return AlertArgs{ID: a.ID} },
	},
	Meta:  []webui.Accessor[Alert]{AlertDevice, Raised},
	Badge: Severity,
}

// ActivityReleases is what has been published. Each is on another site, so its headline is a Link: it opens a tab of its own and says so, with the mark that a link
// to another site has everywhere else. The facts are in order under it, with a dot between them.
var ActivityReleases = webui.List[Release]{
	Title:    "Releases",
	PageSize: 4,
	Load: func(_ context.Context, limit int) ([]Release, webui.Loaded) {
		return service.Releases(limit), webui.Ready()
	},
	Headline: releaseTitle,
	Link:     func(r Release) string { return r.Link },
	Meta:     []webui.Accessor[Release]{releaseAuthor, releaseAge, releaseDownloads},
	Badge:    releaseChannel,
}

// ActivityOnCall is who looks after the collector: a list with a picture of each (an avatar, which is decoration, since the name is beside it) and a badge for whether they are on call.
// The pictures are on this site (/art), so no origin is trusted; a picture from another origin needs TrustedOrigins, and is otherwise left out.
var ActivityOnCall = webui.List[Person]{
	Title:    "People",
	PageSize: 4,
	Load: func(_ context.Context, limit int) ([]Person, webui.Loaded) {
		people := service.OnCall()
		return people[:min(limit, len(people))], webui.Ready()
	},
	Headline: personName,
	Meta:     []webui.Accessor[Person]{personRole, personSince},
	Badge:    personState,
	Image:    func(p Person) string { return "/art/" + slug(p.Name) + ".png" },
}

// ActivityDashboards is cards in a row that scrolls sideways: each is a picture over a headline and its facts. The row can be reached and moved by the keyboard, and each card
// snaps into place. CardsGrid, which is the default, wraps them into rows instead.
var ActivityDashboards = webui.Cards[Dashboard]{
	Title:    "Dashboards",
	Layout:   webui.CardsScroll,
	PageSize: 4,
	Load: func(_ context.Context, limit int) ([]Dashboard, webui.Loaded) {
		boards := service.Dashboards()
		return boards[:min(limit, len(boards))], webui.Ready()
	},
	Headline: dashboardName,
	Meta:     []webui.Accessor[Dashboard]{dashboardOwner, dashboardPanels, dashboardUpdated},
	Image:    func(d Dashboard) string { return "/art/" + slug(d.Name) + ".png" },
}

// slug is a name as a path has it: lower case, with dashes for spaces, since a picture's address may not hold a space.
func slug(name string) string { return strings.ReplaceAll(strings.ToLower(name), " ", "-") }
