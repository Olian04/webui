# webui

Declarative admin panels and control planes. No HTML, CSS or JavaScript — only Go.

## WIP

```go
package main

import (
  "bytes"
  "context"
  _ "embed"
  "image"
  "image/png"
  "log"
  "net/http"

  "github.com/Olian04/webui/pkg/webui"
)

//go:embed resources/logo.png
var logoBytes []byte

var DevicesNav = webui.Nav{Label: "Devices"}

// A page's arguments are one struct. Fields named in the Path are path
// segments; the rest are query parameters.
type DetailsArgs struct {
  Id    string // -> /device/{id}
  Debug bool   // -> ?debug=true
  Limit int    // -> ?limit=50
}

// An accessor is a named, typed projection of Device, optionally writable.
// A table renders one as a cell, a form renders one as an input, so the same
// value is listed in both — the getter is written once, not twice.
var (
  ID = webui.String[Device]{
    Label: "ID",
    Load:  func(d Device) string { return d.Id },
    // No Store == read-only
  }
  IP = webui.String[Device]{
    Label: "IP",
    Load:  func(d Device) string { return d.Ip },
    Store: func(d *Device, val string) { d.Ip = val },
    // Rules are data, so they render as HTML constraint attributes on the
    // client and re-run on the server before Submit.
    Rules: webui.StringRules{
      Required: true,
      MinLen:   7,
      Pattern: &webui.PatternRule{
        Expr:    `^\d{1,3}(\.\d{1,3}){3}$`,
        Message: "must be a valid IPv4 address",
      },
    },
  }
  Occurrences = webui.Int[Device]{
    Label: "Occurrences",
    Load:  func(d Device) int { return d.Count },
  }
  Rate = webui.Float[Device]{
    Label: "Rate",
    Load:  func(d Device) float64 { return float64(d.Count) / d.Duration },
  }
)

var Devices = webui.Page[webui.NoArgs]{
  Path: "/device",
  Nav:  DevicesNav,
  Body: webui.Table[Device]{
    Title: "Devices",
    // Every column header is a sort link, and shows a filter icon on hover. A
    // table keeps its sort, its filters and its page in the address under its
    // ID: ?devices.offset=50&devices.sort=ip&devices.filter.status=healthy. The
    // library owns those parameters and hands Load the result as a Query, and
    // Load does the sorting and the filtering. Query.Sort and Query.Filters are
    // keyed by the accessor's Key, or its Label when it has none, and anything
    // that is not a column is dropped before Load sees it. ID defaults to
    // "table" and must be unique within the page, so name it when a page has
    // more than one table.
    ID:       "devices",
    PageSize: 25, // 0 means the table does not page
    Load: func(ctx context.Context, q webui.Query) (webui.Rows[Device], error) {
      // Total is the count across all pages; leave it zero when unknown and
      // the pager falls back to "a full page may have a successor".
      return service.Page(ctx, q)
    },
    // A Link names its destination page as a field, so Compile can check the
    // target exists. It renders a real <a href>, so middle-click and
    // open-in-new-tab work; an Action here would render a POST instead.
    RowClick: webui.Link[Device, DetailsArgs]{
      Page: Details,
      Args: func(ctx context.Context, d Device) DetailsArgs {
        return DetailsArgs{Id: d.Id}
      },
    },
    // No Actions or BulkActions means no action buttons and no form.
    // No BulkActions means no row select checkboxes and no selection bar.
    // Either needs Key: a request names rows by identity, never by position.
    Columns: []webui.Accessor[Device]{ID, IP, Occurrences, Rate},
  },
}

var Details = webui.Page[DetailsArgs]{
  Path: "/device/{id}",
  Nav: webui.Nav{
    // Has no Label, so no entry of its own, but lights the "Devices" entry
    // when the page is loaded. Matched by value, so the target page's Nav must
    // be unique among pages; Compile says so if it is not.
    Shadow: &DevicesNav,
  },
  // Guard runs before anything is loaded — so an unauthorised device is never
  // read. It gates the UI on page load and authorises the request on every
  // section fetch.
  Guard: func(ctx context.Context, a DetailsArgs) error {
    return auth.AssertDeviceAccess(ctx, a.Id)
  },
  Body: webui.Form[Device]{
    Title: "Configuration",
    // One typed lookup gets the whole argument struct.
    Load: func(ctx context.Context) (Device, error) {
      a, err := webui.ArgsOf[DetailsArgs](ctx)
      if err != nil {
        return Device{}, err
      }
      return service.Load(ctx, a.Id)
    },
    Submit: SaveDevice,
    Fields: []webui.Accessor[Device]{
      webui.Group[Device]{ID, IP},
      Rate,
    },
  },
}

var SaveDevice = webui.Action[Device]{
  Label: "Save", // the button text; a Form's Submit defaults to "Save"
  // Guard takes the action's subject. Same function on both sites: it enables
  // the UI element at render time and guards the request before Run.
  Guard: func(ctx context.Context, d Device) error {
    return auth.AssertCanEdit(ctx, d.Id)
  },
  Run: func(ctx context.Context, d Device) (webui.Effect, error) {
    if taken, err := service.IpTaken(ctx, d.Ip); err != nil {
      // Something the user cannot fix by editing the form.
      return webui.Effect{}, err
    } else if taken {
      // Understood and rejected: re-render with the error on the field.
      return webui.Effect{Fields: webui.Fields[Device]{
        {Field: IP, Message: "already in use by another device"},
      }}, nil
    }
    if err := service.Put(ctx, d.Id, d); err != nil {
      return webui.Effect{}, err
    }
    // Submit succeeded. Zero Effect would mean "stay here and re-render";
    // Redirect carries a Target when the action should navigate.
    return webui.Effect{Toast: "Device saved!"}, nil
  },
}

func main() {
  app := webui.App{
    Brand: webui.Brand{
      Name: "Demo",
      Logo: mustLogo(),
    },
    Theme: webui.Theme{}, // Default theme
    Pages: webui.Pages{
      Devices,
      Details,
    },
  }

  // Compile validates the app and prepares it: path matchers, argument
  // decoders, compiled patterns, leaf addresses. Nothing reflects per request.
  // The handler is never nil — on failure it serves the error at every path
  // under the prefix, so a broken app is diagnosable in the browser.
  // MustCompile is the fail-fast variant.
  handler, err := app.Compile("/admin")
  if err != nil {
    log.Println("webui:", err)
  }

  mux := http.NewServeMux()
  // The handler receives the full request path and strips the prefix itself.
  mux.Handle("/admin/", authMiddleware(handler))

  // Your own routes coexist; the library owns exactly its own subtree.
  mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
  })

  log.Fatal(http.ListenAndServe(":8080", mux))
}

func mustLogo() image.Image {
  img, err := png.Decode(bytes.NewReader(logoBytes))
  if err != nil {
    panic(err)
  }
  return img
}
```

## Running the demo

```
go run ./cmd/demo            # http://localhost:8080/admin/
go run ./cmd/demo -viewer    # guarded controls are disabled, with the reason
go run ./cmd/demo -broken    # the failed-to-compile page
```

Everything works with JavaScript switched off: every control is a real link or
form. The one script (`enhance.js`) replaces a single panel when a link inside it
changes only that panel's arguments, enables the selection bar and the page
search, and nothing else.

The structs declare the app; the compiled runtime is the target of every
runtime action; `ctx` is the gateway between them. So anything that touches the
runtime takes `ctx`, and anything that is a pure function of the model does not
— which is why an accessor's `Load` and `Store` stay `ctx`-free while `Open`,
the loaders, and every `Guard` take it.

`webui.Open` resolves the page through the runtime rather than reading the
declaration, so it returns a real href including the mount prefix, and a page
that was never mounted — or whose declaration was mutated after `Compile` — is
reported instead of silently producing a dead link:

```
mounted:     /admin/device/abc?debug=true
not mounted: webui: Open "/never-mounted/{id}": that page is not mounted in this app
mutated:     webui: Open "/CHANGED/{id}": that page is not mounted in this app
no runtime:  webui: Open "/CHANGED/{id}": no compiled app in this context
```

A `Link` names its destination as a field rather than building a `Target` in a
closure, so the target is data. Two consequences: the page and the argument
struct must agree, checked by the compiler —

```
cannot use Details (variable of struct type Page[DetailsArgs]) as Page[OrphanArgs] value
```

— and `Compile` walks the `Body` tree and checks every target is mounted,
however deeply nested:

```
page "/broken": a link targets "/never-mounted/{id}", which is not mounted in this app
  Fix: Add that page to App.Pages, or point the link at a page that is already
       there.
```

`Compile` validates the app and prepares it — path matchers, argument decoders,
compiled patterns, leaf addresses — so nothing reflects per request. The handler
is never nil: a failed compile still serves, rendering the failure at every path
under the prefix.

Failures are collected rather than reported one at a time, and each is
structured — which page, which argument type, what went wrong, and what to do
about it:

```
page "/device/{id}": the path declares {id} but DetailsArgs has no field for it
  Fix: Add a field named Id to DetailsArgs, or change the placeholder to match
       an existing field.
page "/org/{id}": two fields both map to the argument "id"
  Fix: Rename one field, or give it a distinct name with a `webui:"..."` tag.
page "/event/{id}": field When has unsupported type []string
  Fix: Arguments must be string, bool, int, int64 or float64 — a URL carries
       one value per name.
```

The same set renders as an HTML page, served at every path under the prefix
until the app compiles.

`MustCompile` is the fail-fast variant, mirroring `regexp` and `template`.

A zero path field is the one case reflection cannot catch at startup, so
`Open` reports it:

```
empty path arg: webui: Open "/device/{id}": path argument "id" is empty
```

"The current URL with one argument changed" needs no primitive — it is a
struct copy:

```go
next := cur
next.Limit = 50
return webui.Open(ctx, Details, next)   // /device/abc?debug=true&limit=50
```

`Open` builds an address from the page's own arguments. A table's paging and
sort and the selected tab are view state the library keeps in the address, so an
`Open` link lands on the defaults: first page, unsorted, first tab. A link a
user is *on* (pager, sort header, tab, Refresh) always carries the whole
address, so nothing it does drops another table's state.

A page's `Body` is a single `PageBody`. Leaves (`Table`, `Form`) load and
refresh; layouts only arrange, and they nest:

```go
Body: webui.Stack{ // vertical
  webui.Split{ // side by side, across models
    webui.Form[Device]{ ... },
    webui.Table[Event]{ ... },
  },
  // The selected tab is kept in the address as ?tabs.tab=Raw, so it survives a
  // reload and can be linked to. ID defaults to "tabs". Only the selected panel
  // is loaded.
  webui.Tabs{Panels: []webui.Tab{
    {Label: "Raw", Body: webui.Table[Event]{ ... }},
  }},
},
```

`Stack`, `Split` and `Tabs` are the layouts. All three hold `PageBody`, so none
of them constrains what their children are about. `Group[M]` is a different
thing one level down: accessor composition inside a form, typed to the model.

`webui.String`, `Int` and `Float` are the plain accessors. `Badge` shows a
string as a coloured pill (`Kinds` lists the values it can hold, each with its tone), and `Slider` shows a
number on a range: a bar when read-only — in a table, or in a form without a
`Store` — and a range input when it has one. Both take a `Label` and a `Load`
like the others, and `Slider` needs a `Min` and `Max`.

Every accessor takes an optional `Key`: what `Load` receives in `Query.Sort` when
a table is sorted by that column, or the `Label` when there is none. A form
ignores it. Within one table the keys (or labels) must tell the columns apart.

Context-specific presentation stays off the accessor. Options are decorators,
which are themselves accessors, so the common case stays a bare list:

```go
Fields: []webui.Accessor[Device]{webui.Placeholder[Device]{IP, "10.0.0.1"}},
```

The search box in the top bar lists the app's pages and is a keyboard
combobox: type, move with the arrow keys, Enter follows the highlighted result,
Escape closes it. A page adds its own results with `Search`, which receives what
was typed and returns links built with `Open`:

```go
var Devices = webui.Page[webui.NoArgs]{
  Path: "/device",
  Search: func(ctx context.Context, query string) ([]webui.SearchResult, error) {
    devices, err := service.Find(ctx, query, 8)
    results := make([]webui.SearchResult, len(devices))
    for i, d := range devices {
      results[i] = webui.SearchResult{
        Title:  d.Id,
        Desc:   d.Ip,
        Target: webui.Open(ctx, Details, DetailsArgs{Id: d.Id}),
      }
    }
    return results, err
  },
  ...
}
```

Results are grouped under the page's `Nav.Label`, at most eight from each page.
`Search` runs without the page's arguments, so the page's `Guard` runs first with
zero arguments and a page it refuses contributes nothing, and a page with path
arguments cannot offer `Search` at all — list the things from a page without
placeholders and `Open` the detail page. A result whose `Target` failed, or that
is not an address on this host, is dropped and logged.

## Open

- Validation is declarative. `Rules` is a per-value-type struct of data, not
  closures, so the same set renders as HTML constraint attributes (`required`,
  `minlength`, `pattern`, `min`, `max`) and re-runs on the server before
  `Submit`. A rule is a plain scalar when its zero value already means "no
  constraint" and its message is derivable (`MinLen`); a struct pointer when
  either fails — `Pattern` because a regex explains nothing to a user, `Min`
  because a zero bound is a real constraint and Go will not let you write `&0`.
  The set is closed: a rule the framework does not define cannot be added
  without writing client code, so anything beyond it goes in `Run`.
- `Store` is pure assignment with no error return, and no `ctx`. The ordering
  problem came from `Store` seeing `*M`; rules see only their own value, so
  inter-field dependencies are not expressible and need no ordering.
- Field errors ride on `Effect`, with a nil error. A validation rejection is a
  submission that was understood and refused and the user can fix; an `error`
  is something they cannot fix by editing the form. `Effect` stays non-generic
  by putting the model on the slice (`webui.Fields[Device]`), so bulk actions
  do not inherit a meaningless `[]FieldError[[]Device]`.
- Pipeline: parse -> rules -> `Store` -> `Guard` -> `Run`. `Store` applies to a
  copy of the model `Load` returned, not a zero one, so read-only fields (an ID
  with no `Store`) are intact for `Guard` and `Run`. Parse failures
  short-circuit, so a form with both a malformed number and a duplicate value
  shows the parse error first and the duplicate only on the next submit.
- After a successful action the browser gets a `303` and the toast rides a
  short-lived `HttpOnly` cookie, so a reload does not repeat the POST and the
  URL carries nothing. POSTs are checked with `http.CrossOriginProtection`
  (Fetch metadata and `Origin`); behind a proxy that rewrites `Host`, wrap the
  handler and allow the public origin.
- The re-render after a rejection must echo the raw submitted input, not the
  model value, or the user's bad input disappears and the form looks like it
  reset.
- `Pattern` is not portable: Go's RE2 rejects the lookahead and backreference
  syntax people copy from JavaScript examples. `Compile` compiles every
  `Pattern` at startup and the error says RE2, not "regex". The expression is
  anchored on both sides, because an HTML `pattern` attribute must match the
  whole value and the server has to reach the same verdict as the browser.
- Any struct a user fills with an unkeyed literal from another package trips
  `go vet`'s composites check, so nested literals in the API must expect keyed
  fields (`{Field: IP, Message: "..."}`).
- `Group` listed as a column is rejected by `Validate` rather than flattened —
  flattening would mean defining what a nested group is in a cell.
- A nested `Table` inside a form's `Fields` stays out of scope. If it turns out
  to be wanted, it is solved then.
- Refresh: done. A link inside a panel that changes only that panel's arguments
  fetches the *same URL* with an `X-Webui-Leaf` header and replaces that panel;
  without script it is an ordinary navigation. The page `Guard` runs on every
  request, leaf addresses are derived from the `Body` tree by position, and
  framework parameters never touch the URL. `Vary: X-Webui-Leaf` is set. After
  a leaf moves, sibling panels that embed the old address are fetched again and
  forms keep what was typed (only their `action` is patched). After an action
  the whole page re-renders; a model-typed `Stale[Device]` is deferred.
- Pagination and sorting: done, as library-owned view state. `Table.PageSize`
  turns paging on, every column is sortable, and `Load` receives a `Query` and
  returns `Rows{Items, Total}`, doing the sorting itself; `Total` below what has been shown
  means unknown. Setting a filter returns every table to its first page.
- Column filtering: done, as library-owned view state, like sorting. Hovering a
  header shows a filter icon; it opens a form that is a plain GET, so it works
  without script. A column with a fixed set of options — a `Badge`, whose options
  are the keys of its `Kinds` — gets a multi-select of exactly those, a numeric
  column (`Int`, `Float`, `Slider`) a minimum and a maximum, and every other
  column a text input. `Load` receives them in `Query.Filters` (the options
  chosen, or the one text typed) and `Query.Ranges` (a `Range{Min, Max *float64}`,
  inclusive, either end optional), keyed like `Query.Sort`. A number is compared
  as a number, never as text: "5" is not a way to ask for more than 5. A chosen
  option that is not an option, an empty text, a bound that is not a finite
  number and a column the table does not have never arrive, and a typed filter is
  cut at 200 characters. A multi-select is repeated parameters
  (`devices.filter.status=a&devices.filter.status=b`), which is also what a form
  of checkboxes submits; a range is `devices.min.count=300&devices.max.count=500`. The page has no row of argument controls:
  its query arguments come from the address (a link, `Open`) and have no UI.
- Bulk-action gating: `Action[[]Device].Guard` receives the selection, so
  gating happens at execution rather than per-row at render. Row actions are
  gated at render, per row, by the same `Guard` that authorises the POST.
- Non-string path arguments: built-in decoding for integers and
  `encoding.TextUnmarshaler`, or an escape hatch for the rest.
- Arguments are one struct per page. `Open` is compiler-checked, `Guard` is
  typed, and five concepts (`Arg`, `Args`, `Binding`, `Set`, `Get`) collapse
  into a struct literal. Cost: reflection enters the user-facing path for
  encode/decode, with `Validate()` in front of it so failures land at startup;
  `Pages` goes back to an interface slice since `Page[A]` differs per `A`;
  cross-page reuse becomes struct embedding rather than shared vars; and a zero
  field cannot be distinguished from an absent one without a pointer.
- Optionality: a zero field means absent and stays out of the URL. A field
  where zero is meaningful needs a pointer — the same trade as `Rules`.
- Transport parameters travel as headers, never in the URL; *view state* is in
  the URL, because it is what a copied address has to reproduce. The URL
  carries the page's arguments and the view state of its tables and tabs,
  headers carry read-side transport (`X-Webui-Leaf`), form bodies carry
  write-side routing. View state is named `<id>.<param>` (`devices.offset`,
  `devices.sort`, `devices.desc`, `tabs.tab`), and an argument name may not
  contain `.`, so the two can never collide; `Compile` rejects one that does.
  An ID is a lower-case word, unique within its page (two pages may reuse one),
  and defaults to the component's name. Parameters for a leaf the page does not
  have are ignored. Consequence: an iframe cannot set headers, so embedding a
  leaf later needs its own path and a standalone document, rather than reusing
  the refresh mechanism.
- Why the library owns view state instead of the argument struct: the struct
  would have to carry `Offset`, `Sort`, `Desc` and `Tab` fields for the
  framework's benefit, and the table would name them by string. The cost is
  that nothing outside the leaf can read or set that state — `Guard` and
  sibling leaves cannot depend on the selected tab, and `Open` cannot link to a
  sorted table.
- Pages that link to each other cannot be written as static `var`s. A list whose
  rows `Link` to a detail page, whose form action redirects back with
  `Open(ctx, List, ...)`, is a cycle Go rejects at build time:
  `initialization cycle for List`. Go counts any mention of a package-level
  variable inside an initializer, function literals included, so splitting the
  `Link` and the `Action` into their own `var`s only lengthens the chain. Today
  `Open` reads only `page.Path`, so one way out is a path `const` shared by the
  page and a bare `webui.Page[A]{Path: listPath}` literal in the action. A
  library answer would be a lightweight page reference by path, typed by `A`,
  that `Open` and `Link` accept. Until one is chosen, the demo's save stays on
  the page instead of returning to the list.
- `ArgsOf` keys `ctx` on an unexported type. A string key would collide with
  any other package using the same string, and `go vet` does not catch it.
- `Open` can fail at render time (a zero path argument), so `Target` carries an
  `Err`. Such a link renders disabled and is logged rather than panicking.
- `Open` is `webui.Open(ctx, page, args)`, not a method, so `Page` carries no
  behaviour at all. Type inference still gives a compile error on the wrong
  argument struct: `type OtherArgs does not match inferred type DetailsArgs`.
  Resolving through the runtime means `Target.URL` includes the mount prefix,
  so it is a real href and the renderer no longer prepends anything; and an
  unmounted page is caught at render rather than producing a dead link.
  `Link` declares its destination page and an `Args` function instead of
  building the `Target` itself, which moves target checking from render time to
  `Compile`. `Open` remains for programmatic navigation, such as
  `Effect.Redirect`.
- `Compile(prefix)` replaces `Validate()` and `HttpHandler()`. It is the one
  place the mount prefix is stated. `Open` resolves the prefix
  through the runtime, so `Target.URL` is absolute. The compiled
  form is a separate value, so mutating `App` afterwards has no effect — which
  is the intent.
- `Compile` returns a handler even on failure, serving the error at every path
  under the prefix. The guarantee is therefore not "no handler without a
  passing check" but "no *silently* broken handler". `MustCompile` panics, for
  callers who want the process to refuse to start.
- The compile error page shows type and field names. Fine behind the auth
  middleware an admin panel normally sits behind; worth a conscious decision
  before anyone mounts the prefix publicly.
- `CompileError` carries `Page`, `Args`, `Detail` and `Fix`. Go cannot recover
  the file and line of a struct literal at run time, so a page is identified by
  its `Path` and its argument type name — both must therefore always appear in
  the message, since they are the only coordinates available.
- `Compile` collects every problem instead of stopping at the first. The errors
  are structured rather than parsed from a compiler, so there is no reason to
  report one at a time.
- Argument names come from the lower-cased field name, overridable with a
  `webui:"..."` tag. `Validate` rejects duplicates, unsupported field types, and
  path placeholders with no matching field.
