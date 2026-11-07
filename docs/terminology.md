# Terminology

The words of the library, and the words of its insides, kept apart. A **library term** is one that a person using the library reads or writes: an exported identifier, a field, godoc, `doc.go`, the README, `skills/webui/SKILL.md`, a compile error, a name on the wire (a form field, a header, a data attribute, an address parameter). An **internal term** is one that only the code behind it needs: `internal/`, unexported names, and the comments there.

> The code is being brought to this file in steps (the `View` type, the names on the wire, the prose, the internals). Until the last lands, a name in the "was" notes below may still be in the code.

Two rules follow, and a test holds the first once the last step lands (in `pkg/webui`: the public docs say no internal word):

1. A library term never carries an internal word. Text a user can read does not say IR, node, lower, walk, gate, verdict, leaf, chrome or navbar.
2. One word means one thing in one layer, and one thing has one word in a layer. When a new word is needed, it goes in this file first, in the right half.

This file says what the words are. [`design-decisions.md`](design-decisions.md) says why the API is shaped as it is.

## Library terms

### What a page shows

| Term | Means | Where | Not |
| --- | --- | --- | --- |
| **View** | Anything a page shows, and the unit that a page, a tab, a column and a `When` each hold. A tree: a layout holds Views. | `webui.View`, `Page.View`, `Tab.View`, `Column.View`, `When.View` | PageBody, Body, "body", "bodies", "child" |
| **layout** | A View that arranges other Views and shows no data of its own: `Stack`, `Split`, `Columns`, `Tabs`, and `When`, which loads for its condition alone. | the five types; "a layout" | `Cards.Layout` is how cards are laid out, and says so |
| **panel** | A View that loads its own data and refreshes on its own: `Table`, `Form`, `Editor`, `Diff`, `Markdown`, `List`, `Cards`, `Calendar`, `Toolbar`, `FileUpload`. Drawn in a frame, but for a `Toolbar`, which has none. | the ten types; "a panel's `Load`", "the panel says Could not load" | leaf |
| **view id** | A View's position in the page: `p` for the page's View, `p.0` for the first in a `Stack`, `p.1.0` for the first in the second. Stable. | the form field `_view`, the header `X-Webui-View`, `data-view`, element ids that start with it | `_leaf`, `X-Webui-Leaf`, `data-leaf`, "leaf address" |
| **view state** | What a View keeps in the address: a table's sort, filters and offset, a list's limit, a calendar's month, the selected tab. Named by the View's title as a slug, then `.sort`, `.limit`, `.month`, `.tab`. | `devices.sort=name`, `tabs.tab=raw-events` | |

A page's View is a panel or a layout. There is no other kind, so a new kind of View is one or the other.

### What a View holds

| Term | Means | Notes |
| --- | --- | --- |
| **model** (`M`) | The value a panel's `Load` returns and its accessors read. | One per panel. A `When`'s `Load` reads one for its condition. |
| **row**, **item**, **day** | A model as a `Table`, a `List` or `Cards`, or a `Calendar` shows it. | Use the word of the panel; `model` where it is the same for all. |
| **subject** | What an action acts on: the row, the selected rows, the model, or the `FileHandle`. | `Action[S]`. A toolbar's action has none (`NoArgs`). |
| **accessor** | The one description of a value: how it is read, shown, edited and checked. | A **column** where a `Table` shows it, a **field** or **input** where a `Form` does. |
| **Loader**, **Loaded** | Where a table's rows come from; how a `Load` ended (`Ready`, `Failed`, `RefreshAfter`). | |
| **control** | An `Action`, a `Link` or an `Href`: what a row, an item or a day does when it is used. | `Control` is the interface. |
| **toolbar item** | What a `Toolbar` holds: an `Action`, a `Link` or a `Form`. | `ToolbarItem`. A `Form` here shows values and has no inputs. |

### Pages, addresses and visitors

| Term | Means | Notes |
| --- | --- | --- |
| **address** | A URL of the app: the path and the query. | Not a position (a **view id**), not a picture's source (its **URL**), not a link's target when that is a web address (a **web address**). |
| **path**, **argument**, **`Args`** | The part of an address that names a page, and the typed value that a path and a query make. | A `Page[A]`'s `A` is its arguments. |
| **Guard** | A function that says whether a visitor may open a page or use a control. It **refuses**. | A `Refusal` is what a visitor may be told (`NotFound`, `Unauthorized`, `Forbidden`). |
| **Outcome** | How an action ended: `Success`, `Warning`, `Failure`, `Reject`. | An outcome **rejects** when the input was wrong; a Guard **refuses**. They are not the same word on purpose. |
| **visitor** | The person using the app. | Not "viewer", not "user" (the user is the developer who writes the declaration). |
| **sidebar**, **entry** | The list of pages at the side, and one line in it. `Nav` is an entry, `NavEntry` is one of several that a page lists. | Not "navbar", "navigation" or "menu". |
| **top bar**, **status bar**, **breadcrumb**, **crumb** | The strip with search, Refresh and the theme; the bar that says how old the page is; the trail of parents and one step in it. | Not "shell" or "chrome". |

## Internal terms

These do not appear in anything a user reads. Each is one thing in one layer.

| Term | Means | Where |
| --- | --- | --- |
| **IR** | The data that `Compile` makes of the declaration, and that the runtime and the renderer read. No transport, no generics. | `internal/ir` |
| **node** | An IR value for one View: `ir.Node`, with `Pos()` and `Children()`. A pass loops over `Children()` and does not list the layouts. | `internal/ir` |
| **pos** | A node's position: the path of child indices from the page's root, which the **view id** writes down. A position is not an address. | `ir.Pos` (was `ir.Addr`) |
| **lower** | Turn a declared View, accessor, control or loader into its IR. `Compile` is the public word for the whole of it. | `lowerView`, `lowerAccessor`, `lowerControl`, `lowerLoader` |
| **walk** | Visit every node, a parent before its children. | `ir.Walk` |
| **part** | What the runtime builds for one View: concurrently, and sent in order. A part is made for a layout too. | `internal/runtime` (was `panel`) |
| **props** | A node with its loaded data, which a template draws. | `render.TableProps` and the others (were `TableView`...) |
| **frame** | The bordered region with a header that a panel is drawn in. | `components.PanelProps` |
| **gate** | The in-process handler that says whether an address may be opened, by running its page's Guard. It is asked through `mayOpen`, `mayFollow` and `mayLead`. | `Program.gate` |
| **verdict** | One Guard's answer for one address, kept for the length of a request. | `verdictCache` |
| **access** | A control as it is drawn for a visitor: hidden, or disabled and why. Made of a Guard's answer. | `render.Access` (was `render.Gate`) |
| **condition** | A `When`'s test, and the map from a node to the `When`s above it. | `conditionsOf` (was `gatesOf`) |
| **shell inputs** | What the sidebar and the brand need and the runtime fetches beside the panels. | `internal/runtime/shell.go` |
| **facts**, **view validator** | What `Compile` knows across pages, and the state of checking one page's View. | `pkg/webui/validate.go` |

## How the layers name one thing

| In the declaration (library) | In the IR | In the runtime | In the renderer |
| --- | --- | --- | --- |
| a View | a node | a part | a component |
| a panel | a node that loads | a part that loads | a frame, and `TableProps` |
| an accessor | `ir.Field` | | a cell or an input |
| `Split` | `ir.Columns` of equal weight | | |
| `List`, `Cards` | `ir.Items` | | |
| `Editor` | `ir.Form` with an editor | | |

## Words kept as they are, on purpose

- **body** means an HTTP body (`maxBody`, "a multipart body") and nothing else. A View is never a body.
- **`Cards.Layout`** is how a card list is arranged (a grid or a row that scrolls). It is a field of one panel and not a layout in the sense above.
- **`args.ViewKey`**, "view state": the state of a **View** in the address. A `Tabs` is a View with state too, so this is the word for all of them.
- **"Views:"**, the label a tab strip gives to assistive technology: a tab holds a View.
- **view transition**, the CSS feature. It is the browser's.
- **`.panel`** in the stylesheet is the frame of a panel.
