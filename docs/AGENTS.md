# Agent context: webui

## Layout

| Path               | Role                                                                              |
| ------------------ | --------------------------------------------------------------------------------- |
| `pkg/webui`        | Public API. Declaration AST, split by file. `Compile` lowers to internal runtime. |
| `internal/ir`      | Handoff between declaration and runtime. No transport, no generics.               |
| `internal/args`    | Path/query encode-decode, `webui` tags, optionality.                              |
| `internal/rules`   | RE2 compile, constraint check, HTML attrs from `Rules`.                           |
| `internal/favicon` | Scale `Brand.Logo` to the favicon PNGs (stdlib only).                             |
| `internal/tablequery` | Filter, sort and page rows held in memory, from the columns' accessors (`Table.Rows`). |
| `internal/runtime` | Compiled app in `ctx`, prefix, routes, request pipeline, POST actions, flash, `Open` resolve. |
| `internal/render`  | IR + loaded data → HTML; asset routes. `templates/components` is the design language. |
| `test/`            | Integration tests against `pkg/webui` only. External test package (`webui_test`). |
| `test/util/assert` | Test assertions (`got`, `want`). No import of `pkg/` or `internal/`.              |
| `skills/webui`     | Skill for agents building *with* the library. Method and judgement, no API list; points at `go doc` for the pinned version. `test/skill_test.go` guards it. |
| `cmd/demo`         | Runnable demo app. Not part of the public API.                                    |

`pkg/webui` files: `app`, `page`, `table`, `form`, `layout`, `accessor`, `action`, `outcome`, `link`, `open`, `compile`, `compile_error`, `validate`, `lower`. One package, not one package per type. A type's `validate` and `lower` methods live beside it; `validate.go` and `lower.go` hold the entry points and shared state.

The runnable `Example` functions in `pkg/webui` are the documentation's tests, and are what pkg.go.dev shows. Unit tests are `*_test.go` next to the source they cover, same package. Internal packages export concrete funcs so those tests can call them. Prefer integration tests for the public API.

## Design

| Path                     | Role                                                                          |
| ------------------------ | ----------------------------------------------------------------------------- |
| `docs/design.md`         | The visual language: tokens, metrics, components, states.                     |
| `docs/design-mapping.md` | Which declared type produces which design element, and where the two disagree. |
| `docs/design-decisions.md` | Why the API is shaped as it is, and what each decision costs. |

`internal/render` implements `design.md`. Every colour, radius and elevation is a
design token (the app's `webui.Theme` colours are the only ones it may override); no component reads a literal value. When the two documents and
the code disagree, `design-mapping.md` names the gap on purpose — read its last
two sections before adding a field to close one.

Icons are Font Awesome Free (solid), drawn with a vendored font. The font is
`internal/render/assets/fonts/` (SIL OFL, license beside it); `css/fontawesome.css`
and `icons.txt` are generated from a Font Awesome Free download by
`go run fontawesome_gen.go <download-dir>` in `internal/render/assets`. `icons.txt`
is what `Compile` checks `Nav.Icon` against. Do not edit the generated files.

## Dependency direction

`pkg/webui` → `internal/<phase>`. Never reverse: `internal/` packages do not import `pkg/`.

`internal/` packages may import each other. `pkg/` owns public types and `Compile` glue.

Consumers outside this module cannot import `internal/`. No side effects on import.

Keep algorithms in `internal/`. `pkg/` owns the declaration AST.

## Tests

`test/` must import `pkg/webui` only, not `internal/`. `test/util/assert` is the exception: assertions only, imported by tests.

---

## Go proverbs ([source](https://go-proverbs.github.io/), caveman compress)

Concurrency channels coordinate mutex serializes · not parallelism · small interface sharp · zero value useful · `any` untyped tame it · gofmt settles bikeshed · tiny copy beats dep hairball syscall/cgo build tags isolate · cgo not Go · `unsafe` no contract · clarity beats wit · reflection stay cold path · errors values inspect wrap once · architecture name docs users · panic stays in `main` / hard startup.

## Uber style distill ([guide](https://github.com/uber-go/guide/blob/master/style.md), caveman compress)

Rare `*Iface` · `var _ I = (*T)(nil)` at export boundary · defer unlock pairs · chan buffer zero or one usually · slice/map copy exported API boundaries · typed errors `%w` chain handle once · assert comma-ok · goroutine bounded ctx/waitgroup · no zombie `init()` · globals inject not mutate · exits from `main` only · strconv hot paths · structs field-named literals · table tests sub `t.Run`.

---

Canon links above beat bullet memory when tradeoff unclear.

## The skill

`skills/webui/SKILL.md` teaches agents to build *with* the library. It is method and judgement, not an API list, so that it does not go stale: it sends the reader to `go doc` for the version in `go.mod` and to pkg.go.dev. Keep it that way. Do not add field names, signatures or code to it. If a design principle it states stops being true (how authentication works, how pages are mounted, what the library refuses to do), change the skill in the same commit. `test/skill_test.go` fails if the skill names a `webui.` identifier or a `go doc` symbol that no longer exists.
