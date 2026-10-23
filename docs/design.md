# webui — visual design

A design language for admin panels and control planes: dense, quiet, and
legible at a glance. It borrows its proportions and palette from Grafana,
because Grafana solves the same problem — a lot of operational information on
one screen, read repeatedly by people who already know what they are looking
for.

---

## 1. Principles

**Density over comfort.** These screens are read every day by the same people.
Rows are 34px, not 52px. Padding is measured in 8s, not 16s and 24s. The
reward for tightness is that more of the answer is on screen at once.

**The chrome is quiet; the data is loud.** Everything structural — the
sidebar, the panel frames, the labels — sits at 65% or 40% text
opacity. Full-strength foreground is reserved for values. Colour is reserved
for meaning: a status, a severity, a rejection. Nothing is coloured for
decoration.

**One surface, one job.** There are exactly four surface levels, and each has
a single meaning (§3). A screen that needs a fifth is a screen that is doing
too much.

**Every control is a link.** Sorting, paging, filtering, tabs, row clicks —
all of them are addressable. This is a behavioural rule with visual
consequences: controls look like navigation, not like widgets, and the address
bar is always the truth about what is on screen.

**Absence is configuration.** A table with no bulk operations has no
checkbox column at all — not a disabled one. A column that cannot be filtered
has no filter icon. A control that the visitor may not use is not there either, unless the
application says it should be, with a reason: then it is drawn disabled, with the reason as a tooltip.

**Square, not soft.** 2px radius everywhere. Shadows only on things that float
(menus, toasts). No gradients (but for the default mark, which is a conic one), no glass, no decorative borders. The visual
weight budget goes entirely to separating regions.

---

## 2. The two themes

Dark is the default. Light is a complete, equal alternative — not an
afterthought — and is selected by the same token set with different values.
Nothing in the layout, spacing, or component structure changes between them.

---

## 3. Surfaces and elevation

Four levels, dark values first, light values second.

| Level | Meaning | Dark | Light |
|---|---|---|---|
| **Canvas** | The page behind everything. Panels sit on it with gutters. | `#111217` | `#f4f5f5` |
| **Panel** | A bounded region of content. Also the sidebar and top bar. | `#181b1f` | `#ffffff` |
| **Raised** | Something floating above the page: menus, toasts, tooltips. | `#22252b` | `#ffffff` + shadow |
| **Input** | A field waiting for text. Recessed, not raised. | `#0e0f12` | `#ffffff` |

In dark, an input is *darker* than its panel — a hole, not a button. In light
it is the same white as the panel, and its border does the work instead.

Elevation is never expressed with shadow except on Raised, where the shadow is
`0 4px 12px rgb(0 0 0 / .55)` (dark) or `0 4px 12px rgb(24 26 27 / .18)`
(light).

---

## 4. Colour

### Neutrals

All neutrals derive from one hue per theme at varying alpha, so they compose
over any surface.

| Token | Dark | Light | Use |
|---|---|---|---|
| Text | `rgb(204,204,220)` | `#24292e` | Values, headings, active items |
| Text secondary | 65% of Text | `#5c6269` | Labels, descriptions, inactive nav |
| Text tertiary | 45% of Text | `#85888e` | Decoration only: placeholders, separators, the shortcut hint, icons. 3:1, never text a reader needs |
| Border | 11% of Text | `rgba(36,41,46,.12)` | Panel edges, table rules |
| Border strong | 22% of Text | `rgba(36,41,46,.24)` | Input edges, secondary buttons |
| Hover | 7% of Text | `rgba(36,41,46,.06)` | Row and item hover, table headers |
| Active | 11% of Text | `rgba(36,41,46,.10)` | Selected nav item |

### Semantics

Five meanings, no more. Each has a text colour, a 14% background (12% in light), and a 30%
border (28% in light), so a badge can be built from one token family.

| Meaning | Dark | Light | Where it appears |
|---|---|---|---|
| **Primary / interactive** | `#3d71d9` | `#3d71d9` | Primary buttons, selection, focus ring |
| **Link / sorted** | `#6e9fff` | `#1f62e0` | Sorted column, inline emphasis, gauges |
| **Healthy** | `#6ccf8e` | `#177244` | Loaded, acknowledged, ok |
| **Warning** | `#ff9830` | `#a74a0b` | Degraded, open, active tab underline |
| **Critical** | `#f2686c` | `#c20d54` | Rejections and failures, as text, borders and dots |
| **Critical, solid** | `#d03a3f` | `#c20d54` | The fill of a destructive button, which carries a white label |

The active tab underline is warning-orange rather than primary-blue. This is
deliberate: blue means *you can act on this*, orange means *you are here*.

---

## 5. Typography

Inter, falling back to the system UI stack. One family; no display face.

| Role | Size | Weight | Colour |
|---|---|---|---|
| Panel title | 14 | 500 | Text |
| Body / breadcrumb | 14 | 400 | Text |
| Control label, button | 13.5 | 500 | Text |
| Table cell | 13 | 400 | Text |
| Table header, secondary | 12.5 | 500 | Text secondary |
| Field label | 12 | 400 | Text secondary |
| Description | 12.5 | 400 | Text secondary |
| Section caption | 11 | 500, uppercase, `.05em` | Text secondary |

Addresses, payloads and code (the editor, a diff, a Markdown code block) use a monospace face at 12–12.5 in Text secondary. A table cell is not monospace, since no accessor can ask for it. Numeric columns
use tabular figures and right alignment.

Nothing is bold. 500 is the heaviest weight in the system; 600 appears only in full-page error headings and the status bar's two badges. The exception is the strong emphasis of a
`Markdown` leaf, which is 600: in running text 500 cannot be told from 400, and a
reader must be able to see what the author stressed.

---

## 6. Metrics

Everything is a multiple of 2, and the working gutter is 8.

| Element | Size |
|---|---|
| Panel gutter | 8 |
| Page padding | 12 on the top and sides, 46 below (it clears the status bar) |
| Corner radius | 2, except the pills (the switch, the spinner) |
| Sidebar | 240 (56 when collapsed) |
| Top bar | 48 |
| Status bar | 26 |
| Panel header | 32 |
| Nav item | 32 |
| Control (button, input, select) | 32 |
| Small control | 28 |
| Table header row | 30 |
| Table body row | 34 |

---

## 7. The shell

Four fixed regions. Only the content region scrolls.

```
┌────────────┬──────────────────────────────────────────────┐
│            │  top bar: breadcrumbs · search · refresh     │
│  sidebar   ├──────────────────────────────────────────────┤
│            │                                              │
│            │  content (scrolls)                           │
│            │                                              │
├────────────┴──────────────────────────────────────────────┤
│  status bar                                               │
└───────────────────────────────────────────────────────────┘
```

### Sidebar

240px on the left unless a visitor has dragged it (180px to 480px, never more than half the
window), panel surface, full height, always visible. It never
becomes a top bar and never turns into a hamburger — below 820px it collapses
to a 56px icon rail instead, and a button in its footer collapses it to the same
rail at any width (the choice is a browser preference). The structure of the
application is the one thing that should not disappear.

Its right edge is a handle, with script: a 7px hit area over the border, a `col-resize` cursor, and a 2px
line in the accent colour on hover, on keyboard focus and while it is dragged. It is a window splitter: the arrow keys
move it, Home and End go to the ends, and Enter or a double click puts it back to 240px. The rail has no handle.

Contents, top to bottom: brand mark and product name (48px, matching the top
bar so the two align); grouped navigation; a footer pinned to the bottom.

Navigation items are 32px, 13.5px, text-secondary, with a 16px icon (a Font
Awesome glyph; an item with none holds the place of one, and shows its label's
capitalised initial in the rail). Groups carry an 11px uppercase caption above
them. In the rail the caption has no room, so it becomes a short, thick dash (32px by 3px, rounded, in the border colour `--border-2`, so that it is quieter than the icons around it)
centred in 21px of height for each group, and the caption is its tooltip. Nothing is drawn above the first
group. It is short and thick on purpose: the line under the brand is the width of the rail and 1px, and a rail
whose groups were marked by the same kind of line would have two things that look alike and mean different
things, an edge and *these belong together*. A line across the rail, a dot, a short thin rule, the caption's
first letters (clipped, "CONFIGU") and space alone were tried. The status bar separates its figures with a
middle dot (token `--sep`, *render 0.1 ms · built 14:49:31Z* where script is off), which is a separator between
things in a row, not between groups in a column. The active item takes the Active background, full-strength text, weight
500, and a 2px primary-coloured bar flush against the left edge. An item the
visitor may not open is not drawn.

**A label wider than the sidebar** is cut with an ellipsis, on one line, in the entry, the section's caption and the brand; its whole is the
title. The icon, or the initial that holds its place, keeps its size.

### Top bar

48px, panel surface, sticky. Breadcrumbs on the left — parents at text-
secondary, the current page at full strength and weight 500, separated by `›`
at text-tertiary. On the right: a 290px search field (only when some page has a `Search`: the app's pages are in the sidebar, and the search finds the things they are for; without script it has a *Search* button joined to its right edge and answers with a page of results, and with script the button is gone and the results are listed beneath the box as it is typed in), then the page's *Refresh*
icon button, then, when the app declares external links, a button (the arrow-out-of-box
icon) that opens a popover list of them: a raised surface right-aligned beneath the button, 32px items with
a 16px icon and an 11px uppercase caption above a section, closed by a click elsewhere
or Escape. A link to a web address opens in a tab of its own; a path on the site and a mail
address open in the current one.

The breadcrumb is where location is expressed when the sidebar cannot show it
— for example on a detail page that intentionally has no navigation entry of
its own.

**The search palette** (the results under the box) is 360px wide with 30px items, as tall as the window leaves between the top bar and the
status bar and scrolling inside, dimmed and out of reach while a newer question is pending. The box says the key that opens it (⌘K, or Ctrl K).
The sidebar's foot has the collapse button and the theme switch.

### Refresh

An icon button beside the search. It reloads the page at the address it is on,
state in the address and all. There is no time-range picker and no auto-refresh
interval: this is a control plane, not a dashboard — it refreshes on demand, it
does not poll. There is no row of page parameters: what a table shows is
filtered in its column headers (§10).

### Content

12px padding, 46px below to clear the status bar, and scrolls independently.

A table panel is no taller than the window less the shell around it (the top bar and
the content region's padding), and no shorter than 16rem unless its rows are fewer.
When its rows do not fit, the panel's head and foot (the title and the pager) stay in
place and only its body scrolls, with the header row sticky at its top, so a long
table does not push the page. A panel with few rows is as short as they are. It is a
height limit on the panel and nothing more: the layouts around it are untouched, and
the content region scrolls as usual to reach a form above a table, or a second table.
A filter's form is pinned to the window while it is open, so the scrolling body cannot
clip it.

### Status bar

26px, fixed to the bottom, monospace 11.5. Shows the method and address of the
request behind what is on screen, and a spinner with a short label while
something is in flight. It is the honesty bar: if the screen changed, this says
what changed and how much of it.

On the right it says how the page came to be, in the muted text colour, as two parts of one whole that are
labelled: the **render** time, how long the server took to build the page (`render 0.1 ms`), and the **delivery**
time, what the rest took until the page was loaded (`· delivery 53 ms`): the network, the assets and the browser.
They add up, which they did not when the second was "loaded" and counted from the request, so the render time was in
both. The middle dot between them is the design's separator, and a title on the pair says it all in words. A bar of
each one's share of the whole was tried and dropped: the two numbers already say where the time went. Without
script only the render time is known and it stands alone. Where script is off it also says when the HTML was built (`built 14:49:31Z`), since nothing
else there can say that a page is old. With script that time is not shown: what it was for is knowing that a page was
not built for the request that brought it, which a small uppercase badge in the warning colours says when it is so,
**stale page**, with the reason as its title (the browser showed it from its own cache, a cache or a proxy
answered, a page restored as it was left, or a clock that differs from the server's); and how long ago it was built
was dropped, since a page that is old but was built for the request has nothing to be told. The other badge is **JS
off**, drawn only where script is off. The timings are hidden below 720px; the badges are not. A panel fetched again
updates all of it.

---

## 8. Panels

The unit of content. A bordered rectangle on the panel surface with a 32px
header and no divider between header and body.

The header carries, left to right: the title (14/500); an information icon
holding the description as a popover; and a status icon when the panel failed to
load. A panel has
no menu of its own: reloading is the page's Refresh in the top bar, so a panel
changes only by following its own links, and the header stays quiet.

A panel may have a footer separated by a 1px rule: pagination on the left (with the
table's CSV and JSON download links, as quiet buttons, beside the range),
navigation or actions on the right.

Panels never nest. A `Group` is layout only, with no frame and no caption.

### Arrangements

- **Stacked** — panels in a column, 8px apart. A stack that has height to give, because it is a cell of a
  Split beside something taller or the body of a tab in one, gives it to its tables, which share it, and to its last
  panel when it has none, so the column ends where the row does.
- **Split** — two columns, 8px apart. Panels side by side are as tall as the tallest
  of them, so their borders line up; when a Split wraps onto more rows, the heights
  are equal within a row and not across rows. A panel's foot (a pager, the actions)
  stays at the bottom of its box.  Collapses to one
  column below 1100px, where nothing is stretched. Only panels that are direct
  children of the Split are stretched; a layout inside it, such as Tabs, is one
  column cell, and what is under its strip (a panel, or a stack of them) fills what is left of the cell.
- **Tabbed** — a 1px-ruled strip of 13.5px labels; the active one takes full-
  strength text, weight 500, and a 2px warning-coloured underline. Tabs are
  addressable: the selected tab is in the address bar, so a tab can be linked
  to and survives a reload. The strip is the control. With script a tab does not load a new page: the strip and
  the tab that was chosen are asked for alone, and replace what was there, quieter (0.55) until they arrive, so the
  panels around the tabs are neither read again nor redrawn, and the page is where it was. Without script a tab is
  a link to the page. A strip has as many tabs as it is given. When they need more room than the strip has, because
  there are many, their labels are long or the cell is narrow, the strip scrolls sideways (a thin bar, which a system
  with overlay scrollbars shows while it is scrolled, and the label cut at the edge says there is more) and the tabs
  keep their size, neither shrinking nor wrapping. With script a strip that is opened on a tab out of sight is scrolled
  to it, and one that replaces another keeps where that was scrolled, moved only as far as the chosen tab needs. The
  rule under the strip is a shadow inside it, since a box that scrolls clips what sticks out of it.

### Code panels

An editor and a diff are panels whose body is a code editor, Monaco, laid over markup that already works
without script. The panel is the same panel (a 32px header with the title and description, a footer where
there is something to save); the body has no padding, and the editor fills it.

- **Height.** An editor or a diff is as tall as the window less the shell's chrome and 8rem, and never
  shorter than 16rem (`100dvh` where it is supported). It is not a *grows* panel: the editor scrolls itself,
  so a page with one does not have two scrollbars, and a taller window gives it more lines.
- **Without script.** An editable editor is a textarea on the input surface, with the form's *Save* in the
  panel footer. A viewer, an editor with nothing to save, is the text in a block on the panel surface. Both are
  13px on 20px monospace (`ui-monospace`, `SFMono-Regular`, `Menlo`), tab size 2, no wrapping, and scroll by
  themselves. When script runs Monaco mounts over them and hides them, with the colours of the app's tokens in
its light or its dark theme.
- **States.** Focus is an inset 2px ring in `--ring`; a refused text is an inset 2px ring in `--ring-bad`, with the
  reason under the editor in a rule-topped strip (8px 12px), where a form would put it under a field.
- **A diff** is the two texts side by side when its panel is 900px wide or more, and in one column, the lines of
  both interleaved, when it is not, following the panel as it is resized. Above it is a header row (12px,
  `--text-2`, ruled below) that names the sides: over each side when they are side by side, and as one line, *Deployed →
  Saved*, when they are not, because in one column they are not sides but a change from one text to the other.
  Without script the server makes a unified
  diff: 12.5px on 19px monospace, a grid of the old line number, the new line number, a marker and the text
  (3.5rem, 3.5rem, 1.25rem, the rest), inserted lines on `--green-bg` and deleted lines on `--red-bg`, a summary
  line of `+n` in green and `−m` in red, and a quiet `⋯` for each run of unchanged lines that was left out. Two
  texts that are too large or too different to compare there say so, and two that are the same say *No
  differences*.
- **Language services** have no look of their own: their diagnostics are Monaco's markers, and a page that
  imports none has highlighting only.

### Text

A text set as Markdown is a panel with an article in it (`.md`): 14px on 1.6, in the panel's text colour, and
it works without script. It is markup from outside the design, so each element is given the look of its counterpart in the app,
from the same tokens.

- **Measure.** Prose holds to 84ch; code and tables use the width they are given.
- **Headings** are no heavier than 500. h1 is 18px, h2 15.5px and h3 14px, with 22px above and 8px below; h4 to
  h6 are captions, 11px uppercase and spaced, in `--text-2`, as the sidebar's sections are. **Strong** is 600, the one
  place outside a full-page error where the app is heavier than 500, because in running text 500 is too close to 400
  to tell apart.
- **Links** are `--blue-text` and underline on hover. **Lists** are indented 20px, 3px between items, with the
  marker in `--text-3`, and a task list has the app's checkbox in place of the bullet.
- **Code** is 12.5px monospace and a step quieter than prose: inline on `--hover` (1px 5px), a block on the input
  surface with a 1px rule, an 8px 10px inset and 2px corners. A block that names a language is coloured by the
  editor, which the page then loads.
- **A table** is the app's table, framed: a 30px header on `--hover` at 12.5px, 34px rows at 13px, a rule between
  rows and none after the last, no lines between columns, and it scrolls by itself when it is wide.
- **A picture** is drawn only from an origin the author trusts, no wider than the text and in line with it; any other
  is a link, named by its alt text.

---

## 9. Controls

Four button roles and no others.

| Role | Appearance |
|---|---|
| **Primary** | Solid primary blue, white text. One per region, at most. |
| **Secondary** | Transparent with a strong border. |
| **Ghost** | Transparent, text-secondary, background on hover. Cancels and dismissals. |
| **Destructive** | Solid critical red. |

32px tall, 12px horizontal padding, 13.5/500, 2px radius, optional 14px leading
icon.

**A file chooser** is the browser's own, in an input of the same surface, border and focus ring, with its button drawn as a small
secondary button (24px, 12.5/500, a strong border). The file that is stored is above it, in the text colour, with its size dimmed and, for
a file that can be removed, the *Remove* checkbox at the right. With script the chooser sits in a 1px dashed frame in the strong border
colour, with "or drop a file here" in the tertiary text colour beneath it; while a file is dragged over, the frame takes the primary colour and
an 8% tint of it. A picture of a chosen image is below, at most 96px high in a 1px border, and the stored image's thumbnail is a 32px square beside its name;
both have the zoom-in cursor and a primary focus ring, and open the file at full size in a dialog (the stored one in the page, by an invoker command, with no script): a raised surface with a 1px strong border
over a 60% dim, the image as large as the window allows and never stretched, its name beneath, and a ghost *Close*. *Clear* is a small ghost button at the right of the chooser. A name that is a link is in the link colour and is underlined on hover.

**Inputs** are 32px on the input surface with a strong border; focus takes a
primary border and a 2px primary ring at 25%. A field that is only read is not an input: it is the value as output (14.5px, in no box, under the usual quiet label, with an em dash when empty), so it is visibly not a hole you can type into. Invalid fields take a critical border and ring.

**Selects** replace native chrome with a small CSS chevron so they match input
height and weight exactly.

**A text area** is an input that is at least 32px tall, with `6px 9px` of padding, a line height of 1.45, five rows and a resize handle for
the height only. A password, a telephone, a date, a date and time and a colour input are the browser's own control, in the input's
surface, border and focus ring; a colour fills the whole field.

**A phone number** is a segment: a selector of the country calling code (9.5em wide, square where it meets the number) joined
to the input, which holds the national number, spaced as it is read when there is script. Both are in the page, and the server makes E.164 of them.

**A choice with script** (`select`) is badges in the field and a popover list below it, 30px to a row with a 15px checkbox (round when only one
may be chosen), flipping above when there is no room below and no taller than 260px, which scrolls. Without script it is the browser's own
`<select>`, a list of up to six rows when several may be chosen.

**The rules and the help of a field** are in one popover, on hover and on keyboard focus of the information icon beside its label: at
most 280px wide, opening to the right and below, with the heading "Help:" above the field's help and "Rules:" above its constraints (11px
captions), items in 12.5px text-tertiary; its text is also the icon's accessible name. Nothing is printed beneath a control while its value is good.

**A question that a button asks** is a native dialog over a 50% dim: at most 420px wide or the window less 32px, 16px and 18px of padding,
the raised surface and a strong border, 13.5px text, a ghost *Cancel* (which has the focus) and the action's own small button.

**A toolbar** is a row with no frame: it wraps, with 8px between rows and 10px between controls, bottom-aligned. A value (of a form that the toolbar loads, which a group names for a screen reader) is its label, in
text-secondary and followed by a colon, and what it is, in the text colour, 13px, at least 32px tall; buttons are small. The gap between
the left controls and the right ones takes what is left, and is gone when script finds that they do not fit in one row, and they flow
from the left. A toolbar with nothing to draw is not drawn; one whose options could not be read is an empty panel that says so; a form of it that could not be loaded is its title and "Could not load" in place, the rest standing. One that is not final ends with a quiet "Updating…".

**Checkboxes** are 15px, 2px radius, filling solid primary with a white check
when set.

**Switches** are 30×16 pills, used only for session-level modes, never for
saved data.

**Absent, disabled, or gated.** A control that the visitor may not use is one of
two things, and the application's Guard says which with what it returns. An ordinary
error leaves the control out: there is no button, and a table whose every button is
left out has no column for them, no checkbox column and no bar, and a toolbar with
nothing in it is not drawn. `Disabled("Already granted")` draws it, 45% opaque,
with a raised tooltip on hover saying why: the *gated* treatment, for a control that
the visitor ought to know exists. Plain *disabled* is the same 45% with no
explanation, for what the library itself cannot do (a pager at the end of its range).
The reason opens above the control, or below it when there is no room above. A control never fails silently when pressed: a request made by hand is refused with
the Guard's message. The cost of leaving a control out is that it cannot be
discovered, and that its reason is seen only on a page that is refused outright.
A form's Save is the one control with no disabled form: a form the visitor may not
submit is shown as read-only output, with a Back button in place of Save and Cancel,
since a form of inputs that cannot be sent would only invite typing.

---

## 10. Tables

The densest surface in the system, and the most common.

Header row: 30px, hover surface, 12.5/500 text-secondary, 1px bottom rule,
sticky. Every header is a sort link, with a filter icon beside the name that
appears on hover or focus and stays, in the link colour, once the column is
filtered. It opens a raised popover on a form: a multi-select of checkboxes for
a column with a fixed set of options, a minimum and a maximum for a numeric one,
a text input for any other, and Apply
(and Clear, when a filter is set). The sorted header takes the link colour and a
small caret showing direction.

Body rows: 34px, 1px rule between (none after the last), hover surface on
hover. 

**A clickable row is one real link covering the whole row**, and this is a
requirement, not a rendering detail. The row is `position: relative`; the single
anchor in it carries an `::after` pseudo-element at `inset: 0`, which stretches
the link over every cell:

```css
tbody tr.clickable      { cursor: pointer; position: relative; }
.rowlink::after         { content: ""; position: absolute; inset: 0; }
.rowlink:focus-visible::after { outline: 2px solid var(--primary); outline-offset: -2px; }
```

One anchor, not one per cell: the row is a single tab stop, middle-click opens
it in a new tab, and it works with scripting disabled. A click handler on the
row would satisfy none of those. The cost is that text in the other cells cannot
be selected, which is the right trade for a row whose purpose is to be followed.

Cells: 10px horizontal padding. Numeric columns right-align with tabular
figures. 

Column widths follow what a column is, not what is in it. A table sizes its columns by its rows, so two tables of the
same columns, one above the other, would have had their columns in different places for no reason a reader could
see. Each column is given the room it needs from its declaration: the letters of its header and the sort and filter
beside them, a text or a link about sixteen letters, a number the width of "520.6 KiB", a moment or a bar a little more,
and a select the longest of its choices in a badge, with room for two of them side by side when it holds several. The
table is shared between its columns in those proportions at whatever width it has, so a column of badges is wider
than a column of numbers, a long header is never cut, and two tables of the same columns have them in the same
place. The selection and the row actions are left to what is in them. A cell whose content does not fit in its share
still makes its column wider, so nothing is cut, and a table that is wider than its panel scrolls as it always did.

**Badges** sit in status columns: 19px tall, 2px radius, lowercase 11.5/500,
built from a semantic family's 14% background and 30% border. A status badge
may carry a 6px leading dot in the same colour. A select that holds several values
is a badge for each, 4px apart, wrapping. Edited, a select is a 32px drop-down list in the
style of an input, and with several values a list of up to six rows, with the chosen ones on a 22% primary tint.

**Inline bars** are a cell display mode, not a chart: a 56×5 track on the hover
surface with a link-coloured fill, sitting after a right-aligned number. This
is the one place quantity is shown as a shape, and it is deliberately the
smallest possible version of that idea.

**Bulk selection** adds a 34px checkbox column and, when anything is selected,
an action bar above the header: a 10% primary tint strip with the count, the
operations. When there are no bulk
operations, neither the column nor the bar exists.

**Loading** pulses the rows that are there, out of reach, while the rest of the panel —
header, filters, footer — holds still (`aria-busy` on the panel, drawn by the stylesheet).
**Empty** replaces them with a centred 28px icon, a 13.5/500 line naming the
condition, and a 12.5 text-secondary line naming the way out.

**The pager** is a small secondary *Previous* and *Next*, each disabled at its end, with the range ("1–10 of 37") at the left. A cursor
pager has no total, and adds "· sorted within this page" when the table is sorted.

### Export

A table that is told to offers its rows as a file, from the left of its footer: a quiet *Export* caption (12.5px,
`--text-3`) and two small ghost buttons, *CSV* and *JSON*, beside the range, and the pager keeps the right. The file
is every row the sort and the filters keep, not the page on screen, and the columns the table shows. The links
carry `download`, so the page's script, which replaces a panel when a link leads to the same page, leaves them to the
browser; they are plain links, and work without script.

---

## 11. Feedback

**Toasts** stack bottom-right above the status bar: raised surface, 3px left
border in the semantic colour (green for a confirmation, amber for something to be
aware of, red for a failure), a 13.5/500 title and an optional 12.5
text-secondary line, rising 6px on entry and dismissing after 3.4s. The timer holds
while a toast is hovered or has focus. A close button (×) at the right dismisses it at
once; it is a label for a visually hidden radio, and the stylesheet hides the toast
whose radio is chosen, so it works with scripting disabled, where a toast otherwise
stays until the next page.

**Field errors** appear under the field in 12/critical with a 13px icon, and
the field takes the invalid border. Two kinds of rejection are shown
identically: those the browser catches before anything is sent, and those the
server returns afterwards. From the user's side they are the same event, so
they look the same.

---

## 12. Full-page states

**Not permitted.** Centred, 480px: a 34px icon, a 17/600 title, a 13.5
text-secondary line with the reason the guard gave (or, with none, that the visitor has no access),
and, when there is one, a small ghost button on. No data is shown, because none was read; the page does not say so, since how
the check was made is not for the visitor.

**Failed to compile.** The whole application is replaced, at every address,
until it is fixed — modelled on a build tool's error overlay rather than a
server error page. A 22/600 critical heading, a count, and then every problem
at once: a monospace caption locating it, the message in a critical-tinted
block with a 3px left border, and a **Fix:** line in plain language. Problems
are never reported one at a time.

**A refusal, by its status.** *Not found* has a search icon; *Not signed in* and *Not permitted* a lock, with the message and the link that
the refusal gave. The rest of the states are the same page: *Page not found* (a 404 inside the shell, so that the way back is on screen),
*That address is not valid* (a 400), *Something went wrong* (a 500, with nothing of the cause), *No pages declared* and *Nothing to open from
here* (a breadcrumb or a search with nowhere to go). A panel whose `Load` fails, or returns a refusal, is a header with an alert icon and a
body that says *Could not load*, or the refusal's text and link, and the page stays as it is.

---

## 13. Motion

Motion exists to say *something is happening* and nothing else.

- Hover and border transitions: 110–130ms.
- Toast entry: 160ms rise.
- Skeleton pulse: 1.1s, 100% → 40%.
- Spinner: 700ms linear.

Nothing slides, nothing bounces, nothing animates on first paint.

### Navigation

A whole-page navigation is an ordinary one. The browser keeps the page that is on screen until the next has
something to paint, and the shell (sidebar, top bar, status bar) is the same on every page, so it does not appear to
move: only the content changes. This is what making every control a real link buys, and it costs no script.

There is no cross-fade. A cross-document view transition (`@view-transition { navigation: auto; }` with the shell's
regions named) was in the design and was taken out: its first frames are white, in dark mode too, because the
browser shows its default background while it makes the transition, and nothing a page can say reaches it. A
background on the transition, on its root, hiding the root images and animating none of them were each tried, and
the white came as often. It was measured with Chrome 155 as a white frame in about 3 of 8 navigations with the
transition, and in none of 8 with it skipped. A page that says its canvas is dark must not be white at any frame of
a navigation, so the cross-fade went.

**A panel that reloads and a page that moved still mean different things.** The pulsing rows (§10) say *this one panel
is reloading and everything else you can see is still current*, and it is the only motion a navigation has.

### A page that is still arriving

A page whose panels answer within 50 ms arrives as one piece, as it always has, with no mark that anything was waited
for. A page with a slower panel is sent as it is built, in the order it is written: the sidebar, the top bar and the
panels above the slow one at once, then each panel as it is done. Nothing that is not there yet is drawn as if it were
(no skeleton, no placeholder panel), so what is on screen is always what is current. It says that more is coming in two
places, both drawn by the stylesheet, so the page says it with script off too:

- **A spinner after the content so far**, centred, 18px, in the blue of the bar's own. It is sent with the shell, before
  the content, and drawn after it (the content region is a column when the page is arriving, and the spinner has the
  last `order`), so it is always at the end of what has arrived. What ends the page, which is sent after the last panel,
  hides it (`.arriving:not(:has(~ .arrived))`). Where `:has()` is not supported it is not shown at all.
- **The status bar in two parts.** The request, the method and the address, is in the shell, with a small spinner and
  *loading* where the rest of the bar will be. How the page came to be (the server's time, when it was built, the
  *JS off* and *stale page* badges) is sent when the last panel is, as a bar of its own, 25px high and fixed over the end of the
  first, which it covers. Together they read as the bar of a page that arrived whole.

If the page ends without the mark that ends it, because the connection broke or the server stopped, the browser still
says it is done; with script, the spinner goes and the bar says *incomplete* in the warning colour, since a page that is
still waiting after it has stopped is the worse lie. Two consequences are part of the design and are not hidden:

- **The page grows.** Content below an arriving panel moves down as it comes, because no room is kept for it. A
  panel that is ready but follows a slower one waits for it, so the page is never drawn out of order.
- **The status bar is the honest part.** Its build time, drawn last, says how long all of the page took, not how long
  the first of it did.

With script off the page arrives the same way, since nothing here needs script.
A page with an editor, a diff or Markdown is built whole, because its head depends on its panels, and so arrives as one
piece.

---

## 14. Responsive

One breakpoint that matters: **820px**, where the sidebar collapses to a 56px
icon rail and the search field is dropped. Two minor ones: **1100px**, where a
split becomes a single column, and **760px**, where paired form fields stack.

The design does not attempt a phone layout. A control plane on a phone is a
different product.

---

## 15. Accessibility

- Every text colour a reader needs clears 4.5:1 on its surface in both themes:
  text, text-secondary, the semantic text colours, a badge's text on its own tint, and
  a white label on the primary and destructive fills, including on hover. Text
  tertiary clears 3:1 and is for decoration only. A test computes these from the
  stylesheet's tokens, so a palette edit that slips under fails.
- Status is never colour alone — every badge carries its word.
- Focus is a visible 2px ring, never removed.
- Every interactive element is a real link or a real button, so keyboard
  traversal and "open in new tab" work without any extra handling.
- Descriptions are the information icon's accessible name and also appear in a
  popover on hover and focus, so they are available to assistive technology and
  not only to a pointer.
- A popover stays in view. A description's opens to the right of its
  icon and below it; with script it is moved left until its right edge is inside
  (8px margin), and above the icon when there is no room below. "Inside" is the
  window less the status bar's 26px, and the visible part of each box that scrolls or
  clips and holds the icon, its scrollbar not being part of it: a popover that is
  inside the window but under the scrollbar of the page's own scroll area is cut off all
  the same. A reason on a control that is not allowed is centred
  over it, and anchored to the near side when the control is at an edge, with a
  width that is at most 280px, wrapped. Without script they open where the
  stylesheet puts them, which is the cost of not measuring.
