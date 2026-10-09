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
// Arguments may be string, bool, int, int64 or float64. A field left at its zero
// value is absent from the address. A page's Guard runs before anything is loaded,
// on every request, and a page it refuses is answered with 403. Inside any closure
// that serves the page, [ArgsOf] returns the arguments.
//
// A page whose path is "/" is the landing page: the brand in the sidebar and the
// first breadcrumb link to it. Without one, the root goes to the first entry in the
// navigation.
//
// # Layouts
//
// A page's Body is a single [PageBody]. [Table] and [Form] are leaves; they load
// their own data and refresh on their own. [Stack], [Split] and [Tabs] arrange
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
// once. [String], [Int], [Float], [Badge], [URL], [Datetime], [Timestamp] and [Slider]
// are the accessors; without a Store an accessor is read-only, and a Badge never has one.
// A Datetime or a Timestamp with a Store is a date and time picker, in UTC.
//
// A table says where its rows come from with exactly one of two fields:
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
//
// Every column header is a sort link with a filter beside it: a text box, a minimum
// and maximum for a number, or a choice among the values of a [Badge]. A table keeps
// its sort, filters and page in the address, named by its title, so a copied address
// reproduces the view. A table pages, 25 rows at a time unless it says otherwise. A
// column is named by its Label.
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
// # Outcomes
//
// Run returns an [Outcome], built with one of four constructors, and the library
// decides the toast, its colour, and the response:
//
//   - [Success] and [Warning] are accepted: the action was done, and the user is
//     sent back. A Warning says something the user should be aware of.
//   - [Failure] and [Reject] are not accepted: nothing was done, and the form is
//     shown again with what the user typed kept. A Failure speaks for the whole form,
//     and a Reject for fields, with [Field].
//
// [Outcome.Then] says where to go next, for any of them. The zero Outcome is a quiet
// success. Run's error is for what nobody can fix by editing the form: it is logged,
// and the user sees that something went wrong.
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
// a form reachable from several pages needs no code to say which.
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
// The search box in the top bar lists the app's pages. A [Table] with Search set, and
// a RowClick, adds its rows: each is a result, led by its first column, that goes where
// the row click goes. A table with Rows is searched across every column by the
// library; a table with Load is handed the typed text in [Query.Search]. A result is
// offered only to a visitor who could open the page it leads to.
//
// # Looks
//
// [Brand] gives the name and logo. The logo is also scaled into the browser's
// favicon, unless NoFavicon is set. [App.Menu] is the app's own links, behind a
// button beside Refresh in the top bar: places outside the app, such as signing out
// or the documentation. A [MenuItem] is a [Nav] entry with an ExternalURL, used as
// written; its Section groups it, and the entries without one come first. [Theme] restyles the four colours that carry
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
