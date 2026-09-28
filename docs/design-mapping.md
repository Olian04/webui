# Domain types → design elements

How each declared type becomes something on screen. One rule runs through all
of it:

> **Every visual affordance is the consequence of a declaration, and every
> declaration has exactly one visual consequence.**

If a panel has a checkbox column, it is because the table declares bulk
operations — not because someone chose to put one there. If a page has no
toolbar, it is because it declares no arguments. Nothing is styled into
existence, and nothing is configured twice.

---

## Summary

| Type | Becomes |
|---|---|
| `App` | The shell: sidebar, top bar, toolbar, content, status bar |
| `Brand` | Sidebar brand row (logo mark + name), 48px, aligned with the top bar |
| `Theme` | The token `:root` block — the only thing that differs between dark and light |
| `Page` | One address, one screen |
| `Page.PathTemplate` | The address; its segments are the breadcrumb trail |
| `Page.Args` / `ArgSpec` | The toolbar — one parameter pill per argument |
| `ArgSpec.Kind` | Which control the pill holds (select, text, number, toggle) |
| `ArgSpec.InPath` | Whether the value appears in the breadcrumb or in the toolbar |
| `Page.Guard` | The **Not permitted** full-page state, and the gating of every control |
| `Page.Nav` | A sidebar entry |
| `Nav.Label` | The entry's text |
| `Nav.Shadow` | Which *other* entry lights up while this page is open |
| `Nav.Hidden` | No entry at all |
| `Page.Body` → `Node` | The arrangement of panels inside the content region |
| `Stack` | Panels in a column, 8px apart |
| `Split` | Two columns, top-aligned, 8px apart |
| `Tabs` / `Tab.Label` | The tab strip; the selection is a page argument |
| `Form` | One panel containing a form and a footer with its submit |
| `Table` | One panel containing a table and a pagination footer |
| `Field` (in a `Form`) | A labelled input |
| `Field` (in a `Table`) | A column |
| `Field.Label` | The input label / column header |
| `Field.Kind` | The input type and the column's alignment |
| `Field.Group` | A bordered group inside the form with an uppercase caption |
| `Field.Set` (absent) | The input renders read-only on the hover surface |
| `Field.SortKey` | The column header becomes a sort link |
| `Field.Placeholder` | The input's placeholder |
| `Field.Rules` | HTML validation attributes on the input, and the hint beneath it |
| `Table.RowClick` | Rows become links and take the pointer + hover treatment |
| `Table.Bulk` | The checkbox column and the selection action bar |
| `Form.Submit` | The primary button in the panel footer |
| `Action` | A button; its role decides primary / secondary / destructive |
| `Effect.Toast` | A toast, bottom-right |
| `Effect.Redirect` | A navigation after the toast |
| `Effect.Fields` / `FieldError` | Invalid borders and per-field messages, with typed values preserved |
| `Effect.Stale` | Which panels re-render after the action |
| `Addr` | Which panel a refresh replaces |
| `CompileErrors` | The **Failed to compile** page, served at every address |

---

## The shell

### `App` and `Brand`

`App` is the whole screen. Its `Pages` populate the sidebar and the command
palette; `ByPath` is what makes a breadcrumb parent clickable and what lets one
page link to another without either knowing the other's address.

`Brand.Name` and `Brand.Logo` fill the 48px sidebar brand row. That row is
exactly as tall as the top bar so the two horizontal rules line up across the
whole window — the single most noticeable alignment in the layout.

### `Theme`

`Theme.Tokens` is the entire surface between the design and the application.
Every colour, radius and elevation in the design document is a token; nothing
in any component reads a literal value. Swapping dark for light is swapping the
token block, and nothing about spacing, structure or component behaviour
changes. An application that wants its own palette redefines tokens and gets a
coherent result, because semantics (*healthy*, *warning*, *critical*,
*interactive*) are named, not colours.

### `Page` and `PathTemplate`

One page is one screen and one address. `PathTemplate` is both:

- the address the user sees and shares, and
- the breadcrumb trail, derived from its segments.

Because pages are identified by template rather than by a route table, a
breadcrumb parent and a cross-page link resolve to the same thing, and the
compile step can prove a link's destination exists.

---

## Arguments and the toolbar

### `Args` / `ArgSpec` → the toolbar

This is the highest-leverage mapping in the design.

A page's argument struct **is** the toolbar. Each `ArgSpec` becomes one pill:
`ArgSpec.Name` is the label, `ArgSpec.Kind` chooses the control, and the
current value fills it. Setting a value is a navigation; clearing it is the
small `×` in the pill.

The consequences fall out for free:

- The state of the screen is in the address bar, so it survives reload, back,
  bookmarking and sharing.
- There is no separate "filter UI" to design per page, and no way for a page
  to have a filter that isn't in its address.
- A page with an empty `Args` renders *"This page declares no arguments"* — the
  absence is legible rather than silently missing.

`ArgSpec.InPath` decides where the value shows: a path argument is identity, so
it appears in the **breadcrumb** (`Collector › Devices › dev_27c38b`); a query
argument is a view setting, so it appears in the **toolbar**. Same struct, two
placements, decided by one boolean.

`ArgSpec.Kind` chooses the control — an enumerable kind becomes a select, a
string a text field, a bool a toggle, a number a numeric field — and also
decides what the compile step will reject, which is why the toolbar can never
render a control that the decoder cannot read back.

### `Decode`

Invisible, but it is the reason the toolbar can be trusted: the values in the
pills are the values the page will receive, parsed once at compile time rather
than re-derived per request.

---

## Navigation

### `Nav.Label` → a sidebar entry

Straightforward: one page, one 32px entry, with the active treatment (Active
background, full-strength text, 2px primary bar at the left edge) when it is
the current page.

### `Nav.Shadow` → borrowed highlight

A detail page has no business adding a permanent sidebar entry, but it must not
leave the sidebar looking like nothing is selected. `Shadow` names the page
whose entry should light up instead. Visually: the sidebar stays on *Devices*
while you are on a device, and the **breadcrumb** carries the fact that you
have gone a level deeper.

This is why the breadcrumb lives in the top bar rather than in the content — it
is the sidebar's partner, not the page's heading.

### `Nav.Hidden`

No entry, no shadow. The page is reachable only by link. Nothing appears.

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

The tab strip. The crucial design consequence is that the selection is a page
argument, not component state: the active tab appears in the toolbar as a
parameter, is in the address bar, survives a reload, and can be linked to.
A tab is navigation that happens to look like a tab.

---

## Body: leaves

### `Form` → a panel

One panel. Header carries the title and description; the body carries the
fields; the footer carries `Submit` as the primary button, right-aligned, with
a ghost *Cancel* beside it.

`Bind` is invisible until something fails — see `Effect.Fields` below.

### `Table` → a panel

One panel. Header carries the title and description; the body carries the
table; the footer carries the range (`1–10 of 37`) and the pager, whose
buttons are links carrying an offset argument.

`Load` returning `total = -1` is a real design state: the footer shows the
range without a total and the *Next* button cannot be pre-disabled.

---

## `Field` — one type, two renderings

`Field` appears in both leaves and renders differently in each, from the same
declaration:

| | In a `Form` | In a `Table` |
|---|---|---|
| `Label` | Field label, 12.5/500 | Column header, 12.5/500 |
| `Kind` | Input type | Cell alignment and formatting (numeric → right, tabular figures) |
| `Get` | The value shown | The cell value |
| `Set` absent | **Read-only** — hover surface, text-secondary, weak border | (no effect; a column is always read-only) |
| `Rules` | Validation attributes + the hint line | Ignored |
| `SortKey` | Ignored | Header becomes a sort link; the sorted one takes the link colour and a caret |
| `Group` | A bordered group with an uppercase caption | Ignored |
| `Placeholder` | The placeholder | Ignored |

The "ignored" cells matter as much as the others. A field declared once and
used as both a form input and a table column is the normal case; each position
takes what it can use and silently drops the rest. That is what makes the field
reusable rather than forcing two near-identical declarations.

### `Rules` → validation, twice, from one declaration

Each rule has exactly one visual form:

| Rule | Input attribute | Hint |
|---|---|---|
| Required | `required` | `*` beside the label, in critical |
| `MinLen` / `MaxLen` | `minlength` / `maxlength` | "3–32 characters" |
| `Min` / `Max` (`*Bound`) | `min` / `max` | "1–65535" |
| `Pattern` | `pattern` | The pattern's own message |

The browser refuses before anything is sent; the server re-runs the same set
after. The user sees one behaviour. `Pattern` carries a message because a
regular expression cannot explain itself — that message *is* the hint text, and
it is the reason `Pattern` is a struct while `MinLen` is a bare integer.

---

## Actions and effects

### `Action` → a button

`Form.Submit` renders as the primary button in the panel footer.
`Table.Bulk` renders in the selection action bar. Destructive actions take the
critical role.

### `Table.Bulk` → checkboxes that only exist when declared

Declaring bulk operations adds the 34px checkbox column and, once anything is
selected, the action bar: a 10% primary strip carrying the count, the
operations, and *Clear*. A table that declares none has no column and no bar —
not a disabled one.

Selection itself is deliberately **not** an argument: it never enters the
address bar, because it is request state, not page state. A shared link does
not carry someone else's selection.

### `Table.RowClick` / `RowTarget` / `Link`

Rows become links: pointer cursor, hover surface, and the whole row is the
target. Because `Link` names a page and builds its arguments, the destination
is known at compile time, which is what lets a row link be a real `href`
instead of a click handler — and therefore what lets the whole design work with
JavaScript switched off.

### `Guard` → gating, in two places from one declaration

`Guard` has two visual forms and they are the same rule:

1. **On the page** — the **Not permitted** state. The wording matters: *"the
   guard ran before anything was loaded, so this device was never read."* The
   page shows nothing, because nothing was fetched.
2. **On a control** — the *gated* treatment: 45% opacity plus a tooltip giving
   the reason. The same check that would reject the request also disables the
   button, so there is no second authorisation rule to keep in sync and no
   control that looks available and then fails.

### `Effect`

| Field | Visual |
|---|---|
| `Toast` | A toast, bottom-right, semantic left border, 3.4s |
| `Redirect` | Navigation after the toast is shown |
| `Fields` (`[]FieldError`) | Invalid border + ring on each named field, message beneath, **and the panel re-renders with what the user typed** |
| `Stale` | Which panels re-render; everything else holds still |

`Effect.Fields` with a nil error is the design's most important rejection path:
a save that was understood, refused, and is fixable. It looks identical to a
browser-caught rule failure, which is correct — from the user's side it is the
same event. The only difference is that it could not have been caught earlier,
because it needed the service.

### `CompileErrors` → the compile-failure page

Served at every address under the mount until the application compiles. Because
the errors are structured rather than scraped from a compiler, every problem is
shown at once, each with its location, its message, and a plain-language
**Fix:** line. Nothing about the design encourages reporting them one at a
time.

---

## What the design needs that the types do not have

Four things in the mock are not expressible today. Three are real decisions; one
is small.

### 1. Panel titles and descriptions — `Form` and `Table` have no `Title`

This is the significant one. The entire visual grammar is *titled panels*: a
`Split` of two leaves is two unlabelled boxes without it, and the information
icon carrying a description has nothing to carry. Every panel title in the mock
is invented.

A `Title string` and `Desc string` on both leaves would close it. Worth
weighing: it adds a presentational field to two structural types, which is the
first place the declaration starts describing appearance rather than meaning.
The counter-argument is that a panel's title is part of what the panel *is*, not
how it looks — it is the name of the thing being shown, and the alternative is
that the runtime invents one.

### 2. Collapsible rows — `Stack` has no `Label`

Grafana's row header is the mechanism that makes a long page scannable without
pagination. A `Stack` is the natural home for it: `Label string` plus a
default-collapsed flag. This is additive and low-risk, but it does give `Stack`
a visual identity it currently lacks — today a `Stack` is pure arrangement.

### 3. Sidebar grouping — `Nav` has no section or parent

`Nav` has `Label`, `Shadow` and `Hidden`. The sidebar is therefore flat. The
*Configuration → Settings → Ingest / Retention* grouping in the mock is
invented. Options, roughly in order of cost:

- `Nav.Section string` — a flat caption above a run of entries. Cheapest;
  matches the uppercase captions in the design; no nesting.
- `Nav.Parent` — real two-level nesting, with the expand/collapse behaviour.
  More expressive, but introduces a tree that the compile step now has to
  validate (no cycles, no orphans, no parent that is itself a page with a
  shadow).

The design only needs the first. The second is worth wanting only if the
application will plausibly exceed a screenful of entries.

### 4. Inline bars in table cells — `Field` has no render hint

The bar in the Rate column is a display mode, not a new kind of node: the cell
is still one field value. A small `Render` enum on `Field` (`Plain`, `Badge`,
`Bar`, `Mono`) covers it, and it is worth deciding *now* — because it is the
pressure-release valve for every future request for visual richness in a table,
and the alternative is a third leaf kind for charts. A render hint keeps
`Form`, `Table` as the only two leaves.

---

## What the design deliberately has no type for

Three absences that are choices, not gaps:

- **No time-range picker and no auto-refresh interval.** Both are dashboard
  concepts. A control plane refreshes a leaf on demand; it does not poll. If
  this ever changes it should change as a declaration on a leaf, not as global
  chrome.
- **No chart or statistic leaf.** Two leaves — one that shows a model, one that
  shows a list of models — cover admin panels. A third would be the first
  feature bolted on rather than generated by the existing ideas. The `Field`
  render hint above exists so that the pressure for one has somewhere cheaper
  to go.
- **No per-panel menu beyond refresh.** The hover-revealed `⋮` in the panel
  header does exactly one thing, because refreshing the leaf at an `Addr` is
  the only per-panel operation that exists. Adding a second item to that menu
  would mean inventing a panel-level concept that the body tree does not have.
