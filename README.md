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

var DeviceId = webui.Arg[string]{Id: "id"}
var DebugFlag = webui.Arg[bool]{Id: "debug"}

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
    Store: func(d *Device, val string) error {
      d.Ip = val
      return nil
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

var Devices = webui.Page{
  Path: "/device",
  Nav:  DevicesNav,
  Body: webui.Table[Device]{
    Load: func(ctx context.Context) ([]Device, error) {
      return service.LoadAll(ctx)
    },
    // A Link renders a real <a href>, so middle-click and open-in-new-tab work.
    // An Action here would render a POST instead — for row clicks that mutate.
    RowClick: webui.Link[Device]{
      To: func(d Device) webui.Target { return Details.Open(DeviceId.Set(d.Id)) },
    },
    // No Actions or BulkActions means no action bar.
    // No BulkActions means no row select checkboxes.
    Columns: []webui.Accessor[Device]{ID, IP, Occurrences, Rate},
  },
}

var Details = webui.Page{
  // Arguments named in Path are path segments; the rest are query parameters.
  Args: webui.Args{
    DeviceId,  // -> /device/{id}
    DebugFlag, // -> ?debug=true
  },
  Path: "/device/{id}",
  Nav: webui.Nav{
    // Doesn't show up in the Navbar, but shows as being on the "Devices"
    // entry when the page is loaded. Identity is the pointer.
    Shadow: &DevicesNav,
  },
  // Guard runs before anything is loaded — so an unauthorised device is never
  // read. It gates the UI on page load and authorises the request on every
  // section fetch.
  Guard: func(ctx context.Context) error {
    id, err := DeviceId.Get(ctx)
    if err != nil {
      return err
    }
    return auth.AssertDeviceAccess(ctx, id)
  },
  Body: webui.Form[Device]{
    // The loader names the argument it needs. Arg[T].Get is typed,
    // so id is a string and the page needs no type parameter at all.
    Load: func(ctx context.Context) (Device, error) {
      id, err := DeviceId.Get(ctx)
      if err != nil {
        return Device{}, err
      }
      return service.Load(ctx, id)
    },
    Submit: SaveDevice,
    Fields: []webui.Accessor[Device]{
      webui.Group[Device]{ID, IP},
      Rate,
    },
  },
}

var SaveDevice = webui.Action[Device]{
  // Guard takes the action's subject. Same function on both sites: it enables
  // the UI element at render time and guards the request before Run.
  Guard: func(ctx context.Context, d Device) error {
    return auth.AssertCanEdit(ctx, d.Id)
  },
  Run: func(ctx context.Context, d Device) (webui.Effect, error) {
    if err := service.Put(ctx, d.Id, d); err != nil {
      // Submit failed: restore form fields, show the error.
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

  if err := app.Validate(); err != nil {
    log.Fatal(err)
  }

  mux := http.NewServeMux()
  // HttpHandler receives the full request path and strips the prefix itself.
  mux.Handle("/admin/", authMiddleware(app.HttpHandler("/admin")))

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

`Arg[T]` carries the type, so `Page` needs no type parameter. Values are still
checked by the compiler at both ends:

```
cannot use 42 (untyped int constant) as string value in argument to DeviceId.Set
```

`Validate()` covers the wiring between `Args` and `Path`:

```
page "/device/{id}": path declares {id} but no such Arg is listed
page "/device/{id}": duplicate argument "id"
```

`Open` takes bindings and sorts them into the path and the query string:

```
Details.Open(DeviceId.Set("abc"))                     -> /device/abc
Details.Open(DeviceId.Set("abc"), DebugFlag.Set(true)) -> /device/abc?debug=true
Details.Open(DebugFlag.Set(true))                      -> Err: missing path argument "id"
```

A loader reaching for an argument its page does not list fails on the request,
not at startup — that is the cost of `Load` being an opaque closure:

```
webui: argument "org" is not available on this page
```

A page's `Body` is a single `PageBody`. Leaves (`Table`, `Form`) load and
refresh; layouts only arrange, and they nest:

```go
Body: webui.Stack{ // vertical
  webui.Split{ // side by side, across models
    webui.Form[Device]{ ... },
    webui.Table[Event]{ ... },
  },
  webui.Tabs{
    {Label: "Raw", Body: webui.Table[Event]{ ... }},
  },
},
```

`Stack`, `Split` and `Tabs` are the layouts. All three hold `PageBody`, so none
of them constrains what their children are about. `Group[M]` is a different
thing one level down: accessor composition inside a form, typed to the model.

Context-specific presentation stays off the accessor. Options are decorators,
which are themselves accessors, so the common case stays a bare list:

```go
Columns: []webui.Accessor[Device]{ID, webui.Sortable[Device]{IP, "ip_addr"}, Occurrences},
Fields:  []webui.Accessor[Device]{webui.Placeholder[Device]{IP, "10.0.0.1"}},
```

## Open

- Accessor identity for cross-field and submit-time validation errors. Per-field
  format validation already works through `Store`'s error return.
- `Store` on an accessor listed as a column: ignored for now. Making it mean
  inline-editable cells needs a per-row save interaction that does not exist
  yet — the merge makes that reachable, it does not make it free.
- `Group` listed as a column is meaningless. Flatten it, or reject it in
  `Validate`.
- A nested `Table` inside a form's `Fields` is not expressible, since `Table`
  is a `PageBody` and not an `Accessor[M]`. A real pattern; currently out.
- Section refresh endpoints: query-param namespacing, and re-running the page
  `Guard` on every section fetch. A leaf's identity is now its path through the
  `Body` tree rather than an index into a flat list.
- Pagination, sorting, filtering — v1. Transport is solved: they are query
  `Arg`s like any other, namespaced by hand (`devices_offset`, `events_offset`)
  when a page has two tables. Two pieces remain: `Load` needs to return a total
  alongside the rows for "of N" and last-page detection, and `Table` needs to
  know *which* `Arg`s drive it if the framework is to render the pager itself.
- Bulk-action gating: `Action[[]Device].Guard` receives the selection, so
  gating happens at execution rather than per-row at render.
- Non-string path arguments: built-in decoding for integers and
  `encoding.TextUnmarshaler`, or an escape hatch for the rest.
- Optionality: path arguments are required, query arguments are not. `Arg.Default`
  covers the absent case, but `Get` still returns an error for a *malformed*
  value — absent and malformed need to stay distinguishable.
- Missing primitive: "the current URL with these arguments overridden".
  `Open` builds from scratch, but every pager link, sort header and filter
  toggle needs to preserve the other arguments in play. Something like
  `webui.Here(ctx, Offset.Set(n))`.
- `Arg.Get` must key `ctx` on an unexported type, not the `Id` string directly.
  A string key collides with any other package that uses the same string, and
  `go vet` does not catch it.
- `Open` can fail at render time (a missing path argument), so `Target` carries
  an `Err`. Decide whether such a link renders disabled and logs, or panics.
- Argument identity is the `Id` string, not the variable, which is what allows
  `Args{DeviceId, DebugFlag}` to hold values rather than pointers. Two `Arg`s
  sharing an `Id` are interchangeable; `Validate` rejects duplicates on one page.
