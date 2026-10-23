package main

import (
	"context"
	"errors"
	"flag"
	"strconv"
	"time"

	"github.com/Olian04/webui/pkg/webui"
)

// The pages of the Loading section show what a visitor sees while a page's backends answer. Every panel here
// has a backend that takes a known time, so what arrives when can be seen, and the pages are the cases to try:
// a page with a panel that is slower than 50 ms is sent as it is built, in the order it has its panels, and a page
// whose panels answer sooner is sent in one piece.

var slowSidebar = flag.Duration("slow-sidebar", 0, "how long the Guard of the navigation page 'Answers at once' takes, to see a sidebar that waits on the app")

// Probe is a row of a panel that says how long its backend took.
type Probe struct{ Panel, Backend string }

var (
	ProbePanel   = webui.String[Probe]{Label: "Panel", Load: func(p Probe) string { return p.Panel }}
	ProbeBackend = webui.String[Probe]{Label: "Backend answered after", Load: func(p Probe) string { return p.Backend }}
)

// Note is a text that a panel shows.
type Note struct{ Text string }

var (
	NoteLook = webui.String[Note]{Label: "What to look for", Load: func(n Note) string { return n.Text }}
	NoteText = webui.String[Note]{Label: "Text", Load: func(n Note) string { return n.Text }}
)

// wait is a backend that takes d to answer, and stops when the visitor goes.
func wait(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// intro is the first panel of a page: it answers at once, and says what the page is for.
func intro(text string) webui.Form[Note] {
	return webui.Form[Note]{
		Title:  "What this page shows",
		Load:   func(context.Context) (Note, webui.Loaded) { return Note{Text: text}, webui.Ready() },
		Fields: []webui.Accessor[Note]{NoteLook},
	}
}

// backend is a table whose backend takes d, with as many rows as it is given, so that the panels differ in height.
func backend(title string, d time.Duration, rows int) webui.Table[Probe] {
	return webui.Table[Probe]{
		Title:   title,
		Columns: []webui.Accessor[Probe]{ProbePanel, ProbeBackend},
		Rows: func(ctx context.Context) ([]Probe, webui.Loaded) {
			wait(ctx, d)
			out := make([]Probe, rows)
			for i := range out {
				out[i] = Probe{Panel: title + " " + strconv.Itoa(i+1), Backend: d.String()}
			}
			return out, webui.Ready()
		},
	}
}

// Panels that answer in different times, drawn in the order the page has them: the 250 ms panel is ready before the
// 400 ms one above it and waits for it, and the sidebar and the first panels are there long before.
var LoadingOrder = webui.Page[webui.NoArgs]{
	Path: "/loading/order",
	Nav:  webui.Nav{Label: "In order", Icon: "list-ol", Section: "Loading"},
	Body: webui.Stack{
		intro("The sidebar and the panels above the slow one are on screen at once. The 400 ms panel arrives, and the 250 ms panel, which was ready before it, is drawn after it, because the page arrives in the order it is written. The page moves down as each panel arrives. Reload to see it again."),
		backend("Quick (10 ms)", 10*time.Millisecond, 2),
		backend("Slow (400 ms)", 400*time.Millisecond, 4),
		backend("Medium (250 ms)", 250*time.Millisecond, 3),
		backend("Last (50 ms)", 50*time.Millisecond, 2),
	},
}

// staircaseStep is how much longer each panel of the staircase takes than the one before it.
const staircaseStep = time.Second

// staircasePanels is how many panels the staircase has.
const staircasePanels = 10

// Ten panels, the first answering after a step and each next one a step later than the one before it. The panels load at
// the same time, so the page takes as long as the last of them, and each is drawn as it is done: a panel a step apart from the
// last, at a steady pace, with the spinner and the status bar saying that more is coming until the end.
var LoadingStaircase = webui.Page[webui.NoArgs]{
	Path: "/loading/staircase",
	Nav:  webui.Nav{Label: "Staircase (10 s)", Icon: "stairs", Section: "Loading"},
	Body: staircase(),
}

func staircase() webui.Stack {
	body := webui.Stack{intro("Ten panels, loading at the same time: the first answers after 1 s, and each next one 1 s later than the one before it, so the last answers after 10 s. The page is sent as it is built, so each panel is drawn as it is ready, one a second, with the page growing. The spinner after the content and the \"loading\" in the status bar stay until the last panel is in, and the bar then says how long all of it took. Reload to see it again; stop it half way to see what a page that ends early says.")}
	for i := 1; i <= staircasePanels; i++ {
		body = append(body, backend("Panel "+strconv.Itoa(i)+" ("+strconv.Itoa(i)+" s)", time.Duration(i)*staircaseStep, 2))
	}
	return body
}

// Stamp is a row that says when it was read, so that a panel that was read again can be told from one that was not.
type Stamp struct{ Panel, ReadAt string }

var (
	StampPanel  = webui.String[Stamp]{Label: "Panel", Load: func(s Stamp) string { return s.Panel }}
	StampReadAt = webui.String[Stamp]{Label: "Read at", Load: func(s Stamp) string { return s.ReadAt }}
)

// stamped is a table whose backend takes d and says when it answered, to the millisecond.
func stamped(title string, d time.Duration) webui.Table[Stamp] {
	return webui.Table[Stamp]{
		Title:   title,
		Columns: []webui.Accessor[Stamp]{StampPanel, StampReadAt},
		Rows: func(ctx context.Context) ([]Stamp, webui.Loaded) {
			wait(ctx, d)
			return []Stamp{{Panel: title, ReadAt: time.Now().Format("15:04:05.000")}}, webui.Ready()
		},
	}
}

// Tabs that load only the tab that is shown. With script a tab is followed by asking for the Tabs alone, so the panel
// outside it keeps the moment it was read at, and only the tab that was chosen is read. Without script a tab is a link to
// the page, as it always was.
var LoadingTabs = webui.Page[webui.NoArgs]{
	Path: "/loading/tabs",
	Nav:  webui.Nav{Label: "Tabs read one tab", Icon: "window-restore", Section: "Loading"},
	Body: webui.Stack{
		intro("Follow a tab. The panels above and below the Tabs keep the moment they were read at, because only the Tabs is asked for, and only the tab that you chose is read: its panels say when, and the tab you left is read again only if you come back. Each tab takes 500 ms. Reload the page, which reads all of it, to see the difference. Without script a tab is a link to the page, and the whole page is read."),
		stamped("Above the tabs (read with the page)", 0),
		webui.Tabs{Panels: []webui.Tab{
			{Label: "First", Body: webui.Split{stamped("First, left (500 ms)", 500*time.Millisecond), stamped("First, right (500 ms)", 500*time.Millisecond)}},
			{Label: "Second", Body: stamped("Second (500 ms)", 500*time.Millisecond)},
			{Label: "Third", Body: stamped("Third (500 ms)", 500*time.Millisecond)},
		}},
		stamped("Below the tabs (read with the page)", 0),
	},
}

// Layouts inside layouts. A layout is drawn like the page is: its panels in order, so the quick panel on the right of the
// Split waits for the slow one on its left. Tabs build the strip with the selected tab, so they arrive together.
var LoadingNested = webui.Page[webui.NoArgs]{
	Path: "/loading/nested",
	Nav:  webui.Nav{Label: "Nested layouts", Icon: "layer-group", Section: "Loading"},
	Body: webui.Stack{
		intro("A Split of a slow panel (300 ms) and a quick one (20 ms), then Tabs whose selected tab is slow (200 ms), then a last panel. A layout is sent in order like the page: the quick panel on the right of the Split is ready first and is drawn after the slow one on its left. The Tabs strip arrives together with the tab it selects. Each part waits for the part above it."),
		webui.Split{
			backend("Split, left (300 ms)", 300*time.Millisecond, 3),
			backend("Split, right (20 ms)", 20*time.Millisecond, 3),
		},
		webui.Tabs{Panels: []webui.Tab{
			{Label: "Slow tab (200 ms)", Body: backend("Selected tab (200 ms)", 200*time.Millisecond, 2)},
			{Label: "Other tab (20 ms)", Body: backend("Other tab (20 ms)", 20*time.Millisecond, 2)},
		}},
		backend("After the layouts (100 ms)", 100*time.Millisecond, 2),
	},
}

// Panels that fail. A Load that returns an error is a panel that says it could not load, wherever it is. A Load that
// panics is a 500 for a page that is built whole; once the page has begun to be sent it cannot be, so the panel says it
// could not load too, the cause is logged, and the panels after it are drawn.
var LoadingFailures = webui.Page[webui.NoArgs]{
	Path: "/loading/failures",
	Nav:  webui.Nav{Label: "Panels that fail", Icon: "triangle-exclamation", Section: "Loading"},
	Body: webui.Stack{
		intro("The first panel is slow (300 ms), so the page is on its way when the others fail. A panel whose Load returns an error and a panel whose Load panics each say that they could not load, where they are, and the panel after them is drawn. The cause of each is in the server's log, and is never shown."),
		backend("Slow (300 ms)", 300*time.Millisecond, 2),
		webui.Table[Probe]{
			Title:   "Load returns an error",
			Columns: []webui.Accessor[Probe]{ProbePanel, ProbeBackend},
			Rows: func(ctx context.Context) ([]Probe, webui.Loaded) {
				wait(ctx, 50*time.Millisecond)
				return nil, webui.Failed(errors.New("the backend is unavailable"))
			},
		},
		webui.Form[Probe]{
			Title: "Load panics",
			Load: func(ctx context.Context) (Probe, webui.Loaded) {
				wait(ctx, 100*time.Millisecond)
				panic("this panel's backend fell over")
			},
			Fields: []webui.Accessor[Probe]{ProbePanel},
		},
		backend("After the failures (10 ms)", 10*time.Millisecond, 2),
	},
}

// A page whose head depends on its panels is built whole: a code editor links its script and its stylesheet in the head,
// which is sent before any panel is, so nothing of this page arrives until the slow panel has answered.
var LoadingWhole = webui.Page[webui.NoArgs]{
	Path: "/loading/whole",
	Nav:  webui.Nav{Label: "Built whole (editor)", Icon: "file-code", Section: "Loading"},
	Body: webui.Stack{
		intro("This page has a code editor, and an editor needs its script and stylesheet in the head of the page, which is sent first. So it is not sent as it is built: nothing of it arrives until the 400 ms panel has answered, and then all of it does, as one piece. Compare with 'In order'."),
		backend("Slow (400 ms)", 400*time.Millisecond, 3),
		webui.Editor[Note]{
			Title: "A code editor",
			Load: func(context.Context) (Note, webui.Loaded) {
				return Note{Text: "{\n  \"built\": \"whole\"\n}\n"}, webui.Ready()
			},
			Content:  NoteText,
			Language: webui.LangJSON,
		},
	},
}

// A Guard decides before anything is loaded, so a page that refuses is a 403 at once, whatever its panels are.
var LoadingGuarded = webui.Page[webui.NoArgs]{
	Path:  "/loading/guarded",
	Nav:   webui.Nav{Label: "Guard refuses", Icon: "shield-halved", Section: "Loading"},
	Guard: canEdit[webui.NoArgs],
	Body: webui.Stack{
		intro("This page is for editors. As a viewer it is the 'Not permitted' state at once, with the status 403, and its slow panels were never read. As an editor it is sent as it is built, like 'In order'."),
		backend("Slow (300 ms)", 300*time.Millisecond, 3),
		backend("Medium (150 ms)", 150*time.Millisecond, 2),
	},
}

// A page whose panels answer at once is built whole and sent in one piece, as it always was. Run the demo with
// -slow-sidebar 300ms to see what a sidebar that waits on the app does: the Guard of this page's navigation entry is
// asked for on every page, beside the panels, so every page's first byte waits for it and for nothing else.
var LoadingFast = webui.Page[webui.NoArgs]{
	Path: "/loading/fast",
	Nav:  webui.Nav{Label: "Answers at once", Icon: "bolt", Section: "Loading"},
	Guard: func(ctx context.Context, _ webui.NoArgs) error {
		wait(ctx, *slowSidebar)
		return nil
	},
	Body: webui.Stack{
		intro("Every panel here answers within 50 ms, so the page is built whole and sent in one piece, with nothing flushed early. Start the demo with -slow-sidebar 300ms and every page, this one included, waits 300 ms for its sidebar, because the Guard of this entry is asked for on every page, beside the panels."),
		backend("Instant", 0, 3),
		backend("Quick (20 ms)", 20*time.Millisecond, 2),
	},
}
