// Package webui builds admin panels and control planes from a Go declaration. You
// describe pages made of tables and forms, and the library serves them as a
// working web application: there is no HTML, CSS or JavaScript to write, and the
// result works with JavaScript switched off.
//
// An [App] is a list of [Page] values. [App.Compile] checks the declaration, and
// returns a [net/http.Handler] you mount where you like.
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
//			Rows:    func(context.Context) ([]Device, webui.Loaded) { return service.Devices(), webui.Ready() },
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
// Arguments may be string, bool, int, int64 or float64, and a slice of any of them (see
// Arguments that repeat). A field left at its zero value is absent from the address. A
// page's Guard runs before anything is loaded, on every request, and a page it refuses
// is answered with 403, or with the status of a [Refusal] (see Refusals). Inside any
// closure that serves the page, [ArgsOf] returns the arguments.
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
// A path says its arguments as they are: "/device/dev_27c38b" reads "dev_27c38b" in the
// breadcrumb and the tab title. A page whose argument is not what a person knows the
// thing by sets Crumb, a function of its arguments, to say what the crumb reads, such
// as the device's name. It is also what the page reads as the parent of a page below
// it, "/device/{id}/log", whose path has all of its arguments. It is called whenever a
// page is drawn, with a deadline of two seconds for all the crumbs of a page together,
// and is not called for a parent whose Guard refuses the visitor; a failure, a panic or
// an empty answer leaves the words of the path. A page whose path ends in {name...}
// cannot set Crumb, which is a compile error, and a parent that has an argument that is
// not in the path is asked with that argument empty.
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
// pages, only allows it there, and no other segment of the path may be a slice.
//
// # Arguments that repeat
//
// A slice that is not the rest of a path is a query parameter that repeats: the
// parameter once for each element, in the order they are given, which is how a URL has always
// carried a list. Its elements may be any of the argument types.
//
//	type CompareArgs struct {
//		Device []string // ?device=a&device=b
//		Rating []int    // ?rating=3&rating=5
//	}
//
// The address /compare?device=a&device=b gives Device as ["a", "b"], and [Open] and a
// [Link] build an address the same way, so the page's own links keep every element. A list
// with no element is absent from the address, as a zero field is, and so is an element
// that is an empty string, which a URL cannot tell from none. An address with a value that
// is not of the element's type, "?rating=three", is a 400, as a scalar is. An element
// cannot hold the control character U+001F, which would be read back as two, and
// building an address from one is an error.
//
// # Layouts
//
// A page's Body is a single [PageBody]. [Table], [Form], [Editor], [Diff] and
// [Markdown] are leaves; they load their own data and refresh on their own, a
// [Toolbar] is a leaf that is a row of controls and the values of its forms, and a [FileUpload] is a leaf that is a
// panel for sending one large file. [Stack], [Split], [Columns], [When] and [Tabs] arrange
// other bodies and nest:
//
//	Body: webui.Stack{
//		webui.Split{DeviceForm, Events},
//		webui.Tabs{{Label: "Raw events", Body: Events}},
//	}
//
// The selected tab is kept in the address, so it survives a reload and can be linked
// to, and only the selected panel is loaded.
//
// A [When] shows one body only while a condition holds: Load reads a model, Show is a [BoolLike] of it (a [Bool], or an [OptionalBool] whose not set is no), and Body is what is shown. What it hides is out of reach of a refresh,
// a form post and an export, and it leaves no gap. It loads, for the condition alone, so keep its Load cheap; the body loads its own data.
//
// [Columns] is a row of columns of unequal width, each [Column] with a Weight that is its share relative to the others
// (zero is 1): {1, 3} is 25/75 and {25, 50, 25} is 25/50/25. A [Split] is Columns of equal weights, written as a list of bodies. A column is never narrower than 280px, so where a row cannot hold its columns the last move to a row below, each row sharing its width
// by weight in turn, down to one column on a narrow screen. The weights are numbers, not a pattern, so there is nothing to mistype and no sum that must come to 100.
//
// # Accessors
//
// An accessor is a named, typed projection of a model: the one description of a value
// that is an input in a [Form], a column in a [Table] and, read-only, a value in a
// [Toolbar]. It is written once. [String], [Int], [Float], [Select], [URL], [Datetime],
// [Timestamp], [Slider] and [Bool] are the basic ones; [Email], [Phone], [Website],
// [ColorPicker], [OptionalInt], [OptionalFloat], [OptionalSlider], [OptionalBool] and [File]
// are below, and a [Group] lays accessors side by side in a form. Without a Store an
// accessor is read-only, and a [URL] never has one. An accessor's Rules (a
// [StringRules] or a [NumberRules]: length, a pattern, bounds) are rendered
// as HTML constraint attributes and checked again on the server, and its Placeholder is shown in an input while it is empty. A [Float] shows Precision decimals.
//
// Required is written on the accessor, once, whatever it holds: an empty submission is refused, in the browser and on the server.
// What an empty submission means without it is the accessor's default, which is also what its field in a form starts as: for an
// accessor that cannot be nil, the value that Load gives (a [Int] or a [Float] shows what is stored, 0 for a model that has none, a [Bool] is
// ticked or not), and for one that can, which is the Optional ones, nothing, "not set", which is nil. So an input of the first kind that is emptied
// stores the zero value of its type, as a [String] stores "", and one of the second kind stores nil. A number that may be absent is not a flag on
// [Int]: its Go type differs, and it is an [OptionalInt].
//
// Every accessor but a [Group] has a Help, a line or two on what the field is for,
// which a form and a toolbar show in the popover beside the label that lists the rules,
// under a heading of its own and above them (on hover and on keyboard focus, and in the
// icon's accessible name), where the label and the rules cannot say it: "The share of
// events that are kept: 0.25 keeps one in four." A table does not show it.
//
// A [Select] is a choice from a list of options. Read, it is a badge for each value it
// holds, coloured by its option's [Tone]; edited, it is a drop-down list, or with Multi
// a list to choose several from. Its Options are written out, or read when a request is
// answered with LoadOptions, for what the app keeps in a database.
//
// A Datetime or a Timestamp with a Store is a date and time picker, in UTC; with
// DateOnly it is a date picker, and a day is midnight UTC.
//
// A kind of text that has an input of its own is an accessor of its own: [Email],
// [Phone], [Website] and [ColorPicker]. The browser draws the input of that kind (the
// right keyboard on a phone, a picker for a colour) and refuses an email or a web
// address that is not one, in the words the server uses; the server checks all four
// again. An email is held to what the browser holds it to; a web address to more, an
// http or https address with a host, since a browser accepts "javascript:" and what is
// stored will be linked. A phone number is digits with the punctuation numbers are
// written with, which a browser does not check. One written with a country code is
// stored as E.164, "+46701234567", without the spaces; the input is a selector of the
// country calling code and the national number, which script spaces for reading as it
// is typed, and [Phone.DefaultCountry] is where the selector starts. A colour is
// "#rrggbb", and a picker cannot be empty, so a field with no value shows black. They
// have the rules of a [String], except the colour, which has none; a [String] is the
// accessor for any other text, and for a password or a text area. [URL] is the one that
// links, and has no Store.
//
// A [String] with Secret set is a password input for a value that is never read back,
// such as an API key. The stored value is not in the page, in a table, in a file or in
// a search, and not in the form that answers a refused submission; the input shows a
// row of bullets, the same whatever the value is, when there is one. A submission that
// leaves it empty leaves it as it is, so only a key that is typed is stored. A Secret
// needs a Store, cannot be Required (an empty submission leaves it as it is), and is
// for a form only: not a table's column, an editor's text or a toolbar's value.
//
// A [String] with Multiline set is a text area, for text of more than one line. Its
// rules are MinLen and MaxLen, and it may be Required; a Pattern and the text rules are one line's
// and are refused, and a line break is one "\n" character whatever the browser sent, so
// MaxLen counts what the browser's maxlength counts. In a table the cell shows the
// first lines. It is not for a toolbar, which shows one line for a value.
//
// A number or a choice that may be absent has an accessor of its own, since a Go int
// cannot say it: [OptionalInt], [OptionalFloat], [OptionalSlider] and [OptionalBool] load a pointer,
// and an [OptionalBool] is a yes, a no, or neither. Absent is not zero. It reads as a dash, an
// input that is emptied is stored as nil (and a Required one refuses it, on every accessor that can be emptied, as it does for a [String]), a table sorts
// it before every number and leaves it out of a filter by range, a CSV leaves it empty
// and a JSON export has null. An [OptionalBool] is a choice of Yes and No with an empty one, as
// a [Select] is, and not a checkbox, which cannot be none of them and sends nothing
// when it is off. A [Bool] loads a bool, and is the checkbox: an absent field is no.
//
// An Int, a Float, an OptionalInt, an OptionalFloat, a Datetime and a Timestamp have a
// Format, a function the application writes that says how the value reads where it is
// only read, such as a size as "1.5 GiB":
//
//	webui.Int[Blob]{Label: "Size", Load: func(b Blob) int { return b.Size }, Format: humanBytes}
//
// It changes only what is shown. The column still sorts, filters and exports by the value,
// an input that edits it holds the value, and the search finds a row by the text and by the
// value. The library ships no formats: each is the application's own.
//
// # Files
//
// A file that a visitor sends is a [File]: an input of a form, a native file chooser,
// with the file that is stored shown above it, and in a table or a read-only form that
// file's name, as a link when the app serves it. The library keeps no files: a file is
// held in memory, in [Upload.Data], until the request is answered, and never written to
// disk, so a File is for a file that the app can hold for each visitor that sends one at
// once, with a MaxSize to match. Store is handed the [Upload] to copy where the app
// keeps files, Load says what is kept, and where it is served from is the app's own
// address.
//
//	webui.File[Device]{
//		Label: "Logo", Accept: []webui.FileKind{webui.FileImage}, MaxSize: 1 << 20,
//		Load:  func(d Device) *webui.FileInfo { return d.LogoInfo() },
//		Store: func(d *Device, up *webui.Upload) { d.SetLogo(up) },
//	}
//
// A file is of a kind by what it holds, which the library reads: the first bytes of an
// image or a PDF, and the whole of JSON, CSV and text. What it is called, and the type
// that the browser says it has, are the visitor's to choose and are not trusted; the
// words of the field name the kinds it takes ("Choose a PNG or JPEG file."), and the
// [Upload] says which it was: its Kind, for what to do with it, and its ContentType, a
// media type, for what to serve it as. There is no kind for SVG, which a browser runs
// as a page: an app that takes it says [FileAny] and owns what follows. A file has a
// size limit, 5 MiB unless said, which is checked with the kind before Store sees the
// file. Nothing chosen leaves the stored file alone, and a form that is refused stores
// no file and says to choose it again, since a browser cannot fill a chooser in.
// [File.Removable] adds a Remove checkbox, which calls Store with nil.
//
// With script the chooser is a frame that a file can be dropped on, a small picture of
// an image that is chosen is shown below it (made in the page, and clicked to see the
// file as it was chosen, at its full size, in a dialog), a file that is too large or is
// not one of the kinds that the field takes is said to be so before the request is sent
// (in the words the server uses), and a file that is chosen and not sent can be
// cleared. The input stays the control: a drop puts the file in it, as if it had been
// chosen, so the post is the browser's own and works the same without the script. The
// early check is of the size, the name and the type that the browser gives, and is only
// a hint, the server deciding by what the file holds; a file whose type the browser
// does not know is not said to be wrong.
//
// The file that is stored is shown as a small picture when the app gives one:
// [FileInfo.Thumbnail], a path on this site, which a Store that is handed an image
// makes with [Thumbnail] when it keeps the file. It is shown at 32 pixels beside the
// name, and the visitor is sent that small file and not the image, which is only
// fetched when they click the picture, to see it at its full size in a dialog. That is
// in the page and needs no script: when the [FileInfo] also has a URL, which is the
// file that the dialog shows, the picture is a button that opens a native dialog with
// an invoker command, as Confirm does, and the image in the dialog is lazy, which has
// no box, and so is not fetched, while the dialog is closed (as for Confirm, an older
// browser needs the script). The library never shows the file itself as a thumbnail, so
// without a Thumbnail there is no picture. [Thumbnail] reads PNG, JPEG and GIF, and no
// WebP, which the standard library does not decode; an app that takes more has its own
// scaler.
//
// A File's MaxSize is 5 MiB unless it says, and as large as the app says: the library sets no ceiling, and the body of a page's submission is limited to the sum of its files' MaxSize and a megabyte. [Thumbnail] makes the small picture from an [Upload] and returns
// [ErrThumbnail] for a kind that it does not read (anything but PNG, JPEG and GIF), a
// damaged file, or a picture of more than 25 million pixels, which is held whole in
// memory to be scaled.
//
// A file that is too large to be held in memory, which is a File's limit, is not a form's
// value at all: it is a [FileUpload], a panel of its own. Its Receive is an [Action] whose
// subject is the [FileHandle], the file as it arrives, and whose Run reads it from the
// handle's Reader wherever it is to go, an object store, with nothing held by the library,
// in memory or on disk, and says how it ended with an [Outcome]:
//
//	webui.FileUpload[Release]{
//		Title: "Upload firmware", MaxSize: 2 << 30,
//		Load:   func(ctx context.Context) (Release, webui.Loaded) { return Release{Channel: "stable"}, webui.Ready() }, // the defaults
//		Fields: []webui.Accessor[Release]{channel},                                                   // beside the file
//		Receive: webui.Action[webui.FileHandle[Release]]{
//			Guard: canEdit,
//			Run: func(ctx context.Context, f webui.FileHandle[Release]) (webui.Outcome, error) {
//				if _, err := bucket.PutObject(ctx, "firmware/"+f.Name, f.Reader); err != nil {
//					return webui.Outcome{}, err
//				}
//				return webui.Success("Uploaded " + f.Name), nil
//			},
//		},
//	}
//
// What the visitor chooses beside the file is the panel's model, as a form's is: Load gives
// the defaults and Fields are the inputs (any accessor that a form has), which come before the
// file in the page and in the request, so that they are checked, and the Guard asked with them
// as [FileHandle.Options], before any of the file is read. A refused choice refuses the upload
// without reading it. A panel with nothing to choose says struct{} and has neither.
//
// A file that was chosen in a folder has the folder in [FileHandle.Path], made safe, apart
// from its Name; [FileHandle.RawName] is the name as the browser sent it, to tell the visitor
// that it was changed, and [FileHandle.DeclaredType] is the type that the browser said, which is
// what to prefer for the type of what is stored when the visitor's word is better than the first
// bytes. A test posts the panel's form as a browser does: a multipart body of the field "_leaf",
// which is where the panel is in the page's Body ("p" when it is the Body, "p.0" for the first
// of a [Stack], "p.1.0" for the first in the second, and so on), and then the field "file", in
// that order. Those names are stable.
//
// A file of no bytes is a file, as a folder may have some: Run is handed it, with a reader that is at its
// end, and "Choose a file." is for a post that has no file in it.
//
// It is a native form with one file and a button, and the chooser is the same frame that
// a file can be dropped on. With script the file is sent by the page, as a request of its own
// with a progress bar, and what the server answers is HTML: a list item with the outcome's
// words and, when the outcome goes somewhere, a link to it, which is put in the page as it is,
// with a status that says how it went (200 accepted, 422 refused, 403 not permitted, 500 failed).
// With Multiple the chooser takes several files and has a button for a folder, and a folder can
// be dropped on it; the files are sent one after another, each with its own bar and its own
// result, and Receive is called for each, with the options that were chosen. Without script
// the panel takes one file and posts it, as it always did. The Guard is asked when the panel is drawn, which draws it
// disabled for [Disabled] and leaves it out for any other error, and again when the file
// arrives, after its first 512 bytes, which tell its kind, and before more. MaxSize has no
// default, since how large is the app's to say, and is held to as the file is read.
// Only the kinds that the first bytes tell can be taken, as the file is read once. See
// [FileUpload].
//
// What the app serves it should serve as a download: Content-Disposition: attachment,
// X-Content-Type-Options: nosniff, and the content type that the [Upload] was found to
// have. A visitor's file sent inline from the app's origin is a page of the app's. A
// page that has a File reads a multipart body of as much as its files may be, in the
// order that it is sent in, and runs the page's Guard first, so a visitor it refuses has
// nothing read; an action's own Guard runs after the form is bound, so the file has been
// received when it refuses. A
// program that takes uploads sets ReadTimeout on its server, which ReadHeaderTimeout
// does not bound.
//
// # Toolbars
//
// A [Toolbar] is a row of controls for the page it is on, such as "Create device" above
// a list of devices. It is not a panel, so it has no frame, and it sits on the page in
// a [Stack] with what it is for. It has no type parameter, because it has no model of its
// own: its buttons are about the page, which they read from the request with [ArgsOf], and
// what it shows is in its [Form]s, each of which loads a model of its own:
//
//	Body: webui.Stack{
//		webui.Toolbar{
//			Left: []webui.ToolbarItem{
//				webui.Link[webui.NoArgs, NewDeviceArgs]{Label: "Create device", Role: webui.RolePrimary, Page: NewDevice, Args: newDeviceArgs},
//				webui.Action[webui.NoArgs]{Label: "Sync now", Guard: canEdit, Run: syncNow},
//			},
//			Right: []webui.ToolbarItem{
//				webui.Form[Health]{Title: "Collector", Load: loadHealth, Fields: []webui.Accessor[Health]{Status, Queue}},
//			},
//		},
//		DevicesTable,
//	}
//
// The controls are in two groups. With room, the Left ones are at the left edge and the
// Right ones at the right, with what the page has to spare between them; when there is
// no longer room for them all in a row, they flow from the left in rows, as one group,
// the Left ones first.
//
// An [Action] of [NoArgs] is a button that posts, and its Run says how it ended with an
// [Outcome], as any action does. It has no subject: a toolbar has no model to give it, and an
// Action or a [Link] of anything else does not compile. Its Guard decides whether the visitor
// may, and the same check draws the button or leaves it out, and refuses the request (see
// Controls that a visitor may not use). A [Link] of [NoArgs] is a button that goes to another
// page, with a Label and a Role, built from the request; it is a real link, so it works in a
// new tab and with a keyboard, and it is left out for a visitor whom the page it leads to
// refuses. A toolbar whose controls are all left out is not drawn.
//
// A [Form] is a group of values, read-only: the model its Load returns, shown as its fields'
// labels and values in a row, as a read-only form field or a table cell shows them (a [Select]
// as its badges, a number with its Format, a [Bool] as Yes or No, a dash when there is nothing),
// under its Title, which names the group. The accessors are the ones a form panel uses, and a
// Store on one is ignored, as it is in a table. A Form in a toolbar has no Submit, Desc or
// Help, and no field that is multiline, secret or a file: a toolbar has no inputs, and a Form
// panel is for editing. Several Forms of different models may share a toolbar, and a Form
// whose Load fails says so in its place and leaves the rest of the toolbar as it was.
//
// The toolbar is the leaf that refreshes, as any panel is: a Form whose Load returns
// [Loaded.RefreshAfter] has the whole toolbar asked for again (see Following a job), after the
// shortest delay of its Forms. Unlike a panel, a toolbar says nothing of it in the row, which has little room; without script it is not refreshed.
//
// # Tables
//
// A [Table] is a list of rows of a model M, with columns that are [Accessor] values:
// the same accessor that is an input in a [Form].
//
// A table says where its rows come from with exactly one of three fields:
//
//   - Rows returns every row, and the library filters, sorts and pages them from the
//     columns, comparing numbers as numbers and text as text. Use it when the rows
//     are all at hand or cheap to list in full, such as a slice in memory or a small
//     query. It is all most tables need. It is called on every request that needs
//     the rows, and nothing is kept between them.
//   - Load returns one page of rows and does the filtering, sorting and paging
//     itself. It is handed a [Query], with the window, sort and filters in force (a numeric column's filter is a [Range], either end of it optional), and returns a [Window]. Use it when the source can do that work better than
//     the library, or is too large to list in full, such as a database table or a
//     remote API.
//   - Feed is for a source that pages by a cursor, such as an object store's listing.
//     The rows come in the order the source gives them. Feed is handed the cursor of
//     the page asked for and returns the rows with the cursor of the next. A cursor
//     continues one sequence, so the table has no filters, only First page, Previous
//     and Next, and says which rows it shows. A column can be sorted, but only the page
//     on screen, which the footer says; the next page is ordered within itself.
//
// Every column header is a sort link with a filter beside it: a text box, a minimum
// and maximum for a number, or a choice among the values of a [Select]. A column that
// has no use for one says NoSort or NoFilter. A table keeps
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
// # Caching
//
// The library keeps nothing between requests, and caching is not part of it: how long a
// value may be reused, what it depends on and what makes it wrong are for the app to say.
// Cache inside the function that gets the data, in Rows, Load, a Form's Load or whatever
// they call, and see Example_caching for a small one.
//
//   - Key it by everything the data depends on. An app's Load may read who is looking from
//     ctx, and a value shared between visitors is right only for data that is the same
//     for all of them. A Load is handed a [Query], so key by that too.
//   - Rows is called on every page load, sort, filter, page change, row action and search,
//     so a cache in Rows serves all of them from one listing, which the library then
//     sorts and filters on its own copy.
//   - Join the requests that arrive while the value is being loaded, so that a busy page
//     makes one call and a visitor who leaves does not fail the others, and do not keep an
//     error.
//   - Forget what an action changed by calling your own invalidation from its Run.
//   - A row action and a bulk action find their rows through the same Rows or Load, so what
//     they act on is as old as your cache. An action's Guard and Run are given the row as
//     it was listed, not as it is now.
//
// # Forms and actions
//
// A [Form] loads one model and shows its Fields. An accessor with a Store is an input,
// and its Rules (required, length, pattern, bounds) are rendered as HTML constraint
// attributes and checked again on the server before the action runs. When the browser
// refuses a value it says what the server would say beneath the field ("Must be at most
// 100.", a Pattern's message), and not its own "Please match the format requested."
// Submit is an [Action], and its Run says how it ended. A Form with no Submit is the detail view:
// a panel of labelled values to read, with nothing to edit.
//
// An action that cannot be taken back asks first, with Confirm: its button opens a
// dialog with the question, and only the button in the dialog goes on.
//
//	webui.Action[[]Alert]{Label: "Delete", Role: webui.RoleDestructive, Confirm: "Delete the selected alerts?", ...}
//
// It is a native dialog, opened by an invoker command, so it needs no script in a
// current browser (Chrome and Edge 135, Firefox 144, Safari 26.2); script supplies the
// same for an older one, and an older one with no script cannot open it, and so cannot
// do the action. It is a question for the visitor and not a check: a request made by
// hand is not asked, and what must not happen is refused by the Guard. A form whose
// Save asks is not saved by Enter in a field.
//
// Confirm cannot be set on a RowClick, which has no button to ask from.
//
// # Following a job
//
// An action that starts something long (a backup, a sync) only starts it, and returns [Success] as any action does. The panel that shows the
// job is what follows it up: its Load reads the state of the job, and while the job is not done returns [Loaded.RefreshAfter], which says that the
// panel is not final and is to be asked for again. A page with script asks for the panel again after that time and replaces it, until an answer does
// not say it; a page without script says how long to wait and the visitor reloads. The request that asks again is the same GET that loads the
// panel, so nothing has to be safe to repeat.
//
// # Controls that a visitor may not use
//
// An action's Guard decides whether the visitor may do it, and what it returns decides
// how the control is drawn, in a table's rows, in its selection bar, and in a toolbar.
// An ordinary error leaves the control out, as if there were none, which is right when
// the visitor has no business knowing of it. [Disabled] draws it, disabled, with its
// message in a tooltip, which is right when the visitor ought to know that it exists
// and why they cannot use it ("Already granted"). Which of the two is the application's
// to say, since only it knows, and there is no option for it. Either way the same Guard
// refuses a request made by hand, with the message, so what is drawn is a convenience
// and the Guard is the authority.
//
//	Guard: func(ctx context.Context, d Device) error {
//		if !canEdit(ctx) {
//			return errors.New("requires the editor role") // the button is not there
//		}
//		if d.Locked {
//			return webui.Disabled("This device is locked") // the button is there, and says why it cannot be used
//		}
//		return nil
//	}
//
// A bulk action's Guard is given a selection, which does not exist when the table is
// drawn. It is asked of each row of the window as a selection of that one row, and the
// button is drawn if it accepts any of them, so a bulk Guard must accept a selection of
// one. If it refuses every one, the button is disabled with the message of the first
// that said [Disabled], and is otherwise left out. A table with no bulk action drawn
// has no column of checkboxes and no bar, and one with no row action drawn has no
// column of buttons; that column is there when a row of the page on screen has one, and
// so may come and go between the pages of a table. A Link, in a toolbar or as a table's
// RowClick, is left out when the page it leads to would refuse the visitor, which asks
// that page's Guard when the link is drawn, for each row of a table. A Guard that
// panics leaves its control out, and the panic is logged. A form's Save that its Guard
// refuses, whichever error says so, is not drawn: the form is shown read-only, with a
// Back button.
//
// A [RowClick] that is an [Action] and that its Guard refuses leaves the row shown and
// doing nothing, whichever error says so, since it has no button to leave out or to
// disable. A table with no rows has nothing to select, and draws no bulk action.
// [Disabled] with no message is drawn with "Not allowed.", and one that a page's Guard
// returns answers with a 403 and its message.
//
// # Text rules and patterns
//
// A string field says what its text starts with, ends with and contains, and what it must not:
//
//	Rules: webui.StringRules{
//		StartsWith:  "https://",
//		EndsWith:    ".example.com",
//		NotContains: " ",
//	}
//
// These are [StringRules.StartsWith], [StringRules.EndsWith], [StringRules.Contains]
// and their negations, NotStartsWith, NotEndsWith and NotContains. Case matters, and
// each says what it requires in its own message ("Must start with "https://"."). The
// server tests them with the strings package, and in the browser they are part of the
// input's pattern, together with the field's own, so that the page refuses the values
// that the server refuses, and says which rule it was. Where the need is one of these
// they are the way to say it: a pattern cannot say what a value must not contain
// without a lookahead, which a pattern may not have.
//
// A [PatternRule] is for what they cannot say. A pattern is run twice, by Go on the
// server and by the browser, as the pattern attribute of the input, and a regular
// expression is not the same in the two: Go's is RE2 and a browser's is JavaScript's. A
// pattern that Go accepts and a browser does not is ignored by the browser without a
// word, so that the field is not checked in the page at all, and one that both accept
// can match different values. So a pattern is written in the syntax that both read
// alike, which has been tried against both, and anything else is an error when
// [App.Compile] runs, with what to write instead (a [PatternError] says what is wrong with the pattern and what to write). A program that gets its patterns from
// somewhere else, such as a schema, asks [ValidatePattern] about each one, and leaves
// out the ones that are refused. The syntax is:
//
//   - A character stands for itself, except the ones with a meaning, \ ^ $ . | ? * + ( ) [ ] { }, which are written with a
//     backslash when they are meant as themselves: \. for a dot, \( for a parenthesis. A slash and a dash are written as they are.
//   - A dot is any character but a line end, which in a browser is \n, \r, U+2028 and U+2029.
//   - \d, \D, \w, \W, \b and \B are ASCII, in both. For the letters of any language, \p{L}.
//   - \s and \S are the white space of a browser, which has more of it than Go has: no-break and other spaces too.
//   - \t, \n, \r, \f, \v, and \xHH, a character by two hexadecimal digits.
//   - \p{L} and \P{L}, a letter and what is not one, and the other general categories by their short names: Lu, Ll,
//     Lt, Lm, Lo, M, Mn, Mc, Me, N, Nd, Nl, No, P, Pc, Pd, Ps, Pe, Pi, Pf, Po, S, Sm, Sc, Sk, So, Z, Zs, Zl, Zp, Cc and Cf. They
//     are the Unicode of each engine, so a character added to Unicode lately may be in one and not yet in the other.
//   - A class, [abc], [^abc] or [a-z], that holds characters, ranges of two characters, and \d, \w, \s and \p{L} and their
//     capitals. In a class a ( ) [ ] { } / | or - that is meant as itself is written with a backslash, and none of
//     & ! # $ % * + , . : ; < = > ? @ ^ ~ is written twice in a row.
//   - A group, (...) or (?:...), alternation, |, repetition, * + ? {n} {n,} {n,m}, and the ? after it for as few as possible.
//   - ^ and $, which are the start and the end of the value, and which a pattern does not need, since it has to match
//     the whole value: ab|cd is the value ab or the value cd.
//
// What is left out, and what to write instead:
//
//   - A lookahead or lookbehind, a backreference, an atomic group: Go does not have them, and what they check for
//     is nearly always a text rule above.
//   - Flags, such as (?i): write [Aa] for a letter of either case. A pattern is matched against the whole value, so
//     (?m) and (?s) have nothing to do.
//   - \A, \z and \Q...\E: write ^ and $, and a backslash before each character that has a meaning.
//   - Named groups, (?P<name>...) and (?<name>...): write (?:...), or (...) for one that is kept.
//   - \pL, which a browser does not read: write \p{L}. A script, such as \p{Greek}, is named differently in each, so
//     write its characters as a class.
//   - \x{41}, \u0041 and octal escapes: write \x41, or the character itself.
//   - A backslash before anything that is not a character with a meaning: write \_ as _ and \- as -.
//   - A { } or ] that stands alone, and {,3}: write \{ \} and \].
//   - [[:alpha:]] and the other POSIX classes: write [a-zA-Z].
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
// A link that is a path, such as [Config](/config) or [a
// device](/device/dev_1?minutes=15), is an address in the application whatever it is
// mounted at: Compile's prefix is put in front of it, and it opens in the same tab. It
// is not checked, since the text is not known until it is loaded, and a path that is no
// page is the application's own not-found page. An address with two slashes or a
// backslash is another site, and is not a link. It needs no script. A fenced code block
// that names a language is coloured by the editor, which the page then loads, as for an
// [Editor] but never with a language service.
//
// # Outcomes
//
// Run returns an [Outcome], built with one of four constructors, and the library
// decides the toast, its colour, and the response:
//
//   - [Success] and [Warning] are accepted: the action was done, and the user is
//     sent back to the page they came from, with a 303 so that a reload does not do
//     it again. A Warning says something the user should be aware of.
//   - [Failure] and [Reject] are not accepted: nothing was done, and the POST is
//     answered with the page, the form drawn again with what the user typed kept, and
//     the status 422. Nothing is kept on the server or in a cookie, and a reload makes
//     the browser ask whether to send the form again, which is harmless since nothing
//     was done. A Failure speaks for the whole form, and a Reject for fields, with
//     [Field].
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
// The search box in the top bar finds the things the pages are for, and nothing else:
// it does not list the app's pages (the sidebar does), and an app in which no page has
// a Search has no search box. A [Page] says how with its Search: it is handed what was
// typed and returns the matches, each a [Hit] holding the page's arguments that lead to
// it and what to say about it. A page for one device finds devices, and a result is a
// link to the page at that device.
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
// The results are ordered by how well they match what was typed, which the library
// judges from each Hit's Title, and its Desc for a little over half as much: a title
// that is what was typed comes first, then one that starts with it as a word, then one
// that starts or ends with it, then one that has it as a word, then one that has it as
// the start of a word, and then one that has it in the middle of a word. Below those
// come results that have its characters in order with others between them, and last the
// ones that match neither Title nor Desc. A query of several words that are all there,
// but not together, ranks below the whole phrase. The best result of a page puts its
// group first, and the best come first in each. A source that returns no more than the
// limit it is given should return the best matches, as [SearchRows] does.
//
// Every page's Search is asked at once, each on a goroutine of its own, and keeps at
// most eight results from each, so the search takes as long as the slowest page and not
// as long as all of them, and a page that has not answered within two seconds is left
// out of the answer, however it was written: a Search should look at its context, and
// one that does not is no longer waited for. The answer is sent whole, in the order the
// results are ranked, and not result by result, so that nothing in it moves while the
// visitor reaches for it. It is HTML. With script the box lists it as it is typed in;
// without, the box is a form with a button, and what it submits is answered with a page
// of results in the shell of the app. The query is cut at 200 characters, and an
// address that two pages find is listed once.
//
// # Looks
//
// [Brand] gives the name and logo. The logo is also scaled into the browser's favicon,
// unless NoFavicon is set. [App.ExternalLinks] are the app's links to places outside
// it, such as signing out or the documentation, behind a button beside Refresh in the
// top bar. An [ExternalLink] is a [Nav] entry with a URL, used as written: a web
// address opens in a tab of its own, and a path on this site in the same one. Its
// Section groups it, and the entries without one come first. [Theme] restyles the four colours that carry meaning (accent, ok, warning and critical), each a [Color], a hex string such as "#1f7a6d" that Compile refuses when it is anything else, and the library derives
// the rest for light and dark, which follow the viewer's system. A Theme may also give Light and Dark, each a [ModeColors], the page's Canvas in that mode, from which the surfaces, borders and
// text of the mode are derived, and colours that win over the shared ones in that mode: a look of its own in each, with the viewer still choosing between the modes. Compile checks that what a Theme
// derives can be read, and refuses one that cannot, naming the field and the pair. A [Nav] Icon is the
// name of a Font Awesome Free solid icon, "house" for fa-house, served by the app
// itself.
//
// There is a sidebar when any page declares an entry in it, with a Nav Label or with
// NavEntries, and none when no page does: an app whose pages are all reached by link has
// the width for its pages, and its top bar begins with the app's mark. The declaration
// decides, and not what a visitor may open of it. The theme, light or dark, is an icon
// button in the top bar of every app.
//
// The sidebar is 240px wide, and a label that is wider is cut short with an ellipsis,
// its whole being its title. With script its edge can be dragged to a width of 180 to
// 480px, no more than half the window, which the browser keeps (in its own storage,
// never sent to the server) and which a double click or Enter on the edge resets; the
// icon rail that a narrow window, or the collapse button, makes has a width of its own.
//
// # Figures
//
// A [Stat] is a leaf that shows a few figures of one model as tiles. A tile's Value is any accessor that is read, so a count, a [Select] drawn as a badge,
// a [Slider] as a bar and a formatted size are each themselves; its Change is a [Delta] and its Trend a [Series], both optional.
//
// A Delta is a signed change, shown with its sign and an arrow, green when the way it moved is the good one and red when it is not: a rise is good news
// by default, and LowerIsBetter says a fall is (a response time). A change that reads zero as it is shown has no direction and no colour. A Series is a list
// of numbers drawn as a small chart where it is read, a line (SeriesLine) or bars (SeriesBars), with the values in words as its name, so that a reader who
// cannot see it has them. Both are accessors, so they work in a [Table] column (a Delta sorts and filters as a number, a Series is neither sorted nor
// filtered by, and a file of a table holds the points separated by semicolons), and in a read-only field of a [Form] or a [Toolbar].
//
// See [ExampleStat].
//
// # Items
//
// A [List] lists items of a model: each a headline over a line of small facts, with a badge beside the headline and a picture when it has them. [Cards] are the same
// items as boxes, in a grid that wraps or, with Layout: CardsScroll, a row that scrolls sideways. They are for what is new, where a [Table] would be a column of cells.
//
// Their source is Load(ctx, limit), a plain list, which is asked for one more than is shown so that the leaf knows there is more. It shows PageSize items, five when that is left zero, and a Show more
// link that shows that many more: a real link to the same page with the leaf's name and ".limit" in the address, so it works without script and a copied link shows the same, and with
// script it replaces the list in place. A headline is an accessor for text. Where it leads is Open, a [Link] to a page of the app (an item whose page refuses the visitor is text), or Link, an
// address that opens in a tab of its own. Meta are the small facts, shown as they are in a cell and separated by a dot, and a fact with no value is left out. Badge is a [Select].
//
// Image is the address of a picture: a path on this site, or an https address from one of TrustedOrigins, which are written as they are for [Markdown] and which the page's content security
// policy then allows for this page and no other. A picture from anywhere else is not drawn. See [ExampleList].
//
// # Calendar
//
// A [Calendar] shows a month as a grid of days, each marked when something is on it, with a link when Open says where a day leads. Load is given a [Span], the days that the grid shows (the month, and the days of its
// first and last weeks that are not of it), and Date says when an item is. A day is a day in UTC. The month is in the address as the leaf's name and ".month" ("releases.month=2026-10"), with real links to the months
// either side and back to the current one, so it works without script and a copied link shows the same month. Open is a [Link] built from a [Day], and only a day with items has it. A week begins on Monday, or
// on Sunday with SundayFirst, and WeekNumbers adds ISO 8601 week numbers, which are of Monday weeks. The day that is now is marked by more than colour, and each day is told in full to a reader who goes by the
// grid. See [ExampleCalendar].
//
// # Refusals
//
// What a visitor may be told is a [Refusal]: [NotFound], [Unauthorized] or [Forbidden],
// with the message to show and, from [Refusal.WithLink], a link on. A page's Guard that
// returns one answers with its status (a 404 for a record that is not there, a 401 for
// a credential that is missing, with a link to set it up), and any other error from a
// Guard is a 403 with its text, as before. A panel's Load that fails with one says it in
// the panel with its link, and the page stays as it is, because a page that has begun
// to be sent has its status already; any other [Failed] from a Load is "Could not load",
// with the cause logged and never shown. A Refusal is not logged: it is not a failure.
//
// # Compile errors
//
// [App.Compile] collects every problem in the declaration and reports them together in
// one error that wraps a [CompileError] for each, naming the page, the argument type,
// what is wrong and how to fix it. The handler it returns is never nil: after a failed
// compile it serves the problems, as a page, at every path under the prefix, so a
// broken app is easy to diagnose in the browser. [App.MustCompile] panics instead, for
// a process that should refuse to start.
//
// # Logging
//
// An app says what went wrong that it could not tell the visitor (a Load that failed, a panic in
// the application's code, a table export that was cut, a Format that panicked) to the
// [log/slog.Logger] it is given: [App.Logger]. The program decides where the lines go and at what
// level, and an app that is given none logs nothing at all. It does not fall back to
// [log/slog.Default], so mounting the library adds no output until the program asks for it.
//
//	handler := webui.App{Logger: slog.Default(), Pages: pages}.MustCompile("/admin")
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
// promhttp where it serves the rest of its metrics, so where they are, and who may read
// them, are the program's decisions. Compiling the app again, as a program does that
// replaces it as it changes, counts into the same metrics; so does another app given
// the same registry, whose counts then add to the first's, unless its registry is
// wrapped with a label of its own.
//
// # Security
//
// The library sets no authentication: mount the handler behind your own. A [Page]'s
// Guard and an [Action]'s Guard decide what a visitor may see and do, and the same
// check that draws or leaves out a control also authorises the request. Navigation entries and
// search results a visitor could not open are left out. POST requests are
// protected against cross-site request forgery (CSRF) with
// [net/http.CrossOriginProtection], responses carry a content security policy, and
// nothing the visitor controls is trusted as a redirect target. The compile-error
// page shows type and field names, so mount an app that may fail to compile behind
// the same authentication.
package webui
