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
| `App` | The shell: sidebar, top bar (breadcrumbs, search, refresh), content, status bar |
| `Brand` | Sidebar brand row (logo mark + name), 48px, aligned with the top bar |
| `Theme` | The four colours an app may restyle (`Accent`, `OK`, `Warning`, `Critical`), emitted as `theme.css` |
| `Page` | One address, one screen |
| `Page.Path` | The address; its segments are the breadcrumb trail |
| `Page` argument struct `A` | The address: path fields are the breadcrumb, query fields have no control |
| `Page.Guard` | The **Not permitted** full-page state |
| `Nav.Label` | A sidebar entry (a page with no `Label` has none), shown only to a visitor whose page `Guard` lets them in |
| `Nav.Section` | The uppercase caption above a group of entries. The sidebar is grouped by it, not by position: entries without one first, then each section in the order it is first named |
| `Table.Search` | The table's rows as a group of results in the top bar's search, beneath the app's pages: the first column is the title, the other text columns the line beneath, and the result leads where `RowClick` does. A result appears only if the page it leads to would let the visitor in (its `Guard`) |
| `App.Menu` | The button in the top bar, beside Refresh, and the popover list of the app's own links behind it. A `MenuItem` is a `Nav` entry (label, icon, section caption) with an `ExternalURL`, used as written (the menu groups by `Section`, not by position: entries without one come first, then each section in the order it is first named), so it is outside the app: signing out, the documentation |
| *(a page with no `Label`)* | Lights its nearest ancestor path's entry while open |
| `Page.Body` → `PageBody` | The arrangement of panels inside the content region |
| `Stack` | Panels in a column, 8px apart |
| `Split` | Two columns, top-aligned, 8px apart |
| `Tabs` / `Tab.Label` | The tab strip; the selection is view state in the address (`tabs.tab`) |
| `Form` | One panel containing a form and a footer with its submit |
| `Table` | One panel containing a table and a pagination footer |
| `Form.Title` / `Table.Title`, `Desc` | The panel header text, and an information icon whose popover shows the description on hover and on focus |
| accessor (`String`, `Int`, `Float`) in a `Form` | A labelled input |
| accessor in a `Table` | A column |
| `Label` | The input label / column header |
| value kind | The input type and the column's alignment |
| `Group` | Its fields side by side in one row. Layout only: no frame, no caption |
| no `Store` | The value shown as output, in no box: the field's usual quiet label over the value at 14.5px, and an em dash when it is empty |
| accessor `Label` | What a column's sort link and filter ask `Load` for, and the column's name in the address; every column header is a sort link with a filter beside it |
| `Badge.Kinds` | The tone of each known value. In a `Rows` table a badge column's filter is a multi-select of the values its rows hold; in a `Load` table, of exactly the `Kinds`, or text when there are none. A numeric column's filter is a minimum and a maximum, and any other column's a text input |
| `Placeholder` (on `String`, `Int`, `Float`) | The input's placeholder |
| `Rules` | HTML validation attributes on the input, and the hint beneath it |
| `Badge` | A badge in a status column, or beside its label in a form; always read-only |
| `URL` | A link, only because the page declares the column a `URL`: no other accessor makes a link of its value. It opens in a new tab with `rel="noopener noreferrer"`, carries an arrow, and tells screen readers so. Only a site path or an http, https or mailto address is one; anything else is plain text. Read-only; not the first column of a clickable table |
| `Datetime`, `Timestamp` | A moment, shown in UTC as "2026-10-09 11:27" in a `<time>` element. A table sorts it as a moment, and its filter is a start and an end, in UTC, as date and time inputs. With a `Store` a form shows the native date and time picker (`datetime-local`, to the second, in UTC) and says "Date and time in UTC" in the field's rules popover |
| `Slider` | An inline bar in a table; in a form a bar, or a range input when it has a `Store` |
| `Table.Rows`, `Table.Load` or `Table.Feed` | Where the rows come from. `Rows` lists them all and the library filters, sorts and pages them by the columns; `Load` is handed the window, sort and filters by a source that pages itself; `Feed` is handed a cursor by a source that pages by cursor, in its own order, which a cursor continues and the library cannot reorder or filter, so its columns are plain headers and its footer is "Showing rows 10032–10054" with *First page*, *Previous* and *Next* |
| `Table.RowClick` | Rows become clickable, as a link or as an action, and take the pointer + hover treatment |
| `Table.BulkActions` | The checkbox column, and the selection action bar once a row is selected |
| `Table.Actions` | A button per row in a trailing cell |
| `Table.PageSize`, `Table.Title` | The pager, 25 rows a page unless said; the title, as the address spells a label, names the table's sort and offset in the address |
| `Form.Submit` | The primary button in the panel footer |
| `Action` | A button; its `Role` decides primary / secondary / destructive |
| `Success`, `Warning` | A toast, bottom-right (carried across the redirect by a flash cookie), green or amber; the action was accepted |
| `Failure` | A red toast over the form shown again, with what was typed kept |
| `Reject`, `Field` | Invalid borders and per-field messages, with typed values preserved |
| `Outcome.Then` | A navigation after an accepted action |
| leaf address (`p.0.1`) | Which panel a refresh replaces |
| `CompileError` | The **Failed to compile** page, served at every address |

---

## The shell

### `App` and `Brand`

`App` is the whole screen. Its `Pages` populate the sidebar and the command
palette; `ByPath` is what makes a breadcrumb parent clickable and what lets one
page link to another without either knowing the other's address.

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

### `Page` and `PathTemplate`

One page is one screen and one address. `PathTemplate` is both:

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
address are the values the page will receive, parsed once at compile time rather
than re-derived per request.

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
label of its own, since the brand is its entry. An app with no such page sends
the root to the first entry in the navigation.

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

### `Node` and `Addr`

The body tree is the panel arrangement. `Addr` — the node's path of child
indices — is what a refresh names. Practically, this is the difference between
"the screen reloaded" and "one panel reloaded": with JavaScript, a control
belonging to a leaf re-renders the panel at that address and the rest of the
screen never moves; without it, the same link reloads the page. The skeleton
state in §10 of the design document exists precisely to make that address
visible while it is happening.

### `Stack`

A column of panels, 8px apart.

### `Split`

Two columns, top-aligned so unequal heights do not stretch. A `Split`
constrains nothing about what its children are *about* — a form for one model
beside a table of another is the ordinary case, and the design has to hold two
panels of different heights and different content next to each other without
implying a relationship between them. Top alignment and identical panel chrome
do that work.

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

---

## Accessors — one type, two renderings

An accessor (`String`, `Int`, `Float`, `Badge`, `URL`, `Datetime`, `Timestamp`, `Slider`) appears in both leaves and
renders differently in each, from the same declaration:

| | In a `Form` | In a `Table` |
|---|---|---|
| `Label` | Field label, 12.5/500 | Column header, 12.5/500 |
| value kind | Input type | Cell alignment and formatting (numeric → right, tabular figures) |
| `Load` | The value shown | The cell value |
| `Store` absent | **Read-only** — shown as output, not as a disabled input | (no effect; a column is always read-only) |
| `Rules` | Validation attributes + the hint line | Ignored |
| `Label` | (the label) | Header is a sort link and has a filter beside it; the sorted one takes the link colour and a caret |
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

### `Table.BulkActions` → checkboxes that only exist when declared

Declaring bulk operations adds the 34px checkbox column and, once anything is
selected, the action bar: a 10% primary strip carrying the count, the
operations, and *Clear*. A table that declares none has no column and no bar —
not a disabled one.

Selection itself is deliberately **not** an argument: it never enters the
address bar, because it is request state, not page state. A shared link does
not carry someone else's selection.

### `Table.RowClick` / `Link` / `Action`

Rows become clickable: pointer cursor, hover surface, and the whole row is the
target. A `Link` is the simple case. Because it names a page and builds its
arguments, the destination is known at compile time, which is what lets a row
link be a real `href` instead of a click handler, so middle-click and
open-in-new-tab work, the table can be searched, and the whole design works with
JavaScript switched off.

An `Action` is for what a link cannot say: where to go depends on the row, or the
click changes something. The row is one button, stretched over the cell as the
anchor is, inside the table's form, so it too works without script. It is a POST,
so it cannot be opened in a new tab; it needs the table's `Key`; a table with one
cannot be searched; and a row its `Guard` refuses is shown but does nothing.

### `Guard` → gating, in two places from one declaration

`Guard` has two visual forms and they are the same rule:

1. **On the page** — the **Not permitted** state. The wording matters: *"the
   guard ran before anything was loaded, so this device was never read."* The
   page shows nothing, because nothing was fetched.
2. **On a control** — the *gated* treatment: 45% opacity plus a tooltip giving
   the reason. The same check that would reject the request also disables the
   button, so there is no second authorisation rule to keep in sync and no
   control that looks available and then fails. A form's *Submit* is the one
   exception: refused, it is not drawn, because a form that cannot be saved
   is shown read-only (see `Form`).

### `Outcome`

An action says how it ended and the library draws it. Accepted outcomes redirect
(a 303, so a reload does not repeat the POST) and carry their toast across the
redirect; refused ones show the form again, in place, with what the user typed.

| Constructor | Accepted? | Visual |
|---|---|---|
| `Success(msg)` | yes | A toast with a green left border, 3.4s; none when `msg` is empty |
| `Warning(msg)` | yes | The same with an amber border: it was done, and the user should be aware of `msg` |
| `Failure(msg)` | no | The form shown again with what was typed, and a toast with a red border saying `msg` |
| `Reject(Field(...)...)` | no | The form shown again with what was typed, an invalid border + ring on each named field and its message beneath, and a "Not saved" toast |
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

1. **Panel titles and descriptions — closed.** `Form` and `Table` carry `Title`
   and `Desc`. A panel's title is part of what the panel *is* — the name of the
   thing being shown — and the alternative was that the runtime invents one.
2. **Collapsible rows — deferred.** `Stack` is a slice type, so it cannot carry
   a `Label` without becoming a struct. A long page is scannable with `Tabs`
   until that is wanted.
3. **Sidebar grouping — closed, flat.** `Nav.Section` is a caption above a group
   of entries, named by each entry that belongs to it. Nesting (`Nav.Parent`) would need the compile step to validate a
   tree and is not worth it until an app exceeds a screenful of entries.
4. **Badges and inline bars — closed.** `Badge` and `Slider` are accessors
   alongside `String`, `Int` and `Float`, not a display mode bolted on top: a
   status is a badge, and a quantity is a slider — read-only it is the progress
   bar, with a `Store` it is the control. They are the pressure-release valve
   for visual richness in a table, so `Form` and `Table` stay the only two
   leaves. Monospace identifiers are not covered: no accessor says "show this in
   the monospace face".

---

## What the design deliberately has no type for

Three absences that are choices, not gaps:

- **No time-range picker and no auto-refresh interval.** Both are dashboard
  concepts. A control plane refreshes a leaf on demand; it does not poll. If
  this ever changes it should change as a declaration on a leaf, not as global
  chrome.
- **No chart or statistic leaf.** Two leaves — one that shows a model, one that
  shows a list of models — cover admin panels. A third would be the first
  feature bolted on rather than generated by the existing ideas. `Badge` and
  `Slider` exist so that the pressure for one has somewhere cheaper to go.
- **No per-panel menu.** A panel's header carries a title, a description and a
  status. Reloading is the page's Refresh, so a panel is replaced only by
  following its own sort, pager or row links, which are the only per-panel
  operations that exist. A panel-level menu would mean inventing a concept the
  body tree does not have.
