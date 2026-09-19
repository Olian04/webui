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
  Id     string // -> /device/{id}
  Debug  bool   // -> ?debug=true
  Offset int    // -> ?offset=50
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
    Load: func(ctx context.Context) ([]Device, error) {
      return service.LoadAll(ctx)
    },
    // A Link renders a real <a href>, so middle-click and open-in-new-tab work.
    // An Action here would render a POST instead — for row clicks that mutate.
    RowClick: webui.Link[Device]{
      To: func(d Device) webui.Target { return Details.Open(DetailsArgs{Id: d.Id}) },
    },
    // No Actions or BulkActions means no action bar.
    // No BulkActions means no row select checkboxes.
    Columns: []webui.Accessor[Device]{ID, IP, Occurrences, Rate},
  },
}

var Details = webui.Page[DetailsArgs]{
  Path: "/device/{id}",
  Nav: webui.Nav{
    // Doesn't show up in the Navbar, but shows as being on the "Devices"
    // entry when the page is loaded. Identity is the pointer.
    Shadow: &DevicesNav,
  },
  // Guard runs before anything is loaded — so an unauthorised device is never
  // read. It gates the UI on page load and authorises the request on every
  // section fetch.
  Guard: func(ctx context.Context, a DetailsArgs) error {
    return auth.AssertDeviceAccess(ctx, a.Id)
  },
  Body: webui.Form[Device]{
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

A page's arguments are one struct the user owns, so `Open` is checked by the
compiler rather than at render, and `Guard` gets a typed parameter:

```
/device/abc
/device/abc?debug=true&offset=50
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
next.Offset = 50
return Details.Open(next)   // /device/abc?debug=true&offset=50
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
- Pipeline: parse -> rules -> `Store` -> `Guard` -> `Run`. Parse failures
  short-circuit, so a form with both a malformed number and a duplicate value
  shows the parse error first and the duplicate only on the next submit.
- The re-render after a rejection must echo the raw submitted input, not the
  model value, or the user's bad input disappears and the form looks like it
  reset.
- `Pattern` is not portable: Go's RE2 rejects the lookahead and backreference
  syntax people copy from JavaScript examples. `Validate()` must compile every
  `Pattern` at startup, and the docs must say RE2, not "regex".
- Any struct a user fills with an unkeyed literal from another package trips
  `go vet`'s composites check, so nested literals in the API must expect keyed
  fields (`{Field: IP, Message: "..."}`).
- `Group` listed as a column is rejected by `Validate` rather than flattened —
  flattening would mean defining what a nested group is in a cell.
- A nested `Table` inside a form's `Fields` stays out of scope. If it turns out
  to be wanted, it is solved then.
- Refresh: decided. The POC reloads the whole page on any argument change;
  leaf-initiated refresh is a later enhancement over the *same* links, so it
  needs no user-facing API change. A control owned by a leaf refreshes that
  leaf; a control owned by the page navigates the page. Four things must hold
  from the start: pager/sort/filter controls are real `<a href>` carrying
  complete URLs, the page `Guard` runs on every request, leaf addresses are
  derived from the `Body` tree, and framework parameters never touch the URL.
  Open within this: `Vary` on the leaf header, and cross-leaf refresh after an
  action (whole-page reload, or model-typed staleness such as `Stale[Device]`).
- Pagination, sorting, filtering — v1. Transport is solved: they are ordinary
  query `Arg`s. Two independent offsets on one page means two `Arg`s, which the
  user names; the framework namespaces nothing. Two pieces remain: `Load` needs
  to return a total alongside the rows for "of N" and last-page detection, and
  `Table` needs to know *which* `Arg`s drive it if the framework is to render
  the pager itself.
- Bulk-action gating: `Action[[]Device].Guard` receives the selection, so
  gating happens at execution rather than per-row at render.
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
- Framework parameters travel as headers, never in the URL. The URL carries
  user state, headers carry read-side transport, form bodies carry write-side
  routing — so nothing framework-owned appears in a URL anyone might copy, and
  the query string needs no reserved prefix. Consequence: an iframe cannot set
  headers, so embedding a leaf later needs its own path and a standalone
  document, rather than reusing the refresh mechanism.
- `ArgsOf` keys `ctx` on an unexported type. A string key would collide with
  any other package using the same string, and `go vet` does not catch it.
- `Open` can fail at render time (a zero path argument), so `Target` carries an
  `Err`. Such a link renders disabled and is logged rather than panicking.
- `Compile(prefix)` replaces `Validate()` and `HttpHandler()`. It is the one
  place the mount prefix is stated. `Target.URL` stays app-relative; the
  renderer prepends the prefix, so `Open` never needs to know it. The compiled
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
