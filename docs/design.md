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
| Text tertiary | 40% of Text | `#8e9197` | Hints, placeholders, section captions |
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
| **Healthy** | `#6ccf8e` | `#1a7f4b` | Loaded, acknowledged, ok |
| **Warning** | `#ff9830` | `#b5510d` | Degraded, open, active tab underline |
| **Critical** | `#e5484d` | `#cf0e5b` | Rejections, failures, destructive actions |

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
| Table header, field label, secondary | 12.5 | 500 | Text secondary |
| Hint, description | 12 | 400 | Text tertiary |
| Section caption | 11 | 500, uppercase, `.05em` | Text tertiary |

Identifiers, addresses, timestamps and payloads use a monospace face at 12–12.5
in Text secondary. Anything the user might copy is monospace. Numeric columns
use tabular figures and right alignment.

Nothing is bold. 500 is the heaviest weight in the system; 600 appears only in
full-page error headings.

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
to a 56px icon rail instead. The structure of the application is the one thing
that should not disappear.

Contents, top to bottom: brand mark and product name (48px, matching the top
bar so the two align); grouped navigation; a footer pinned to the bottom.

Navigation items are 32px, 13.5px, text-secondary, with a 16px icon. Groups
carry an 11px uppercase caption above them. An item can hold a count on the
right as a small pill. The active item takes the Active background, full-
strength text, weight 500, and a 2px primary-coloured bar flush against the
left edge. Expandable groups show a chevron that rotates 180° when open;
children indent to 42px and drop their icons.

### Top bar

48px, panel surface, sticky. Breadcrumbs on the left — parents at text-
secondary, the current page at full strength and weight 500, separated by `›`
at text-tertiary. On the right: a 290px search field, then the page's *Refresh*
icon button.

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

### Status bar

26px, fixed to the bottom, monospace 11.5. Shows the method and address of the
request behind what is on screen, and a spinner with a short label while
something is in flight. It is the honesty bar: if the screen changed, this says
what changed and how much of it.

---

## 8. Panels

The unit of content. A bordered rectangle on the panel surface with a 32px
header and no divider between header and body.

The header carries, left to right: the title (14/500); an information icon
holding the description as a popover; and an optional status dot. A panel has
no menu of its own: reloading is the page's Refresh in the top bar, so a panel
changes only by following its own links, and the header stays quiet.

A panel may have a footer separated by a 1px rule: pagination on the left,
navigation or actions on the right.

Panels never nest. If content inside a panel needs its own frame, it gets a
bordered group with an 11px uppercase caption instead.

### Rows

Panels can be grouped under a collapsible row header: a 30px clickable line
with a rotating chevron, a 14/500 label, and an optional count in text-
tertiary. Collapsing hides the panels beneath it. Rows are how a long page is
made scannable without pagination.

### Arrangements

- **Stacked** — panels in a column, 8px apart.
- **Split** — two columns, 8px apart, top-aligned so unequal heights don't
  stretch. Optionally weighted 1.6 : 1. Collapses to one column below 1100px.
- **Tabbed** — a 1px-ruled strip of 13.5px labels; the active one takes full-
  strength text, weight 500, and a 2px warning-coloured underline. Tabs are
  addressable: the selected tab is in the address bar, so a tab can be linked
  to and survives a reload. The strip is the control.

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
permissions and never silently fails when pressed.

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
may carry a 6px leading dot in the same colour.

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

---

## 11. Feedback

**Toasts** stack bottom-right above the status bar: raised surface, 3px left
border in the semantic colour, a 13.5/500 title and an optional 12.5
text-secondary line, rising 6px on entry and dismissing after 3.4s.

**Field errors** appear under the field in 12/critical with a 13px icon, and
the field takes the invalid border. Two kinds of rejection are shown
identically: those the browser catches before anything is sent, and those the
server returns afterwards. From the user's side they are the same event, so
they look the same.

**Notes** — used in this mock to explain the system — are a bordered block with
a 3px primary left border on the panel surface, 12.5 text-secondary, with
full-strength lead-ins. Warning-orange is used inside a note to mark something
unresolved.

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

.sidebar   { view-transition-name: sidebar; }
.topbar    { view-transition-name: topbar; }
.content   { view-transition-name: content; }
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

---

## 14. Responsive

One breakpoint that matters: **820px**, where the sidebar collapses to a 56px
icon rail and the search field is dropped. Two minor ones: **1100px**, where a
split becomes a single column, and **760px**, where paired form fields stack.

The design does not attempt a phone layout. A control plane on a phone is a
different product.

---

## 15. Accessibility

- Text-secondary on panel and text on canvas both clear 4.5:1 in both themes;
  text-tertiary is used only for supporting text that is never the sole carrier
  of meaning.
- Status is never colour alone — every badge carries its word.
- Focus is a visible 2px ring, never removed.
- Every interactive element is a real link or a real button, so keyboard
  traversal and "open in new tab" work without any extra handling.
- Descriptions live in title attributes on the information icon, so they are
  available to assistive technology rather than only on hover.
