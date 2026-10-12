// Package webui builds admin panels and control planes from a Go declaration. You
// describe pages made of tables and forms, and the library serves them as a
// working web application: there is no HTML, CSS or JavaScript to write, and the
// result works with JavaScript switched off.
//
// An [App] is a list of [Page] values. [App.Compile] checks the declaration, and
// returns an [net/http.Handler] you mount where you like.
//
// # Quick start
//
// A page with a table of devices:
//
//	type Device struct{ ID, IP string }
//
//	var (
//		ID = webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.ID }}
//		IP = webui.String[Device]{Label: "IP", Load: func(d Device) string { return d.IP }}
//	)
//
//	var Devices = webui.Page[webui.NoArgs]{
//		Path: "/device",
//		Nav:  webui.Nav{Label: "Devices", Icon: "display"},
//		Body: webui.Table[Device]{
//			Title:   "Devices",
//			Rows:    func(context.Context) ([]Device, error) { return service.Devices(), nil },
//			Columns: []webui.Accessor[Device]{ID, IP},
//		},
//	}
//
//	func main() {
//		app := webui.App{Brand: webui.Brand{Name: "Acme"}, Pages: webui.Pages{Devices}}
//		http.Handle("/admin/", app.MustCompile("/admin"))
//		log.Fatal(http.ListenAndServe(":8080", nil))
//	}
//
// That is a sidebar, a breadcrumb, and a table whose every column sorts and filters,
// at /admin/device. A runnable version is among the package examples.
//
// The declaration is meant to read as a static configuration: pages, accessors and
// actions are package-level variables, read top to bottom.
//
// # Pages and arguments
//
// A [Page] has a path, an optional [Nav] entry, an optional Guard, and a Body. Its
// type parameter is the page's argument struct. A field named by a placeholder in
// the path is a path segment, and every other field is a query parameter:
//
//	type DeviceArgs struct {
//		ID      string `webui:"id"` // /device/{id}
//		Minutes int                 // ?minutes=15
//	}
//
//	var Details = webui.Page[DeviceArgs]{Path: "/device/{id}", Body: ...}
//
// Arguments may be string, bool, int, int64 or float64, and one more, below. A field
// left at its zero value is absent from the address. A page's Guard runs before anything is loaded,
// on every request, and a page it refuses is answered with 403. Inside any closure
// that serves the page, [ArgsOf] returns the arguments.
//
// A page with arguments in its path has no entry in the sidebar by itself, since an entry
// is a link with one address and "/site/{name}" has none until a name is filled in. Its
// NavEntries lists them: one for each address, loaded whenever a page is drawn, each with the
// words it says and the arguments it opens.
//
//	var SitePage = webui.Page[SiteArgs]{
//		Path: "/site/{name}",
//		Nav:  webui.Nav{Icon: "location-dot", Section: "Sites"},
//		NavEntries: func(ctx context.Context) ([]webui.NavEntry[SiteArgs], error) {
//			...
//		},
//	}
//
// Nav gives the entries their icon and section, and no label. The entry of the site on screen
// is lit, and so is it for a page that borrows the place, such as "/site/{name}/log". An
// entry is left out for a visitor whom the page's Guard refuses, checked with the entry's own
// arguments. Loading is bounded: a deadline of two seconds, and the first fifty entries; and
// a failure leaves the page drawn without them.
//
// A page whose path is "/" is the landing page, and the brand in the sidebar and the first
// breadcrumb link to it. Without one they link to where the root would go: the first entry in
// the navigation the visitor may open, or failing that the first page whose path has no
// arguments and whose guard lets them in. Typing the root's address goes there too, with no
// message. If there is nowhere to go, the brand is plain text.
//
// # The rest of a path
//
// The last segment of a path may be written {name...}, which takes every segment from
// there on. Its field is a []string with one element for each segment, which is how a
// page can be at any depth of a tree, such as the folders of a bucket:
//
//	type ObjectArgs struct {
//		Bucket string   `webui:"bucket"` // /bucket/photos/...
//		Prefix []string `webui:"prefix"` // ...2026/summer/img.png
//	}
//
//	var Object = webui.Page[ObjectArgs]{Path: "/bucket/{bucket}/{prefix...}", Body: ...}
//
// The address /bucket/photos/2026/summer/img.png gives Prefix as
// ["2026", "summer", "img.png"]. Each segment is a crumb in the breadcrumb that leads to
// the page at that depth, so a nested folder reads as the folders above it. Like any
// path argument it is required: an address with nothing after the bucket is not this
// page. An element is one whole segment, so an empty one, or one that contains a slash,
// cannot be made into an address, and a doubled slash in an address is refused.
// Only the last segment may be a rest, because [net/http.ServeMux], which routes the
// pages, only allows it there, and no other argument may be a []string.
//
// # Layouts
//
// A page's Body is a single [PageBody]. [Table], [Form], [Editor], [Diff] and
// [Markdown] are leaves; they load their own data and refresh on their own. [Stack], [Split] and [Tabs] arrange
// other bodies and nest:
//
//	Body: webui.Stack{
//		webui.Split{DeviceForm, Events},
//		webui.Tabs{Panels: []webui.Tab{{Label: "Raw events", Body: Events}}},
//	}
//
// The selected tab is kept in the address, so it survives a reload and can be linked
// to, and only the selected panel is loaded.
//
// # Tables
//
// A [Table] is a list of rows of a model M, with columns that are [Accessor] values.
// The same accessor is a column in a table and an input in a form, so it is written
// once. [String], [Int], [Float], [Select], [URL], [Datetime], [Timestamp] and [Slider]
// are the accessors; without a Store an accessor is read-only, and a [URL] never has one.
// A [Select] is a choice from a list of options. Read, it is a badge for each value it holds,
// coloured by its option's [Tone]; edited, it is a drop-down list, or with Multi a list to
// choose several from.
// A Datetime or a Timestamp with a Store is a date and time picker, in UTC.
//
// A table says where its rows come from with exactly one of three fields:
//
//   - Rows returns every row, and the library filters, sorts and pages them from the
//     columns, comparing numbers as numbers and text as text. Use it when the rows
//     are all at hand or cheap to list in full, such as a slice in memory or a small
//     query. It is all most tables need. It is called on every request that needs
//     the rows, and nothing is kept between them.
//   - Load returns one page of rows and does the filtering, sorting and paging
//     itself. It is handed a [Query], with the window, sort and filters in force,
//     and returns a [Window]. Use it when the source can do that work better than
//     the library, or is too large to list in full, such as a database table or a
//     remote API.
//   - Feed is for a source that pages by a cursor, such as an object store's listing.
//     The rows come in the order the source gives them. Feed is handed the cursor of
//     the page asked for and returns the rows with the cursor of the next. A cursor
//     continues one sequence, so the table has no sort links or filters, only First
//     page, Previous and Next, and says which rows it shows.
//
// Every column header is a sort link with a filter beside it: a text box, a minimum
// and maximum for a number, or a choice among the values of a [Select]. A table keeps
// its sort, filters and page in the address, named by its title, so a copied address
// reproduces the view. A table pages, 25 rows at a time unless it says otherwise. A
// column is named by its Label.
//
// A table with Export set offers its rows as a download, CSV or JSON, from two links in its
// footer. The file is every row the table's sort and filters keep, not the page on screen;
// for a source that pages, the library asks it for the rows a batch at a time, up to
// 100,000. See [Table.Export] for what the cells hold.
//
// A table with a RowClick makes each row a link to another page, with a [Link]:
//
//	RowClick: webui.Link[Device, DeviceArgs]{
//		Page: Details,
//		Args: func(_ context.Context, d Device) DeviceArgs { return DeviceArgs{ID: d.ID} },
//	}
//
// A link names its destination page, so the compiler checks that the arguments fit it
// and [App.Compile] checks that the page is mounted. When the destination depends on
// the row, such as a folder or an object, RowClick is an [Action] instead: its Run is
// handed the row and chooses where to go with [Outcome.Then]. Actions and BulkActions
// add a button per row, and checkboxes with an action bar, over a table with a Key.
//
// # Forms and actions
//
// A [Form] loads one model and shows its Fields. An accessor with a Store is an input,
// and its Rules (required, length, pattern, bounds) are rendered as HTML constraint
// attributes and checked again on the server before the action runs. Submit is an
// [Action], and its Run says how it ended. A Form with no Submit is the detail view:
// a panel of labelled values to read, with nothing to edit.
//
// # Code editors
//
// An [Editor] shows one text of a model in a code editor, Monaco, the editor of VS
// Code, with syntax highlighting for its [Language] and everything a code editor has.
// Its Content is a [String]: with a Store the editor edits the text and saves it with
// its Submit, as a [Form] does, and without one it is a viewer. A [Diff] shows what
// changed between two texts, side by side when the page is wide enough, and only
// reads.
//
//	webui.Editor[Config]{
//		Title:    "Configuration",
//		Load:     loadConfig,
//		Content:  ConfigText,
//		Language: webui.LangYAML,
//		Submit:   webui.Action[Config]{Run: saveConfig},
//	}
//
// The editor is part of the program and is linked by the pages that have one. Without
// script an editor is a text box with a Save button, a viewer is the text in a block,
// and a Diff is a unified diff, so nothing needs script to work.
//
// A language has a Lang constant, one for each language the editor highlights. A few
// also have a language service, which gives diagnostics, completion and the like:
// JSON, CSS (with SCSS and Less), HTML (with Handlebars and Razor), and TypeScript
// (with JavaScript). A service is a worker of its own, so it is an optional import:
//
//	import _ "github.com/Olian04/webui/pkg/monaco/json"
//
// and without it the editor highlights and nothing more. The packages are
// pkg/monaco/json, pkg/monaco/css, pkg/monaco/html and pkg/monaco/typescript. A
// service goes only to an editor that can be edited, by the visitor who is shown it:
// a viewer, a visitor whose Submit Guard refuses them, and every [Diff] are
// highlighted only, so no service is sent for text that cannot change.
//
// A service can be told more than the text. An editor's Context is a folder, usually
// embedded, with a directory for each language it gives context to: the declaration files
// of the types the text may use, and compiler options, for TypeScript, and a schema for JSON.
//
//	//go:embed context
//	var files embed.FS // context/typescript/env.d.ts, context/json/schema.json
//
//	webui.Editor[Rule]{ /* ... */ Language: webui.LangTypeScript, Context: files }
//
// An editor reads the directory named for its own language, which can hold a tsconfig.json
// (only its compilerOptions are read), *.d.ts files at any depth, such as the types of a
// library under node_modules/@types, and for JSON a schema.json with any schemas it refers
// to. A file the editor would ignore is a compile error. The files are not in the page: it
// names them by address, each with a key that only a page drawn for a visitor who may edit
// carries, so the browser keeps them and others cannot fetch them. The key comes from
// [App.Secret], which is random unless set, and must be the same on every instance of an
// application that runs as several. The TypeScript service of a page is one program, so
// the TypeScript editors of one page share their context.
//
// # Markdown
//
// A [Markdown] shows one text of a model set as markdown: CommonMark with GitHub's
// tables, task lists, strikethrough and bare links. It is for text the application
// has, such as a runbook or release notes, and it only reads.
//
//	webui.Markdown[Handbook]{Title: "Runbook", Load: loadHandbook, Content: HandbookText}
//
// The text is trusted no further than any other text of the model. Raw HTML in it is
// never rendered, a link goes only to an http or https address (which opens in a tab of
// its own), a mailto, or a path in the application, and an image is not drawn but is a
// link to the picture, since the page would refuse a picture from elsewhere.
//
// An image is drawn when its origin is one the author trusts. TrustedOrigins lists them, each
// as https://host or https://host:port, and a picture from one of them appears where it is
// written, even inside a link. The page that has the Markdown, and no other, tells the
// browser it may fetch pictures from them, in the img-src of its content security policy.
//
//	webui.Markdown[Notes]{..., TrustedOrigins: []string{"https://cdn.example.com"}}
//
// A link that is a path, such as [Config](/config) or [a device](/device/dev_1?minutes=15),
// is an address in the application whatever it is mounted at: Compile's prefix is put in
// front of it, and it opens in the same tab. It is not checked, since the text is not known
// until it is loaded, and a path that is no page is the application's own not-found page.
// An address with two slashes or a backslash is another site, and is not a link.
// It needs no script. A fenced code
// block that names a language is coloured by the editor, which the page then loads, as
// for an [Editor] but never with a language service.
//
// # Outcomes
//
// Run returns an [Outcome], built with one of four constructors, and the library
// decides the toast, its colour, and the response:
//
//   - [Success] and [Warning] are accepted: the action was done, and the user is
//     sent back to the page they came from. A Warning says something the user should
//     be aware of.
//   - [Failure] and [Reject] are not accepted: nothing was done, and the form is
//     shown again with what the user typed kept. A Failure speaks for the whole form,
//     and a Reject for fields, with [Field].
//
// Those are the defaults, and an outcome can replace them, for any of the four kinds.
// [Outcome.Then] takes the user to another page. [Outcome.Back] takes them back to the
// page they came from, even when the action failed and there is nothing to retry.
// [Outcome.Stay] leaves them on the form, even when the action succeeded. An outcome says
// at most one of the three. The zero Outcome is a quiet success. Run's error is for what
// nobody can fix by editing the form: it is logged, and the user sees that something went
// wrong.
//
//	Run: func(ctx context.Context, d Device) (webui.Outcome, error) {
//		if service.IPTaken(d.IP, d.ID) {
//			return webui.Reject(webui.Field[Device](IP, "already in use")), nil
//		}
//		service.SetIP(d.ID, d.IP)
//		return webui.Success("Device saved"), nil
//	}
//
// A form that saved, and its Cancel button, return to the page the user came from, so
// a form reachable from several pages needs no code to say which. A form opened by its
// address, with no page before it, stays where it is, and Cancel goes to its parent in the
// breadcrumb.
//
// # Links between pages
//
// [Open] builds the address of a page from its arguments, for a redirect or anywhere
// else a link is wanted. It resolves through the compiled app, so the result includes
// the mount prefix, and a page that was never mounted is reported rather than turned
// into a dead link.
//
// Two pages that link to each other would be a Go initialization cycle if each named
// the other's variable. Declare the path of the page that is linked back to as a
// [PageID] constant, which is not a variable, and name that:
//
//	const DevicesPath webui.PageID[webui.NoArgs] = "/device"
//
// # Search
//
// The search box in the top bar lists the app's pages, and finds the things the pages
// are for. A [Page] says how with its Search: it is handed what was typed and returns
// the matches, each a [Hit] holding the page's arguments that lead to it and what to say
// about it. A page for one device finds devices, and a result is a link to the page at
// that device.
//
//	var Details = webui.Page[DeviceArgs]{
//		Path: "/device/{id}",
//		Search: func(ctx context.Context, query string, limit int) ([]webui.Hit[DeviceArgs], error) {
//			...
//		},
//	}
//
// For rows that are held in memory, [SearchRows] makes the Search: it looks in the fields
// it is given and the library does the matching. Results are listed under the page's
// name in the navigation, or under the entry it borrows its place from. A result is
// offered only to a visitor who could open the page it leads to: the page's Guard is run
// for each, with its own arguments.
//
// # Looks
//
// [Brand] gives the name and logo. The logo is also scaled into the browser's
// favicon, unless NoFavicon is set. [App.ExternalLinks] are the app's links to
// places outside it, such as signing out or the documentation, behind a button beside
// Refresh in the top bar. An [ExternalLink] is a [Nav] entry with a URL, used as
// written: a web address opens in a tab of its own, and a path on this site in the
// same one. Its Section groups it, and the entries without one come first. [Theme] restyles the four colours that carry
// meaning (accent, ok, warning and critical) and the library derives the rest for
// light and dark, which follow the viewer's system. A [Nav] Icon is the name of a Font
// Awesome Free solid icon, "house" for fa-house, served by the app itself.
//
// # Errors
//
// [App.Compile] collects every problem in the declaration and reports them together
// in one error that wraps a [CompileError] for each, naming the page, the argument
// type, what is wrong and how to fix it. The handler it returns is never nil: after a failed compile it serves
// the problems, as a page, at every path under the prefix, so a broken app is easy to
// diagnose in the browser. [App.MustCompile] panics instead, for a process that
// should refuse to start.
//
// # Metrics
//
// An app can count what it does, as Prometheus metrics, by being given the registry to put
// them in: [App.Metrics]. They are about the app and nothing else, with no Go runtime or
// process metrics, which the program has of its own: webui_http_requests_total and
// webui_http_request_duration_seconds for the requests each page answers,
// webui_http_requests_in_flight, webui_load_duration_seconds and webui_load_errors_total
// for the application's own Load functions, webui_actions_total for what actions come to
// (success, warning, failure, reject, invalid, denied or error), webui_exports_total,
// webui_search_duration_seconds, webui_search_errors_total, webui_nav_entries_errors_total
// and webui_build_info. A page is labelled by its path template, "/device/{id}", never by
// the address that was asked, so the number of series does not grow with the data.
//
// The app serves none of it. The program registers its own registry and serves it with
// promhttp where it serves the rest of its metrics, so where they are, and who may read them,
// are the program's decisions.
//
// # Security
//
// The library sets no authentication: mount the handler behind your own. A [Page]'s
// Guard and an [Action]'s Guard decide what a visitor may see and do, and the same
// check that disables a control also authorises the request. Navigation entries and
// search results a visitor could not open are left out. POST requests are
// protected against cross-site request forgery (CSRF) with
// [net/http.CrossOriginProtection], responses carry a content security policy, and
// nothing the visitor controls is trusted as a redirect target. The compile-error
// page shows type and field names, so mount an app that may fail to compile behind
// the same authentication.
package webui
