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
has no filter icon. Nothing is greyed out except things you could do if you were
someone else.

**Square, not soft.** 2px radius everywhere. Shadows only on things that float
(menus, toasts). No gradients, no glass, no decorative borders. The visual
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

Five meanings, no more. Each has a text colour, a 14% background, and a 30%
border, so a badge can be built from one token family.

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
| Description | 12 | 400 | Text secondary |
| Section caption | 11 | 500, uppercase, `.05em` | Text secondary |

Identifiers, addresses, timestamps and payloads use a monospace face at 12–12.5
in Text secondary. Anything the user might copy is monospace. Numeric columns
use tabular figures and right alignment.

Nothing is bold. 500 is the heaviest weight in the system; 600 appears only in
full-page error headings. The exception is the strong emphasis of a
`Markdown` leaf, which is 600: in running text 500 cannot be told from 400, and a
reader must be able to see what the author stressed.

---

## 6. Metrics

Everything is a multiple of 2, and the working gutter is 8.

| Element | Size |
|---|---|
| Panel gutter | 8 |
| Page padding | 8 vertical, 12 horizontal |
| Corner radius | 2 (everything, without exception) |
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

Fixed 240px on the left, panel surface, full height, always visible. It never
becomes a top bar and never turns into a hamburger — below 820px it collapses
to a 56px icon rail instead, and a button in its footer collapses it to the same
rail at any width (the choice is a browser preference). The structure of the
application is the one thing that should not disappear.

Contents, top to bottom: brand mark and product name (48px, matching the top
bar so the two align); grouped navigation; a footer pinned to the bottom.

Navigation items are 32px, 13.5px, text-secondary, with a 16px icon (a Font
Awesome glyph; an item with none holds the place of one, and shows its label's
capitalised initial in the rail). Groups carry an 11px uppercase caption above
them. In the rail the caption has no room, so it becomes the design's separator, a middle dot (`·`, token
`--sep`, 18px, `--text-3`) centred in 17px of height for each group, and the caption is its tooltip. Nothing is
drawn above the first group. A line was tried first and dropped: the rail already has one under the brand, and
a line says *edge* where a group says *these belong together*. The same dot separates the figures of the status
bar (*0.3 ms · loaded 32 ms · built 10:14:18 · just now*), so the design has one separator and lines are for
edges. The active item takes the Active background, full-strength text, weight
500, and a 2px primary-coloured bar flush against the left edge. An item the
visitor may not open is not drawn.

### Top bar

48px, panel surface, sticky. Breadcrumbs on the left — parents at text-
secondary, the current page at full strength and weight 500, separated by `›`
at text-tertiary. On the right: a 290px search field, then the page's *Refresh*
icon button, then, when the app declares external links, a button (the arrow-out-of-box
icon) that opens a popover list of them: a raised surface right-aligned beneath the button, 32px items with
a 16px icon and an 11px uppercase caption above a section, closed by a click elsewhere
or Escape. A link to a web address opens in a tab of its own; a path on the site and a mail
address open in the current one.

The breadcrumb is where location is expressed when the sidebar cannot show it
— for example on a detail page that intentionally has no navigation entry of
its own.

### Refresh

An icon button beside the search. It reloads the page at the address it is on,
state in the address and all. There is no time-range picker and no auto-refresh
interval: this is a control plane, not a dashboard — it refreshes on demand, it
does not poll. There is no row of page parameters: what a table shows is
filtered in its column headers (§10).

### Content

8/12 padding, scrolls independently, bottom padding clears the status bar.

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

On the right it says how the page came to be, in the muted text colour: how long the
server took to build it (`0.1 ms`), how long the browser took to get and show it
(`loaded 53 ms`), and when its HTML was built, in the viewer's time with its age
(`built 14:49:31 · just now`). Two small uppercase badges in the warning colours say what
the viewer should know: **JS off**, drawn only where script is off, and **cached?**, shown by
script when the page was not built for the request that brought it. The timings are
hidden below 720px; the badges are not. A panel fetched again updates all of it.

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

Panels never nest. If content inside a panel needs its own frame, it gets a
bordered group with an 11px uppercase caption instead.

### Arrangements

- **Stacked** — panels in a column, 8px apart. A stack that has height to give, because it is a cell of a
  Split beside something taller or the body of a tab in one, gives it to its tables, which share it, and to its last
  panel when it has none, so the column ends where the row does.
- **Split** — two columns, 8px apart. Panels side by side are as tall as the tallest
  of them, so their borders line up; when a Split wraps onto more rows, the heights
  are equal within a row and not across rows. A panel's foot (a pager, the actions)
  stays at the bottom of its box. Optionally weighted 1.6 : 1. Collapses to one
  column below 1100px, where nothing is stretched. Only panels that are direct
  children of the Split are stretched; a layout inside it, such as Tabs, is one
  column cell, and what is under its strip (a panel, or a stack of them) fills what is left of the cell.
- **Tabbed** — a 1px-ruled strip of 13.5px labels; the active one takes full-
  strength text, weight 500, and a 2px warning-coloured underline. Tabs are
  addressable: the selected tab is in the address bar, so a tab can be linked
  to and survives a reload. The strip is the control.

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

**Inputs** are 32px on the input surface with a strong border; focus takes a
primary border and a 2px primary ring at 25%. Read-only fields drop to the
hover surface with text-secondary and a weak border — visibly not a hole you
can type into. Invalid fields take a critical border and ring.

**Selects** replace native chrome with a small CSS chevron so they match input
height and weight exactly.

**Checkboxes** are 15px, 2px radius, filling solid primary with a white check
when set.

**Switches** are 30×16 pills, used only for session-level modes, never for
saved data.

**Disabled vs. gated.** Disabled is 45% opacity with no explanation. *Gated* —
a control you are not permitted to use — is also 45%, but reveals a raised
tooltip on hover saying why. A control is never silently removed because of
permissions and never silently fails when pressed. The exception is a form's
Save: a form the visitor may not submit is shown as read-only output, with a Back
button in place of Save and Cancel, since a form of inputs that cannot be sent
would only invite typing.

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
hover. Selected rows take a 13% primary tint.

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
figures. Identifiers are monospace text-secondary.

**Badges** sit in status columns: 19px tall, 2px radius, lowercase 11.5/500,
built from a semantic family's 14% background and 30% border. A status badge
may carry a 6px leading dot in the same colour. A select that holds several values
is a badge for each, 4px apart, wrapping. Edited, a select is a 32px drop-down list in the
style of an input, and with several values a list of up to six rows, with the chosen
ones on the selected-row tint.

**Inline bars** are a cell display mode, not a chart: a 56×5 track on the hover
surface with a link-coloured fill, sitting after a right-aligned number. This
is the one place quantity is shown as a shape, and it is deliberately the
smallest possible version of that idea.

**Bulk selection** adds a 34px checkbox column and, when anything is selected,
an action bar above the header: a 10% primary tint strip with the count, the
available operations, and a *Clear* pushed to the right. When there are no bulk
operations, neither the column nor the bar exists.

**Loading** replaces the row bodies with pulsing 9px skeleton bars at staggered
widths while the rest of the panel — header, filters, footer — holds still.
**Empty** replaces them with a centred 28px icon, a 13.5/500 line naming the
condition, and a 12.5 text-tertiary line naming the way out.

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
text-secondary line explaining that the check ran *before anything was loaded*,
and a secondary button back. No data is shown, because none was read.

**Failed to compile.** The whole application is replaced, at every address,
until it is fixed — modelled on a build tool's error overlay rather than a
server error page. A 22/600 critical heading, a count, and then every problem
at once: a monospace caption locating it, the message in a critical-tinted
block with a 3px left border, and a **Fix:** line in plain language. Problems
are never reported one at a time.

---

## 13. Motion

Motion exists to say *something is happening* and nothing else.

- Hover and border transitions: 110–130ms.
- Toast entry: 160ms rise.
- Skeleton pulse: 1.1s, 100% → 40%.
- Spinner: 700ms linear.

Nothing slides, nothing bounces, nothing animates on first paint.

### Navigation

A whole-page navigation cross-fades, and the cross-fade is **entirely CSS**:

```css
@view-transition { navigation: auto; }

.sidebar   { view-transition-name: wui-sidebar; }
.topbar    { view-transition-name: wui-topbar; }
.content   { view-transition-name: wui-content; }
```

The browser snapshots the old document, loads the new one, and animates each
named region independently. The sidebar is its own snapshot and is identical on
every page, so it does not appear to move — the shell stays put and only the
content changes. This is the visual reward for making every control a real
link, and it costs no script at all.

Duration is 180ms, eased. `prefers-reduced-motion: reduce` sets
`navigation: none` and zeroes the region animations.

Support is Chrome 126+ and Safari 18.2+; Firefox has same-document view
transitions and the cross-document form is pending. Where it is unsupported the
navigation is an ordinary one — which is what it would have been anyway, so
there is nothing to fall back to.

**A whole-page cross-fade and a panel skeleton mean different things and must
never be substituted for each other.** The cross-fade says *the page moved*.
The skeleton (§10) says *this one panel is reloading and everything else you
can see is still current*. Fading the whole content region while one panel
reloads would claim that all of it is stale, which is false.

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
  *JS off* and *cached?* badges) is sent when the last panel is, as a bar of its own, 25px high and fixed over the end of the
  first, which it covers. Together they read as the bar of a page that arrived whole.

If the page ends without the mark that ends it, because the connection broke or the server stopped, the browser still
says it is done; with script, the spinner goes and the bar says *incomplete* in the warning colour, since a page that is
still waiting after it has stopped is the worse lie. Two consequences are part of the design and are not hidden:

- **The page grows.** Content below an arriving panel moves down as it comes, because no room is kept for it. A
  panel that is ready but follows a slower one waits for it, so the page is never drawn out of order.
- **The status bar is the honest part.** Its build time, drawn last, says how long all of the page took, not how long
  the first of it did.

The cross-fade of a navigation (above) still runs: it is the browser's, between two documents, and a page that is
sent as it is built is one document. With script off the page arrives the same way, since nothing here needs script.
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
