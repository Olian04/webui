# webui

[![Go Reference](https://pkg.go.dev/badge/github.com/Olian04/webui/pkg/webui.svg)](https://pkg.go.dev/github.com/Olian04/webui/pkg/webui)

Build admin panels in pure Go. Declare your pages and webui serves a fast,
server-rendered UI, with **no HTML, CSS or JavaScript to write**.

- **Zero frontend.** Pages, tables, forms and actions are plain Go values.
- **Type-safe.** The compiler ties each link to its page and each argument to its
  path, and `Compile` catches the rest at startup, with a fix for each.
- **Batteries included.** Sorting, filtering, pagination, search, validation, toasts
  and dark mode, with no setup.
- **Works without JavaScript.** Every control is a real link or form, and a tiny
  script only makes it snappier.
- **Secure by default.** CSRF protection and a strict content security policy out of
  the box, plus guards per page and per action. A visitor never sees nav entries or
  search results they can't open.

# Using webui

## Install

```sh
go get github.com/Olian04/webui@latest
```

webui needs Go 1.27 or later. Import the one package it exports:

```go
import "github.com/Olian04/webui/pkg/webui"
```

## A first page

A page with a table of devices, served at `/admin/device`:

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Olian04/webui/pkg/webui"
)

type Device struct{ ID, IP string }

// An accessor is written once, and is a column in a table and an input in a form.
var (
	ID = webui.String[Device]{Label: "ID", Load: func(d Device) string { return d.ID }}
	IP = webui.String[Device]{Label: "IP", Load: func(d Device) string { return d.IP }}
)

var Devices = webui.Page[webui.NoArgs]{
	Path: "/device",
	Nav:  webui.Nav{Label: "Devices", Icon: "display"},
	Body: webui.Table[Device]{
		Title: "Devices",
		// Rows is every row. The library sorts, filters and pages them.
		Rows: func(context.Context) ([]Device, error) {
			return []Device{{"dev_1", "10.0.0.1"}, {"dev_2", "10.0.0.2"}}, nil
		},
		Columns: []webui.Accessor[Device]{ID, IP},
	},
}

func main() {
	app := webui.App{Brand: webui.Brand{Name: "Acme"}, Pages: webui.Pages{Devices}}

	// Compile checks the declaration and returns an http.Handler.
	http.Handle("/admin/", app.MustCompile("/admin"))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

Run it and open <http://localhost:8080/admin/device>. You get a sidebar, a
breadcrumb, and a table whose columns all sort and filter, with the state kept in the
address so a copied link reproduces the view. Add a `Form` for an editable page, a
`Link` to open a row, and a `Guard` to say who may see it.

webui does not authenticate. Mount the handler behind your own login.

## Try the full demo

The repository has a larger demo, an imaginary telemetry collector, that shows the
library end to end:

```sh
git clone https://github.com/Olian04/webui && cd webui
go run ./cmd/demo                    # http://localhost:8080/admin/
go run ./cmd/demo -viewer            # guarded controls and pages are refused, with the reason
go run ./cmd/demo -accent '#2f9e8f'  # override the theme's accent colour
go run ./cmd/demo -broken            # the failed-to-compile page
```

## Documentation

The full documentation is on pkg.go.dev, with runnable examples:

**<https://pkg.go.dev/github.com/Olian04/webui/pkg/webui>**

It covers pages and their arguments, layouts, tables (paging, sorting, filters and
search), forms and validation, outcomes (success, warning, failure, rejection and
where to go next), links between pages, theming, errors, and security.

---

# Developing webui

This half is for people changing the library.

## Layout

| Path | Role |
| --- | --- |
| `pkg/webui` | The public API: the declaration types, and `Compile`, which validates them and lowers them to the runtime. Its package documentation and examples are what pkg.go.dev shows. |
| `internal/ir` | The handoff between the declaration and the runtime. Plain data and closures with the types erased: no transport, no generics. |
| `internal/args` | Encoding and decoding a page's path and query arguments, from the struct's fields and `webui` tags. |
| `internal/rules` | RE2 compilation and the server-side check of a field's constraints. |
| `internal/tablequery` | Filtering, sorting, paging and searching rows held in memory, for `Table.Rows`. |
| `internal/runtime` | The compiled app: routes, the request pipeline, actions, flash, search, and `Open`. |
| `internal/render` | The IR and loaded data as HTML, and the assets. `templates/components` is the component library. |
| `internal/favicon` | Scales `Brand.Logo` into the favicon. |
| `test` | Integration tests, which import `pkg/webui` only. |
| `cmd/demo` | The runnable demo. It is not part of the API. |

Dependencies point one way: `pkg/webui` depends on `internal/*`, never the reverse, and
`test` never imports `internal`. [`docs/AGENTS.md`](docs/AGENTS.md) is the full set of
conventions, and is the source of truth when advice conflicts.

A declaration becomes a page in four steps. `pkg/webui` validates it, collecting every
problem, and lowers it, which is the one place type parameters are erased, into
`internal/ir`. `internal/runtime` turns that into routes and a request pipeline
(decode the arguments, run the page's Guard, load, render; for a POST: parse, rules,
bind, Guard, Run). `internal/render` renders the result with
[templ](https://templ.guide) components and serves the stylesheet, scripts and icon
font.

## Working on it

```sh
make help       # the targets
make generate   # regenerate the *_templ.go files from the .templ sources
make test       # generate, then go test -race -shuffle=on
make lint       # generate, vet, verify modules, govulncheck, golangci-lint
make run        # build and run the demo
```

The generated `*_templ.go` files are committed, so the module builds for anyone who
imports it. Run `make generate` after editing a `.templ` file; CI fails if they are
out of date. The icon font and its class list are generated from a Font Awesome Free
download by `internal/render/assets/fontawesome_gen.go`; see
[`docs/AGENTS.md`](docs/AGENTS.md).

## Tests

Tests for the public API are integration tests in `test`: they compile an app and
make requests to it, and import `pkg/webui` only. Unit tests for an internal package
sit beside the code in the same package. The runnable examples in `pkg/webui` are
tests too, so the documentation on pkg.go.dev cannot drift from the code.

Code that only a test uses does not belong in the library. If nothing reachable from
the public API needs it, it is deleted, or moved into the tests.

## Documentation

| Where | What |
| --- | --- |
| `pkg/webui/doc.go`, and every exported identifier | The documentation for users, which pkg.go.dev renders. Write it as complete sentences that start with the name, and put runnable examples in `example_*_test.go`. |
| [`docs/design-decisions.md`](docs/design-decisions.md) | Why the library is shaped as it is, and what each decision costs. Read it before changing the API. |
| [`docs/design.md`](docs/design.md) | The visual language: tokens, metrics, components and states. |
| [`docs/design-mapping.md`](docs/design-mapping.md) | Which declared type produces which design element, and where the two disagree. |
| [`docs/AGENTS.md`](docs/AGENTS.md) | Layout, dependency direction and Go conventions, for people and tools. |

## The demo

The demo is a static configuration that shows the whole library; each file has its
own corner, and `cmd/demo/demo_test.go` is the list of what it shows.

| File                    | Shows                                                                                                                                                                                                                                                                                                      |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `devices.go`            | a table with paging, sorting, column filters (multi-select, range, text), search and row links; a form in tabs beside a table; a path and query argument (`?minutes=`); a device form that returns to whichever page opened it, with no code; `Placeholder`, `Group`, a rejection with `Reject`, a `Failure`, a `Warning` and `Then`     |
| `sites.go`              | a nested path with a parent breadcrumb and a borrowed nav entry, two stateful tables on one page with their own `ID`s, a second page contributing search results                                                                                                                                           |
| `alerts.go`             | row and bulk actions, `RolePrimary` and `RoleDestructive`, a row `Link` to an alert page (a path argument) whose device table links on with a query argument, a form of read-only fields with one action, gating under `-viewer`                                                                           |
| `settings.go`           | rules (`Required`, length, pattern, bounds), `Float`, a writable `Slider`                                                                                                                                                                                                                                  |
| `system.go`             | the landing page (`Path: "/"`, reached from the brand and the first breadcrumb), `Nav.Icon`, an entry with no icon (its initial in the collapsed sidebar); a read-only form (no `Submit`), a `Badge` and a read-only `Slider` as a bar, a page `Guard` that refuses viewers, a table with an unknown total |
| `main.go` and `logo.go` | `Brand.Logo`, `Theme.Accent`, `Compile` and `MustCompile`, the compile-error page                                                                                                                                                                                                                          |

## Conventions

- **Commits** are [Conventional Commits](https://www.conventionalcommits.org) with a
  capital after the colon (`feat(webui): Add ...`), and `!` for a breaking change.
  Work happens on a branch, not on `main`.
- **Changing the public API** is a decision, not a side effect. Prefer behaviour the
  library infers or owns over a hook the user implements, prefer what the compiler
  can check over what `Compile` has to, and keep the surface small.
- **Everything works without JavaScript.** The script adds speed and conveniences
  and nothing else; a feature that needs it is not finished.

## Releasing

Releases are cut from the **Release** workflow (Actions, *Run workflow*), which asks
for a version (`X.Y.Z`, or `X.Y.Z-N` for the Nth prerelease before it), tags the
commit, and publishes the release with GoReleaser. Nothing is built: webui is a
library, and consumers fetch the tag with `go get`.
