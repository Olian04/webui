# components

The design language of `docs/design.md`, expressed as templ components.

This package renders. It does not load data, decide policy, or know what a page
is. **Nothing here imports `internal/ir`** — the mapping from IR to props
belongs to `internal/render`, which keeps the design language usable and
testable on its own and stops every IR change from rippling into markup.

---

## The API rules

Six rules, applied without exception. They are the whole of the design.

### 1. One props struct per component, zero value is the default

Go has no optional arguments. A `<Component>Props` struct is the closest thing,
and it means a field can be added later without touching a single call site.

```go
@c.Button(c.ButtonProps{Href: "/device", Variant: c.VariantPrimary}) { Save }
```

The zero value is always the sensible default, which is why `VariantSecondary`
is `""`: a page may have no primary action, but it may not have two.

### 2. Choices that will grow are typed strings, never booleans

`Tone` can gain a sixth meaning. `isWarning bool` cannot — it can only be joined
by `isError bool`, and then the two can both be true.

```go
type Tone string
const (ToneNeutral Tone = ""; TonePrimary; ToneOK; ToneWarning; ToneCritical)
```

There are five tones because the design has five meanings. A sixth constant
means the design grew a meaning it has no colour for, and that is a design
decision, not a call site's.

### 3. The main content is `{ children... }`. Everything else that takes markup
is a slot

A slot is a `templ.Component` field. Make the common configurable; make the
uncommon composable. `Panel` has `Title string` because every panel has one, and
`Actions templ.Component` because the panel has no opinion about what goes
there.

```go
@c.Panel(c.PanelProps{Title: "Devices", Footer: pager()}) { table }
```

The test for "should this be a prop or a slot": if the component would have to
*know* about the thing to render it, it is a prop. If it would only have to
*place* it, it is a slot.

### 4. Every component carries `Attrs templ.Attributes`

This is why the package does not grow a prop every time a caller needs an `id`,
a `data-`, an `hx-` or an `aria-`. One escape hatch, present everywhere, is the
difference between a library that stays this size and one that doesn't.

### 5. Absence is the configuration

A table that declares no bulk operations gets no checkbox column — not a
disabled one. A page with no arguments renders a sentence saying so, not an
empty strip. Components express this by having *no* `ShowCheckboxes bool`:
you either call `SelectCell` or you don't.

Consequence: there is no `Sortable bool` on `Column`. A non-empty `SortHref` is
what makes a column sortable, because a sortable column with no address is not
a thing this design can render.

### 6. Controls that navigate are links

`Button` takes `Href` and renders `<a>`. This is not a convenience. It is what
makes the UI work with scripting disabled, middle-clickable, and keyboard
reachable without any help. A `<button>` is for submitting a form and nothing
else.

---

## Catalogue

Grouped the way MUI groups its own, which is a taxonomy worth borrowing even
when the component set is not.

| File | Components |
| --- | --- |
| `types.go` | `Tone`, `Size`, `Variant`, `Align`, `SortDir` |
| `icon.templ` | `Icon` — any Font Awesome Free solid icon, by name |
| **Inputs** | |
| `button.templ` | `Button`, `IconButton` |
| `field.templ` | `Field`, `FieldRow`, `Input`, `Checkbox`, `Switch`, `Form`, `Rules` |
| **Data display** | |
| `table.templ` | `Table`, `TableHead`, `TableBody`, `Column`, `Row`, `RowLink`, `Cell`, `SelectAllColumn`, `SelectCell`, `ActionBar`, `Gauge`, `Pager` |
| `badge.templ` | `Badge` |
| **Surfaces** | |
| `panel.templ` | `Panel`, `InfoTip` |
| **Layout** | |
| `layout.templ` | `Stack`, `Split`, `Tabs` |
| **Navigation** | |
| `nav.templ` | `Sidebar`, `SidebarFooter`, `SidebarCollapse`, `SidebarRow`, `NavSection`, `NavItem` |
| `topbar.templ` | `TopBar`, `Breadcrumbs`, `Search` |
| **Feedback** | |
| `feedback.templ` | `Toast`, `ToastRegion`, `EmptyState`, `StatusBar` |
| `state.templ` | `StatePage`, `CompileError` |

The document and shell around them are in `../shell.templ` and `../page.templ`.
`internal/render` assembles pages from these and nothing else; `go run ./cmd/demo`
shows the result.

---

## Two seams worth knowing

**`Row` and `RowLink` share an address.** An anchor may only live inside a cell,
so a clickable row is `Row{Href}` plus exactly one `RowLink` in one of its
cells, with the same address. The stylesheet stretches that anchor over the
whole row, which is what makes the row one tab stop rather than six.

The alternative — putting the href in templ's context in `Row` and reading it in
`Cell` — removes the duplication but adds implicit behaviour to a library whose
whole claim is that nothing is hidden. The duplication is two characters and is
generated in one place by `internal/render`, so it was the better trade. If that
stops being true, the context version is the move.

**`Rules` mirrors `ir.Rules` without importing it.** It is deliberately a
restatement — "the HTML attributes a rule set produces" — so that the component
library does not depend on the IR. `internal/render` maps between them. If the
two drift, the mapping function is the one place that breaks.

---

## Generating

`*_templ.go` is generated by `make generate` (`go tool templ generate ./...`), which
`make test`, `make lint` and `make build` run first. templ is pinned in the `tool`
block of `go.mod`, so the CLI version is pinned with everything else. The generated
files are committed, so the module builds for anyone who imports it, and CI fails
if they are out of date; run `make generate` after editing a `.templ` file. They are
excluded from lint, since generated code will not satisfy `revive`'s `unused-parameter`.
