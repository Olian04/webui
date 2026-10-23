# Domain types → design elements

How each declared type becomes something on screen. One rule runs through all
of it:

> **Every visual affordance is the consequence of a declaration, and every
> declaration has exactly one visual consequence.**

If a panel has a checkbox column, it is because the table declares bulk
operations — not because someone chose to put one there. If a column has no
filter icon, it is because it declares nothing to filter by. Nothing is styled
into existence, and nothing is configured twice.

---

## Summary

| Type | Becomes |
|---|---|
| `App` | The shell: sidebar (with script its edge can be dragged to a width of 180 to 480px, which the browser keeps), top bar (breadcrumbs, search, refresh), content, status bar |
| `Brand` | Sidebar brand row (logo mark + name), 48px, aligned with the top bar |
| `Theme` | The four colours an app may restyle (`Accent`, `OK`, `Warning`, `Critical`), emitted as `theme.css` |
| `Page` | One address, one screen |
| `Page.Path` | The address; its segments are the breadcrumb trail. A last segment written `{name...}` takes the rest of the address as a `[]string`, and each of its elements is a crumb that links to the page at that depth |
| `Page.Crumb` | What the page's crumb reads, in the breadcrumb and the tab title, in place of the value that the path has for the segment; it names the page as the parent of one below it too, when the path has the arguments the parent needs. Asked whenever a full page is drawn, within two seconds for all of a page's crumbs, and not for a parent whose `Guard` refuses the visitor |
| `Page` argument struct `A` | The address: path fields are the breadcrumb, query fields have no control |
| `Page.Guard` | The **Not permitted** full-page state; a `Refusal` it returns is the **Not found**, **Not signed in** or **Not permitted** state of its status, with its message and, when it has one, a link on |
| `Nav.Label` | A sidebar entry (a page with no `Label` has none), shown only to a visitor whose page `Guard` lets them in |
| `Page.NavEntries` | An entry for each of a page's arguments, read when a page is drawn, in the place and with the icon of the page's `Nav`; the one whose arguments fit the address is lit |
| `Nav.Section` | The uppercase caption above a group of entries. The sidebar is grouped by it, not by position: entries without one first, then each section in the order it is first named |
| `Page.Search` | The page's things as a group of results in the top bar's search, which lists nothing else and is not drawn at all when no page has a `Search`; empty, it shows nothing, and it says "Searching…" and "Nothing matches." Each `Hit` is the page's arguments that it leads to, a title and a line beneath, grouped under the page's name in the navigation (or the entry it borrows from). `SearchRows` makes a `Search` from rows in memory |
| Search results, ranked | Ranked by how well the `Hit`'s title (and its line beneath, for a little over half as much) matches what was typed, the best group first and the best result first in each; each page keeps its best eight. A result appears only if the page would let the visitor in (its `Guard`, run with the hit's own arguments) |
| Search, with and without script | Every page's search is asked at once and the answer is sent whole, as HTML: with script the box lists it as it is typed; without, the box is a form with a *Search* button joined to its right edge, and its answer is a page of results in the shell |
| `App.ExternalLinks` | The button in the top bar, beside Refresh, and the popover list of the app's links to places outside it behind it. An `ExternalLink` is a `Nav` entry (label, icon, section caption) with a `URL`, used as written (a web address opens in a tab of its own; the list groups by `Section`, not by position: entries without one come first, then each section in the order it is first named), so it is outside the app: signing out, the documentation |
| *(a page with no `Label`)* | Lights its nearest ancestor path's entry while open |
| `Page.Body` → `PageBody` | The arrangement of panels inside the content region |
| `Stack` | Panels in a column, 8px apart |
| `Split` | Two columns, 8px apart. The panels of a row are as tall as the tallest, and the rows are independent |
| `Tabs` / `Tab.Label` | The tab strip; the selection is view state in the address (`tabs.tab`) |
| `Toolbar` | A row of controls with no frame: buttons, links and the values of the page's arguments, read-only, wrapping, with the buttons and the values' bottoms aligned. No inputs and no form of its own beyond the buttons' |
| `Form` | One panel containing a form and a footer with its submit |
| `Table` | One panel containing a table and a pagination footer |
| `FileUpload` | One panel containing a file chooser and a button, in a native `multipart/form-data` form of its own with the leaf first: one file, of any size up to its `MaxSize` (which has no default), handed to `Receive` as it arrives, with nothing held. `FileHandle` also has the folder (`Path`), the name as sent (`RawName`) and the type the browser said (`DeclaredType`). A drop zone with script (the `File` script), a Disabled Guard draws the chooser and button disabled with the reason and any other error leaves the panel out |
| `Editor` | One panel containing a code editor (Monaco), and, when its `Content` has a `Store`, a footer with *Save*; without script a textarea, or the text in a monospace block |
| `Diff` | One panel containing two texts and what changed between them, side by side or stacked; without script a unified diff made by the server |
| `Markdown` | One panel containing a text set as markdown, as an article in the app's own type and colours |
| `Editor.Language`, `Diff.Language` | The highlighting of the text, and for an editor that can be edited the language service that may be loaded for it |
| `Editor.Context` | What an editable editor's language service is given besides the text, as files the page names by address and the browser fetches |
| `Markdown.TrustedOrigins` | The origins a picture in the text is drawn from, and the only ones the page's content security policy lets the browser fetch from; any other picture is a link |
| `Table.Export` | *Export* with two small links, CSV and JSON, in the footer: every row the sort and the filters keep, not the page on screen |
| `Form.Title` / `Table.Title`, `Desc` | The panel header text, and an information icon whose popover shows the description on hover and on focus |
| accessor (`String`, `Int`, `Float`) in a `Form` | A labelled input |
| accessor in a `Table` | A column |
| value kind | The input type and the column's alignment |
| `Format` (on `Int`, `Float`, `OptionalInt`, `OptionalFloat`, `Datetime`, `Timestamp`) | How the value reads where it is only read: a table cell, a field with no `Store`. The column still sorts, filters and exports by the value, and an editable input holds the value |
| `Group` | Its fields side by side in one row. Layout only: no frame, no caption |
| no `Store` | The value shown as output, in no box: the field's usual quiet label over the value at 14.5px, and an em dash when it is empty |
| `Label` | The input label and the column header. What a column's sort link and filter ask `Load` for, and the column's name in the address; every column header is a sort link with a filter beside it, unless the accessor says `NoSort` or `NoFilter` |
| `Select.Options`, `Select.LoadOptions` | The values a select can hold, given or read when a request is answered,, in order, each with what it says and its tone. In a `Rows` table a select column's filter is a multi-select of the values its rows hold; in a `Load` table, of exactly the `Options`, or text when there are none. A numeric column's filter is a minimum and a maximum, and any other column's a text input |
| accessor `Help` | In a form or a toolbar (read-only too), a section of the popover that is beside the label (the information icon, which lists the rules): under the heading "Help:", above "Rules:". The popover is on hover and keyboard focus, and its text is the icon's accessible name. A field with a `Help` and no rules has the icon for the help alone. Not in a table |
| `Placeholder` (on `String`, `Int`, `Float`, their `Optional` forms, `Email`, `Phone`, `Website`) | The input's placeholder |
| `Email`, `Phone`, `Website`, `ColorPicker` | A string of a kind that has an input of its own: `type="email"`, `"tel"`, `"url"` or `"color"`. They have a `String`'s rules and `Placeholder`, except a `ColorPicker`, which has neither, since it is the browser's picker, and an empty one shows black. Read, they are text: `URL` is the link |
| Checks of those four | The browser flags an email or a web address that is not one (`typeMismatch`, in the server's words, through `setCustomValidity`); the server checks all four again: an email by the HTML grammar, a web address as http or https with a host, a phone number as digits and the punctuation of one, a colour as `#rrggbb`. A `Website` says "A web address, http or https" in the rules popover, since the server is stricter than the browser |
| `File` | In a form, a native file chooser (`type="file"`, with `accept` from the kinds, a hint only) with the stored file above it: its name (a link when `Load` gives a URL) and its size, and with `Removable` a *Remove* checkbox. The form is `multipart/form-data` only when it has one. The popover lists what it takes ("A PNG or JPEG file", "Up to 5 MiB"). In a table or a read-only form it is the stored file's name, a link when it has a URL, and a dash for none. It is not a toolbar item |
| `File.Required` | A file that is stored and is not being removed, or a new one; the browser asks for one only when none is stored, and a refusal says "Choose a file." |
| `FileInfo.Thumbnail`, `FileInfo.URL` | The app's small picture of the stored file, 32px beside its name, in the page. With a `URL` too it is a button that opens that file at full size in a native dialog with an invoker command (as `Action.Confirm` does), whose lazy image is fetched only then, with no script; a `Thumbnail` alone is only the picture. Both are paths of this site, and without a `Thumbnail` there is no picture: the file itself is never shown shrunk |
| `webui.Thumbnail` | What an app's `Store` or `Run` makes the `Thumbnail` with, from `Upload.Data`: a PNG of the uploaded image fitted inside a square, with its own proportions, never enlarged, from PNG, JPEG or GIF (no WebP), and not from a picture of more than 25 million pixels |
| `File` with script | A dashed frame around the chooser that a file can be dropped on (lit while one is over it), "or drop a file here" beneath it, and a *Clear* button for a file that is chosen and not sent. A chosen image (PNG, JPEG, GIF or WebP, up to 4 MiB) is shown below as a small picture, scaled down in the page to 192px, only when the field takes images, and opens as it was chosen at full size in a dialog when it is clicked. Still one file to a field: a drop of several takes the first |
| `File`, checked early | A file that is too large or is not a kind that the field takes is refused at once, with the server's words, by the browser's own validation message, and does not submit; a file whose type the browser does not give is not |
| `Phone` with script | A segment, a selector of the country calling code (`+46 Sweden`), joined to the input that holds the national number, which is spaced as it is typed (`70 123 45 67`, by the country's grouping where one is known, else in threes) and has its trunk `0` dropped. The selector starts at `DefaultCountry`, else the country of the browser's language, else empty; pasting `+46 70 …` picks the country by its code. A number that is there and is not international (or has an extension) is left as the one plain input it was. Without script: the one `type="tel"` input, as typed |
| `Phone`, as sent | A hidden input with the field's name carries E.164, `+46701234567`, and the server stores a number with a code that way, with no spaces (without a code, as typed). The input has the length and pattern rules of the field removed: the server holds the number that is sent to them |
| `Datetime.DateOnly`, `Timestamp.DateOnly` | A date: `type="date"` with no `step`, "Date in UTC" in the rules popover, shown in a table and a form as "2026-10-09", and a table's filter bounds are date inputs. The stored value is midnight UTC (Unix seconds, or the date alone for a `Datetime`) |
| `Rules` | HTML validation attributes on the input, and the rules listed in the popover beside its label |
| `Select` | A choice from a list of options. Read, in a status column or beside its label in a form, it is a badge for each value, in its option's tone. With a `Store` it is a drop-down list (a `<select>`, so it needs no script), or with `Multi` a list to choose several from; with script, a field of badges and a popover list of the options (see design.md); `Required` refuses nothing chosen, and a value that is not one of the options is refused on the server |
| `URL` | A link, with `Text` for its words (the same on every row), only because the page declares the column a `URL`: no other accessor makes a link of its value. It opens in a new tab with `rel="noopener noreferrer"`, carries an arrow, and tells screen readers so. Only a site path or an http, https or mailto address is one; anything else is plain text. Read-only; not the first column of a clickable table |
| `Datetime`, `Timestamp` | A moment, shown in UTC as "2026-10-09 11:27" in a `<time>` element. A table sorts it as a moment, and its filter is a start and an end, in UTC, as date and time inputs. With a `Store` a form shows the native date and time picker (`datetime-local`, to the second, in UTC) and says "Date and time in UTC" in the field's rules popover |
| `Slider` | An inline bar in a table, and in a form that is only read; with a `Store`, see the next row |
| `Slider` with a `Store` | A number input that is the control (it is submitted, has the bounds and the `step`, and works without script) with a range beside it, for script, to drag. `StepSize` is the distance between the values, from `Min`, or a power of ten that gives a hundred steps or more over the range; the value is shown with its decimals. The server holds a value to the step too, in the words the browser uses (`step="any"` submitted whatever the pointer was over, such as 81.6719540426991) |
| `OptionalInt`, `OptionalFloat`, `OptionalSlider` | The number, slider and bar of the same names, for a value that may be absent (`Load` returns a pointer). Absent is an empty input with the field's placeholder, and a dash where it is only read, in a table and in a read-only form; an empty slider is dimmed, since its thumb must rest somewhere. Emptying the input stores `nil`, and `Required` refuses it. In a table it sorts first and is in no range; the CSV cell is empty and the JSON is `null` |
| `Bool` | Yes, no, or not set: a drop-down of Yes and No with an empty option (a `<select>`, so it needs no script; a checkbox cannot be neither, and an unchecked one sends nothing), and a neutral Yes or No badge where it is read, with a dash for neither. Its table filter is a choice among them, and the JSON is `true`, `false` or `null` |
| `Table.Rows`, `Table.Load` | Where the rows come from. `Rows` lists them all and the library filters, sorts and pages them by the columns; `Load` is handed the window, sort and filters by a source that pages itself |
| `Table.Feed` | A source that pages by cursor, in its own order, which a cursor continues and the library cannot filter: its columns have no filter and sort only the page on screen, and its footer is "Showing rows 10032–10054" with *First page*, *Previous* and *Next* |
| `Table.RowClick` | Rows become clickable, as a link or as an action, and take the pointer + hover treatment |
| `Table.Key` | What names a row to a request: required with `Actions` or `BulkActions`, since a button posts the key of its row and never its position |
| `Table.BulkActions` | The checkbox column, and the selection action bar once a row is selected. Each is drawn if its `Guard` accepts a selection of any one row of the window, and with none drawn there is no column and no bar |
| `Table.Actions` | A button per row in a trailing cell, left out for a row whose `Guard` returns an error and disabled, with its message, for one that returns `Disabled`; the cell is empty for a row with none, and the column is there when some row of the window has a button |
| `Table.PageSize`, and `Table.Title` in the address | The pager, 25 rows a page unless said; the title, as the address spells a label, names the table's sort and offset in the address |
| `Form.Submit` | The primary button in the panel footer |
| `Action` | A button; its `Role` decides primary / secondary / destructive |
| `Action.Guard`, `Disabled` | What the `Guard` returns decides how the control is drawn, and refuses the request in either case. An error leaves the button out; `Disabled(message)` draws it at 45% with the message in a raised tooltip (the *gated* treatment). In a table's rows, its selection bar (a bulk `Guard` is asked of each row as a selection of one) and a toolbar. A form's Save is not drawn for either: the form is read-only, with *Back*. A `Guard` that panics leaves the control out and is logged |
| `Action.Confirm` | A question asked before the action is done: the button opens a native `<dialog>` (an invoker command, `command="show-modal"`, so no script is needed in a current browser) with the question, **Cancel** and a button that is the action's own, with its `name` and `value`, inside the same form, so the request is the one the button would have made. Backdrop click and Escape close it (`closedby="any"`); Cancel has focus. In a table, one dialog for each row and action, by position and never by key |
| `Action.Confirm`, and a refusal | A button refused by its `Guard` has no dialog, whether it is left out or disabled. A form's Save that asks is not submitted by Enter in a field (a disabled button ahead of the dialog's is the form's default button). Not a check: a request made by hand is not asked |
| `Success`, `Warning` | A toast, bottom-right (carried across the redirect by a flash cookie), green or amber; the action was accepted |
| `Failure` | A red toast over the form shown again, with what was typed kept |
| `Reject`, `Field` | Invalid borders and per-field messages, with typed values preserved |
| `Outcome.Then` | A navigation after an accepted action |
| leaf address (`p.0.1`) | Which panel a refresh replaces |
| `CompileError` | The **Failed to compile** page, served at every address |

---

## The shell

### `App` and `Brand`

`App` is the whole screen. Its `Pages` populate the sidebar; the compiled app's lookup of pages by path is what makes a breadcrumb parent clickable and
what lets one page link to another without either knowing the other's address.

The browser tab's icon is generated: `Brand.Logo` is scaled to 32 px (the tab)
and 180 px (a phone's home screen), fitted inside a square and never stretched,
and served by the app, with no file to produce or host. An app with no `Logo`
gets the library's own mark as an SVG. `Brand.NoFavicon` turns it all off, for an
app that declares its own.

`Brand.Name` and `Brand.Logo` fill the 48px sidebar brand row. That row is
exactly as tall as the top bar so the two horizontal rules line up across the
whole window — the single most noticeable alignment in the layout.

### `Theme`

`Theme` is the surface between the design and the application, and it is a
struct of static fields, one for each thing an app may restyle: `Accent`
(interactive: primary buttons, links, the active entry, focus), and `OK`,
`Warning` and `Critical` (the semantics a badge or a message carries). Each is a
hex colour, checked at `Compile`.

The design underneath is still tokens, and nothing in any component reads a
literal value, but the app does not name them. It names a colour, and the
renderer derives what the design needs from it, per mode, with `color-mix`: for
the accent a hover (deeper, so a white label keeps its contrast) and a readable
text tint, for a status colour its text tint and the faint background and border of
a badge, and for `Critical` also the solid fill of a destructive button. Dark lightens and light deepens,
so one colour is right in both.

Surfaces and text are not themeable on purpose. They are what makes light and
dark differ, and light or dark is the viewer's choice, not the app's; a single
colour cannot be right in both. A field left empty keeps the design's colour, so
a zero `Theme` serves no `theme.css` at all.

### `Page.Path`

One page is one screen and one address. The path (a `PageID`) is both:

- the address the user sees and shares, and
- the breadcrumb trail, derived from its segments.

Because pages are identified by template rather than by a route table, a
breadcrumb parent and a cross-page link resolve to the same thing, and the
compile step can prove a link's destination exists.

---

## Arguments

### The argument struct

A page's argument struct is what its address means. A **path** argument is
identity, so it appears in the **breadcrumb** (`Collector › Devices ›
dev_27c38b`). A **query** argument has no control of its own: it arrives in the
address — from a link, from `Open` — and the page's loaders read it. What a
user filters by hand, they filter in a table's column headers, which is
library-owned view state and not a page argument.

The consequences:

- The state of the screen is in the address bar, so it survives reload, back,
  bookmarking and sharing.
- There is no row of page parameters to design per page.
- The page's argument struct stays what it says: the things the page is *about*.

### `Decode`

Invisible, but it is the reason an address can be trusted: the values in the
address are the values the page will receive: the argument codec is built once at compile time, and it decodes the address on every request, so what
the page is handed is what it declared.

---

## Navigation

### `Nav.Label` → a sidebar entry

Straightforward: one page, one 32px entry, with the active treatment (Active
background, full-strength text, 2px primary bar at the left edge) when it is
the current page.

`Nav.Icon` is the name of a Font Awesome Free solid icon, `"house"` for
`fa-house`, drawn at the left of the entry. The renderer uses it as a class
(`<i class="fa-solid fa-house">`) and serves the icon font itself, so the
content security policy stays `'self'` and nothing is fetched from a CDN. A
name that is not in the free solid set (or is written `fa-house`) is a compile
error, so a typo cannot draw nothing. Every icon the library draws for itself
(info, alert, filter, sort carets, refresh) is a Font Awesome icon too. The sidebar can be collapsed to a 56px rail of icons, by the
button in its footer (the choice is a browser preference, so it stays out of
the address) and always below 820px. An entry with no icon holds the place of one
in the full sidebar, so labels line up, and shows its label's first letter,
capitalised, in the rail.

### The landing page

A page declared at `Path: "/"` is the landing page. The brand in the sidebar
(logo and name) and the first breadcrumb link to it. It usually has no `Nav`
label of its own, since the brand is its entry. An app with no such page links the
brand to what the root would send the visitor to: the first entry in the navigation that
they may open, or failing that the first page whose path has no arguments and whose guard
lets them in. The link is worked out for each visitor, so it never points at a page they
cannot open. The root itself redirects there without a message; a toast about the missing
page was noise to the visitors it was shown to, who cannot do anything about it. With
nowhere to go, the brand is plain text and the root says so.

### A page with arguments → an entry for each address

An entry is a link, and a link has one address. A page at `/site/{name}` has none until a
name is given, so on its own it has no entry (a `Nav.Label` on it is a compile error). With
`NavEntries` it has one for each address the function returns, each a `NavEntry` of the page's
arguments and what it says, in the place the page has in the sidebar and under the icon and
section of its `Nav`. The entry of the address on screen is lit, and so is it for a page
below it that borrows its place.

### A page with no entry → the ancestor's highlight

A detail page has no business adding a permanent sidebar entry, but it must not
leave the sidebar looking like nothing is selected. Nothing is declared for
this: a page with no `Label` lights the entry of its nearest ancestor path, the
longest proper prefix of its own that is a page with a `Label` (the root is never
one). `/device/{id}` lights *Devices*, the page at `/device`. Visually: the
sidebar stays on *Devices* while you are on a device, and the **breadcrumb**
carries the fact that you have gone a level deeper. It is the same derivation
the breadcrumb makes from the path.

This is why the breadcrumb lives in the top bar rather than in the content — it
is the sidebar's partner, not the page's heading.

### A page with no entry and no ancestor with one

The page is reachable only by link, and nothing in the sidebar lights. There is
nothing to declare for it: it is a page with no `Label`.

---

## Body: layouts

### The body tree

The body tree is the panel arrangement. A leaf's address — its path of child
indices — is what a refresh names. Practically, this is the difference between
"the screen reloaded" and "one panel reloaded": with JavaScript, a control
belonging to a leaf re-renders the panel at that address and the rest of the
screen never moves; without it, the same link reloads the page. The skeleton
state in §10 of the design document exists precisely to make that address
visible while it is happening.

### `Stack`

A column of panels, 8px apart.

### `Split`

Two columns. The panels of a row are as tall as the tallest of them, so their borders
line up, and a panel's footer (a pager, a *Save*) stays at the bottom of its box. The rows
are independent: five panels in a `Split` are three rows, each as tall as its tallest
panel and not as the rows around it, and a row with one panel leaves that panel as tall as
it is. Below 1100px it is one column and nothing is stretched. A `Split` constrains
nothing about what its children are *about* — a form for one model beside a table of
another is the ordinary case, and the design has to hold two panels of different content
next to each other without implying a relationship between them. Identical panel chrome
and a shared height do that work; the height says only *these are side by side*. Only
panels that are direct children of the `Split` are stretched: a layout inside it, such as
`Tabs`, is one cell, and its panel keeps its own height.

### `Tabs` and `Tab.Label`

The tab strip. The crucial design consequence is that the selection is address
state, not component state: it is in the address bar (`tabs.tab=raw`), survives
a reload, and can be linked to. It is view state the library keeps, not a page
argument, so it has no other control — the strip itself is the control.
A tab is navigation that happens to look like a tab.

---

## Body: leaves

### `Form` → a panel

One panel. Header carries the title and description; the body carries the
fields; the footer carries `Submit` as the primary button, right-aligned, with
a ghost *Cancel* beside it. Cancel returns to the page the user came from, which the library remembers
(`webui.from` in the address, set by the link that opened the form), and to the
parent in the breadcrumb when there is none. When the `Submit` action's `Guard` refuses the
visitor there is nothing to save or cancel, so the footer holds a ghost *Back* to the same
address instead, and no footer at all when there is nowhere to go back to. Every field is then
read-only, as are the fields of a form with no `Submit`: a field is editable only where it has a
`Store` and the visitor may submit.

`Bind` is invisible until something fails — see `Reject` below.

### `Table` → a panel

One panel. Header carries the title and description; the body carries the
table; the footer carries the range (`1–10 of 37`) and the pager, whose
buttons are links carrying an offset argument.

A `Load` that returns a `Total` smaller than what it has shown (a `Rows` table always
knows its total) means the total is unknown, which is a real design state: the footer
shows the range without a total and the *Next* button cannot be pre-disabled.

### `Editor` → a panel with a code editor

One panel. Header carries the title and description. The body is a code editor with the
`Language`'s highlighting, line numbers and search; with a `Store` on `Content` the editor edits
the text and the footer carries *Save*, as a `Form`'s does, and without one it is a viewer and
has no footer. The text is saved through the same pipeline as a form: parse, `Rules`, `Store`,
`Guard`, `Run`. `Content`'s `MaxLen` is the bound on the text, and a refused text is shown back as it
was typed, with the reason in a strip under the editor.

Everything works without script: an editable editor is a textarea and a viewer is the text in a monospace
block, and Monaco, which the page then loads, mounts over them. A page with an editor
links its script and stylesheet in the head.

A language service (diagnostics, completion) is an optional import for a few languages, one worker each, and goes only to
an editor that can be edited by a visitor who may edit it: a viewer is only ever highlighted. `Context` is what
the service is given besides the text, a folder of declarations and schemas, which the page does not hold: it names the
files by an address that carries a key that only such a page gives out (`App.Secret`), so a browser keeps them.

### `Diff` → a panel with a diff

One panel. Header as above; the body is `Original` and `Modified` with what changed between them,
side by side when its panel is wide (900px) and in one column when it is not, with a header that names each side by
its accessor's `Label` (which must differ): over each side, or as one line, *Original → Modified*, when there are no
sides to name. A `Diff` only reads, so there is no footer and an accessor with a
`Store` is a compile error; it never loads a language service, so only highlighting is shipped for it.
Without script the server makes a unified diff, bounded: two texts too large or too different to compare
there say so, and two that are the same say *No differences*.

### `Markdown` → a panel with an article

One panel. Header as above; the body is one text of the model, set as CommonMark with GitHub's tables,
task lists, strikethrough and bare links, in the app's own type and colours (see
the design's *Text*). It only reads. The text is trusted no further than any other text of the model: raw
HTML is never rendered, and a link goes only to an http or https address, a mailto or a path in the
application. A path is an address in the application whatever it is mounted at, so it is given the prefix and
opens in the same tab; any other link opens in a tab of its own. A picture is a link to it, named by its alt text,
unless its origin is one of `TrustedOrigins`, which the page's content security policy then allows for this
page and no other. It needs no script; a fenced block that names a language is coloured by the editor, which the page
then loads, and is plain text in a monospace block without script.

### `Table.Export` → two download links

A table does not export unless it says so, since an export is a way to take every row at once. Where it does, the
footer carries a quiet *Export* and two small links, *CSV* and *JSON*, beside the range. The file is every row that the
table's sort and filters keep (a `Load` table is asked for them a batch at a time, a `Feed` is followed from its first
page to its last, and a file is cut at 100,000 rows, which the response says in a header), the columns the table shows
with their labels as the header, and no row actions or links. It runs the page's `Guard`, as any request for the page
does. A CSV cell is the text the table shows, except that a number is a number and a text that a spreadsheet would run
as a formula is quoted; JSON is a list of objects keyed by label, with numbers, moments (RFC 3339, UTC) and selects
as their own kinds. The links carry `download`, so the page's script leaves them to the browser.

---

## Accessors — one type, two renderings

An accessor (`String`, `Email`, `Phone`, `Website`, `ColorPicker`, `File`, `Int`, `Float`, `Select`, `URL`, `Datetime`, `Timestamp`, `Slider`, `OptionalInt`, `OptionalFloat`, `OptionalSlider`, `Bool`) appears in a `Form` and in a `Table`
and renders differently in each, from the same declaration:

| | In a `Form` | In a `Table` |
|---|---|---|
| `Label` | Field label, 12/400, text-secondary | Column header, 12.5/500 |
| value kind | Input type | Cell alignment and formatting (numeric → right, tabular figures) |
| `Load` | The value shown | The cell value |
| `Store` absent | **Read-only** — shown as output, not as a disabled input | (no effect; a column is always read-only) |
| `Rules` | Validation attributes + the rules popover | Ignored |
| `NoSort`, `NoFilter` | Ignored | Leave the header's sort link, and its filter, off |
| `Help` | A section of the popover beside the label | Not shown |
| `Format` | Where only read: how the value reads | How the cell reads; the column still sorts, filters and exports by the value |
| `Group` | Fields side by side, with no frame | Ignored |
| `Placeholder` | The placeholder | Ignored |

The "ignored" cells matter as much as the others. A field declared once and
used as both a form input and a table column is the normal case; each position
takes what it can use and silently drops the rest. That is what makes the field
reusable rather than forcing two near-identical declarations.

### `Rules` → validation, twice, from one declaration

Each rule has exactly one visual form:

| Rule | Input attribute | Shown as |
|---|---|---|
| Required | `required` | `*` beside the label, in critical |
| `MinLen` / `MaxLen` | `minlength` / `maxlength` | "3–32 characters", in the popover |
| `Min` / `Max` (`NumberRules`) | `min` / `max` | "1–65535", in the popover |
| `Pattern` | `pattern` | The pattern's message, in the popover and beneath a refused value |
| `String.Secret` | `type="password"`, `autocomplete="new-password"` | "Leave empty to keep the current value", in the popover; the stored value is never in the page, and a row of bullets (the input's placeholder, always eight) says that there is one |
| `Select` with `Multi` | `multiple` | "Hold Ctrl or ⌘ to choose more than one", in the popover |
| `Slider` with a `StepSize` | `step` | "In steps of n", in the popover |
| `File` | `accept`, `required` | What it takes ("A PNG or JPEG file") and "Up to 5 MiB", in the popover; "Choose the file again" in a form that was refused, which a browser cannot refill |
| `String.Multiline` | `<textarea>` with `required`, `minlength`, `maxlength` (no `pattern`) | Five rows, as tall as the visitor drags it; a cell shows the first three lines |
| `StartsWith`, `EndsWith`, `Contains` and their `Not` forms | `pattern`, with the field's own | What each requires ("Starts with "https://""), in the popover, and as its message beneath a refused value |

Nothing is printed beneath a control while its value is good. Beside the label of an
editable field with any rule other than *Required* there is an information icon whose
popover lists the rules: the length, the bounds, and the pattern's message. It opens on
hover and on focus, like a panel's description, so it works without a pointer or script.

The browser refuses before anything is sent; the server re-runs the same set
after. The user sees one behaviour. `Pattern` carries a message because a
regular expression cannot explain itself — that message is the error shown when
a value is refused, and the reason `Pattern` is a struct while `MinLen` is a bare
integer.

---

## Actions and effects

### `Action` → a button

`Form.Submit` renders as the primary button in the panel footer.
`Table.BulkActions` render in the selection action bar. Destructive actions take the
critical role.

### `Toolbar` → controls for the page

A button is the same button as a form's, in the same three roles, small, left out or disabled with its reason as its `Guard`
says. A `Link` in a toolbar is the same button as an anchor, left out when the page it leads to refuses the visitor. A toolbar with
nothing left to draw is not drawn. An accessor is a value of the page's arguments, read: its label
(dim, with a colon) and the value as a read-only form field shows it (badges, a number with its `Format`, a link, a dash for nothing), with
its `Help` in the popover beside the label. A `Store` on it does nothing. There are no inputs: nothing in a toolbar changes the page's
arguments, so there is no form for them, no *Apply* button and no address that applies them; the buttons post in one form, and a toolbar
of links and values has no form at all. `Left` and `Right` are one row with a gap between the groups that takes what is left
over; when there is not room for every control in a row, which script measures, the gap goes and the controls flow from the left in
rows, as one group.

### `Table.BulkActions` → checkboxes that only exist when declared

Declaring bulk operations adds the 34px checkbox column and, once anything is
selected, the action bar: a 10% primary strip carrying the count, the
operations. A table that declares none has no column and no bar —
not a disabled one.

Selection itself is deliberately **not** an argument: it never enters the
address bar, because it is request state, not page state. A shared link does
not carry someone else's selection.

### `Table.RowClick` / `Link` / `Action`

Rows become clickable: pointer cursor, hover surface, and the whole row is the
target. A `Link` is the simple case. Because it names a page and builds its
arguments, the destination is known at compile time, which is what lets a row
link be a real `href` instead of a click handler, so middle-click and
open-in-new-tab work, and the whole design works with
JavaScript switched off.

An `Action` is for what a link cannot say: where to go depends on the row, or the
click changes something. The row is one button, stretched over the cell as the
anchor is, inside the table's form, so it too works without script. It is a POST,
so it cannot be opened in a new tab; it needs the table's `Key`; and a row its
`Guard` refuses is shown but does nothing. A `Link` to a page that refuses the visitor is a plain row, which asks that
page's `Guard` once for each row.

### `Guard` → gating, in two places from one declaration

`Guard` has two visual forms and they are the same rule:

1. **On the page** — the **Not permitted** state, with the reason the `Guard`
   gave, which is for the visitor to read. The page shows nothing, because
   nothing was fetched, and does not say how the check was made.
2. **On a control** — the control is left out, or the *gated* treatment: 45%
   opacity plus a tooltip giving the reason, as the `Guard` returns an ordinary
   error or `Disabled`. The same check that would reject the request decides
   the control, so there is no second authorisation rule to keep in sync and no
   control that looks available and then fails. A form's *Submit* has no
   disabled form: refused, it is not drawn, because a form that cannot be saved
   is shown read-only (see `Form`).

### `Outcome`

An action says how it ended and the library draws it. Accepted outcomes redirect
(a 303, so a reload does not repeat the POST) and carry their toast across the
redirect; refused ones are answered with the page, a 422 whose form is drawn again with
what the user typed, since a refusal did nothing and a reload may send it again.

| Constructor | Accepted? | Visual |
|---|---|---|
| `Success(msg)` | yes | A toast with a green left border, 3.4s; none when `msg` is empty |
| `Warning(msg)` | yes | The same with an amber border: it was done, and the user should be aware of `msg` |
| `Failure(msg)` | no | The form shown again with what was typed, and a toast with a red border saying `msg` |
| `Reject(Field(...)...)` | no | The form shown again with what was typed, an invalid border + ring on each named field and its message beneath, and a "Not saved" toast |
| `.Back()` | any | Back to the page the user came from, even for an outcome that would have shown the form again; what was typed is not kept, and the message is the toast there |
| `.Stay()` | any | Stays where it is, even for a success that would have returned to where the form was opened from |
| `.Then(target)` | any | Navigation, whatever the outcome: the user goes there instead of what it would have done, with its message as a toast there. A refused outcome that is told where to go does not show the form again, so what was typed is not kept |

`Reject` is the design's most important rejection path: a save that was
understood, refused, and is fixable. It looks identical to a browser-caught rule
failure, which is correct — from the user's side it is the same event. The only
difference is that it could not have been caught earlier, because it needed the
service. `Failure` is the same event for a reason that belongs to no one field.
There is no default toast: an action that says nothing shows nothing.

### `CompileError` → the compile-failure page

Served at every address under the mount until the application compiles. Because
the errors are structured rather than scraped from a compiler, every problem is
shown at once, each with its location, its message, and a plain-language
**Fix:** line. Nothing about the design encourages reporting them one at a
time.

---

## What the design needed that the types did not have

Four things in the original design mock were not expressible. Three are now closed;
one is deferred on purpose.

1. **Panel titles and descriptions — closed.** Every panel (`Form`, `Table`,
   `Editor`, `Diff`, `Markdown`) carries `Title` and `Desc`. A panel's title is part of what the panel *is* — the name of the
   thing being shown — and the alternative was that the runtime invents one.
2. **Collapsible rows — deferred.** `Stack` is a slice type, so it cannot carry
   a `Label` without becoming a struct. A long page is scannable with `Tabs`
   until that is wanted.
3. **Sidebar grouping — closed, flat.** `Nav.Section` is a caption above a group
   of entries, named by each entry that belongs to it. Nesting (`Nav.Parent`) would need the compile step to validate a
   tree and is not worth it until an app exceeds a screenful of entries.
4. **Badges and inline bars — closed.** `Select` and `Slider` are accessors
   alongside `String`, `Int` and `Float`, not a display mode bolted on top: a
   status is a select, drawn as a badge, and a quantity is a slider — read-only it is the progress
   bar, with a `Store` it is the control. They are the pressure-release valve
   for visual richness in a table, so a table and a form do not grow a display mode
   of their own. Monospace identifiers are not covered: no accessor says "show this in
   the monospace face".

---

## What the design deliberately has no type for

Three absences that are choices, not gaps:

- **No time-range picker and no auto-refresh interval.** Both are dashboard
  concepts. A control plane refreshes a leaf on demand; it does not poll. If
  this ever changes it should change as a declaration on a leaf, not as global
  chrome.
- **No chart or statistic leaf.** A leaf shows a model (`Form`, `Editor`, `Diff`,
  `Markdown`: one text of it) or a list of models (`Table`), and each of the later
  ones was a kind of text a model has, which no accessor could draw. A chart is
  not a kind of text, and would be the first feature bolted on rather than
  generated by the existing ideas. `Select` and
  `Slider` exist so that the pressure for one has somewhere cheaper to go.
- **No per-panel menu.** A panel's header carries a title, a description and a
  status. Reloading is the page's Refresh, so a panel is replaced only by
  following its own sort, pager or row links, which are the only per-panel
  operations that exist. A panel-level menu would mean inventing a concept the
  body tree does not have.
